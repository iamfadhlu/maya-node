import base64
import logging
import threading

import websocket
import json
from copy import deepcopy

from utils.common import (
    Transaction,
    Coin,
    Asset,
    get_share,
    HttpClient,
    Jsonable,
    get_cacao_asset,
    round_bankers,
)

from chains.aliases import get_alias, get_alias_address, get_aliases
from chains.bitcoin import Bitcoin
from chains.dash import Dash
from chains.kuji import Kuji
from chains.thorchain import Thorchain
from chains.ethereum import Ethereum
from chains.arbitrum import Arbitrum
from chains.binance import Binance
from chains.radix import Radix
from chains.mayachain import Mayachain
from tenacity import retry, stop_after_delay, wait_fixed

CACAO = get_cacao_asset()
INIT_BTC_CACAO_BALANCE = 10000000000000
INIT_BTC_ASSET_BALANCE = 100000000
INIT_BTC_LP_UNITS = 10000000000000

SUBSCRIBE_BLOCK = {
    "jsonrpc": "2.0",
    "id": 0,
    "method": "subscribe",
    "params": {"query": "tm.event='NewBlock'"},
}


class MayachainClient(HttpClient):
    """
    A client implementation to mayachain API
    """

    def __init__(self, api_url, enable_websocket=False):
        super().__init__(api_url)

        self.wait_for_node()
        self.bifrost = HttpClient(self.get_bifrost_url())
        self.wait_for_bifrost()
        self.rpc = HttpClient(self.get_rpc_url())

        if enable_websocket:
            self.ws = websocket.WebSocketApp(
                self.get_ws_url(),
                on_open=self.ws_open,
                on_error=self.ws_error,
                on_message=self.ws_message,
            )
            self.events = []
            threading.Thread(target=self.ws.run_forever, daemon=True).start()

    def get_ws_url(self):
        url = self.get_rpc_url()
        url = url.replace("http", "ws")
        return f"{url}/websocket"

    def get_rpc_url(self):
        url = self.base_url.replace("1317", "26657")
        return url

    def get_bifrost_url(self):
        url = self.base_url.replace("1317", "6040")
        return url

    @retry(stop=stop_after_delay(120), wait=wait_fixed(1))
    def wait_for_node(self):
        current_height = self.get_block_height()
        if current_height < 1:
            logging.warning("Mayachain starting, waiting")
            raise Exception

    @retry(stop=stop_after_delay(120), wait=wait_fixed(1))
    def wait_for_bifrost(self):
        p2pid = self.get_bifrost_p2pid()
        if len(p2pid) <= 0:
            logging.warning("Bifrost starting, waiting")
            raise Exception

    def ws_open(self):
        """
        Websocket connection open, subscribe to events
        """
        self.ws.send(json.dumps(SUBSCRIBE_BLOCK))

    def ws_message(self, msg):
        """
        Websocket message handler
        """
        msg = json.loads(msg)
        if "data" not in msg["result"]:
            return
        value = msg["result"]["data"]["value"]
        block_height = value["block"]["header"]["height"]
        result = self.get_events(block_height)["result"]
        if result["txs_results"]:
            for tx in result["txs_results"]:
                self.process_events(tx["events"], block_height)
        if result["end_block_events"]:
            self.process_events(result["end_block_events"], block_height)

    def process_events(self, events, block_height):
        for event in events:
            if event["type"] in [
                "message",
                "transfer",
                "coin_spent",
                "coin_received",
                "tx",
                "coinbase",
                "burn",
                "tss_keysign",
            ]:
                continue
            if event["type"] == "rewards" and event["attributes"][0]["value"] == 'MA==':
                continue
            self.decode_event(event)
            event = Event(event["type"], event["attributes"], block_height)
            self.events.append(event)

    def decode_event(self, event):
        attributes = []
        for attr in event["attributes"]:
            key = base64.b64decode(attr["key"]).decode("utf-8")
            if attr["value"]:
                value = base64.b64decode(attr["value"]).decode("utf-8")
            else:
                value = ""
            attributes.append({key: value})
        event["attributes"] = attributes

    def ws_error(self, error):
        """
        Websocket error handler
        """
        logging.error(error)
        raise Exception("mayachain websocket error")

    def get_block_height(self):
        """
        Get the current block height of mayachain
        """
        data = self.fetch("/mayachain/lastblock")
        if data is None:
            return 0
        return int(data[0]["mayachain"])

    def get_vault_address(self, chain):
        data = self.fetch("/mayachain/inbound_addresses")
        for d in data:
            if chain == d["chain"]:
                return d["address"]
        return "address not found"

    def get_vault_pubkey(self):
        data = self.fetch("/mayachain/inbound_addresses")
        return data[0]["pub_key"]

    def get_vault_data(self, height=None):
        url = "/mayachain/network"
        if height:
            url = f"/mayachain/network?height={height}"
        return self.fetch(url)

    def get_asgard_vaults(self):
        return self.fetch("/mayachain/vaults/asgard")

    def get_yggdrasil_vaults(self):
        return self.fetch("/mayachain/vaults/yggdrasil")

    def get_pools(self):
        return self.fetch("/mayachain/pools")

    def get_router_address(self, chain):
        data = self.fetch("/mayachain/vaults/pubkeys")
        routers = data["asgard"][0]["routers"]
        for router in routers:
          if router["chain"] == chain:
            return router["router"]
        raise Exception("router not found for chain " + chain)

    def get_pool(self, asset):
        for p in self.get_pools():
            if p["asset"] == asset:
                return p
        return None

    def get_events(self, block_height):
        return self.rpc.fetch(f"/block_results?height={block_height}")

    def get_bifrost_p2pid(self):
        return self.bifrost.fetch_plain("/p2pid")


