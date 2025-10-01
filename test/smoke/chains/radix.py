import logging
import os
import random
import time

from radix_engine_toolkit import *
from tenacity import retry, stop_after_delay, wait_fixed

from chains.aliases import aliases_xrd, aliases_maya, get_aliases, get_alias_address
from chains.chain import GenericChain
from utils.common import Coin, HttpClient, get_cacao_asset, Asset

CACAO = get_cacao_asset()

logging.basicConfig(
    format="%(levelname).1s[%(asctime)s] %(message)s",
    level=os.environ.get("LOGLEVEL", "INFO"),
)

def decimal_string_to_maya_subunits(dec):
  parts = dec.split(".")
  integer_part = parts[0]
  fractional_part = parts[1] if len(parts) > 1 else ''
  radix_subunits = int(integer_part + fractional_part.ljust(18, '0'))
  return int(radix_subunits / pow(10, 10))

def maya_subunits_to_decimal_string(subunits):
    # this is super hacky but works okay for the amounts we're using
    subunits_str = str(subunits) + "0" * 10
    amount_dec = Decimal(subunits_str[:-18] + '.' + subunits_str[-18:])
    return amount_dec.as_str()

class MockRadix(HttpClient):
    """
    A client implementation for a (real, not mocked) radix validator
    """

    network_id = 0xF0

    estimated_txn_fee = 500000000

    private_keys = {}  # a map of private keys to radix network addresses

    private_keys_hexes = [
        "ee9cb7d5076ad1578de8e5d160dee794b61dcf03ef31233cc7cecaf7454eeadb", # MASTER (account_loc16xdw5jm4l70r37p8fzk29777e52e99dh5dk443ygwpg2ql3h7um770)
        "099fbf134e23ac176ac3e34e0c88578e0354dd403aa937e88f059fa0560d0425", # CONTRIB (account_loc168u46dsk3ae4v8s3uvccwyxsu90zc6ewmhp7rd0k7hs32xavsjkf49)
        "22e05c2479608958020c9da290f24daf9ad72c4927fa88f1da5c19d64a50efc8", # USER-1 (account_loc1693rhqss8thtsv5jlsta5mkxl2l27qqnrc6dp699a84vcd2cjlc7c5)
        "76736c78f64e84131218944e6f7ea06d4ec84a2caf363b215a6c951a98bcf41c", # PROVIDER-1 (account_loc169h7jctav80kpm4h9sw7n9egh80admp44ctqfm0d0u9sj3hvu9xqvu)
        "39e78bb8c6274e1f07a03668af0fc14160bb7829be034cd6033f61bb34fe6fce", # PROVIDER-2 (account_loc168wpt8m8s9h200d4cs7gtge5akr6exyzwylxng9v0jwaj8hc0cux2g)
    ]

    def __init__(self, base_url, router_address):
        super().__init__(base_url)

        self.next_nonce = 1

        self.wait_for_node()
        address_book = known_addresses(self.network_id)
        self.native_token_address = address_book.resource_addresses.xrd
        self.faucet_address = address_book.component_addresses.faucet

        self.maya_router_address = Address(router_address)

        self.init_accounts()
        self.seed_master_account()
        self.create_test_resource()

    def set_vault_address_by_pubkey(self, pubkey):
        """
        Set vault address by pubkey
        """
        aliases_xrd["VAULT"] = self.get_address_from_pubkey(pubkey).address_string()

    def get_balance(self, address, symbol="XRD"):
        """
        Get RDX balance for an address
        """
        if address == "VAULT" or address == aliases_xrd["VAULT"]:
            return self.get_vault_balance(symbol)
        response = self.post("/state/account", {"network": "localnet", "account_address": address})
        if not response['vaults'] or len(response['vaults']) == 0:
          return 0
        vaults = response['vaults']

        if symbol == "XRD":
            vault = next((v for v in vaults if v['resource_amount']['resource_address'] == self.native_token_address.address_string()), None)
        else:
            vault = next((v for v in vaults if v['resource_amount']['resource_address'] == self.test_token_address), None)
        return decimal_string_to_maya_subunits(vault['resource_amount']['amount'])

    def get_vault_balance(self, symbol):
        vault_addr = Address(aliases_xrd["VAULT"])
        vault_addr_bytes = vault_addr.bytes().hex()
        if symbol == "XRD":
          resource_addr = self.native_token_address
        else:
          resource_addr = Address(self.test_token_address)
        resource_addr_bytes = resource_addr.bytes().hex()
        response = self.post("/transaction/call-preview", {
          "network": "localnet",
          "target": {
            "type" : "Method",
            "component_address" : self.maya_router_address.address_string(),
            "method_name" : "get_vault_balance"
          },
          "arguments" : [
            "4d8000" + vault_addr_bytes, # vault address
            "4d8000" + resource_addr_bytes # resource address
          ]
        })
        dec_value = response["output"]["programmatic_json"]["value"]
        return decimal_string_to_maya_subunits(dec_value)

    @classmethod
    def get_address_from_pubkey(cls, pubkey):
        """
        Get radix testnet address for a public key

        :param string pubkey: public key
        :returns: string bech32 encoded address
        """
        public_key: PublicKey = PublicKey.SECP256K1(pubkey)
        return derive_virtual_account_address_from_public_key(public_key, cls.network_id)

    @retry(stop=stop_after_delay(30), wait=wait_fixed(1))
    def wait_for_node(self):
        """
        Check that the node is ready to receive requests
        """
        self.post("/status/network-status", {"network": "localnet"})

    def init_accounts(self):
        for key in self.private_keys_hexes:
            private_key_bytes = bytes.fromhex(key)
            private_key = PrivateKey.new_secp256k1(private_key_bytes)
            address = derive_virtual_account_address_from_public_key(
                private_key.public_key(), self.network_id
            )
            self.private_keys[address.address_string()] = private_key

    def seed_master_account(self):
        acc_addr = aliases_xrd["MASTER"]
        self.__free_xrd(acc_addr, self.private_keys[acc_addr])

    def create_test_resource(self):
        mint_to = aliases_xrd["MASTER"]
        manifest = (
          ManifestBuilder()
            .faucet_lock_fee()
            .create_fungible_resource_manager(OwnerRole.NONE(), False, 10, Decimal("100000"), FungibleResourceRoles(None, None, None, None, None, None), MetadataModuleConfig({}, {}), None)
            .account_try_deposit_entire_worktop_or_abort(Address(mint_to), None)
            .build(self.network_id)
        )
        intent_hash = self.__submit_manifest(manifest, self.private_keys[mint_to])
        receipt = self.__get_transaction_receipt(intent_hash)
        new_resource_address = receipt["committed"]["receipt"]["state_updates"]["new_global_entities"][0]["entity_address"]
        self.test_token_address = new_resource_address

    def get_current_epoch(self):
        return self.post("/status/network-status", {"network": "localnet"})["current_epoch_round"]["epoch"]

    def submit_transaction(self, txn):
        return self.post("/transaction/submit", {"network": "localnet", "notarized_transaction_hex": txn})

    def transfer(self, txns):
        """
        Make a transaction/transfer on radix network
        """
        if not isinstance(txns, list):
            txns = [txns]

        for txn in txns:
            if not isinstance(txn.coins, list):
                txn.coins = [txn.coins]

            if txn.to_address in get_aliases():
                txn.to_address = get_alias_address(txn.chain, txn.to_address)

            if txn.from_address in get_aliases():
                txn.from_address = get_alias_address(txn.chain, txn.from_address)

            # update memo with actual address (over alias name)
            is_synth = txn.is_synth()
            for alias in get_aliases():
                chain = txn.chain
                asset = txn.get_asset_from_memo()
                if asset:
                    chain = asset.get_chain()
                if is_synth:
                    chain = CACAO.get_chain()
                if txn.memo.startswith("ADD"):
                    if asset and txn.chain == asset.get_chain():
                        chain = CACAO.get_chain()
                addr = get_alias_address(chain, alias)
                txn.memo = txn.memo.replace(alias, addr)

            if txn.memo == "SEED":
                self.__token_transfer(txn)
            elif txn.memo.startswith("ADD") or is_synth:
                self.__deposit_to_maya_router(txn)
            elif txn.memo.startswith("SWAP") or is_synth:
                self.__deposit_to_maya_router(txn)

    def __token_transfer(self, txn):
        if txn.coins[0].asset.get_symbol() == "XRD":
            resource_addr = self.native_token_address
        else:
            resource_addr = Address(self.test_token_address)
        amount = Decimal(maya_subunits_to_decimal_string(txn.coins[0].amount))
        manifest = (
            ManifestBuilder()
            .faucet_lock_fee()
            .account_withdraw(Address(txn.from_address), resource_addr, amount)
            .take_from_worktop(resource_addr, amount, ManifestBuilderBucket("bucket"))
            .account_try_deposit_or_abort(Address(txn.to_address), ManifestBuilderBucket("bucket"), None)
            .build(self.network_id)
        )
        txn.id = self.__submit_manifest(manifest, self.private_keys[txn.from_address])
        txn.gas = [ Coin("XRD.XRD", 0) ] # fees are paid from faucet; no gas is consumed from the "from" account

    def __get_address_from_private_key(self, private_key_hex):
        private_key_bytes = bytes.fromhex(private_key_hex)
        private_key = PrivateKey.new_secp256k1(private_key_bytes)
        address = derive_virtual_account_address_from_public_key(
            private_key.public_key(), self.network_id
        )
        return address

    def __deposit_to_maya_router(self, txn):
        if txn.coins[0].asset.get_symbol() == "XRD":
            memo = (
                "ADD:XRD.XRD:" + aliases_maya["PROVIDER-1"]) if txn.memo.startswith("ADD:") else \
                "SWAP:MAYA.CACAO:" + aliases_maya["USER-1"]
            resource_addr = self.native_token_address.address_string()
        else:
            memo = (
                "ADD:XRD.TEST-" + self.test_token_address + ":" + aliases_maya["PROVIDER-1"]) if txn.memo.startswith("ADD:") else \
                "SWAP:MAYA.CACAO:" + aliases_maya["USER-1"]
            resource_addr = self.test_token_address

        amount = "Decimal(\"{amount}\")".format(amount=maya_subunits_to_decimal_string(txn.coins[0].amount))

        instructions_string = """
    CALL_METHOD Address(\"{faucet}\") \"lock_fee\" Decimal(\"25\");
    CALL_METHOD Address(\"{source}\") \"withdraw\" Address(\"{resource_addr}\") {amount};
    TAKE_ALL_FROM_WORKTOP Address(\"{resource_addr}\") Bucket(\"bucket\");
    CALL_METHOD Address(\"{router}\") \"user_deposit\" Address(\"{source}\") Address(\"{vault_address}\") Bucket(\"bucket\") \"{memo}\";
    """.format(
            faucet=self.faucet_address.address_string(),
            source=txn.from_address,
            amount=amount,
            resource_addr=resource_addr,
            router=self.maya_router_address.address_string(),
            vault_address=aliases_xrd["VAULT"],
            memo=memo
        )

        instructions = Instructions.from_string(instructions_string, self.network_id)
        manifest = TransactionManifest(instructions, [])
        txn.id = self.__submit_manifest(manifest, self.private_keys[txn.from_address])
        txn.gas = [ Coin("XRD.XRD", 0) ] # fees are paid from faucet; no gas is consumed from the "from" account

    def __free_xrd(self, destination, private_key):
        manifest = (
            ManifestBuilder()
            .faucet_lock_fee()
            .faucet_free_xrd()
            .take_all_from_worktop(self.native_token_address, ManifestBuilderBucket("bucket"))
            .account_deposit_batch(Address(destination), [ManifestBuilderBucket("bucket")])
            .build(self.network_id)
        )
        self.__submit_manifest(manifest, private_key)

    def __submit_manifest(self, manifest, private_key_for_signing):
        current_epoch = self.get_current_epoch()
        header = TransactionHeader(
            network_id=self.network_id,
            start_epoch_inclusive=1,
            end_epoch_exclusive=5000,
            nonce=self.next_nonce,
            notary_public_key=private_key_for_signing.public_key(),
            notary_is_signatory=True,
            tip_percentage=0,
        )
        self.next_nonce += 1

        notarized_transaction = (
            TransactionBuilder()
            .header(header)
            .manifest(manifest)
            .sign_with_private_key(private_key_for_signing)
            .notarize_with_private_key(private_key_for_signing)
        )

        notarized_transaction_hex = "".join(hex(i)[2:].zfill(2) for i in notarized_transaction.compile())
        self.submit_transaction(notarized_transaction_hex)
        intent_hash_string = notarized_transaction.intent_hash().as_str()
        intent_hash_hex = notarized_transaction.intent_hash().bytes().hex()

        # wait for transaction to get committed
        max_time_to_wait = time.time() + 10
        while time.time() < max_time_to_wait:
            if self.__get_transaction_status(intent_hash_string) == 'CommittedSuccess':
              return intent_hash_hex
            time.sleep(1)
        raise Exception(
            "Transaction {intent_hash} did not get committed successfully".format(intent_hash=intent_hash_string))

    def __get_transaction_status(self, intent_hash_string):
        return self.post(
            "/transaction/status",
            {"network": "localnet",
             "intent_hash": intent_hash_string}
        )['known_payloads'][0]['status']

    def __get_transaction_receipt(self, intent_hash_string):
        return self.post(
            "/transaction/receipt",
            {"network": "localnet",
             "intent_hash": intent_hash_string}
        )

