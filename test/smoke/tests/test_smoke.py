import json
import logging
import os
import unittest
from copy import deepcopy
from pprint import pformat

from deepdiff import DeepDiff

from chains.arbitrum import Arbitrum
from chains.binance import Binance
from chains.bitcoin import Bitcoin
from chains.dash import Dash
from chains.ethereum import Ethereum
from chains.kuji import Kuji
from chains.radix import Radix
from chains.thorchain import Thorchain
from mayachain.mayachain import MayachainState, Event
from utils.breakpoint import Breakpoint
from utils.common import Transaction, get_cacao_asset, DEFAULT_CACAO_ASSET

CACAO = get_cacao_asset()
# Init logging
logging.basicConfig(
    format="%(levelname).1s[%(asctime)s] %(message)s",
    level=os.environ.get("LOGLEVEL", "INFO"),
)


def get_balance(idx):
    """
    Retrieve expected balance with given id
    """
    file = "data/smoke_test_balances.json"
    with open(file) as f:
        contents = f.read()
        contents = contents.replace(DEFAULT_CACAO_ASSET, CACAO)
        balances = json.loads(contents)
        for bal in balances:
            if idx == bal["TX"]:
                return bal


def get_events():
    """
    Retrieve expected events
    """
    file = "data/smoke_test_events.json"
    with open(file) as f:
        contents = f.read()
        contents = contents.replace(DEFAULT_CACAO_ASSET, CACAO)
        events = json.loads(contents)
        return [Event(e["type"], e["attributes"]) for e in events]
    raise Exception("could not load events")


