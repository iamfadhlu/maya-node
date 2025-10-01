import os
import base64
import time
import asyncio
import threading
import durationpy

import ecdsa

from terra_sdk.core import AccAddress, AccPubKey
from terra_sdk.client.lcd import LCDClient
from terra_sdk.key.mnemonic import MnemonicKey as TerraMnemonicKey
from terra_sdk.core.fee import Fee
from terra_sdk.core.bech32 import get_bech
from terra_sdk.core.bank import MsgSend
from terra_sdk.client.lcd.api.tx import CreateTxOptions

from utils.segwit_addr import address_from_public_key
from utils.common import Coin, HttpClient, get_cacao_asset, Asset
from chains.aliases import aliases_kuji, get_aliases, get_alias_address
from chains.chain import GenericChain

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


# override mnemonickey class from Terra to get a maya address
class MnemonicKey(TerraMnemonicKey):
    @property
    def acc_address(self) -> AccAddress:
        """Mayachain Bech31 account address.
        Default derivation via :data:`public_key` is provided.

        Raises:
            ValueError: if Key was not initialized with proper public key

        Returns:
            AccAddress: account address
        """
        if not self.raw_address:
            raise ValueError("could not compute acc_address: missing raw_address")
        return AccAddress(get_bech("kujira", self.raw_address.hex()))

    @property
    def acc_pubkey(self) -> AccPubKey:
        """Mayachain Bech32 account pubkey.
        Default derivation via :data:`public_key` is provided.
        Raises:
            ValueError: if Key was not initialized with proper public key
        Returns:
            AccPubKey: account pubkey
        """
        if not self.raw_pubkey:
            raise ValueError("could not compute acc_pubkey: missing raw_pubkey")
        return AccPubKey(get_bech("kujirapub", self.raw_pubkey.hex()))


class MockKuji(HttpClient):
    """
    An client implementation for a mock kuji server.
    """

    mnemonic = {
        "CONTRIB": "belt wise road fiber advice task metal "
        + "point method casino wreck solar syrup aim myth "
        + "ball damage stamp width employ expire couch sea awake",
        "MASTER": "turn toddler salad marine journey orphan sing "
        + "cake master else forum either cook furnace endless "
        + "sister alien leg mercy copy exile fox myself cup",
        "USER-1": "turn actual solve lion clerk hint reunion silver large polar "
        + "baby palace shadow fortune general bleak alone harvest glue minute pull "
        + "grain forward glue",
        "PROVIDER-1": "original clog fish enroll inflict such burger record alert maid "
        + "twenty marriage camp hard nest evil winner anxiety unfold isolate flat "
        + "cinnamon minute sword",
    }
    block_stats = {
        "tx_rate": 200000,
        "tx_size": 1,
    }
    gas_price_factor = 1000000000
    gas_limit = 200000
    default_gas = 2000000
    chain_id = "harpoon-2"

    def __init__(self, base_url):
        self.base_url = base_url
        self.lcd_client = LCDClient(base_url, "localterra")
        self.lcd_client.chain_id = self.chain_id
        threading.Thread(target=self.scan_blocks, daemon=True).start()
        self.init_wallets()
        self.broadcast_fee_txs()

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

    def broadcast_fee_txs(self):
        """
        Generate 100 txs to build cache for bifrost to estimate fees
        """
        sequence = self.wallets["CONTRIB"].sequence() - 1
        for x in range(100):
            sequence += 1
            tx = self.wallets["CONTRIB"].create_and_sign_tx(
                CreateTxOptions(
                    msgs=[
                        MsgSend(
                            get_alias_address(Kuji.chain, "CONTRIB"),
                            get_alias_address(Kuji.chain, "MASTER"),
                            "100000ukuji",  # send 0.1 kujira
                        )
                    ],
                    sequence=sequence,
                    memo="fee generation",
                    fee=Fee(200000, "20000ukuji"),  # fee 0.02 kujira
                )
            )
            self.lcd_client.tx.broadcast_sync(tx)
            self.sequences["CONTRIB"] = sequence

    def scan_blocks(self):
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        lcd_client = LCDClient(self.base_url, "localterra")
        self.lcd_client.chain_id = self.chain_id
        height = int(lcd_client.tendermint.block_info()["block"]["header"]["height"])
        fee_cache = []
        while True:
            try:
                txs = lcd_client.tx.tx_infos_by_height(height)
                height += 1
                for tx in txs:
                    fee = tx.auth_info.fee
                    if len(fee.amount) != 1:
                        continue
                    fee_coin = fee.amount[0]
                    if fee_coin.denom != "ukuji":
                        continue
                    gas = fee.gas_limit
                    amount = int(fee_coin.amount) * 100 * self.gas_price_factor
                    price = amount / gas
                    fee = price * self.gas_limit
                    fee = fee / self.gas_price_factor
                    fee_cache.append(fee)
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
    def get_address_from_pubkey(cls, pubkey, prefix="kujira"):
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
            symbol = coin.denom[1:].upper()
            if symbol != "kuji":
                continue
            asset = f"{Kuji.chain}.{symbol}"
            result.append(Coin(asset, coin.amount * 100))
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
        aliases_kuji["VAULT"] = addr

    def transfer(self, txn):
        """
        Make a transaction/transfer on local Kuji
        """
        if not isinstance(txn.coins, list):
            txn.coins = [txn.coins]

        from_alias = txn.from_address
        wallet = self.wallets[from_alias]
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
        tx = wallet.create_and_sign_tx(
             CreateTxOptions(
                 msgs=[
                     MsgSend(
                         txn.from_address, txn.to_address, txn.coins[0].to_cosmos_kuji()
                     )
                 ],
                 memo=txn.memo,
                 fee=Fee(200000, "20000ukuji"),
                 sequence=sequence
             )
         )
        # Kuji has cosmos-sdk 0.47.0+ version which deprecated broadcast block mode
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
        txn.gas = [Coin("KUJI.KUJI", self.default_gas)]


class Kuji(GenericChain):
    """
    A local simple implementation of Kuji chain
    """

    name = "Kuji"
    chain = "KUJI"
    coin = Asset("KUJI.KUJI")
    cacao_fee = 2000000

    @classmethod
    def _calculate_gas(cls, pool, txn):
        """
        Calculate gas according to CACAO mayachain fee
        """
        return Coin(cls.coin, 2000000)
