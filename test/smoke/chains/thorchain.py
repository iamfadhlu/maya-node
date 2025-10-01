import os
import base64
import logging
import asyncio
import threading
import ecdsa
import durationpy
import time
import requests

from terra_sdk.core import AccAddress, AccPubKey
from terra_sdk.client.lcd import LCDClient
from terra_sdk.core.fee import Fee
from terra_sdk.core.bech32 import get_bech
from terra_sdk.key.mnemonic import MnemonicKey as TerraMnemonicKey
from utils.msgs import MsgSend
from terra_sdk.core.bank import MsgSend as MsgSendBank

from terra_sdk.client.lcd.api.tx import CreateTxOptions, SignerOptions

from utils.segwit_addr import address_from_public_key
from utils.common import HttpClient, Coins, Coin, Asset, get_cacao_asset
from chains.aliases import get_alias_address, get_aliases, get_alias, aliases_thor
from chains.chain import GenericChain
from chains.account import Account
from terra_sdk.core.bank import MsgMultiSend, MultiSendInput, MultiSendOutput
from terra_sdk.core import Coins as TerraCoins
from terra_sdk.core import Coin as TerraCoin
from terra_sdk.core import SignDoc
from decimal import Decimal, getcontext

getcontext().prec = 8

# Init logging
logging.basicConfig(
    format="%(levelname).1s[%(asctime)s] %(message)s",
    level=os.environ.get("LOGLEVEL", "INFO"),
)

CACAO = get_cacao_asset()

# wallet helper functions
# Thanks to https://github.com/hukkinj1/cosmospy


def generate_wallet():
    privkey = ecdsa.SigningKey.generate(curve=ecdsa.SECP256k1).to_string().hex()
    pubkey = privkey_to_pubkey(privkey)
    address = address_from_public_key(pubkey)
    return {"private_key": privkey, "public_key": pubkey, "address": address}


def privkey_to_pubkey(privkey):
    privkey_obj = ecdsa.SigningKey.from_string(
        bytes.fromhex(privkey), curve=ecdsa.SECP256k1
    )
    pubkey_obj = privkey_obj.get_verifying_key()
    return pubkey_obj.to_string("compressed").hex()


def privkey_to_address(privkey):
    pubkey = privkey_to_pubkey(privkey)
    return address_from_public_key(pubkey)

def get_thorchain_fee():
    thor_url = "http://localhost:1318/thorchain/network"
    timeout_seconds = 10
    default_gas = 2000000
    response = requests.get(thor_url, timeout=timeout_seconds)

    if response.status_code == 200:
        response_json = response.json()
        return int(response_json["native_tx_fee_rune"])
    else:
        logging.info(f"Failed to get THORChains fee")
        return default_gas


# override mnemonickey class from Terra to get a thor address
class MnemonicKey(TerraMnemonicKey):
    @property
    def acc_address(self) -> AccAddress:
        """Thorchain Bech32 account address.
        Default derivation via :data:`public_key` is provided.

        Raises:
            ValueError: if Key was not initialized with proper public key

        Returns:
            AccAddress: account address
        """
        if not self.raw_address:
            raise ValueError("could not compute acc_address: missing raw_address")
        return AccAddress(get_bech("tthor", self.raw_address.hex()))

    @property
    def acc_pubkey(self) -> AccPubKey:
        """Thorchain Bech32 account pubkey.
        Default derivation via :data:`public_key` is provided.
        Raises:
            ValueError: if Key was not initialized with proper public key
        Returns:
            AccPubKey: account pubkey
        """
        if not self.raw_pubkey:
            raise ValueError("could not compute acc_pubkey: missing raw_pubkey")
        return AccPubKey(get_bech("tthorpub", self.raw_pubkey.hex()))