class TestSmoke(unittest.TestCase):
    """
    This runs tests with a pre-determined list of transactions and an expected
    balance after each transaction (/data/balance.json). These transactions and
    balances were determined earlier via a google spreadsheet
    https://docs.google.com/spreadsheets/d/1sLK0FE-s6LInWijqKgxAzQk2RiSDZO1GL58kAD62ch0/edit#gid=439437407
    """

    def test_smoke(self):
        export = os.environ.get("EXPORT", None)
        export_events = os.environ.get("EXPORT_EVENTS", None)

        failure = False
        snaps = []
        arb = Arbitrum()  # init local arbitrum chain
        bnb = Binance()  # init local binance chain
        btc = Bitcoin()  # init local bitcoin chain
        dash = Dash()  # init local dash chain
        kuji = Kuji()  # init local kuji chain
        eth = Ethereum()  # init local ethereum chain
        xrd = Radix()  # init local radix chain
        thor = Thorchain()  # init local thorchain chain
        mayachain = MayachainState()  # init local mayachain
        mayachain.network_fees = {  # init fixed network fees
            "ARB": 100000000,
            "BNB": 37500,
            "BTC": 10000,
            "DASH": 10000,
            "KUJI": 20000,
            "ETH": 65000,
            "THOR": 20000,
            "XRD": 20000
        }

        file = "data/smoke_test_transactions.json"

        with open(file, "r") as f:
            contents = f.read()
            loaded = json.loads(contents)

        for i, txn in enumerate(loaded):
            txn = Transaction.from_data(txn)
            logging.info(f"{i} {txn}")

            if txn.chain == Binance.chain:
                bnb.transfer(txn)  # send transfer on binance chain
            if txn.chain == Bitcoin.chain:
                btc.transfer(txn)  # send transfer on bitcoin chain
            if txn.chain == Dash.chain:
                dash.transfer(txn)  # send transfer on dash chain
            if txn.chain == Kuji.chain:
                kuji.transfer(txn)  # send transfer on kuji chain
            if txn.chain == Thorchain.chain:
                thor.transfer(txn)  # send transfer on thorchain chain
            if txn.chain == Radix.chain:
                xrd.transfer(txn)  # send transfer on radix chain
            if txn.chain == Ethereum.chain or txn.chain == Arbitrum.chain:
                if txn.chain == Ethereum.chain:
                    eth.transfer(txn)  # send transfer on ethereum chain
                else:
                    arb.transfer(txn)  # send transfer on arbitrum chain
                # convert the coin amount to mayachain amount which is 1e8
                for idx, c in enumerate(txn.coins):
                    txn.coins[idx].amount = c.amount / 1e10
                for idx, c in enumerate(txn.gas):
                    txn.gas[idx].amount = c.amount / 1e10

            if txn.memo == "SEED":
                continue
            outbounds = mayachain.handle(txn)  # process transaction in mayachain

            for txn in outbounds:
                if txn.chain == Binance.chain:
                    bnb.transfer(txn)  # send outbound txns back to Binance
                if txn.chain == Bitcoin.chain:
                    btc.transfer(txn)  # send outbound txns back to Bitcoin
                if txn.chain == Dash.chain:
                    dash.transfer(txn)  # send outbound txns back to Dash
                if txn.chain == Kuji.chain:
                    kuji.transfer(txn)  # send outbound txns back to Kuji
                if txn.chain == Thorchain.chain:
                    thor.transfer(txn)  # send transfer on thorchain chain
                if txn.chain == Radix.chain:
                    xrd.transfer(txn)  # send transfer on radix chain
                if txn.chain == Ethereum.chain or txn.chain == Arbitrum.chain:
                    temp_txn = deepcopy(txn)
                    for idx, c in enumerate(temp_txn.coins):
                        temp_txn.coins[idx].amount = c.amount * 1e10
                    for idx, c in enumerate(temp_txn.gas):
                        temp_txn.gas[idx].amount = c.amount * 1e10
                    temp_txn.fee.amount = temp_txn.fee.amount * 1e10
                    if txn.chain == Ethereum.chain:
                        eth.transfer(temp_txn)  # send outbound txns back to Ethereum
                    else:
                        arb.transfer(temp_txn)  # send outbound txns back to Arbitrum

            mayachain.handle_rewards()

            arb_out = []
            for out in outbounds:
                if out.coins[0].asset.get_chain() == "ARB":
                    arb_out.append(out)
            bnb_out = []
            for out in outbounds:
                if out.coins[0].asset.get_chain() == "BNB":
                    bnb_out.append(out)
            btc_out = []
            for out in outbounds:
                if out.coins[0].asset.get_chain() == "BTC":
                    btc_out.append(out)
            dash_out = []
            for out in outbounds:
                if out.coins[0].asset.get_chain() == "DASH":
                    dash_out.append(out)
            kuji_out = []
            for out in outbounds:
                if out.coins[0].asset.get_chain() == "KUJI":
                    kuji_out.append(out)
            thor_out = []
            for out in outbounds:
                if out.coins[0].asset.get_chain() == "THOR":
                    thor_out.append(out)
            xrd_out = []
            for out in outbounds:
                if out.coins[0].asset.get_chain() == "XRD":
                    xrd_out.append(out)
            eth_out = []
            for out in outbounds:
                if out.coins[0].asset.get_chain() == "ETH":
                    eth_out.append(out)
            mayachain.handle_gas(arb_out)  # subtract gas from pool(s)
            mayachain.handle_gas(bnb_out)  # subtract gas from pool(s)
            mayachain.handle_gas(btc_out)  # subtract gas from pool(s)
            mayachain.handle_gas(dash_out)  # subtract gas from pool(s)
            mayachain.handle_gas(kuji_out)  # subtract gas from pool(s)
            mayachain.handle_gas(eth_out)  # subtract gas from pool(s)
            mayachain.handle_gas(xrd_out)  # subtract gas from pool(s)
            mayachain.handle_gas(thor_out)  # subtract gas from pool(s)

            # generated a snapshop picture of mayachain and bnb
            snap = Breakpoint(mayachain, bnb).snapshot(i, len(outbounds))
            snaps.append(snap)
            expected = get_balance(i)  # get the expected balance from json file

            diff = DeepDiff(
                snap, expected, ignore_order=True
            )  # empty dict if are equal
            if len(diff) > 0:
                logging.info(f"Transaction: {i} {txn}")
                logging.info(">>>>>> Expected")
                logging.info(pformat(expected))
                logging.info(">>>>>> Obtained")
                logging.info(pformat(snap))
                logging.info(">>>>>> DIFF")
                logging.info(pformat(diff))
                if not export:
                    raise Exception("did not match!")

            # log result
            if len(outbounds) == 0:
                continue
            result = "[+]"
            if "REFUND" in outbounds[0].memo:
                result = "[-]"
            for outbound in outbounds:
                logging.info(f"{result} {outbound.short()}")

        if export:
            with open(export, "w") as fp:
                json.dump(snaps, fp, indent=4)

        if export_events:
            with open(export_events, "w") as fp:
                json.dump(mayachain.events, fp, default=lambda x: x.__dict__, indent=4)

        # check events against expected
        expected_events = get_events()
        for event, expected_event in zip(mayachain.events, expected_events):
            if event != expected_event:
                logging.error(
                    f"Event Mayachain {event} \n   !="
                    f"  \nEvent Expected {expected_event}"
                )

                if not export_events:
                    raise Exception("Events mismatch")

        if failure:
            raise Exception("Fail")


if __name__ == "__main__":
    unittest.main()
