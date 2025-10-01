import argparse
import time
import logging
import os
import sys
import json
from copy import deepcopy
from tenacity import retry, stop_after_delay, wait_fixed

from utils.segwit_addr import decode_address
from chains.binance import Binance, MockBinance
from chains.kuji import Kuji, MockKuji
from chains.bitcoin import Bitcoin, MockBitcoin
from chains.dash import Dash, MockDash
from chains.thorchain import Thorchain, MockThorchain
from chains.ethereum import Ethereum, MockEthereum
from chains.arbitrum import Arbitrum, MockArbitrum
from chains.radix import Radix, MockRadix
from chains.mayachain import Mayachain, MockMayachain
from mayachain.mayachain import MayachainState, MayachainClient, INIT_BTC_CACAO_BALANCE, INIT_BTC_ASSET_BALANCE, INIT_BTC_LP_UNITS
from scripts.health import Health
from utils.common import Transaction, Coin, Asset, get_cacao_asset
from chains.aliases import aliases_bnb, get_alias, aliases_thor, get_alias_address

# Init logging
logging.basicConfig(
    format="%(levelname).1s[%(asctime)s] %(message)s",
    level=os.environ.get("LOGLEVEL", "INFO"),
)

CACAO = get_cacao_asset()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--binance",
        default="http://localhost:26660",
        help="Mock binance server",
    )
    parser.add_argument(
        "--kuji",
        default="http://localhost:31317",
        help="Local kuji server",
    )
    parser.add_argument(
        "--thor",
        default="http://localhost:1318",
        help="Local thor server",
    )
    parser.add_argument(
        "--bitcoin",
        default="http://mayachain:password@localhost:18443",
        help="Regtest bitcoin server",
    )
    parser.add_argument(
        "--dash",
        default="http://mayachain:password@localhost:19898",
        help="Regtest dash server",
    )
    parser.add_argument(
        "--ethereum",
        default="http://localhost:8545",
        help="Localnet ethereum server",
    )
    parser.add_argument(
        "--arbitrum",
        default="http://localhost:8547",
        help="Localnet arbitrum server",
    )
    parser.add_argument(
        "--radix",
        default="http://localhost:3333/core",
        help="Localnet radix server",
    )
    parser.add_argument(
        "--mayachain", default="http://localhost:1317", help="Mayachain API url"
    )
    parser.add_argument(
        "--midgard", default="http://localhost:8080", help="Midgard API url"
    )
    parser.add_argument(
        "--generate-balances", default=False, type=bool, help="Generate balances (bool)"
    )
    parser.add_argument(
        "--fast-fail", default=False, type=bool, help="Generate balances (bool)"
    )
    parser.add_argument(
        "--no-verify", default=False, type=bool, help="Skip verifying results"
    )
    parser.add_argument(
        "--chain", default="", help="Chain to run smoke tests on"
    )

    parser.add_argument(
        "--bitcoin-reorg",
        default=False,
        type=bool,
        help="Trigger a Bitcoin chain reorg",
    )

    parser.add_argument(
        "--ethereum-reorg",
        default=False,
        type=bool,
        help="Trigger an Ethereum chain reorg",
    )

    parser.add_argument(
        "--arbitrum-reorg",
        default=False,
        type=bool,
        help="Trigger an Arbitrum chain reorg",
    )

    parser.add_argument(
        "--bootstrap-only",
        default=False,
        type=bool,
        help="Only perform liquidity adds to seed the mocknet",
    )

    args = parser.parse_args()

    txn_list = "data/smoke_test_transactions.json"
    with open(txn_list, "r") as f:
        txns = json.load(f)

    health = Health(
        args.mayachain, args.midgard, args.binance, fast_fail=args.fast_fail
    )

    smoker = Smoker(
        args.binance,
        args.kuji,
        args.bitcoin,
        args.dash,
        args.thor,
        args.ethereum,
        args.arbitrum,
        args.radix,
        args.mayachain,
        health,
        txns,
        args.generate_balances,
        args.fast_fail,
        args.no_verify,
        args.bitcoin_reorg,
        args.ethereum_reorg,
        args.arbitrum_reorg,
        args.bootstrap_only,
    )
    try:
        smoker.run()
        sys.exit(smoker.exit)
    except Exception as e:
        logging.error(e)
        logging.exception("Smoke tests failed")
        sys.exit(1)


