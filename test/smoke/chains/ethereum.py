import logging
import json
import requests

from web3 import Web3, HTTPProvider
from web3.middleware import ExtraDataToPOAMiddleware, SignAndSendRawMiddlewareBuilder
from eth_keys import KeyAPI
from utils.common import Coin, get_cacao_asset, get_gas_from_recipient, Asset
from chains.aliases import aliases_eth, get_aliases, get_alias_address
from chains.chain import GenericChain

CACAO = get_cacao_asset()


def calculate_gas(msg):
    return MockEthereum.default_gas + Ethereum.gas_per_byte * len(msg)


class MockEthereum:
    """
    An client implementation for a localnet/rinkebye/ropston Ethereum server
    """

    default_gas = 100000
    gas_price = 2
    seed = "SEED"
    stake = "ADD"
    passphrase = ""
    tokens = dict()
    zero_address = "0x0000000000000000000000000000000000000000"
    default_account_address = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
    vault_contract_addr = "0x17aB05351fC94a1a67Bf3f56DdbB941aE6c63E25"
    token_contract_addr = "0x52C84043CD9c865236f11d9Fc9F56aa003c1f922"

    private_keys = [
        "56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027",
        "289c2857d4598e37fb9647507e47a309d6133539bf21a8b9cb6df88fd5232032",
        "e810f1d7d6691b4a7a73476f3543bd87d601f9a53e7faf670eac2c5b517d83bf",
        "a96e62ed3955e65be32703f12d87b6b5cf26039ecfa948dc5107a495418e5330",
        "9294f4d108465fd293f7fe299e6923ef71a77f2cb1eb6d4394839c64ec25d5c0",
    ]
    loaded_accounts = []
    def __init__(self, base_url):
        self.url = base_url
        self.web3 = Web3(HTTPProvider(base_url))
        self.web3.middleware_onion.inject(ExtraDataToPOAMiddleware, layer=0)
        
        self.imported_accounts = {
            account.address: account
            for account in [self.web3.eth.account.from_key(key) for key in self.private_keys]
        }
        # self.accounts is a list of addresses
        self.accounts = [account.address for account in self.imported_accounts.values()]
        
        
        # set self.web3.eth.default_account to the account where the address is = self.default_account_address
        # self.default_account = self.get_address_from_pubkey(self.default_account_address)
        self.web3.eth.default_account = self.accounts[0]
        self.web3.middleware_onion.inject(
            SignAndSendRawMiddlewareBuilder.build(self.imported_accounts.get(self.accounts[0])),
            layer=0
        )
        self.loaded_accounts.append(self.accounts[0])
        
        self.vault = self.get_vault()
        token = self.get_token()
        symbol = token.functions.symbol().call()
        self.tokens[symbol] = token

    @classmethod
    def get_address_from_pubkey(cls, pubkey):
        """
        Get Ethereum address for a specific hrp (human readable part)
        bech32 encoded from a public key(secp256k1).

        :param string pubkey: public key
        :returns: string 0x encoded address
        """
        eth_pubkey = KeyAPI.PublicKey.from_compressed_bytes(pubkey)
        return eth_pubkey.to_address()

    def set_vault_address(self, addr):
        """
        Set the vault eth address
        """
        aliases_eth["VAULT"] = addr

    def get_block_height(self):
        """
        Get the current block height of Ethereum localnet
        """
        block = self.web3.eth.getBlock("latest")
        return block["number"]

    def get_token(self):
        abi = json.load(open("data/token.json"))
        token = self.web3.eth.contract(address=self.token_contract_addr, abi=abi)
        return token

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
        Get ETH or token balance for an address
        """
        if symbol == "ETH":
            return self.web3.eth.get_balance(Web3.to_checksum_address(address), "latest")

        if address == "VAULT" or address == aliases_eth["VAULT"]:
            address = self.vault.address

        return (
            self.tokens[symbol]
            .functions.balanceOf(Web3.to_checksum_address(address))
            .call()
        )

    def wait_for_node(self):
        """
        Ethereum pow localnet node is started with directly mining 4 blocks
        to be able to start handling transactions.
        It can take a while depending on the machine specs so we retry.
        """
        current_height = self.get_block_height()
        while current_height < 2:
            current_height = self.get_block_height()

    def transfer(self, txn):
        """
        Make a transaction/transfer on localnet Ethereum
        """
        if not isinstance(txn.coins, list):
            txn.coins = [txn.coins]

        if txn.to_address in aliases_eth.keys():
            txn.to_address = get_alias_address(txn.chain, txn.to_address)

        if txn.from_address in aliases_eth.keys():
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
        if txn.memo == self.seed:

            if txn.coins[0].asset.get_symbol() == Ethereum.chain:
                tx_hash = self.vault.functions.deposit(
                    Web3.to_checksum_address(txn.to_address),
                    Web3.to_checksum_address(self.zero_address),
                    0,
                    txn.memo,
                ).transact(
                    {
                        "value": txn.coins[0].amount,
                        "gas": calculate_gas(txn.memo),
                    }
                )
            else:
                tx_hash = (
                    self.tokens[txn.coins[0].asset.get_symbol().split("-")[0]]
                    .functions.transfer(
                        Web3.to_checksum_address(txn.to_address), txn.coins[0].amount
                    )
                    .transact()
                )
        else:
            memo = txn.memo

            if txn.coins[0].asset.get_symbol().split("-")[0] == Ethereum.chain:
                tx_hash = self.vault.functions.deposit(
                    Web3.to_checksum_address(txn.to_address),
                    Web3.to_checksum_address(self.zero_address),
                    0,
                    txn.memo,
                ).transact({"value": txn.coins[0].amount})
            else:
                # approve the tx first
                symbol = txn.coins[0].asset.get_symbol().split("-")[0]
                tx_hash = (
                    self.tokens[symbol]
                    .functions.approve(
                        Web3.to_checksum_address(self.vault.address), txn.coins[0].amount
                    )
                    .transact()
                )
                receipt = self.web3.eth.wait_for_transaction_receipt(tx_hash)
                spent_gas = receipt.get('gasUsed') * receipt.get('effectiveGasPrice')
                token_address = self.tokens[symbol].address
                tx_hash = self.vault.functions.deposit(
                    Web3.to_checksum_address(txn.to_address),
                    token_address,
                    txn.coins[0].amount,
                    memo,
                ).transact()

        receipt = self.web3.eth.wait_for_transaction_receipt(tx_hash)
        txn.id = receipt.get('transactionHash').hex().upper()
        txn.gas = [
            Coin(
                "ETH.ETH",
                (receipt.get('gasUsed') * receipt.get('effectiveGasPrice') + spent_gas),
            )
        ]


class Ethereum(GenericChain):
    """
    A local simple implementation of Ethereum chain
    """

    name = "Ethereum"
    gas_per_byte = 68
    chain = "ETH"
    coin = Asset("ETH.ETH")
    withdrawals = {}
    swaps = {}

    @classmethod
    def _calculate_gas(cls, pool, txn):
        """
        Calculate gas according to CACAO mayachain fee
        """
        eth_url = "http://localhost:8545"
        gas = 39540
        if txn.gas is not None and txn.gas[0].asset.is_eth():
            gas = txn.gas[0].amount
        if txn.memo == "WITHDRAW:ETH.ETH:1000":
            gas = 39836
        elif txn.memo.startswith("SWAP:ETH.ETH:"):
            gas = 39812
        elif (txn.memo.startswith("SWAP:ETH.TKN-0X52C84043CD9C865236F11D9FC9F56AA003C1F922") or
              txn.memo.startswith("WITHDRAW:ETH.TKN-0X52C84043CD9C865236F11D9FC9F56AA003C1F922")):
            gas = get_gas_from_recipient(txn.id, eth_url)
        elif txn.memo == "WITHDRAW:ETH.ETH":
            gas = 39848
        return Coin(cls.coin, gas * 3)