class Radix(GenericChain):
    """
    A local simple implementation of radix chain
    """

    name = "Radix"
    chain = "XRD"
    coin = Asset("XRD.XRD")

    @classmethod
    def _calculate_gas(cls, pool, txn):
        """
        Calculate gas according to CACAO mayachain fee
        """
        if txn.memo == "WITHDRAW:XRD.XRD:1000":
          return Coin(cls.coin, 62942468)
        elif txn.memo == "WITHDRAW:XRD.XRD":
          return Coin(cls.coin, 62942643)
        elif txn.memo.startswith("SWAP:XRD.XRD"):
            return Coin(cls.coin, 62942468)
        elif txn.memo.startswith("WITHDRAW:XRD.TEST-RESOURCE_LOC1TH5R2TN083J0WJ8PQS2Y44A39HVXN6GCYGFELH97YFF69SJA08U0RC:1000"):
            return Coin(cls.coin, 65223038)
        elif txn.memo.startswith("WITHDRAW:XRD.TEST-RESOURCE_LOC1TH5R2TN083J0WJ8PQS2Y44A39HVXN6GCYGFELH97YFF69SJA08U0RC"):
            return Coin(cls.coin, 65223268)
        elif txn.memo.startswith("SWAP:XRD.TEST-RESOURCE_LOC1TH5R2TN083J0WJ8PQS2Y44A39HVXN6GCYGFELH97YFF69SJA08U0RC"):
            return Coin(cls.coin, 65223038)
        else:
          return Coin(cls.coin, 0)