class Smoker:
    def __init__(
        self,
        bnb,
        kuji_url,
        btc,
        dash,
        thor_url,
        eth,
        arb,
        xrd,
        maya,
        health,
        txns,
        gen_balances=False,
        fast_fail=False,
        no_verify=False,
        bitcoin_reorg=False,
        ethereum_reorg=False,
        arbitrum_reorg=False,
        bootstrap_only=False,
    ):
        self.binance = Binance()
        self.kuji = Kuji()
        self.bitcoin = Bitcoin()
        self.dash = Dash()
        self.thor = Thorchain()
        self.ethereum = Ethereum()
        self.radix = Radix()
        self.arbitrum = Arbitrum()
        self.mayachain = Mayachain()
        self.mayachain_state = MayachainState()

        self.health = health

        self.txns = txns

        self.mayachain_client = MayachainClient(maya, enable_websocket=True)
        pubkey = self.mayachain_client.get_vault_pubkey()
        # extract pubkey from bech32 encoded pubkey
        # removing first 5 bytes used by amino encoding
        raw_pubkey = decode_address(pubkey)[5:]

        self.mayachain_state.set_vault_pubkey(pubkey)

        self.mayachain_state.reserve = 22000000000000000
        poolBTC = self.mayachain_state.get_pool("BTC.BTC")
        poolBTC.cacao_balance = INIT_BTC_CACAO_BALANCE
        poolBTC.asset_balance = INIT_BTC_ASSET_BALANCE
        poolBTC.lp_units = INIT_BTC_LP_UNITS
        poolBTC.status = "Available"
        self.mayachain_state.set_pool(poolBTC)

        if INIT_BTC_ASSET_BALANCE != 0:
            acct = self.bitcoin.get_account("bcrt1qzf3gsk7edzwl9syyefvfhle37cjtql35tlzesk")
            init_btc_coin = Coin("BTC.BTC", INIT_BTC_ASSET_BALANCE)
            acct.add(init_btc_coin)
            self.bitcoin.set_account(acct)

        self.mock_mayachain = MockMayachain(maya)

        # setup bitcoin
        self.mock_bitcoin = MockBitcoin(btc)
        bitcoin_address = MockBitcoin.get_address_from_pubkey(raw_pubkey)
        self.mock_bitcoin.set_vault_address(bitcoin_address)

        # setup dash
        self.mock_dash = MockDash(dash)
        dash_address = MockDash.get_address_from_pubkey(raw_pubkey)
        self.mock_dash.set_vault_address(dash_address)

        # setup ethereum
        self.mock_ethereum = MockEthereum(eth)
        ethereum_address = MockEthereum.get_address_from_pubkey(raw_pubkey)
        self.mock_ethereum.set_vault_address(ethereum_address)

        # setup arbitrum
        self.mock_arbitrum = MockArbitrum(arb)
        arbitrum_address = MockArbitrum.get_address_from_pubkey(raw_pubkey)
        self.mock_arbitrum.set_vault_address(arbitrum_address)

        # setup radix
        self.mock_radix = MockRadix(xrd, self.mayachain_client.get_router_address("XRD"))
        self.mock_radix.set_vault_address_by_pubkey(raw_pubkey)

        # setup binance
        self.mock_binance = MockBinance(bnb)
        self.mock_binance.set_vault_address_by_pubkey(raw_pubkey)

        # setup kuji
        self.mock_kuji = MockKuji(kuji_url)
        self.mock_kuji.set_vault_address_by_pubkey(raw_pubkey)

        # setup thorchain
        self.mock_thor = MockThorchain(thor_url)
        self.mock_thor.set_vault_address_by_pubkey(raw_pubkey)

        self.generate_balances = gen_balances
        self.fast_fail = fast_fail
        self.no_verify = no_verify or bootstrap_only
        self.bitcoin_reorg = bitcoin_reorg
        self.ethereum_reorg = ethereum_reorg
        self.arbitrum_reorg = arbitrum_reorg
        self.bootstrap_only = bootstrap_only
        self.mayachain_client.events = []
        self.exit = 0
        self.readyToCheckHealth = False

    def error(self, err):
        self.exit = 1
        if self.fast_fail:
            raise Exception(err)
        else:
            logging.error(err)

    def check_pool_attribute(self, sa, ra, asset, description):
        if int(sa) != int(ra):
            self.error(
                f"Bad Pool-{asset} balance: {description} "
                f"Sim {sa} != Real {ra}"
            )

    def check_pools(self):
        # compare simulation pools vs real pools
        real_pools = self.mayachain_client.get_pools()
        for rpool in real_pools:
            spool = self.mayachain_state.get_pool(Asset(rpool["asset"]))
            self.check_pool_attribute(
                spool.cacao_balance, rpool["balance_cacao"], rpool['asset'], "CACAO")
            self.check_pool_attribute(
                spool.asset_balance, rpool["balance_asset"], rpool['asset'], "ASSET")
            self.check_pool_attribute(
                spool.synth_balance, rpool["synth_supply"], rpool['asset'], "SYNTH SUPPLY")
            self.check_pool_attribute(
                spool.lp_units, rpool["LP_units"], rpool['asset'], "LP UNITS")
            self.check_pool_attribute(
                spool.synth_units(), rpool["synth_units"], rpool['asset'], "SYNTH UNITS")
            self.check_pool_attribute(
                spool.pool_units(), rpool["pool_units"], rpool['asset'], "POOL UNITS")

    def check_binance(self):
        # compare simulation binance vs mock binance
        mock_accounts = self.mock_binance.accounts()
        for macct in mock_accounts:
            for name, address in aliases_bnb.items():
                if name == "MASTER":
                    continue  # don't care to compare MASTER account
                if address == macct["address"]:
                    sacct = self.binance.get_account(address)
                    for bal in macct["balances"]:
                        sim_coin = Coin(
                            f"BNB.{bal['denom']}", sacct.get(
                                f"BNB.{bal['denom']}")
                        )
                        bnb_coin = Coin(f"BNB.{bal['denom']}", bal["amount"])
                        if sim_coin != bnb_coin:
                            self.error(
                                f"Bad binance balance: {name} {bnb_coin} != {sim_coin}"
                            )

    def check_cosmos(self, chain, mock):
        for addr, sim_acct in chain.accounts.items():
            name = get_alias(chain.chain, addr)
            if name == "MASTER":
                continue  # don't care to compare MASTER account
            if name == "VAULT" and chain.chain == "MAYA":
                continue  # don't care about vault for mayachain
            mock_coins = mock.get_balance(addr)
            for mock_coin in mock_coins:
                sim_coin = sim_acct.get(mock_coin.asset)
                sim_coin = Coin(mock_coin.asset, sim_acct.get(mock_coin.asset))
                if sim_coin != mock_coin:
                    self.error(
                        f"Bad {chain.name} balance: {name} {mock_coin} != {sim_coin}"
                    )

    def check_thorchain(self, chain, mock):
        for addr, sim_acct in chain.accounts.items():
            name = get_alias(chain.chain, addr)
            if name == "MASTER":
                continue  # don't care to compare MASTER account
            if name == "VAULT":
                continue  # don't care about vault for mayachain
            mock_coins = mock.get_balance(addr)
            for mock_coin in mock_coins:
                sim_coin = sim_acct.get(mock_coin.asset)
                sim_coin = Coin(mock_coin.asset, sim_acct.get(mock_coin.asset))
                if sim_coin != mock_coin:
                    self.error(
                        f"Bad {chain.name} balance1: {name} {mock_coin} != {sim_coin}"
                    )

    def check_chain(self, chain, mock, reorg):
        # compare simulation account balances vs mock chain balances
        for addr, sim_acct in chain.accounts.items():
            name = get_alias(chain.chain, addr)
            if name == "MASTER":
                continue  # don't care to compare MASTER account
            if name == "VAULT" and chain.chain == "MAYA":
                continue  # don't care about vault for mayachain
            mock_coin = Coin(chain.coin, mock.get_balance(addr))

            sim_coin = Coin(chain.coin, sim_acct.get(chain.coin))

            # dont raise error on reorg balance being invalidated
            # sim is not smart enough to subtract funds on reorg
            if mock_coin.amount == 0 and reorg:
                return
            if chain.chain == "XRD":
                # We allow max 1 subunit (1e-18 XRD) of balance deviation due to rounding (of fees, specifically)
                max_allowed_rounding_error = 1
            else:
                # Amounts (fees) on other chains happen to convert without rounding errors
                max_allowed_rounding_error = 0
            if abs(sim_coin.amount - mock_coin.amount) > max_allowed_rounding_error:
                self.error(
                    f"Bad {chain.name} balance: {name} {mock_coin} != ({sim_coin} +/- {max_allowed_rounding_error})"
                )

    def check_ethereum(self):
        # compare simulation ethereum vs mock ethereum
        for addr, sim_acct in self.ethereum.accounts.items():
            name = get_alias(self.ethereum.chain, addr)
            if name == "MASTER":
                continue  # don't care to compare MASTER account
            for sim_coin in sim_acct.balances:
                if not sim_coin.asset.is_eth():
                    continue
                mock_coin = Coin(
                    "ETH." + sim_coin.asset.get_symbol(),
                    self.mock_ethereum.get_balance(
                        addr, sim_coin.asset.get_symbol().split("-")[0]
                    ),
                )
                # dont raise error on reorg balance being invalidated
                # sim is not smart enough to subtract funds on reorg
                if mock_coin.amount == 0 and self.ethereum_reorg:
                    return
                if sim_coin != mock_coin:
                    self.error(
                        f"Bad ETH balance: {name} {mock_coin} != {sim_coin}")

    def check_arbitrum(self):
        # compare simulation arbitrum vs mock arbitrum
        for addr, sim_acct in self.arbitrum.accounts.items():
            name = get_alias(self.arbitrum.chain, addr)
            if name == "MASTER":
                continue  # don't care to compare MASTER account
            for sim_coin in sim_acct.balances:
                if not sim_coin.asset.is_arb():
                    continue
                mock_coin = Coin(
                    "ETH." + sim_coin.asset.get_symbol(),
                    self.mock_arbitrum.get_balance(
                        addr, sim_coin.asset.get_symbol().split("-")[0]
                    ),
                )
                # dont raise error on reorg balance being invalidated
                # sim is not smart enough to subtract funds on reorg
                if mock_coin.amount == 0 and self.arbitrum_reorg:
                    return
                if sim_coin != mock_coin:
                    self.error(
                        f"Bad ARB balance: {name} {mock_coin} != {sim_coin}")

    def check_vaults(self, block_height):
        # check vault data
        vdata = self.mayachain_client.get_vault_data(block_height)
        if int(vdata["total_reserve"]) != self.mayachain_state.reserve:
            sim = self.mayachain_state.reserve
            real = vdata["total_reserve"]
            self.error(f"Mismatching reserves: {sim} != {real}")
        if int(vdata["bond_reward_rune"]) != self.mayachain_state.bond_reward:
            sim = self.mayachain_state.bond_reward
            real = vdata["bond_reward_rune"]
            self.error(f"Mismatching bond reward: {sim} != {real}")

    def check_events(self, events, sim_events):
        events = set(events)
        sim_events = set(sim_events)
        if events != sim_events:
            for e in events - sim_events:
                print(">>>>>>>>>>>>>>> MISSING SIM EVENT")
                print(e)
            for e in sim_events - events:
                print("<<<<<<<<<<<<<<< EXTRA SIM EVENT")
                print(e)
            self.error("Events mismatch\n")

    @retry(stop=stop_after_delay(30), wait=wait_fixed(0.3), reraise=True)
    def run_health(self):
        self.health.run()

    def broadcast_chain(self, txn):
        """
        Broadcast tx to respective chain mock server
        """
        if txn.chain == Binance.chain:
            return self.mock_binance.transfer(txn)
        if txn.chain == Bitcoin.chain:
            return self.mock_bitcoin.transfer(txn)
        if txn.chain == Dash.chain:
            return self.mock_dash.transfer(txn)
        if txn.chain == Thorchain.chain:
            return self.mock_thor.transfer(txn)
        if txn.chain == Ethereum.chain:
            return self.mock_ethereum.transfer(txn)
        if txn.chain == Arbitrum.chain:
            return self.mock_arbitrum.transfer(txn)
        if txn.chain == Kuji.chain:
            return self.mock_kuji.transfer(txn)
        if txn.chain == Radix.chain:
            return self.mock_radix.transfer(txn)
        if txn.chain == MockMayachain.chain:
            return self.mock_mayachain.transfer(txn)

    def broadcast_simulator(self, txn):
        """
        Broadcast tx to simulator state chain
        """
        if txn.chain == Binance.chain:
            return self.binance.transfer(txn)
        if txn.chain == Kuji.chain:
            return self.kuji.transfer(txn)
        if txn.chain == Bitcoin.chain:
            return self.bitcoin.transfer(txn)
        if txn.chain == Dash.chain:
            return self.dash.transfer(txn)
        if txn.chain == Ethereum.chain:
            tx_copy = deepcopy(txn)
            return self.ethereum.transfer(tx_copy)
        if txn.chain == Arbitrum.chain:
            tx_copy = deepcopy(txn)
            return self.arbitrum.transfer(tx_copy)
        if txn.chain == Thorchain.chain:
            return self.thor.transfer(txn)
        if txn.chain == Mayachain.chain:
            return self.mayachain.transfer(txn)
        if txn.chain == Radix.chain:
            return self.radix.transfer(txn)

    def set_network_fees(self):
        """
        Retrieve network fees on chain for each txn
        and update mayachain state
        """
        btc = self.mock_bitcoin.block_stats
        dash = self.mock_dash.block_stats
        thor = self.mock_thor.block_stats
        kuji = self.mock_kuji.block_stats
        arb = self.mock_arbitrum.block_stats
        fees = {
            "BNB": self.mock_binance.singleton_gas,
            "ETH": self.mock_ethereum.gas_price * self.mock_ethereum.default_gas,
            "ARB": self.mock_arbitrum.gas_price * self.mock_arbitrum.default_gas,
            "BTC": btc["tx_size"] * btc["tx_rate"],
            "DASH": dash["tx_size"] * self.mock_dash.estimated_dash_gas_rate,
            "KUJI": kuji["tx_size"] * kuji["tx_rate"],
            "THOR": thor["tx_size"] * thor["tx_rate"],
            "XRD": self.mock_radix.estimated_txn_fee
        }
        self.mayachain_state.set_network_fees(fees)
        self.mayachain_state.set_btc_tx_rate(btc["tx_rate"])
        self.mayachain_state.set_dash_tx_rate(dash["tx_rate"])
        self.mayachain_state.set_kuji_tx_rate(kuji["tx_rate"])
        self.mayachain_state.set_thor_tx_rate(thor["tx_rate"])
        self.mayachain_state.set_arb_tx_rate(arb["tx_rate"])

    def sim_trigger_tx(self, txn):
        # process transaction in mayachain
        self.set_network_fees()
        if txn.chain == Ethereum.chain or txn.chain == Arbitrum.chain:
            for idx, coin in enumerate(txn.coins):
                txn.coins[idx].amount = int(coin.amount / 1e10)
            for idx, c in enumerate(txn.gas):
                txn.gas[idx].amount = int(c.amount / 1e10)
        outbounds = self.mayachain_state.handle(txn)

        for outbound in outbounds:
            out = deepcopy(outbound)
            # update simulator state with outbound txs
            if out.chain == Ethereum.chain:
                for idx, coin in enumerate(out.coins):
                    out.coins[idx].amount = int(coin.amount * 1e10)
                for idx, c in enumerate(out.gas):
                    out.gas[idx].amount = int(c.amount * 1e10)
                out.fee.amount = out.fee.amount * 1e10
            self.broadcast_simulator(out)

        return outbounds

    def sim_catch_up(self, txn):
        # At this point, we can assume that the transaction on real mayachain
        # has already occurred, and we can now play "catch up" in our simulated
        # mayachain state

        outbounds = []
        processed = False
        processed_events = False
        pending_txs = 0
        block_height = None
        old_events = self.mayachain_state.events
        old_event_count = len(old_events)

        for x in range(0, 200):  # 200 attempts
            events = deepcopy(self.mayachain_client.events)
            sim_events = self.mayachain_state.events
            count_events = len(events) - len(old_events)

            if count_events >= 0 and processed:
                processed_events = True

            new_events = events[-count_events:] if count_events > 0 else []
            for evt_t in new_events:

                # we have more real events than sim, fill in the gaps
                if evt_t.type == "gas":
                    todo = []
                    # with the given gas pool event data, figure out
                    # which outbound txns are for this gas pool, vs
                    # another later on
                    count = 0
                    for out in outbounds:
                        # a gas pool matches a txn if their from
                        # the same blockchain
                        event_chain = Asset(evt_t.get("asset")).get_chain()
                        out_chain = out.coins[0].asset.get_chain()
                        if event_chain == out_chain:
                            todo.append(out)
                            count += 1
                            if count >= int(evt_t.get("transaction_count")):
                                break
                    self.mayachain_state.handle_gas(todo)

                elif evt_t.type == "rewards":
                    self.mayachain_state.handle_rewards()
                    block_height = evt_t.height

                elif evt_t.type == "outbound" and pending_txs > 0:
                    # figure out which outbound event is which tx
                    for out in outbounds:
                        coin_o = out.coins_str()
                        coin_e = evt_t.get("coin")

                        if coin_o == coin_e:
                            self.mayachain_state.generate_outbound_events(txn, [
                                                                          out])
                            pending_txs -= 1
                        elif out.coins[0].asset in coin_e:
                            msg = f"out coins not matching {coin_o} != {coin_e}"
                            logging.error(msg)
                            break

                elif evt_t.type == "scheduled_outbound":
                    for out in outbounds:
                        if out.coins[0].asset != evt_t.get("coin_asset"):
                            continue

                        # this races too much, so we just use event amounts in some cases
                        self.mayachain_state.generate_scheduled_outbound_events(
                            txn,
                            evt_t,
                            out,
                        )

                elif not processed:
                    outbounds = self.sim_trigger_tx(txn)
                    pending_txs = len(outbounds)
                    processed = True
                    continue

            old_events = events
            if len(events) == len(sim_events) and pending_txs <= 0 and processed_events:
                break
            time.sleep(0.3)
        if pending_txs > 0:
            count_events = len(events) - old_event_count
            self.check_events(events[-count_events:],
                              sim_events[-count_events:])
            msg = f"failed to send all outbound transactions {pending_txs}"
            self.error(msg)
        count_events = len(events) - old_event_count
        return (
            outbounds,
            block_height,
            events[-count_events:],
            sim_events[-count_events:],
        )

    def log_result(self, tx, outbounds):
        """
        Log result after a tx was processed
        """
        if len(outbounds) == 0:
            return
        result = "[+]"
        if "REFUND" in outbounds[0].memo:
            result = "[-]"
        for outbound in outbounds:
            logging.info(f"{result} {outbound.short()}")

    def run(self):
        for i, txn in enumerate(self.txns):
            txn = Transaction.from_data(txn)

            # only process adds from provider one if running in bootstrap only mode
            if (
                self.bootstrap_only
                and not (txn.memo == "SEED" and txn.to_address == "PROVIDER-1")
                and not (
                    txn.from_address == "PROVIDER-1"
                    and txn.memo.startswith("ADD")
                    and txn.to_address == "VAULT"
                    and (txn.chain != "MAYA" or txn.coins[0].asset == "MAYA.CACAO")
                )
            ):
                continue

            if self.bitcoin_reorg:
                # get block hash from bitcoin we are going to invalidate later
                if i == 14 or i == 24:
                    current_height = self.mock_bitcoin.get_block_height()
                    block_hash = self.mock_bitcoin.get_block_hash(
                        current_height)
                    logging.info(
                        f"Block to invalidate {current_height} {block_hash}")

                # now we processed some btc txs and we invalidate an older block
                # to make those txs not valid anymore and test mayanode reaction
                if i == 18 or i == 28:
                    self.mock_bitcoin.invalidate_block(block_hash)
                    logging.info("Reorg triggered")

            if self.ethereum_reorg:
                # get block hash from ethereum we are going to invalidate later
                if i == 14 or i == 24:
                    current_height = self.mock_ethereum.get_block_height()
                    block_hash = self.mock_ethereum.get_block_hash(
                        current_height)
                    logging.info(
                        f"Block to invalidate {current_height} {block_hash}")

                # now we processed some eth txs and we invalidate an older block
                # to make those txs not valid anymore and test mayanode reaction
                if i == 18 or i == 28:
                    self.mock_ethereum.set_block(current_height)
                    logging.info("Reorg triggered")

            if self.arbitrum_reorg:
                # get block hash from arbitrum we are going to invalidate later
                if i == 14 or i == 24:
                    current_height = self.mock_arbitrum.get_block_height()
                    block_hash = self.mock_arbitrum.get_block_hash(
                        current_height)
                    logging.info(
                        f"Block to invalidate {current_height} {block_hash}")

                # now we processed some arb txs and we invalidate an older block
                # to make those txs not valid anymore and test mayanode reaction
                if i == 18 or i == 28:
                    self.mock_arbitrum.set_block(current_height)
                    logging.info("Reorg triggered")

            logging.info(f"{i:2} {txn}")

            self.broadcast_chain(txn)
            self.broadcast_simulator(txn)

            if txn.memo == "SEED":
                continue
            elif txn.memo == "ADD":
                self.readyToCheckHealth = True

            outbounds, block_height, events, sim_events = self.sim_catch_up(txn)

            # check if we are verifying the results
            if self.no_verify:
                continue

            self.check_events(events, sim_events)
            self.check_pools()

            self.check_binance()
            self.check_cosmos(self.kuji, self.mock_kuji)
            self.check_thorchain(self.thor, self.mock_thor)
            self.check_chain(self.bitcoin, self.mock_bitcoin,
                             self.bitcoin_reorg)
            self.check_chain(self.dash, self.mock_dash, self.bitcoin_reorg)
            self.check_ethereum()
            self.check_chain(self.radix, self.mock_radix, False)
            self.check_arbitrum()

            if CACAO.get_chain() == "MAYA":
                self.check_chain(self.mayachain, self.mock_mayachain, None)

            if self.readyToCheckHealth:
                self.run_health()

            self.log_result(txn, outbounds)


if __name__ == "__main__":
    main()
