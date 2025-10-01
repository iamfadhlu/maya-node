import logging
import json
import requests

from web3 import Web3, HTTPProvider
from web3.middleware import ExtraDataToPOAMiddleware, SignAndSendRawMiddlewareBuilder
from eth_keys import KeyAPI
from utils.common import Coin, get_cacao_asset, get_gas_from_recipient, get_gas_price_from_tx, Asset
from chains.aliases import aliases_arb, get_aliases, get_alias_address
from chains.chain import GenericChain

CACAO = get_cacao_asset()


def calculate_gas(msg):
    return MockArbitrum.default_gas + Arbitrum.gas_per_byte * len(msg)

def get_arbitrum_gas(maya_hash):
    """
    Get corresponding arb gas from
    maya out hash
    """
    arb_url = "http://localhost:8547"
    gasPrice = get_gas_price_from_tx(maya_hash, arb_url)
    gas = get_gas_from_recipient(maya_hash, arb_url)

    unrounded_gas_amt = gas * gasPrice
    rounded_gas_amt = int(unrounded_gas_amt/10000000000)
    if unrounded_gas_amt > int(rounded_gas_amt*10000000000):
        rounded_gas_amt = rounded_gas_amt + 1
    return int(rounded_gas_amt)

class MockArbitrum:
    """
    An client implementation for a localnet/rinkebye/ropston Arbitrum server
    """
    default_gas = int(5000000)
    gas_price = 10
    block_stats = {
        "tx_rate": 10,
        "tx_size": default_gas,
    }
    seed = "SEED"
    stake = "ADD"
    zero_address = "0x0000000000000000000000000000000000000000"
    default_account_address = "0x3f1eae7d46d88f08fc2f8ed27fcb2ab183eb2d0e"
    vault_contract_addr = "0xda52b25ddB0e3B9CC393b0690Ac62245Ac772527"

    priv_keys = {
        "0x3f1eae7d46d88f08fc2f8ed27fcb2ab183eb2d0e": "0xb6b15c8cb491557369f3c7d2c287b053eb229daa9c22138887752191c9520659",
        "0x1dD3B27eD3CAA2fba3345D105819f86D9D414778": "0x5f59133458bfa3cd887b1d03b41b568f0de5006ac531fafe27f4dcbbdd723178",
        "0x0d5ef8047b37C8D26D066ff1De85bdEB8D5D2916": "0xec55d4fe85f9a3031a051239149eb0cd3531f295a932c8660fa16e6f8c99e133",
        "0x140AD7786f9578c8ff6371c69bC3fFC776863933": "0x3d5320f3501c27cde5d09dc75cbe109f748c43473769fa8fd6ea62e4fad6eaae"
    }
    loaded_accounts = []
    def __init__(self, base_url):
        self.url = base_url
        self.web3 = Web3(HTTPProvider(base_url))
        self.web3.middleware_onion.inject(ExtraDataToPOAMiddleware, layer=0)
        
        self.imported_accounts = {
            account.address: account
            for account in [self.web3.eth.account.from_key(key) for key in self.priv_keys.values()]
        }
        # self.accounts is a list of addresses
        self.accounts = [account.address for account in self.imported_accounts.values()]
        
        self.web3.eth.default_account = self.accounts[0]
        
        self.web3.middleware_onion.inject(
            SignAndSendRawMiddlewareBuilder.build(self.imported_accounts.get(self.accounts[0])),
            layer=0
        )
        self.loaded_accounts.append(self.accounts[0])
        
        self.vault = self.get_vault()
        self.send_init_txs(100)

    @classmethod
    def get_address_from_pubkey(cls, pubkey):
        """
        Get Arbitrum address for a specific hrp (human readable part)
        bech32 encoded from a public key(secp256k1).

        :param string pubkey: public key
        :returns: string 0x encoded address
        """
        arb_pubkey = KeyAPI.PublicKey.from_compressed_bytes(pubkey)
        return arb_pubkey.to_address()

    def set_vault_address(self, addr):
        """
        Set the vault eth address
        """
        aliases_arb["VAULT"] = addr

    def get_block_height(self):
        """
        Get the current block height of Arbitrum localnet
        """
        block = self.web3.eth.getBlock("latest")
        return block["number"]

    def get_vault(self):
        abi = json.load(open("data/vault.json"))
        vault = self.web3.eth.contract(address=self.vault_contract_addr, abi=abi)
        return vault

    def get_block_hash(self, block_height):
        """
        Get the block hash for a height
        """
        block = self.web3.eth.getBlock(block_height)
        return block["hash"].hex()

    def get_block_stats(self, block_height=None):
        """
        Get the block hash for a height
        """
        return {
            "avg_tx_size": 1,
            "avg_fee_rate": 1,
        }

    def set_block(self, block_height):
        """
        Set head for reorg
        """
        payload = json.dumps({"method": "debug_setHead", "params": [block_height]})
        headers = {"content-type": "application/json", "cache-control": "no-cache"}
        try:
            requests.request("POST", self.url, data=payload, headers=headers)
        except requests.exceptions.RequestException as e:
            logging.error(f"{e}")

    def get_balance(self, address, symbol):
        """
        Get ARB balance for an address
        """
        return self.web3.eth.get_balance(Web3.to_checksum_address(address), "latest")
    
    def get_passphrase(self, address):
        blank_passphrase_addresses = [
            "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC",
            "0x970e8128ab834e8eac17ab8e3812f010678cf791",
            "0xf6da288748ec4c77642f6c5543717539b3ae001b",
            "0xfabb9cc6ec839b1214bb11c53377a56a6ed81762",
            "0x1f30a82340f08177aba70e6f48054917c74d7d38",
        ]
        if not address in blank_passphrase_addresses:
            return "passphrase"
        return ""
    
    def get_private_key(self, address):
        # Mock addresses and keys
        if address in self.priv_keys.keys():
            return self.priv_keys[address]

        return ""

    def wait_for_node(self):
        """
        Arbitrum pow localnet node is started with directly mining 4 blocks
        to be able to start handling transactions.
        It can take a while depending on the machine specs so we retry.
        """
        current_height = self.get_block_height()
        while current_height < 2:
            current_height = self.get_block_height()

    def transfer(self, txn):
        """
        Make a transaction/transfer on localnet Arbitrum
        """
        if not isinstance(txn.coins, list):
            txn.coins = [txn.coins]

        if txn.to_address in aliases_arb.keys():
            txn.to_address = get_alias_address(txn.chain, txn.to_address)

        if txn.from_address in aliases_arb.keys():
            txn.from_address = get_alias_address(txn.chain, txn.from_address)

        # update memo with actual address (over alias name)
        is_synth = txn.is_synth()
        for alias in get_aliases():
            chain = txn.chain
            asset = txn.get_asset_from_memo()
            if asset:
                chain = asset.get_chain()
            # we use CACAO BNB address to identify a cross chain liqudity provision
            if txn.memo.startswith(self.stake) or is_synth:
                chain = CACAO.get_chain()
            addr = get_alias_address(chain, alias)
            txn.memo = txn.memo.replace(alias, addr)

            for account in self.imported_accounts.keys():
                if txn.from_address.lower() == account.lower():
                    self.web3.eth.default_account = account
                    if account not in self.loaded_accounts:
                        self.web3.middleware_onion.inject(
                            SignAndSendRawMiddlewareBuilder.build(self.imported_accounts.get(account)), layer=0
                        )
                        self.loaded_accounts.append(account)

        spent_gas = 0
        from_address_checksum = Web3.to_checksum_address(txn.from_address)
        to_address_checksum = Web3.to_checksum_address(txn.to_address)
        tx = {
            'nonce': self.web3.eth.get_transaction_count(from_address_checksum),
            'to': to_address_checksum,
            'value': txn.coins[0].amount,
            'gas': 3500000,
            'chainId': 412346,
            'gasPrice': 100000000,
            'data': txn.memo.encode('utf-8').hex()
        }
        signed_tx = self.web3.eth.account.sign_transaction(tx, self.get_private_key(txn.from_address))
        tx_hash = self.web3.eth.send_raw_transaction(signed_tx.raw_transaction)
        receipt = self.web3.eth.wait_for_transaction_receipt(tx_hash)
        txn.id = receipt.get('transactionHash').hex().upper()
        txn.gas = [
            Coin(
                "ARB.ETH",
                (receipt.get('gasUsed') * receipt.get('effectiveGasPrice') + spent_gas) * 1,
            )
        ]

    def send_init_txs(self, number_of_txs):
        from_address = "0x3f1eae7d46d88f08fc2f8ed27fcb2ab183eb2d0e"
        to_address = "0xA4548B48096f9f4F0aFbcBB0E6E17c927772922d"
        # Set up the transaction
        from_address_with_checksum = Web3.to_checksum_address(
            from_address
        )
        to_address_with_checksum = Web3.to_checksum_address(
            to_address
        )

        txs = []
        # Send a total of number_of_txs
        for i in range(number_of_txs):
            # gasPrice = 0.1 + 0.05 + (i % 12) / 100
            tx = {
                "nonce": self.web3.eth.get_transaction_count(from_address_with_checksum),
                "to": to_address_with_checksum,
                "value": self.web3.to_wei(0.1, "ether"),
                "gas": 2000000,
                'gasPrice': 100000000,
                "data": "0x0f4d14e9000000000000000000000000000000000000000000000000000082f79cd90000",
                "chainId": 412346,
            }

            # Sign and send the tx
            signed_tx = self.web3.eth.account.sign_transaction(tx, self.get_private_key(from_address))
            tx_hash = self.web3.eth.send_raw_transaction(signed_tx.raw_transaction)
            txs.append(tx_hash)
        logging.info(f"Total init ARB txs: {len(txs)}")


class Arbitrum(GenericChain):
    """
    A local simple implementation of Arbitrum chain
    """

    name = "Arbitrum"
    gas_per_byte = 68
    chain = "ARB"
    coin = Asset("ARB.ETH")
    withdrawals = {}
    swaps = {}

    @classmethod
    def _calculate_gas(cls, pool, txn):
        """
        Calculate gas according to CACAO mayachain fee
        """
        gas = 39540
        if txn.gas is not None and txn.gas[0].asset.is_arb():
            gas = txn.gas[0].amount * 3
        if (txn.memo == "WITHDRAW:ARB.ETH:1000" or
            txn.memo.startswith("SWAP:ARB.ETH:") or
                txn.memo == "WITHDRAW:ARB.ETH"):
            gas = get_arbitrum_gas(txn.id)

        return Coin(cls.coin, gas)