class MayachainState:
    """
    A complete implementation of the mayachain logic/behavior
    """

    cacao_fee = 2000000000
    synth_multiplier = 2
    target_surplus = 10_000_00000000  # target outbound fee surplus

    def __init__(self):
        self.pools = []
        self.events = []
        self.reserve = 0
        self.liquidity = {}
        self.total_bonded = 0
        self.bond_reward = 0
        self.vault_pubkey = None
        self.network_fees = {}
        self.gas_spent_cacao = 0
        self.gas_withheld_cacao = 0
        self.btc_estimate_size = 188
        self.thor_estimate_size = 1
        self.dash_estimate_size = 180
        self.arb_estimate_size = 5000000
        self.gaia_estimate_size = 1
        self.kuji_estimate_size = 1
        self.btc_tx_rate = 0
        self.dash_tx_rate = 0
        self.gaia_tx_rate = 0
        self.kuji_tx_rate = 0
        self.thor_tx_rate = 0
        self.arb_tx_rate = 0

    def set_btc_tx_rate(self, tx_rate):
        """
        Set median BTC tx rate , used to calculate gas
        """
        self.btc_tx_rate = tx_rate


    def set_dash_tx_rate(self, tx_rate):
        """
        Set median DASH tx rate , used to calculate gas
        """
        self.dash_tx_rate = tx_rate

    def set_kuji_tx_rate(self, tx_rate):
        """
        Set median KUJI tx rate , used to calculate gas
        """
        self.kuji_tx_rate = tx_rate

    def set_thor_tx_rate(self, tx_rate):
        """
        Set median THOR tx rate , used to calculate gas
        """
        self.thor_tx_rate = tx_rate

    def set_arb_tx_rate(self, tx_rate):
        """
        Set median ARB tx rate , used to calculate gas
        """
        self.arb_tx_rate = tx_rate

    def set_vault_pubkey(self, pubkey):
        """
        Set vault pubkey bech32 encoded, used to generate hashes
        to order broadcast of outbound transactions.
        """
        self.vault_pubkey = pubkey

    def set_network_fees(self, fees):
        """
        Set network fees used to calculate dynamic fees per chain
        """
        self.network_fees = fees

    def get_pool(self, asset):
        """
        Fetch a specific pool by asset
        """
        asset = Asset(asset).get_layer1_asset()
        for pool in self.pools:
            if pool.asset == asset:
                return pool

        return Pool(asset)

    def set_pool(self, pool):
        """
        Set a pool
        """
        for i, p in enumerate(self.pools):
            if p.asset == pool.asset:
                if (
                    pool.asset_balance == 0 or pool.cacao_balance == 0
                ) and pool.status == "Available" and pool.asset != "BTC.BTC":
                    pool.status = "Staged"

                    # Generate pool event with new status
                    event = Event(
                        "pool", [{"pool": pool.asset}, {
                            "pool_status": pool.status}]
                    )
                    self.events.append(event)

                self.pools[i] = pool
                return

        self.pools.append(pool)

    def handle_gas(self, txs):
        """
        Subtracts gas from pool

        :param list Transaction: list outbound transaction updated with gas

        """
        gas_coins = {}
        gas_coin_count = {}
        for tx in txs:
            if not tx.gas:
                continue
            gases = tx.gas
            if (
                tx.gas[0].asset.is_btc()
                or tx.gas[0].asset.is_dash()
            ):
                gases = tx.max_gas

            for gas in gases:
                if gas.asset not in gas_coins:
                    gas_coins[gas.asset] = Coin(gas.asset)
                    gas_coin_count[gas.asset] = 0
                gas_coins[gas.asset].amount += gas.amount
                gas_coin_count[gas.asset] += 1

        if not len(gas_coins.items()):
            return

        for asset, gas in gas_coins.items():
            pool = self.get_pool(gas.asset)
            # figure out how much cacao is an equal amount to gas.amount
            cacao_amt = pool.get_asset_in_cacao(gas.amount)
            self.reserve -= cacao_amt  # take cacao from the reserve

            # only append gas spent if it's not a native MAYAChain asset
            if not asset.is_maya():
                self.gas_spent_cacao += cacao_amt

            pool.add(cacao_amt, 0)  # replenish gas costs with cacao
            pool.sub(0, gas.amount)  # subtract gas from pool
            self.set_pool(pool)
            # add gas event
            event = Event(
                "gas",
                [
                    {"asset": asset},
                    {"asset_amt": gas.amount},
                    {"cacao_amt": cacao_amt},
                    {"transaction_count": gas_coin_count[asset]},
                ],
            )
            self.events.append(event)

    def get_gas_asset(self, chain):
        if chain == "MAYA":
            return Mayachain.coin
        if chain == "BNB":
            return Binance.coin
        if chain == "BTC":
            return Bitcoin.coin
        if chain == "DASH":
            return Dash.coin
        if chain == "KUJI":
            return Kuji.coin
        if chain == "ETH":
            return Ethereum.coin
        if chain == "ARB":
            return Arbitrum.coin
        if chain == "THOR":
            return Thorchain.coin
        if chain == "XRD":
            return Radix.coin
        return None

    def get_gas(self, chain, tx):
        if chain == "ETH":
            return Ethereum._calculate_gas(None, tx)
        elif chain == "ARB":
            return Arbitrum._calculate_gas(None, tx)
        elif chain == "XRD":
            return Radix._calculate_gas(None, tx)
        return self.get_max_gas(chain)

    def get_max_gas(self, chain):
        if chain == "MAYA":
            return Coin(CACAO, self.cacao_fee)
        gas_asset = self.get_gas_asset(chain)
        if chain == "ARB":
            amount = int(self.arb_tx_rate * 3 / 2) * self.arb_estimate_size
        if chain == "BTC":
            amount = int(self.btc_tx_rate * 3 / 2) * self.btc_estimate_size
        if chain == "THOR":
            amount = int(self.thor_tx_rate) * self.thor_estimate_size
        if chain == "DASH":
            amount = int(self.dash_tx_rate * 3 / 2) * self.dash_estimate_size
        if chain == "KUJI":
            amount = int(self.kuji_tx_rate * 3 / 2) * self.kuji_estimate_size
            amount = int(amount / 100) * 100  # round KUJI to 6 digits max
        if chain == "XRD":
            amount = int(self.network_fees["XRD"] * 3/2)
        if chain == "BNB":
            amount = self.network_fees["BNB"]
        return Coin(gas_asset, amount)

    def _calc_outbound_fee_multiplier(self):
        min_multiplier = 15_000
        max_multiplier = 20_000
        surplus = self.gas_withheld_cacao - self.gas_spent_cacao
        if surplus <= 0:
            return max_multiplier
        elif surplus >= self.target_surplus:
            return min_multiplier
        else:
            m_diff = max_multiplier - min_multiplier
            m_reduced = get_share(surplus, self.target_surplus, m_diff)
            return max_multiplier - m_reduced

    def get_cacao_fee(self, chain):
        if chain not in self.network_fees:
            return self.cacao_fee
        chain_fee = self.network_fees[chain]
        if chain_fee == 0:
            return self.cacao_fee
        gas_asset = self.get_gas_asset(chain)
        pool = self.get_pool(gas_asset)
        if pool.asset_balance == 0 or pool.cacao_balance == 0:
            return self.cacao_fee
        multiplier_bps = self._calc_outbound_fee_multiplier()
        # convert from 1e11 to 1e8
        if chain == "ARB":
            chain_fee = int(chain_fee / 1000)
        chain_fee = get_share(chain_fee, 10_000, multiplier_bps)
        if chain == "KUJI":
            chain_fee = int(chain_fee / 100) * 100
        return pool.get_asset_in_cacao(chain_fee)

    def get_asset_fee(self, chain):
        if chain in self.network_fees:
            multiplier_bps = self._calc_outbound_fee_multiplier()
            net_fee = self.network_fees[chain]
            asset_fee = get_share(net_fee, 10_000,
                                  multiplier_bps)
            if chain == "KUJI":
                asset_fee = int(asset_fee / 100) * 100
            if chain == "THOR":
                asset_fee = int(asset_fee)
            if chain == "ARB":
                asset_fee = int(asset_fee / 1000)
            return asset_fee
        gas_asset = self.get_gas_asset(chain)
        pool = self.get_pool(gas_asset)
        return pool.get_cacao_in_asset(self.cacao_fee)

    def handle_fee(self, in_tx, txs):
        """
        Subtract transaction fee from given transactions
        using dynamic fees calculated from averages on chains
        """
        outbounds = []
        if not isinstance(txs, list):
            txs = [txs]

        for tx in txs:
            # fee amount in cacao value
            cacao_fee = self.get_cacao_fee(tx.chain)
            if not tx.gas:
                tx.gas = [self.get_gas(tx.chain, in_tx)]

            for coin in tx.coins:
                if coin.is_cacao():
                    if coin.amount <= cacao_fee:
                        cacao_fee = coin.amount
                    coin.amount -= cacao_fee
                    if coin.amount > 0:
                      outbounds.append(tx)

                      # only do the fee logic state changes if there's an outbound
                      if cacao_fee > 0:
                          # add fee event
                          event = Event(
                              "fee",
                              [
                                  {"tx_id": in_tx.id},
                                  {"coins": f"{cacao_fee} {coin.asset}"},
                                  {"pool_deduct": 0},
                              ],
                          )
                          self.events.append(event)
                          tx.fee = Coin(coin.asset, cacao_fee)

                else:
                    pool = self.get_pool(coin.asset)
                    asset_fee = 0
                    if pool.status == "Staged":
                        cacao_fee = 0
                    else:
                        if coin.asset == self.get_gas_asset(tx.chain):
                            asset_fee = self.get_asset_fee(tx.chain)
                        else:
                            asset_fee = pool.get_cacao_in_asset(cacao_fee)
                        if coin.amount <= asset_fee:
                            asset_fee = coin.amount
                        cacao_fee = pool.get_cacao_disbursement_for_asset_add(
                            asset_fee)
                        if cacao_fee > pool.cacao_balance:
                            cacao_fee = pool.cacao_balance
                        pool.sub(cacao_fee, 0)
                        if coin.asset.is_synth:
                            pool.synth_balance -= asset_fee
                        else:
                            pool.add(0, asset_fee)
                        self.set_pool(pool)

                        coin.amount -= asset_fee
                        if coin.asset.is_btc() and not coin.asset.is_synth_asset() and not asset_fee == 0:
                            tx.max_gas = [Coin(coin.asset, int(
                                self.network_fees["BTC"] * 3/2))]
                            btc_max_gas = self.get_max_gas("BTC")
                            gap = tx.max_gas[0].amount - btc_max_gas.amount
                            if gap > 0:
                                coin.amount += gap
                            else:
                                tx.gas = tx.max_gas


                        if coin.asset.is_dash() and not coin.asset.is_synth_asset() and not asset_fee == 0:
                            tx.max_gas = [Coin(coin.asset, int(
                                self.network_fees["DASH"] * 3/2))]
                            dash_max_gas = self.get_max_gas("DASH")
                            gap = tx.max_gas[0].amount - dash_max_gas.amount
                            if gap > 0:
                                coin.amount += gap
                            else:
                                tx.gas = tx.max_gas

                        if coin.asset.is_kuji() and not coin.asset.is_synth_asset() and not asset_fee == 0:
                            asset_fee = int(asset_fee / 100) * 100
                            tx.max_gas = [Coin(self.get_gas_asset(Kuji.chain), int(
                                self.network_fees["KUJI"] * 3/2))]

                            tx.max_gas[0].amount = int(
                                tx.max_gas[0].amount / 100) * 100
                            gap = int(asset_fee / 2) - self.kuji_estimate_size * int(
                                self.kuji_tx_rate * 3 / 2
                            )
                            gap = int(gap / 100) * 100

                        if coin.asset.is_xrd() and not coin.asset.is_synth_asset() and not asset_fee == 0:
                            max_fee = self.network_fees["XRD"] * 3/2
                            if coin.asset.get_symbol() == "XRD":
                              asset_fee = int(asset_fee / 100) * 100
                            tx.max_gas = [Coin(self.get_gas_asset(Radix.chain), max_fee)]
                            gap = int(asset_fee / 2) - max_fee
                            gap = int(gap / 100) * 100

                        if coin.asset.is_thor() and not coin.asset.is_synth_asset() and not asset_fee == 0:
                            asset_fee = int(asset_fee)
                            tx.max_gas = [Coin(coin.asset, int(self.network_fees["THOR"]))]
                            gap = int(asset_fee / 2) - self.thor_estimate_size * int(
                                self.thor_tx_rate)
                            gap = int(gap / 100) * 100
                            if gap > 0:
                                coin.amount += gap
                            else:
                                tx.gas = tx.max_gas

                        if coin.asset.get_chain() == "ETH" and not asset_fee == 0:
                            if coin.asset.is_eth():
                                tx.max_gas = [Coin(coin.asset, int(
                                    self.network_fees["ETH"] * 3/2))]

                            elif coin.asset.is_erc():
                                fee_in_gas_asset = self.get_asset_fee(tx.chain)
                                gas_asset = self.get_gas_asset("ETH")
                                tx.max_gas = [
                                    Coin(gas_asset, int(fee_in_gas_asset / 2))
                                ]

                        if coin.asset.get_chain() == "ARB" and not asset_fee == 0:
                            if coin.asset.is_eth():
                                tx.max_gas = [Coin(coin.asset, int(
                                    self.network_fees["ARB"] * 3/2))]

                            elif coin.asset.is_erc():
                                fee_in_gas_asset = self.get_asset_fee(tx.chain)
                                gas_asset = self.get_gas_asset("ARB")
                                tx.max_gas = [
                                    Coin(gas_asset, int(fee_in_gas_asset / 2))
                                ]

                        if (cacao_fee > 0 or asset_fee > 0):
                            # add fee event
                            self.events.append(Event(
                                "fee",
                                [
                                    {"tx_id": in_tx.id},
                                    {"coins": f"{asset_fee} {coin.asset}"},
                                    {"pool_deduct": cacao_fee},
                                ],
                            ))
                            # TODO: Uncomment after implementing mint event
                            # # Note that currently this lacks a check for Derived status.
                            # if coin.asset.is_synth and asset_fee > 0:
                            #     self.events.append(Event(
                            #         "mint_burn",
                            #         [
                            #             {"supply": "burn"},
                            #             {"denom": f"{tx.coins[0].asset.upper()}"},
                            #             {"amount": f"{asset_fee}"},
                            #             {"reason": "burn_native_fee"},
                            #         ],
                            #     ))

                    if coin.amount > 0:
                        tx.fee = Coin(coin.asset, asset_fee)
                        outbounds.append(tx)

            # add to the reserve / withheld gas only if there are outbounds
            if len(outbounds) > 0:
                self.reserve += cacao_fee
                # only add to surplus if it's an external L1 outbound
                if not coin.asset.is_maya():
                    self.gas_withheld_cacao += cacao_fee
        return outbounds

    def _total_liquidity(self):
        """
        Total up the liquidity fees from all pools
        """
        total = 0
        for value in self.liquidity.values():
            total += value
        return total

    def _total_bonded(self):
        total_bonded = 0
        for pool in self.pools:
            if pool.asset.is_btc():
                if pool.lp_units != 0:
                    total_bonded = round_bankers(
                        pool.cacao_balance * 10000000000 / pool.lp_units, 0.5)
                    if pool.asset_balance > 0:
                        total_bonded = 2 * total_bonded
                else:
                    total_bonded = 0
            # TODO:  Rather than the hardcoded starting BTC.BTC pool node units, build them into the genesis?
            # Note, bond providers with false 'Bonded' are not counted.
            # Also note that this total_bonded may not yet fully match the Go totalBonded.
        return total_bonded

    def handle_rewards(self):
        """
        Calculate block rewards
        """
        if self.reserve == 0:
            return

        total_bonded = self._total_bonded()

        # get the total provided liquidity
        # TODO: skip non-available pools
        total_provided_liquidity = 0
        for pool in self.pools:
            total_provided_liquidity += pool.cacao_balance

        if total_provided_liquidity == 0:  # nothing provided liquidity, no rewards
            return

        xD = total_bonded / total_provided_liquidity
        part = float(1000)
        total = float(10000)
        allocation = float(self._total_liquidity())
        ten_perc_liquidity = allocation/(total/part)
        ten_perc_liquidity = round_bankers(ten_perc_liquidity, 0.5)
        rewards_liquidity = self._total_liquidity() - int(ten_perc_liquidity * 2)
        if xD <= 0.75:
            bond_reward = rewards_liquidity
        else:
            yD = 1 / xD
            partD = yD * 4
            bond_reward = partD * rewards_liquidity

        # calculate if we need to move liquidity from the pools to the bonders,
        # or move bond rewards to the pools
        pool_reward = 0
        lp_deficit = 0
        lp_split = rewards_liquidity - bond_reward
        if lp_split >= self._total_liquidity():
            pool_reward = lp_split - self._total_liquidity()
        else:
            lp_deficit = self._total_liquidity() - lp_split

        if self.reserve < bond_reward + pool_reward:
            return

        # subtract our rewards from the reserve
        self.reserve -= bond_reward + pool_reward
        self.bond_reward += bond_reward  # add to bond reward pool

        # Generate rewards event
        reward_event = Event("rewards", [{"bond_reward": bond_reward}])

        if pool_reward > 0:
            # TODO: subtract any remaining gas, from the pool rewards
            if self._total_liquidity() > 0:
                for key, value in self.liquidity.items():
                    share = get_share(
                        value, self._total_liquidity(), pool_reward)
                    pool = self.get_pool(key)
                    pool.cacao_balance += share
                    self.set_pool(pool)

                    # Append pool reward to event
                    reward_event.attributes.append({pool.asset: str(share)})
            else:
                pass  # TODO: Pool Rewards are based on Depth Share
        else:
            for key, value in self.liquidity.items():
                share = get_share(lp_deficit, self._total_liquidity(), value)
                pool = self.get_pool(key)
                if share == 0:
                    continue
                pool.cacao_balance -= share
                self.reserve += share
                self.set_pool(pool)

                # Append pool reward to event
                reward_event.attributes.append({pool.asset: str(-share)})

        # generate event REWARDS
        self.events.append(reward_event)

        # clear summed liquidity fees
        self.liquidity = {}

    def refund(self, tx, code, reason):
        """
        Returns a list of refund transactions based on given tx
        """
        out_txs = []
        for coin in tx.coins:
            # check we have gas liquidity
            chain = coin.asset.get_chain()
            if chain != CACAO.get_chain():
                gas_asset = self.get_gas_asset(coin.asset.get_chain())
                pool = self.get_pool(gas_asset)
                if pool.cacao_balance == 0:
                    continue

            # check if refund against empty pool cacao balance
            # we swallow the tx cause we wont be able to figure out fee
            pool = self.get_pool(coin.asset)
            if not coin.is_cacao() and pool.cacao_balance == 0:
                continue

            out_txs.append(
                Transaction(
                    tx.chain,
                    tx.to_address,
                    tx.from_address,
                    [coin],
                    f"REFUND:{tx.id}",
                )
            )

        in_tx = deepcopy(tx)  # copy of transaction

        out_txs = self.handle_fee(tx, out_txs)

        if len(out_txs) == 0:
            reason = f"{reason}; fail to refund ({in_tx.coins[0].amount} {in_tx.coins[0].asset.upper()}): not enough asset to pay for fees"

        # generate event REFUND for the transaction
        event = Event(
            "refund",
            [{"code": code}, {"reason": reason}, *in_tx.get_attributes()],
        )

        self.events.append(event)
        if len(out_txs) == 0:
          # Since no refund txout, burn all synths and send RUNE to the Reserve
          for coin in in_tx.coins:
              if coin.asset.is_synth:
                  self.events.append(
                      Event(
                          "reserve",
                          [
                              {"contributor_address": in_tx.from_address},
                              {"amount": f"{coin.amount}"},
                              *in_tx.get_attributes(),
                          ],
                      )
                  )
              if coin.is_cacao():
                  self.reserve += coin.amount

        return out_txs

    def generate_scheduled_outbound_events(self, in_tx, evt, outbound):
        """
        Generate scheduled outbound events for txs
        """
        event = Event(
            "scheduled_outbound",
            [
                {"chain": outbound.chain},
                {"to_address": outbound.to_address},
                {"vault_pub_key": self.vault_pubkey},
                {"coin_asset": outbound.coins[0].asset},
                {"coin_amount": evt.get("coin_amount")},
                {"coin_decimals": "0"},
                {"memo": outbound.memo},
                {"gas_rate": evt.get("gas_rate")},
                {"in_hash": in_tx.id},
                {"out_hash": ""},
                {"module_name": ""},
                {"max_gas_asset_0": outbound.gas[0].asset},
                {"max_gas_amount_0": evt.get("max_gas_amount_0")},
                {"max_gas_decimals_0": evt.get("max_gas_decimals_0")},
            ],
        )
        self.events.append(event)

    def generate_outbound_events(self, in_tx, txs):
        """
        Generate outbound events for txs
        """
        for tx in txs:
            event = Event(
                "outbound", [{"in_tx_id": in_tx.id}, *tx.get_attributes()])
            self.events.append(event)

    def order_outbound_txs(self, txs):
        """
        Sort txs by tx custom hash function to replicate real mayachain order
        """
        if txs:
            txs.sort(key=lambda tx: tx.custom_hash(self.vault_pubkey))

    def handle(self, tx):
        """
        This is a router that sends a transaction to the correct handler.
        It will return transactions to send

        :param tx: tx IN
        :returns: txs OUT

        """
        tx = deepcopy(tx)  # copy of transaction
        out_txs = []

        if tx.chain == "MAYA":
            self.reserve += self.cacao_fee
        if tx.memo.startswith("ADD:"):
            out_txs = self.handle_add_liquidity(tx)
        elif tx.memo.startswith("DONATE:"):
            out_txs = self.handle_donate(tx)
        elif tx.memo.startswith("WITHDRAW:"):
            out_txs = self.handle_withdraw(tx)
        elif tx.memo.startswith("SWAP:"):
            out_txs = self.handle_swap(tx)
        elif tx.memo.startswith("RESERVE"):
            out_txs = self.handle_reserve(tx)
        else:
            if tx.memo == "":
                out_txs = self.refund(tx, 105, "memo can't be empty")
            else:
                out_txs = self.refund(tx, 105, f"invalid tx type: {tx.memo}")
        self.order_outbound_txs(out_txs)
        return out_txs

    def handle_reserve(self, tx):
        """
        Add cacao to the reserve
        MEMO: RESERVE
        """
        amount = 0
        for coin in tx.coins:
            if coin.is_cacao():
                self.reserve += coin.amount
                amount += coin.amount

        # generate event for RESERVE transaction
        event = Event(
            "reserve",
            [
                {"contributor_address": tx.from_address},
                {"amount": amount},
                *tx.get_attributes(),
            ],
        )
        self.events.append(event)

        return []

    def handle_donate(self, tx):
        """
        Add assets to a pool
        MEMO: DONATE:<asset(req)>
        """
        # parse memo
        parts = tx.memo.split(":")
        if len(parts) < 2:
            if tx.memo == "":
                return self.refund(tx, 105, "memo can't be empty")
            return self.refund(tx, 105, f"invalid tx type: {tx.memo}")

        asset = Asset(parts[1])

        # check that we have one cacao and one asset
        if len(tx.coins) > 2:
            # FIXME real world message
            return self.refund(tx, 105, "refund reason message")

        for coin in tx.coins:
            if not coin.is_cacao():
                if not asset == coin.asset:
                    # mismatch coin asset and memo
                    return self.refund(tx, 105, "Invalid symbol")

        pool = self.get_pool(asset)
        for coin in tx.coins:
            if coin.is_cacao():
                pool.add(coin.amount, 0)
            else:
                pool.add(0, coin.amount)
        self.set_pool(pool)

        # generate event for ADD transaction
        event = Event("donate", [{"pool": pool.asset}, *tx.get_attributes()])
        self.events.append(event)

        return []

    def handle_add_liquidity(self, tx):
        """
        handles a liquidity provision transaction
        MEMO: ADD:<asset(req)>
        """
        # parse memo
        parts = tx.memo.split(":")
        if len(parts) < 2:
            if tx.memo == "":
                return self.refund(tx, 105, "memo can't be empty")
            return self.refund(tx, 105, f"invalid tx type: {tx.memo}")

        # empty asset
        if parts[1] == "":
            return self.refund(tx, 105, "Invalid symbol")

        asset = Asset(parts[1])

        # cant have cacao memo
        if asset.is_cacao():
            return self.refund(tx, 105, "asset cannot be cacao: unknown request")

        # cant have synth asset
        if asset.is_synth:
            return self.refund(tx, 1, "fail to validate add liquidity")

        # check that we have one cacao and one asset
        if len(tx.coins) > 2:
            # FIXME real world message
            return self.refund(tx, 105, "refund reason message")

        # check for mismatch coin asset and memo
        if len(tx.coins) == 2:
            for coin in tx.coins:
                if not coin.is_cacao():
                    if not asset == coin.asset:
                        return self.refund(
                            tx, 105, "did not find both coins: unknown request"
                        )

        pool = self.get_pool(asset)

        orig_cacao_amt = 0
        orig_asset_amt = 0
        for coin in tx.coins:
            if coin.is_cacao():
                orig_cacao_amt = coin.amount
            else:
                orig_asset_amt = coin.amount

        # check address to provider to from memo
        if tx.chain == CACAO.get_chain():
            cacao_address = tx.from_address
            asset_address = None
        else:
            cacao_address = None
            asset_address = tx.from_address
        if len(parts) > 2:
            if tx.chain != CACAO.get_chain():
                cacao_address = parts[2]
            else:
                asset_address = parts[2]

        fetch_address = asset_address
        if cacao_address != "":
            fetch_address = cacao_address
        lp = pool.get_liquidity_provider(fetch_address)
        if lp.units == 0 and lp.pending_tx is None:
            if lp.cacao_address is None:
                lp.cacao_address = cacao_address
            if lp.asset_address is None:
                lp.asset_address = asset_address
            pool.set_liquidity_provider(lp)

        if asset_address is not None and not lp.asset_address == asset_address:
            return self.refund(
                tx,
                100,
                "mismatch of asset address",
            )

        liquidity_units, cacao_amt, asset_amt, pending_txid = pool.add_liquidity(
            cacao_address, asset_address, orig_cacao_amt, orig_asset_amt, asset, tx.id
        )
        self.set_pool(pool)

        # liquidity provision cross chain so event will be dispatched on asset
        # liquidity provision
        if liquidity_units == 0:
            # generate event for liquidity provision transaction
            event = Event(
                "pending_liquidity",
                [
                    {"pool": pool.asset},
                    {"type": "add"},
                    {"cacao_address": cacao_address or ""},
                    {"cacao_amount": orig_cacao_amt},
                    {"asset_amount": orig_asset_amt},
                    {"asset_address": asset_address or ""},
                    {f"{tx.chain}_txid": tx.id},
                ],
            )
            self.events.append(event)
            return []

        if pool.lp_units > 0 and len(pool.liquidity_providers) == 1 and pool.asset != "BTC.BTC":
            self.events.append(
                Event("pool", [{"pool": pool.asset},
                      {"pool_status": "Available"}])
            )
        # generate event for liquidity provision transaction
        event = Event(
            "add_liquidity",
            [
                {"pool": pool.asset},
                {"liquidity_provider_units": liquidity_units},
                {"cacao_address": cacao_address or ""},
                {"cacao_amount": cacao_amt},
                {"asset_amount": asset_amt},
                {"asset_address": asset_address or ""},
                {f"{tx.chain}_txid": tx.id},
            ],
        )
        if pending_txid:
            if tx.chain == CACAO.get_chain():
                event.attributes.append(
                    {f"{pool.asset.get_chain()}_txid": pending_txid or ""}
                )
            else:
                event.attributes.append(
                    {f"{CACAO.get_chain()}_txid": pending_txid or ""}
                )
        self.events.append(event)

        return []

    def handle_withdraw(self, tx):
        """
        handles a withdrawing transaction
        MEMO: WITHDRAW:<asset(req)>:<address(op)>:<basis_points(op)>
        """
        withdraw_basis_points = 10000

        # parse memo
        parts = tx.memo.split(":")
        if len(parts) < 2:
            if tx.memo == "":
                return self.refund(tx, 105, "memo can't be empty")
            return self.refund(tx, 105, f"invalid tx type: {tx.memo}")

        # get withdrawal basis points, if it exists in the memo
        if len(parts) >= 3:
            withdraw_basis_points = int(parts[2])

        # empty asset
        if parts[1] == "":
            return self.refund(tx, 105, "Invalid symbol")

        asset = Asset(parts[1])

        # add any cacao to the reserve
        for coin in tx.coins:
            if coin.asset.is_cacao():
                self.reserve += coin.amount
            else:
                coin.amount = 0

        pool = self.get_pool(asset)
        lp = pool.get_liquidity_provider(tx.from_address)
        if lp.is_zero():
            # FIXME real world message
            return self.refund(tx, 105, "refund reason message")

        chain = asset.get_chain()
        # calculate gas prior to update pool in case we empty the pool
        # and need to subtract
        gas = self.get_gas(chain, tx)
        # get the fee that are supposed to be charged, this will only be
        # used if it is the last withdraw
        if asset == self.get_gas_asset(chain):
            dynamic_fee = int(self.network_fees[chain] * 3/2)
        else:
            dynamic_fee = int(
                round(pool.get_cacao_in_asset(self.get_cacao_fee(chain)))
            )
        tx_cacao_gas = self.get_gas(CACAO.get_chain(), tx)
        withdraw_units, cacao_amt, asset_amt = pool.withdraw(
            tx.from_address, withdraw_basis_points
        )
        cacao_amt += lp.pending_cacao

        # if this is our last liquidity provider of bnb, subtract a little BNB for gas.
        emit_asset = asset_amt
        outbound_asset_amt = asset_amt

        self.btc_estimate_size = 255
        self.dash_estimate_size = 278

        if (pool.lp_units == 0 and pool.synth_units() == 0) or (pool.asset.is_btc() and pool.lp_units == INIT_BTC_LP_UNITS):
            self.btc_estimate_size = 188
        if pool.lp_units == 0 and pool.synth_units() == 0 and pool.synth_balance == 0:
            if pool.asset.is_bnb():
                gas_amt = gas.amount
                if CACAO.get_chain() == "BNB":
                    gas_amt *= 2
                outbound_asset_amt -= gas_amt
                emit_asset -= gas_amt
                pool.asset_balance += gas_amt
                asset_amt -= gas_amt
            elif pool.asset.is_thor():
                gas_amt = gas.amount
                outbound_asset_amt -= gas_amt
                emit_asset -= gas_amt
                pool.asset_balance += gas_amt
                asset_amt -= gas_amt
            elif pool.asset.is_eth():
                if asset.get_chain() == "ARB":
                    dynamic_fee = int(self.arb_estimate_size * self.arb_tx_rate * 3 / 2)
                    dynamic_fee = int(dynamic_fee / 1000)
                gas = self.get_gas(asset.get_chain(), tx)
                asset_amt -= dynamic_fee
                outbound_asset_amt -= dynamic_fee
                pool.asset_balance += gas.amount
            elif pool.asset.is_btc():
                # the last withdraw tx , it need to spend everything
                # usually it is only 1 UTXO left
                self.btc_estimate_size = 188
                # left enough gas asset otherwise it will get into negative
                emit_asset -= dynamic_fee
                estimate_gas_asset = (
                    int(self.btc_tx_rate * 3 / 2) * self.btc_estimate_size
                )
                gas = Coin(gas.asset, estimate_gas_asset)
                outbound_asset_amt -= int(estimate_gas_asset)
                pool.asset_balance += dynamic_fee
                asset_amt -= dynamic_fee
            elif pool.asset.is_dash():
                # the last withdraw tx , it need to spend everything
                # usually it is only 1 UTXO left
                self.dash_estimate_size = 180
                # left enough gas asset otherwise it will get into negative
                emit_asset -= dynamic_fee
                estimate_gas_asset = (
                    int(self.dash_tx_rate * 3 / 2) * self.dash_estimate_size
                )
                gas = Coin(gas.asset, estimate_gas_asset)
                outbound_asset_amt -= int(estimate_gas_asset)
                pool.asset_balance += dynamic_fee
                asset_amt -= dynamic_fee
            elif pool.asset.is_kuji() and not pool.asset.is_usk():
                # the last withdraw tx , it need to spend everything
                # left enough gas asset otherwise it will get into negative
                emit_asset -= dynamic_fee
                estimate_gas_asset = (
                    int(self.kuji_tx_rate * 3 / 2) * self.kuji_estimate_size
                )
                estimate_gas_asset = int(estimate_gas_asset / 100) * 100
                gas = Coin(gas.asset, estimate_gas_asset)
                outbound_asset_amt -= int(estimate_gas_asset)
                pool.asset_balance += dynamic_fee
                asset_amt = outbound_asset_amt
            elif pool.asset.get_symbol() == "XRD":
                emit_asset -= gas.amount
                outbound_asset_amt -= dynamic_fee
                pool.asset_balance += gas.amount
                asset_amt = asset_amt - dynamic_fee

        self.set_pool(pool)

        # get from address VAULT cross chain
        from_address = tx.to_address
        if from_address != "VAULT":  # don't replace for unit tests
            from_alias = get_alias(tx.chain, from_address)
            from_address = get_alias_address(asset.get_chain(), from_alias)

        # get to address cross chain
        to_address = tx.from_address
        if to_address not in get_aliases():  # don't replace for unit tests
            to_alias = get_alias(tx.chain, to_address)
            to_address = get_alias_address(asset.get_chain(), to_alias)

        out_txs = [
            Transaction(
                asset.get_chain(),
                from_address,
                to_address,
                [Coin(asset, outbound_asset_amt)],
                f"OUT:{tx.id.upper()}",
                gas=[gas],
                max_gas=[Coin(gas.asset, dynamic_fee)],
            ),
            Transaction(
                CACAO.get_chain(),
                tx.to_address,
                tx.from_address,
                [Coin(CACAO, cacao_amt)],
                f"OUT:{tx.id.upper()}",
                gas=[tx_cacao_gas],
            ),
        ]

        if withdraw_units > 0:
            # generate event for WITHDRAW transaction
            withdraw_event = Event(
                "withdraw",
                [
                    {"pool": pool.asset},
                    {"liquidity_provider_units": withdraw_units},
                    {"basis_points": withdraw_basis_points},
                    {"asymmetry": "0.000000000000000000"},
                    {"emit_asset": asset_amt},
                    {"emit_cacao": cacao_amt},
                    {"imp_loss_protection": "0"},
                    *tx.get_attributes(),
                ],
            )
            self.events.append(withdraw_event)
        else:
            event = Event(
                "pending_liquidity",
                [
                    {"pool": pool.asset},
                    {"type": "withdraw"},
                    {"cacao_address": from_address or ""},
                    {"cacao_amount": cacao_amt},
                    {"asset_amount": asset_amt},
                    {"asset_address": to_address or ""},
                    {f"{tx.chain}_txid": tx.id},
                ],
            )
            self.events.append(event)
        outbound = self.handle_fee(tx, out_txs)
        return outbound

    def handle_swap(self, tx):
        """
        Does a swap (or double swap)
        MEMO: SWAP:<asset(req)>:<address(op)>:<target_trade(op)>
        """
        # parse memo
        parts = tx.memo.split(":")
        if len(parts) < 2:
            if tx.memo == "":
                return self.refund(tx, 105, "memo can't be empty")
            return self.refund(tx, 105, f"invalid tx type: {tx.memo}")

        address = tx.from_address
        # check address to send to from memo
        if len(parts) > 2 and parts[2] != "":
            address = parts[2]
            # checking if address is for mainnet, not testnet
            if address.lower().startswith("bnb"):
                reason = f"{address} is not recognizable"
                return self.refund(tx, 105, reason)

        # get trade target, if exists
        target_trade = 0
        if len(parts) > 3:
            target_trade = int(parts[3] or "0")

        asset = Asset(parts[1])

        # check that we have one coin
        if len(tx.coins) != 1:
            reason = "not expecting multiple coins in a swap: unknown request"
            return self.refund(tx, 105, reason)

        source = tx.coins[0].asset
        target = asset

        # refund if we're trying to swap with the coin we given ie
        # swapping bnb with bnb
        if source == target and source.is_synth == target.is_synth:
            reason = "swap Source and Target cannot be the same.: unknown request"
            return self.refund(tx, 105, reason)

        if (
            ("maya" in address or "SYNTH" in address)
            and not target.is_synth
            and not target.is_cacao()
        ):
            reason = (
                "swap destination address is not the same chain as "
                "the target asset: unknown request"
            )
            return self.refund(tx, 105, reason)
        pool = self.get_pool(target)
        if target.is_cacao():
            pool = self.get_pool(source)

        if pool.is_zero():
            return self.refund(tx, 108, f"{asset} pool doesn't exist")

        pools = []
        in_tx = tx

        # check if we have enough to cover the fee
        cacao_fee = self.get_cacao_fee(target.get_chain())

        in_coin = in_tx.coins[0]
        if in_coin.is_cacao() and in_coin.amount <= cacao_fee:
            return self.refund(tx, 108, "fail swap, not enough fee")

        swap_events = []

        # check if its a double swap
        if not source.is_cacao() and not target.is_cacao():
            pool = self.get_pool(source)
            if pool.is_zero():
                return self.refund(tx, 108, "fail swap, invalid balance")

            emit, liquidity_fee, liquidity_fee_in_cacao, swap_slip, pool = self.swap(
                tx.coins[0], CACAO
            )

            # check if we have enough to cover the fee
            if emit.is_cacao() and emit.amount <= cacao_fee:
                return self.refund(tx, 108, "fail swap, not enough fee")

            if str(pool.asset) not in self.liquidity:
                self.liquidity[str(pool.asset)] = 0
            self.liquidity[str(pool.asset)] += liquidity_fee_in_cacao

            # here we copy the tx to break references cause
            # the tx is split in 2 events and gas is handled only once
            in_tx = deepcopy(tx)

            # generate event for SWAP transaction
            event = Event(
                "swap",
                [
                    {"pool": pool.asset},
                    {"swap_target": 0},
                    {"swap_slip": swap_slip},
                    {"liquidity_fee": liquidity_fee},
                    {"liquidity_fee_in_cacao": liquidity_fee_in_cacao},
                    {"emit_asset": f"{emit.amount} {emit.asset}"},
                    {"streaming_swap_quantity": "1"},
                    {"streaming_swap_count": "1"},
                    *in_tx.get_attributes(),
                ],
            )
            swap_events.append(event)

            # and we remove the gas on in_tx for the next event so we don't
            # have it twice
            in_tx.gas = None

            self.set_pool(pool)
            in_tx.coins[0] = emit
            source = CACAO

        # set asset to non-cacao asset
        asset = source
        if asset.is_cacao():
            asset = target

        # check if we have enough to cover the fee
        cacao_fee = self.get_cacao_fee(target.get_chain())
        in_coin = in_tx.coins[0]
        if in_coin.is_cacao() and in_coin.amount <= cacao_fee:
            return self.refund(tx, 108, "fail swap, not enough fee")

        pool = self.get_pool(asset)

        if target.is_synth and pool.is_zero():
            return self.refund(tx, 108, f"{asset} pool doesn't exist")

        if pool.is_zero():
            return self.refund(tx, 108, "fail swap, invalid balance")

        emit, liquidity_fee, liquidity_fee_in_cacao, swap_slip, pool = self.swap(
            in_tx.coins[0], target
        )
        pools.append(pool)

        # check if we have enough to cover the fee
        if emit.is_cacao() and emit.amount <= cacao_fee:
            return self.refund(
                tx,
                108,
                f"output CACAO ({emit.amount}) is not enough to pay transaction fee",
            )

        # check emit is non-zero and is not less than the target trade
        if emit.is_zero() or (emit.amount < target_trade):
            reason = f"emit asset {emit.amount} less than price limit {target_trade}"
            return self.refund(tx, 108, reason)

        if str(pool.asset) not in self.liquidity:
            self.liquidity[str(pool.asset)] = 0
        self.liquidity[str(pool.asset)] += liquidity_fee_in_cacao

        # save pools
        for pool in pools:
            self.set_pool(pool)

        # get from address VAULT cross chain
        from_address = in_tx.to_address
        if from_address != "VAULT":  # don't replace for unit tests
            from_alias = get_alias(in_tx.chain, from_address)
            if target.is_synth:
                from_address = get_alias_address(target.get_chain(), "VAULT")
            else:
                from_address = get_alias_address(
                    target.get_chain(), from_alias)

        out_txs = [
            Transaction(
                target.get_chain(),
                from_address,
                address,
                [emit],
                f"OUT:{tx.id.upper()}",
            )
        ]

        if emit.asset.is_synth:
            out_txs[0].id = Transaction.empty_id
            # TODO: Uncomment after implementing mint event
            # self.events.append(Event(
            #     "mint_burn",
            #     [
            #            {"supply": "mint"},
            #            {"denom": f"{emit.asset.lower()}"},
            #            {"amount": f"{emit.amount}"},
            #         {"reason": "swap"},
            #     ],
            #    ))

        event = Event(
            "swap",
            [
                {"pool": pool.asset},
                {"swap_target": target_trade},
                {"swap_slip": swap_slip},
                {"liquidity_fee": liquidity_fee},
                {"liquidity_fee_in_cacao": liquidity_fee_in_cacao},
                {"emit_asset": f"{emit.amount} {emit.asset}"},
                {"streaming_swap_quantity": "1"},
                {"streaming_swap_count": "1"},
                *in_tx.get_attributes(),
            ],
        )
        swap_events.append(event)

        # emit the events
        for e in swap_events:
            self.events.append(e)

        outbound = self.handle_fee(tx, out_txs)
        return outbound

    def swap(self, coin, asset):
        """
        Does a swap returning amount of coins emitted and new pool

        :param Coin coin: coin sent to swap
        :param Asset asset: target asset
        :returns: list of events
            - emit (int) - number of coins to be emitted for the swap
            - liquidity_fee (int) - liquidity fee
            - liquidity_fee_in_cacao (int) - liquidity fee in cacao
            - swap_slip (int) - trade slip
            - pool (Pool) - pool with new values

        """
        if not coin.is_cacao():
            asset = coin.asset

        pool = self.get_pool(asset)
        if coin.is_cacao():
            X = pool.cacao_balance
            Y = pool.asset_balance
        else:
            X = pool.asset_balance
            Y = pool.cacao_balance

        if asset.is_synth:
            X = X * self.synth_multiplier
            Y = Y * self.synth_multiplier

        x = coin.amount
        emit = self._calc_asset_emission(X, x, Y)
        # decimals to 6 if KUJI chain
        if asset.chain == Kuji.chain:
            emit = int(emit / 100) * 100

        # calculate the liquidity fee (in cacao)
        liquidity_fee = self._calc_liquidity_fee(X, x, Y)

        liquidity_fee_in_cacao = liquidity_fee
        if coin.is_cacao():
            liquidity_fee_in_cacao = pool.get_asset_in_cacao(liquidity_fee)

        # calculate trade slip
        swap_slip = self._calc_swap_slip(X, x)

        # if we emit zero, return immediately
        if emit == 0:
            return Coin(asset, emit), 0, 0, 0, pool

        new_pool = deepcopy(pool)  # copy of pool
        if coin.is_cacao():
            new_pool.add(x, 0)
            if asset.is_synth:
                new_pool.synth_balance += emit
            else:
                new_pool.sub(0, emit)
            emit = Coin(asset, emit)
        else:
            if asset.is_synth:
                new_pool.synth_balance -= x
            else:
                new_pool.add(0, x)
            new_pool.sub(emit, 0)
            emit = Coin(CACAO, emit)

        return emit, liquidity_fee, liquidity_fee_in_cacao, swap_slip, new_pool

    def _calc_liquidity_fee(self, X, x, Y):
        """
        Calculate the liquidity fee from a trade
        ( x^2 *  Y ) / ( x + X )^2

        :param int X: first balance
        :param int x: asset amount
        :param int Y: second balance
        :returns: (int) liquidity fee

        """
        liquidity_fee = (x**2) * Y // (x + X) ** 2
        return round_bankers(liquidity_fee, 0.992533)

    def _calc_swap_slip(self, X, x):
        """
        Calculate the trade slip from a trade
        expressed in basis points (10,000)
        x / (X + x)

        :param int X: first balance
        :param int x: asset amount
        :returns: (int) trade slip

        """

        swap_slip = 10000 * x / (X + x)
        return int(round(swap_slip))

    def _calc_asset_emission(self, X, x, Y):
        """
        Calculates the amount of coins to be emitted in a swap
        ( x * X * Y ) / ( x + X )^2

        :param int X: first balance
        :param int x: asset amount
        :param int Y: second balance
        :returns: (int) asset emission

        """
        return int((x * X * Y) // (x + X) ** 2)



class Event(Jsonable):
    """
    Event class representing events generated by mayachain
    using tendermint sdk events
    """

    def __init__(self, event_type, attributes, height=None):
        self.type = event_type
        for attr in attributes:
            for key, value in attr.items():
                attr[key] = str(value)
        self.attributes = attributes
        self.height = height

    def __str__(self):
        attrs = " ".join(map(str, self.attributes))
        return f"Event {self.type} | {attrs}"

    def __hash__(self):
        attrs = deepcopy(
            sorted(self.attributes, key=lambda x: sorted(x.items())))
        for attr in attrs:
            for key, value in attr.items():
                if value is not None:
                    attr[key] = value.upper()
        if self.type == "outbound":
            attrs = [a for a in attrs if list(a.keys())[0] != "id"]
        if self.type == "refund":
            attrs = [a for a in attrs if list(a.keys())[0] != "reason"]
        return hash(str(attrs))

    def __repr__(self):
        return str(self)

    def __eq__(self, other):
        return (self.type, hash(self)) == (other.type, hash(other))

    def __lt__(self, other):
        return (self.type, hash(self)) < (other.type, hash(other))

    def get(self, attr):
        for a in self.attributes:
            if list(a.keys())[0] == attr:
                return a[attr]
        return None


class Pool(Jsonable):
    def __init__(self, asset, cacao_amt=0, asset_amt=0, status="Available"):
        if isinstance(asset, str):
            self.asset = Asset(asset)
        else:
            self.asset = asset
        self.cacao_balance = cacao_amt
        self.asset_balance = asset_amt
        self.synth_balance = 0
        self.lp_units = 0
        self.liquidity_providers = []
        self.status = status

    def get_asset_in_cacao(self, val):
        """
        Get an equal amount of given value in cacao
        """
        if self.is_zero():
            return 0

        return get_share(self.cacao_balance, self.asset_balance, val)

    def get_cacao_in_asset(self, val):
        """
        Get an equal amount of given value in asset
        """
        if self.is_zero():
            return 0

        amount = get_share(self.asset_balance, self.cacao_balance, val)
        if self.asset.chain == Kuji.chain:
            amount = int(amount / 100) * 100
        return amount

    def get_cacao_disbursement_for_asset_add(self, val):
        """
        Get the equivalent amount of cacao for a given amount of asset added to
        the pool, taking slip into account. When this amount is withdrawn from
        the pool, the constant product of depths rule is preserved.
        """
        if self.is_zero():
            return 0

        return get_share(self.cacao_balance, self.asset_balance + val, val)

    def get_asset_fee(self):
        """
        Calculates how much asset we need to pay for the 1 cacao transaction fee
        """
        if self.is_zero():
            return 0

        return self.get_cacao_in_asset(2000000)

    def sub(self, cacao_amt, asset_amt):
        """
        Subtracts from pool
        """
        self.cacao_balance -= cacao_amt
        self.asset_balance -= asset_amt

        if self.asset_balance < 0 or self.cacao_balance < 0:
            logging.error(f"Overdrawn pool: {self}")
            raise Exception("insufficient funds")

    def add(self, cacao_amt, asset_amt):
        """
        Add to pool
        """
        self.cacao_balance += cacao_amt
        self.asset_balance += asset_amt

    def is_zero(self):
        """
        Check if pool has zero balance
        """
        return self.cacao_balance == 0 and self.asset_balance == 0

    def synth_units(self):
        """
        Calculate dynamic synth units
        (L*S)/(2*A-S)
        L = LP units
        S = synth balance
        A = asset balance
        """
        if self.asset_balance == 0:
            return 0
        numerator = self.lp_units * self.synth_balance
        denominator = 2 * self.asset_balance - self.synth_balance
        if denominator == 0:
            denominator = 1
        return int(numerator / denominator)

    def pool_units(self):
        """
        Calculate total pool units
        (L+S)
        L = LP units
        S = synth balance
        """
        return self.synth_units() + self.lp_units

    def get_liquidity_provider(self, address):
        """
        Fetch a specific liquidity provider by address
        """
        for lp in self.liquidity_providers:
            if lp.address == address:
                return lp

        return LiquidityProvider(address)

    def set_liquidity_provider(self, lp):
        """
        Set a liquidity provider
        """
        for i, s in enumerate(self.liquidity_providers):
            if s.address == lp.address:
                self.liquidity_providers[i] = lp
                return

        self.liquidity_providers.append(lp)

    def add_liquidity(
        self, cacao_address, asset_address, cacao_amt, asset_amt, asset, txid
    ):
        """
        add liquidity cacao/asset for an address
        """
        fetch_address = asset_address
        if cacao_address != "":
            fetch_address = cacao_address
        lp = self.get_liquidity_provider(fetch_address)

        asset_amt += lp.pending_asset
        cacao_amt += lp.pending_cacao

        # handle cross chain liquidity provision
        if asset_amt == 0 and asset_address is not None:
            lp.pending_cacao += cacao_amt
            lp.pending_tx = txid
            self.set_liquidity_provider(lp)
            return 0, 0, 0, None
        if cacao_amt == 0 and cacao_address is not None:
            lp.pending_asset += asset_amt
            lp.pending_tx = txid
            self.set_liquidity_provider(lp)
            return 0, 0, 0, None

        lp.pending_cacao = 0
        lp.pending_asset = 0
        units = self._calc_liquidity_units(
            self.cacao_balance,
            self.asset_balance,
            cacao_amt,
            asset_amt,
        )

        self.add(cacao_amt, asset_amt)
        self.lp_units += units
        lp.units += units
        lp.cacao_deposit_value += get_share(units,
                                            self.lp_units, self.cacao_balance)
        lp.asset_deposit_value += get_share(units,
                                            self.lp_units, self.asset_balance)
        self.set_liquidity_provider(lp)
        return units, cacao_amt, asset_amt, lp.pending_tx

    def withdraw(self, address, withdraw_basis_points):
        """
        Withdraw from an address with given withdraw basis points
        """
        if withdraw_basis_points > 10000 or withdraw_basis_points < 0:
            raise Exception(
                "withdraw basis points should be between 0 - 10,000")

        lp = self.get_liquidity_provider(address)
        units, cacao_amt, asset_amt = self._calc_withdraw_units(
            lp.units, withdraw_basis_points
        )
        # decimals to 6 if KUJI chain
        if self.asset.chain == Kuji.chain:
            asset_amt = int(asset_amt / 100) * 100
        lp.units -= units
        lp.cacao_deposit_value -= get_share(units,
                                            self.lp_units, self.cacao_balance)
        lp.asset_deposit_value -= get_share(units,
                                            self.lp_units, self.asset_balance)
        self.set_liquidity_provider(lp)
        self.lp_units -= units
        self.sub(cacao_amt, asset_amt)
        return units, cacao_amt, asset_amt

    def _calc_liquidity_units(self, R, A, r, a):
        """
        Calculate liquidity provider units
        slipAdjustment = (1 - ABS((R a - r A)/((r + R) (a + A))))
        units = ((P (a R + A r))/(2 A R))*slidAdjustment
        R = pool cacao balance after
        A = pool asset balance after
        r = provided cacao
        a = provided asset
        """
        P = self.pool_units()
        R = float(R)
        A = float(A)
        r = float(r)
        a = float(a)
        if R == 0.0 or A == 0.0 or P == 0:
            return int(r)
        slipAdjustment = 1 - abs((R * a - r * A) / ((r + R) * (a + A)))
        units = (P * (a * R + A * r)) / (2 * A * R)
        return int(units * slipAdjustment)

    def _calc_withdraw_units(self, lp_units, withdraw_basis_points):
        """
        Calculate amount of cacao/asset to withdraw
        Returns liquidity provider units, cacao amount, asset amount
        """
        units_to_claim = get_share(withdraw_basis_points, 10000, lp_units)
        withdraw_cacao = get_share(
            units_to_claim, self.pool_units(), self.cacao_balance)
        withdraw_asset = get_share(
            units_to_claim, self.pool_units(), self.asset_balance
        )
        units_after = lp_units - units_to_claim
        if units_after < 0:
            logging.error(f"Overdrawn liquidity provider units: {self}")
            raise Exception("Overdrawn liquidity provider units")
        return units_to_claim, withdraw_cacao, withdraw_asset

    def __repr__(self):
        return "<Pool %s Cacao: %d | Asset: %d>" % (
            self.asset,
            self.cacao_balance,
            self.asset_balance,
        )

    def __str__(self):
        return (
            "Pool %s Cacao: %d | Asset: %d | Units: %s | Synth Units: %s | Synth: %s"
            % (
                self.asset,
                self.cacao_balance,
                self.asset_balance,
                self.lp_units,
                self.synth_units(),
                self.synth_balance,
            )
        )


class LiquidityProvider(Jsonable):
    def __init__(self, address, units=0):
        self.address = address
        self.units = 0
        self.pending_cacao = 0
        self.pending_asset = 0
        self.pending_tx = None
        self.cacao_deposit_value = 0
        self.asset_deposit_value = 0
        self.cacao_address = None
        self.asset_address = None

    def add(self, units):
        """
        Add liquidity provider units
        """
        self.units += units

    def sub(self, units):
        """
        Subtract liquidity provider units
        """
        self.units -= units
        if self.units < 0:
            logging.error(f"Overdrawn liquidity provider: {self}")
            raise Exception("insufficient liquidity provider units")

    def is_zero(self):
        return self.units <= 0

    def __repr__(self):
        return "<Liquidity Provider %s Units: %d>" % (self.address, self.units)

    def __str__(self):
        return "Liquidity Provider %s Units: %d" % (self.address, self.units)