class MockThorchain(HttpClient):
    """
    A local simple implementation of thorchain chain
    """

    chain = "THOR"
    mnemonic = {
        "CONTRIB": "satisfy adjust timber high purchase tuition stool "
        + "faith fine install that you unaware feed domain "
        + "license impose boss human eager hat rent enjoy dawn",
        "MASTER": "vintage announce rapid clip spare stomach matter camp noble habit "
        + "beef amateur chimney time fuel machine culture end toe oval isolate "
        + "laptop solar gift",
        "USER-1": "sock true leave evil budget lonely foster danger reopen anxiety "
        + "dash naive list advance unhappy trust inmate culture bounce museum light "
        + "more pear story",
        "PROVIDER-1": "discover blue crunch cart club fish airport crazy roast hybrid "
        + "scheme picnic veteran mango beach narrow luxury glory dynamic crawl symbol "
        + "win sell dress",
    }
    block_stats = {
        "tx_rate": get_thorchain_fee(),
        "tx_size": 1,
    }
    gas_price_factor = 1000000000
    gas_limit = 2000000
    default_gas = 2000000

    def __init__(self, base_url):
        self.base_url = base_url
        self.lcd_client = LCDClient(base_url, "localterra")
        self.lcd_client.chain_id = "thorchain"
        threading.Thread(target=self.scan_blocks, daemon=True).start()
        self.init_wallets()

    def init_wallets(self):
        """
        Init wallet instances
        """
        self.wallets = {}
        self.sequences = {}
        for alias in self.mnemonic:
            mk = MnemonicKey(mnemonic=self.mnemonic[alias], coin_type=118)
            self.wallets[alias] = self.lcd_client.wallet(mk)
            self.sequences[alias] = 0

    def scan_blocks(self):
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        lcd_client = LCDClient(self.base_url, "localterra")
        self.lcd_client.chain_id = "thorchain"
        height = int(lcd_client.tendermint.block_info()["block"]["header"]["height"])
        fee_cache = []
        while True:
            try:
                txs = lcd_client.tx.tx_infos_by_height(height)
                height += 1
                for tx in txs:
                    fee = tx.auth_info.fee
                    fee_cache.append(2000000)
                    if len(fee_cache) > 100:
                        fee_cache.pop(0)
                if len(fee_cache) != 100:
                    continue
                if (height - 1) % 10 == 0:
                    tx_rate = int(sum(fee_cache) / 100) // 100000 * 100000
                    self.block_stats["tx_rate"] = tx_rate
            except Exception:
                continue
            finally:
                default = "1.0s"
                backoff = os.environ.get("BLOCK_SCANNER_BACKOFF", default)
                if backoff == "" or backoff is None:
                    backoff = default
                backoff = durationpy.from_str(backoff).total_seconds()
                time.sleep(backoff)

    @classmethod
    def get_address_from_pubkey(cls, pubkey, prefix="tthor"):
        """
        Get bnb testnet address for a public key

        :param string pubkey: public key
        :returns: string bech32 encoded address
        """
        return address_from_public_key(pubkey, prefix)

    def get_block(self, block_height=None):
        """
        Get the block data for a height
        """
        return self.lcd_client.tendermint.block_info(block_height)

    def get_balance(self, account):
        """
        Get the balance account
        """
        coins = self.lcd_client.bank.balance(account)[0]
        result = []
        for coin in coins.to_list():
            symbol = coin.denom.upper()
            if symbol != "RUNE":
                continue
            asset = f"{Thorchain.chain}.{symbol}"
            result.append(Coin(asset, coin.amount))
        return result

    def get_block_txs(self, block_height=None):
        """
        Get the block txs data for a height
        """
        return self.lcd_client.tx.tx_infos_by_height(block_height)

    def set_vault_address_by_pubkey(self, pubkey):
        """
        Set vault address by pubkey
        """
        self.set_vault_address(self.get_address_from_pubkey(pubkey))

    def set_vault_address(self, addr):
        """
        Set the vault bnb address
        """
        aliases_thor["VAULT"] = addr

    def transfer(self, txn):
        """
        Make a transaction/transfer on local Thor
        """
        if not isinstance(txn.coins, list):
            txn.coins = [txn.coins]

        wallet = self.wallets[txn.from_address]
        from_alias = txn.from_address
        sequence = self.sequences[from_alias]

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
            # we use CACAO BNB address to identify a cross chain liqudity provision
            if txn.memo.startswith("ADD") or is_synth:
                chain = CACAO.get_chain()
            addr = get_alias_address(chain, alias)
            txn.memo = txn.memo.replace(alias, addr)

        # create transaction
        coins = Coins(txn.coins)
        if txn.memo == "SEED":
            tx = wallet.create_and_sign_tx(
                CreateTxOptions(
                    msgs=[
                        MsgSendBank(
                            txn.from_address, txn.to_address, txn.coins[0].to_cosmos_thor()
                        )
                    ],
                    memo=txn.memo,
                    fee=Fee(2000000, "20000rune"),
                    sequence=sequence,
                )
            )
        else:
            tx = wallet.create_and_sign_tx(
                CreateTxOptions(
                    msgs=[
                        MsgSend(
                            txn.from_address, txn.to_address, coins
                        )
                    ],
                    memo=txn.memo,
                    fee=Fee(2000000, "0rune"),
                    sequence=sequence,
                )
            )
        # THOR has cosmos-sdk 0.47.0+ version which deprecated broadcast block mode
        # so we need to use braodcast sync mode and keep track of sequence numbers
        data = {
            "tx_bytes": base64.b64encode(tx.to_proto().SerializeToString()).decode(),
            "mode": "BROADCAST_MODE_SYNC"
          }
        # We need to replace the terra_sdk broadcast_sync method with our own
        # because the response is different
        result = self.post("/cosmos/tx/v1beta1/txs", data)
        self.sequences[from_alias] = sequence + 1
        txn.id = result['tx_response']['txhash']
        txn.gas = [Coin("THOR.RUNE", self.default_gas)]


class Thorchain(GenericChain):
    """
    A local simple implementation of thorchain chain
    """

    name = "THORChain"
    chain = "THOR"
    coin = Asset("THOR.RUNE")

    def __init__(self):
        super().__init__()

        # seeding the users, these seeds are established in build/docker/mocknet/thorchain/genesis.sh
        acct = Account("tthor1z63f3mzwv3g75az80xwmhrawdqcjpaekk0kd54")
        acct.add(Coin(self.coin, 5000000000000))
        self.set_account(acct)

        acct = Account("tthor1wz78qmrkplrdhy37tw0tnvn0tkm5pqd6zdp257")
        acct.add(Coin(self.coin, 25000000000100))
        self.set_account(acct)

        acct = Account("tthor1xwusttz86hqfuk5z7amcgqsg7vp6g8zhsp5lu2")
        acct.add(Coin(self.coin,  5090000000000))
        self.set_account(acct)

    @classmethod
    def _calculate_gas(cls, pool, txn):
        """
        With given coin set, calculates the gas owed
        """
        return Coin(cls.coin, 2000000)
