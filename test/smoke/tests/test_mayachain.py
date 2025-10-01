import unittest
from utils.common import Transaction, Coin, get_cacao_asset, mock_http_requests

mock_http_requests()
from mayachain.mayachain import (
    MayachainState,
    Pool,
    Event,
)
from chains.binance import Binance
from chains.mayachain import Mayachain

CACAO = get_cacao_asset()


class TestMayachainState(unittest.TestCase):
    def test_get_cacao_fee(self):
        # no network fees defined
        # default to 1 CACAO
        mayachain = MayachainState()
        cacao_fee = mayachain.get_cacao_fee("BNB")
        self.assertEqual(cacao_fee, 2000000000)

        # happy path
        mayachain.network_fees = {"BNB": 37500}
        mayachain.pools = [Pool("BNB.BNB", 50 * Coin.ONE, 50 * Coin.ONE)]
        cacao_fee = mayachain.get_cacao_fee("BNB")
        self.assertEqual(cacao_fee, 75000)

        mayachain.network_fees = {"BTC": 1}
        mayachain.pools = [Pool("BTC.BTC", 50 * Coin.ONE, 50 * Coin.ONE)]
        cacao_fee = mayachain.get_cacao_fee("BTC")
        self.assertEqual(cacao_fee, 2)

        mayachain.network_fees = {"ETH": 1}
        mayachain.pools = [Pool("ETH.ETH", 50 * Coin.ONE, 50 * Coin.ONE)]
        cacao_fee = mayachain.get_cacao_fee("ETH")
        self.assertEqual(cacao_fee, 2)

    def test_get_gas(self):
        # no network fees defined
        # default to 1 CACAO
        mayachain = MayachainState()
        gas = mayachain.get_gas("BTC", {})
        self.assertEqual(gas, Coin("BTC.BTC", 0))

        # happy path
        mayachain.network_fees = {"BTC": 99813}
        mayachain.pools = [Pool("BTC.BTC", 59983570781, 127225819)]
        gas = mayachain.get_gas("BTC", {})
        self.assertEqual(gas, Coin("BTC.BTC", 0))

    def test_handle_fee(self):
        mayachain = MayachainState()
        mayachain.network_fees = {"BNB": 37500}
        mayachain.pools = [Pool("BNB.BNB", 100 * Coin.ONE, 10 * Coin.ONE)]
        mayachain.pools[0].lp_units = 10 * Coin.ONE
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin(CACAO, 10 * Coin.ONE)],
            "SWAP:BNB.BNB",
        )

        outbound = mayachain.handle(tx)
        self.assertEqual(outbound[0].coins[0].amount, 82569628)
        self.assertEqual(mayachain.reserve, 899251)
        self.assertEqual(mayachain.gas_withheld_cacao, 899251)

    def test_swap_bep2(self):
        if CACAO.get_chain() == "MAYA":
            return
        mayachain = MayachainState()
        mayachain.network_fees = {"BNB": 37500}
        events = mayachain.events

        # no pool, should emit a refund
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 10 * Coin.ONE)],
            "SWAP:BNB.CACAO-67C",
        )

        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 0)

        # check refund event not generated for swap with no pool
        expected_events = []
        self.assertEqual(events, expected_events)

        mayachain.pools = []

        # no pool, should emit a refund
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin(CACAO, 10 * Coin.ONE)],
            "SWAP:BNB.BNB",
        )

        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 1)
        self.assertEqual(outbound[0].memo, "REFUND:TODO")
        self.assertEqual(outbound[0].coins[0], Coin(CACAO, 9 * Coin.ONE))

        # check refund event generated for swap with no pool
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "100000000 BNB.CACAO-67C"},
                    {"pool_deduct": "0"},
                ],
            ),
            Event(
                "refund",
                [
                    {"code": "108"},
                    {"reason": "fail swap, invalid balance"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(events, expected_events)

        # init pool
        mayachain.pools = [Pool("BNB.BNB", 50 * Coin.ONE, 50 * Coin.ONE)]

        # do a regular swap
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 1)
        self.assertEqual(outbound[0].memo, "OUT:TODO")
        self.assertEqual(outbound[0].coins[0], Coin("BNB.BNB", 694369444))

        # check swap event generated for successful swap
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "75000 BNB.BNB"},
                    {"pool_deduct": "165441"},
                ],
            ),
            Event(
                "swap",
                [
                    {"pool": "BNB.BNB"},
                    {"swap_target": "0"},
                    {"swap_slip": "1667"},
                    {"liquidity_fee": "138888888"},
                    {"liquidity_fee_in_cacao": "138888888"},
                    {"emit_asset": "694444444 BNB.BNB"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(events, expected_events)

        # swap with two coins on the inbound tx
        tx.coins = [Coin("BNB.BNB", 10 * Coin.ONE),
                    Coin("BNB.LOK-3C0", 10 * Coin.ONE)]
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 1)
        self.assertEqual(outbound[0].memo, "REFUND:TODO")
        self.assertEqual(outbound[0].coins[0], Coin("BNB.BNB", 999887500))

        # check refund event generated for swap with two coins
        reason = "not expecting multiple coins in a swap: unknown request"
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "112500 BNB.BNB"},
                    {"pool_deduct": "156762"},
                ],
            ),
            Event(
                "refund",
                [{"code": "105"}, {"reason": reason}, *tx.get_attributes()],
            ),
        ]
        self.assertEqual(events, expected_events)

        # swap with zero return, refunds and doesn't change pools
        tx.coins = [Coin(CACAO, 1)]
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 0)
        self.assertEqual(mayachain.pools[0].cacao_balance, 5999790975)

        # check refund event generated for swap with zero return
        expected_events += [
            Event(
                "fee",
                [{"tx_id": "TODO"}, {"coins": f"1 {CACAO}"}, {"pool_deduct": "0"}],
            ),
        ]
        self.assertEqual(events, expected_events)

        # swap with zero return, not enough coin to pay fee so no refund
        tx.coins = [Coin(CACAO, 1)]
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 0)
        self.assertEqual(mayachain.pools[0].cacao_balance, 5999790975)

        # check refund event generated for swap with zero return
        expected_events += [
            Event(
                "fee",
                [{"tx_id": "TODO"}, {"coins": f"1 {CACAO}"}, {"pool_deduct": "0"}],
            ),
        ]
        self.assertEqual(events, expected_events)

        # swap with limit
        tx.coins = [Coin(CACAO, 500000000)]
        tx.memo = "SWAP:BNB.BNB::999999999999999999999"
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 1)
        self.assertEqual(outbound[0].memo, "REFUND:TODO")
        self.assertEqual(outbound[0].coins[0], Coin(CACAO, 499843242))
        self.assertEqual(mayachain.pools[0].cacao_balance, 5999790975)

        # check refund event generated for swap with limit
        reason = "emit asset 305739584 less than price limit 999999999999999999999"
        expected_events += [
            Event(
                "fee",
                [{"tx_id": "TODO"}, {"coins": f"156753 {CACAO}"}, {"pool_deduct": "0"}],
            ),
            Event(
                "refund",
                [{"code": "108"}, {"reason": reason}, *tx.get_attributes()],
            ),
        ]
        self.assertEqual(events, expected_events)

        # swap with custom address
        tx.coins = [Coin(CACAO, 500000000)]
        tx.memo = "SWAP:BNB.BNB:NOMNOM:"
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 1)
        self.assertEqual(outbound[0].memo, "OUT:TODO")
        self.assertEqual(outbound[0].to_address, "NOMNOM")

        # check swap event generated for successful swap
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "112500 BNB.BNB"},
                    {"pool_deduct": "182802"},
                ],
            ),
            Event(
                "swap",
                [
                    {"pool": "BNB.BNB"},
                    {"swap_target": "0"},
                    {"swap_slip": "769"},
                    {"liquidity_fee": "25479186"},
                    {"liquidity_fee_in_cacao": "35504005"},
                    {"emit_asset": "305739584 BNB.BNB"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(events, expected_events)

        # refund swap when address is a different network
        tx.coins = [Coin(CACAO, 500000000)]
        tx.memo = "SWAP:BNB.BNB:BNBNOMNOM"
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 1)
        self.assertEqual(outbound[0].memo, "REFUND:TODO")

        # check refund event generated for swap with different network
        expected_events += [
            Event(
                "fee",
                [{"tx_id": "TODO"}, {"coins": f"182792 {CACAO}"}, {"pool_deduct": "0"}],
            ),
            Event(
                "refund",
                [
                    {"code": "105"},
                    {"reason": "BNBNOMNOM is not recognizable"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(events, expected_events)

        # do a double swap
        mayachain.pools = [
            Pool("BNB.LOK-3C0", 30 * Coin.ONE, 30 * Coin.ONE),
            Pool("BNB.BNB", 50 * Coin.ONE, 50 * Coin.ONE),
        ]

        tx.coins = [Coin("BNB.BNB", 10 * Coin.ONE)]
        tx.memo = "SWAP:BNB.LOK-3C0"

        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 1)
        self.assertEqual(outbound[0].memo, "OUT:TODO")
        self.assertEqual(outbound[0].coins[0], Coin("BNB.LOK-3C0", 457856148))

        # check 2 swap events generated for double swap
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "55548 BNB.LOK-3C0"},
                    {"pool_deduct": "80728"},
                ],
            ),
            Event(
                "outbound",
                [
                    {"in_tx_id": tx.id},
                    {"id": Transaction.empty_id},
                    {"chain": CACAO.get_chain()},
                    {"from": "PROVIDER-1"},
                    {"to": "VAULT"},
                    {"coin": f"694444444 {CACAO}"},
                    {"memo": "SWAP:BNB.LOK-3C0"},
                ],
            ),
            Event(
                "swap",
                [
                    {"pool": "BNB.BNB"},
                    {"swap_target": "0"},
                    {"swap_slip": "1667"},
                    {"liquidity_fee": "138888888"},
                    {"liquidity_fee_in_cacao": "138888888"},
                    {"emit_asset": "694444444 BNB.CACAO-67C"},
                    *tx.get_attributes(),
                ],
            ),
            Event(
                "swap",
                [
                    {"pool": "BNB.LOK-3C0"},
                    {"swap_target": "0"},
                    {"swap_slip": "1880"},
                    {"liquidity_fee": "105998077"},
                    {"liquidity_fee_in_cacao": "105998077"},
                    {"emit_asset": "457911696 BNB.LOK-3C0"},
                    {"id": "TODO"},
                    {"chain": CACAO.get_chain()},
                    {"from": "PROVIDER-1"},
                    {"to": "VAULT"},
                    {"coin": f"694444444 {CACAO}"},
                    {"memo": "SWAP:BNB.LOK-3C0"},
                ],
            ),
        ]
        self.assertEqual(events, expected_events)

    def test_swap_native(self):
        if CACAO.get_chain() == "BNB":
            return

        mayachain = MayachainState()
        mayachain.network_fees = {"BNB": 37500}
        events = mayachain.events

        # no pool, should emit a refund
        tx = Transaction(
            Mayachain.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin(CACAO, 1000 * Coin.ONE)],
            "SWAP:BNB.BNB",
        )

        outbounds = mayachain.handle(tx)
        self.assertEqual(len(outbounds), 1)

        # check refund event generated for swap with native CACAO
        expected_events = [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "2000000000 MAYA.CACAO"},
                    {"pool_deduct": "0"},
                ],
            ),
            Event(
                "refund",
                [
                    {"code": "108"},
                    {"reason": "BNB.BNB pool doesn't exist"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(events, expected_events)

        # init pool
        mayachain.pools = [
            Pool("BNB.BNB", 50 * Coin.ONE, 50 * Coin.ONE),
            Pool("BNB.LOK-3C0", 50 * Coin.ONE, 50 * Coin.ONE),
        ]
        mayachain.pools[0].lp_units = 50 * Coin.ONE
        mayachain.pools[1].lp_units = 50 * Coin.ONE

        # do a regular swap
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 1)
        self.assertEqual(outbound[0].memo, "OUT:TODO")
        self.assertEqual(outbound[0].coins[0], Coin("BNB.BNB", 226682369))

        # check swap event generated for successful swap
        expected_events += [
            Event(
                "swap",
                [
                    {"pool": "BNB.BNB"},
                    {"swap_target": "0"},
                    {"swap_slip": "9524"},
                    {"liquidity_fee": "4535147392"},
                    {"liquidity_fee_in_cacao": "4535147392"},
                    {"emit_asset": "226757369 BNB.BNB"},
                    {"streaming_swap_quantity": "1"},
                    {"streaming_swap_count": "1"},
                    *tx.get_attributes(),
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "75000 BNB.BNB"},
                    {"pool_deduct": "1649796"},
                ],
            ),
        ]
        self.assertEqual(events, expected_events)

        # swap with two coins on the inbound tx
        tx.chain = "BNB"
        tx.coins = [Coin("BNB.BNB", 1000000000),
                    Coin("BNB.LOK-3C0", 1000000000)]
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 2)
        self.assertEqual(outbound[0].coins[0], Coin("BNB.LOK-3C0", 998350282))
        self.assertEqual(outbound[1].coins[0], Coin("BNB.BNB", 999925000))

        # check refund event generated for swap with two coins
        reason = "not expecting multiple coins in a swap: unknown request"
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "75000 BNB.BNB"},
                    {"pool_deduct": "1649744"},
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "1649718 BNB.LOK-3C0"},
                    {"pool_deduct": "1649174"},
                ],
            ),
            Event(
                "refund",
                [{"code": "105"}, {"reason": reason}, *tx.get_attributes()],
            ),
        ]
        self.assertEqual(events, expected_events)

        # swap with zero return, refunds and doesn't change pools
        tx.chain = "MAYA"
        tx.coins = [Coin(CACAO, 1)]
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 0)
        self.assertEqual(mayachain.pools[0].cacao_balance, 104996700460)

        # check refund event generated for swap with zero return
        expected_events += [
            Event(
                "refund",
                [
                    {"code": "108"},
                    {"reason": "fail swap, not enough fee"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(events, expected_events)

        # swap with zero return, not enough coin to pay fee so no refund
        tx.coins = [Coin(CACAO, 1)]
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 0)
        self.assertEqual(mayachain.pools[0].cacao_balance, 104996700460)

        # check refund event generated for swap with zero return
        expected_events += [
            Event(
                "refund",
                [
                    {"code": "108"},
                    {"reason": "not expecting multiple coins in a swap: unknown request"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(events, expected_events)

        # swap with limit
        tx.coins = [Coin(CACAO, 50000000000)]
        tx.memo = "SWAP:BNB.BNB::999999999999999999999"
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 1)
        self.assertEqual(outbound[0].memo, "REFUND:TODO")
        self.assertEqual(outbound[0].coins, [Coin(CACAO, 48000000000)])
        self.assertEqual(mayachain.pools[0].cacao_balance, 104996700460)

        # check refund event generated for swap with limit
        reason = "emit asset 1043104713 less than price limit 999999999999999999999"
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": f"2000000000 {CACAO}"},
                    {"pool_deduct": "0"},
                ],
            ),
            Event(
                "refund",
                [{"code": "108"}, {"reason": reason}, *tx.get_attributes()],
            ),
        ]
        self.assertEqual(events, expected_events)

        # swap with custom address
        tx.coins = [Coin(CACAO, 500000000)]
        tx.memo = "SWAP:BNB.BNB:NOMNOM:"
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 1)
        self.assertEqual(outbound[0].memo, "OUT:TODO")
        self.assertEqual(outbound[0].to_address, "NOMNOM")

        # check swap event generated for successful swap
        expected_events += [
            Event(
                "swap",
                [
                    {"pool": "BNB.BNB"},
                    {"swap_target": "0"},
                    {"swap_slip": "47"},
                    {"liquidity_fee": "107223"},
                    {"liquidity_fee_in_cacao": "2358503"},
                    {"emit_asset": "22516198 BNB.BNB"},
                    {"streaming_swap_quantity": "1"},
                    {"streaming_swap_count": "1"},
                    *tx.get_attributes(),
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "75000 BNB.BNB"},
                    {"pool_deduct": "1665404"},
                ],
            ),
        ]
        self.assertEqual(events, expected_events)

        # refund swap when address is a different network
        tx.coins = [Coin(CACAO, 50000000000)]
        tx.memo = "SWAP:BNB.BNB:BNBNOMNOM"
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 1)
        self.assertEqual(outbound[0].memo, "REFUND:TODO")

        # check refund event generated for swap with different network
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": f"2000000000 {CACAO}"},
                    {"pool_deduct": "0"},
                ],
            ),
            Event(
                "refund",
                [
                    {"code": "105"},
                    {"reason": "BNBNOMNOM is not recognizable"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(events, expected_events)

        # do a double swap
        mayachain.pools = [
            Pool("BNB.BNB", 50 * Coin.ONE, 50 * Coin.ONE),
            Pool("BNB.LOK-3C0", 50 * Coin.ONE, 50 * Coin.ONE),
        ]
        mayachain.pools[0].lp_units = 50 * Coin.ONE
        mayachain.pools[1].lp_units = 50 * Coin.ONE
        tx.chain = "BNB"
        tx.coins = [Coin("BNB.BNB", 1000000000)]
        tx.memo = "SWAP:BNB.LOK-3C0"
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 1)
        self.assertEqual(outbound[0].memo, "OUT:TODO")
        self.assertEqual(outbound[0].coins[0], Coin("BNB.LOK-3C0", 535353401))

        # check 2 swap events generated for double swap
        expected_events += [
            Event(
                "swap",
                [
                    {"pool": "BNB.BNB"},
                    {"swap_target": "0"},
                    {"swap_slip": "1667"},
                    {"liquidity_fee": "138888888"},
                    {"liquidity_fee_in_cacao": "138888888"},
                    {"emit_asset": "694444444 MAYA.CACAO"},
                    {"streaming_swap_quantity": "1"},
                    {"streaming_swap_count": "1"},
                    *tx.get_attributes(),
                ],
            ),
            Event(
                "swap",
                [
                    {"pool": "BNB.LOK-3C0"},
                    {"swap_target": "0"},
                    {"swap_slip": "1220"},
                    {"liquidity_fee": "74360499"},
                    {"liquidity_fee_in_cacao": "74360499"},
                    {"emit_asset": "535395597 BNB.LOK-3C0"},
                    {"streaming_swap_quantity": "1"},
                    {"streaming_swap_count": "1"},
                    {"id": "TODO"},
                    {"chain": "BNB"},
                    {"from": "PROVIDER-1"},
                    {"to": "VAULT"},
                    {"coin": f"694444444 {CACAO}"},
                    {"memo": "SWAP:BNB.LOK-3C0"},
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "42196 BNB.LOK-3C0"},
                    {"pool_deduct": "53819"},
                ],
            ),
        ]
        self.assertEqual(events, expected_events)

    def test_donate_bep2(self):
        if CACAO.get_chain() == "MAYA":
            return

        mayachain = MayachainState()
        mayachain.network_fees = {"BNB": 37500}

        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 150000000), Coin(CACAO, 50000000000)],
            "DONATE:BNB.BNB",
        )

        outbound = mayachain.handle(tx)
        self.assertEqual(outbound, [])

        # check event generated for successful add
        expected_events = [
            Event("donate", [{"pool": "BNB.BNB"}, *tx.get_attributes()]),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # bad add memo should refund
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 150000000), Coin(CACAO, 50000000000)],
            "DONATE:",
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 2)

        # check refund event generated for add with bad memo
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "112500 BNB.BNB"},
                    {"pool_deduct": "37471896"},
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "37443813 BNB.CACAO-67C"},
                    {"pool_deduct": "0"},
                ],
            ),
            Event(
                "refund",
                [{"code": "105"}, {"reason": "Invalid symbol"}, *tx.get_attributes()],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # mismatch asset and memo
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 150000000), Coin(CACAO, 50000000000)],
            "DONATE:BNB.TCAN-014",
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 2)

        # check refund event generated for add with mismatch asset and memo
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "112500 BNB.BNB"},
                    {"pool_deduct": "37443813"},
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "37387711 BNB.CACAO-67C"},
                    {"pool_deduct": "0"},
                ],
            ),
            Event(
                "refund",
                [{"code": "105"}, {"reason": "Invalid symbol"}, *tx.get_attributes()],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # cannot add with cacao in memo
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 150000000), Coin(CACAO, 50000000000)],
            f"DONATE:{CACAO}",
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 2)

        # check refund event generated for add with cacao in memo
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "112500 BNB.BNB"},
                    {"pool_deduct": "37387711"},
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "37331755 BNB.CACAO-67C"},
                    {"pool_deduct": "0"},
                ],
            ),
            Event(
                "refund",
                [{"code": "105"}, {"reason": "Invalid symbol"}, *tx.get_attributes()],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # cannot add with > 2 coins
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [
                Coin("BNB.BNB", 150000000),
                Coin(CACAO, 50000000000),
                Coin("BNB-LOK-3C0", 30000000000),
            ],
            "DONATE:BNB.BNB",
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 2)

        # check refund event generated for add with > 2 coins
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "112500 BNB.BNB"},
                    {"pool_deduct": "37331755"},
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "37275925 BNB.CACAO-67C"},
                    {"pool_deduct": "0"},
                ],
            ),
            Event(
                "refund",
                [
                    {"code": "105"},
                    {"reason": "refund reason message"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

    def test_donate_native(self):
        if CACAO.get_chain() == "BNB":
            return
        mayachain = MayachainState()
        mayachain.network_fees = {"BNB": 37500}

        tx = Transaction(
            Mayachain.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin(CACAO, 50000000000)],
            "DONATE:BNB.BNB",
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(outbound, [])

        expected_events = [
            Event("donate", [{"pool": "BNB.BNB"}, *tx.get_attributes()]),
        ]
        self.assertEqual(mayachain.events, expected_events)

        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 150000000)],
            "DONATE:BNB.BNB",
        )

        outbound = mayachain.handle(tx)
        self.assertEqual(outbound, [])

        # check event generated for successful add
        expected_events += [
            Event("donate", [{"pool": "BNB.BNB"}, *tx.get_attributes()]),
        ]
        self.assertEqual(mayachain.events, expected_events)

    def test_reserve_bep2(self):
        if CACAO.get_chain() == "MAYA":
            return
        mayachain = MayachainState()
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin(CACAO, 50000000000)],
            "RESERVE",
        )

        outbound = mayachain.handle(tx)
        self.assertEqual(outbound, [])

        self.assertEqual(mayachain.reserve, 50000000000)

        # check event generated for successful reserve
        expected_events = [
            Event(
                "reserve",
                [
                    {"contributor_address": tx.from_address},
                    {"amount": tx.coins[0].amount},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

    def test_reserve_native(self):
        if CACAO.get_chain() == "BNB":
            return
        mayachain = MayachainState()
        tx = Transaction(
            Mayachain.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin(CACAO, 50000000000)],
            "RESERVE",
        )

        outbound = mayachain.handle(tx)
        self.assertEqual(outbound, [])
        self.assertEqual(mayachain.reserve, 52000000000)

        # check event generated for successful reserve
        expected_events = [
            Event(
                "reserve",
                [
                    {"contributor_address": tx.from_address},
                    {"amount": tx.coins[0].amount},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

    def test_gas_bep2(self):
        if CACAO.get_chain() == "MAYA":
            return
        mayachain = MayachainState()
        mayachain.network_fees = {"BNB": 37500}

        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 150000000), Coin(CACAO, 50000000000)],
            "ADD:BNB.BNB:PROVIDER-1",
        )

        outbound = mayachain.handle(tx)
        self.assertEqual(outbound, [])

        pool = mayachain.get_pool("BNB.BNB")
        self.assertEqual(pool.cacao_balance, 50000000000)
        self.assertEqual(pool.asset_balance, 150000000)
        self.assertEqual(pool.get_liquidity_provider(
            "PROVIDER-1").units, 50000000000)
        self.assertEqual(pool.lp_units, 50000000000)

        # check event generated for successful liquidity provision
        expected_events = [
            Event("pool", [{"pool": pool.asset},
                  {"pool_status": "Available"}]),
            Event(
                "add_liquidity",
                [
                    {"pool": pool.asset},
                    {"liquidity_provider_units": pool.lp_units},
                    {"cacao_address": tx.from_address},
                    {"cacao_amount": "50000000000"},
                    {"asset_amount": "150000000"},
                    {"asset_address": "PROVIDER-1"},
                    {"BNB_txid": "TODO"},
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # should refund if no memo
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 150000000), Coin(CACAO, 50000000000)],
            "",
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 2)

        # check refund event generated for provider with no memo
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "112500 BNB.BNB"},
                    {"pool_deduct": "37471896"},
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "37443813 BNB.CACAO-67C"},
                    {"pool_deduct": "0"},
                ],
            ),
            Event(
                "refund",
                [
                    {"code": "105"},
                    {"reason": "memo can't be empty"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # check gas event generated after we sent to chain
        outbound[0].gas = [Coin("BNB.BNB", 37500)]
        outbound[1].gas = [Coin("BNB.BNB", 37500)]
        mayachain.handle_gas(outbound)

        # first new gas event
        expected_events += [
            Event(
                "gas",
                [
                    {"asset": "BNB.BNB"},
                    {"asset_amt": "75000"},
                    {"cacao_amt": "24975019"},
                    {"transaction_count": "2"},
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

    def test_gas_native(self):
        if CACAO.get_chain() == "BNB":
            return
        mayachain = MayachainState()
        mayachain.network_fees = {"BNB": 37500}

        tx = Transaction(
            Mayachain.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin(CACAO, 50000000000)],
            "ADD:BNB.BNB:PROVIDER-1",
        )

        outbound = mayachain.handle(tx)
        self.assertEqual(outbound, [])

        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 150000000)],
            "ADD:BNB.BNB:PROVIDER-1",
        )

        outbound = mayachain.handle(tx)
        self.assertEqual(outbound, [])

        pool = mayachain.get_pool("BNB.BNB")
        self.assertEqual(pool.cacao_balance, 50000000000)
        self.assertEqual(pool.asset_balance, 150000000)
        self.assertEqual(pool.get_liquidity_provider(
            "PROVIDER-1").units, 50000000000)
        self.assertEqual(pool.lp_units, 50000000000)

        # check event generated for successful liquidity provision
        expected_events = [
            Event(
                "pending_liquidity",
                [
                    {"pool": pool.asset},
                    {"type": "add"},
                    {"cacao_address": tx.from_address},
                    {"cacao_amount": 50000000000},
                    {"asset_amount": 0},
                    {"asset_address": "PROVIDER-1"},
                    {"MAYA_txid": tx.id},
                ],
            ),
            Event("pool", [{"pool": pool.asset},
                  {"pool_status": "Available"}]),
            Event(
                "add_liquidity",
                [
                    {"pool": pool.asset},
                    {"liquidity_provider_units": pool.lp_units},
                    {"cacao_address": tx.from_address},
                    {"cacao_amount": "50000000000"},
                    {"asset_amount": "150000000"},
                    {"asset_address": "PROVIDER-1"},
                    {"BNB_txid": "TODO"},
                    {"MAYA_txid": "TODO"},
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # should refund if no memo
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 150000000), Coin(CACAO, 50000000000)],
            "",
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 2)

        # check refund event generated for provider with no memo
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "75000 BNB.BNB"},
                    {"pool_deduct": "24987506"},
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "24975019 MAYA.CACAO"},
                    {"pool_deduct": "0"},
                ],
            ),
            Event(
                "refund",
                [
                    {"code": "105"},
                    {"reason": "memo can't be empty"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # check gas event generated after we sent to chain
        outbound[0].gas = [Coin("BNB.BNB", 37500)]
        outbound[1].gas = [Coin("BNB.BNB", 37500)]
        mayachain.handle_gas(outbound)

        # first new gas event
        expected_events += [
            Event(
                "gas",
                [
                    {"asset": "BNB.BNB"},
                    {"asset_amt": "75000"},
                    {"cacao_amt": "24975019"},
                    {"transaction_count": "2"},
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

    def test_liquidity_provider_bep2(self):
        if CACAO.get_chain() == "MAYA":
            return

        mayachain = MayachainState()
        mayachain.network_fees = {"BNB": 37500}

        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 150000000), Coin(CACAO, 50000000000)],
            "ADD:BNB.BNB:PROVIDER-1",
        )

        outbound = mayachain.handle(tx)
        self.assertEqual(outbound, [])

        pool = mayachain.get_pool("BNB.BNB")
        self.assertEqual(pool.cacao_balance, 50000000000)
        self.assertEqual(pool.asset_balance, 150000000)
        self.assertEqual(pool.get_liquidity_provider(
            "PROVIDER-1").units, 50000000000)
        self.assertEqual(pool.lp_units, 50000000000)

        # check event generated for successful liquidity provision
        expected_events = [
            Event("pool", [{"pool": pool.asset},
                  {"pool_status": "Available"}]),
            Event(
                "add_liquidity",
                [
                    {"pool": pool.asset},
                    {"liquidity_provider_units": pool.lp_units},
                    {"cacao_address": tx.from_address},
                    {"cacao_amount": "50000000000"},
                    {"asset_amount": "150000000"},
                    {"asset_address": "PROVIDER-1"},
                    {"BNB_txid": "TODO"},
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # should refund if no memo
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 150000000), Coin(CACAO, 50000000000)],
            "",
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 2)

        # check refund event generated for provider with no memo
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "112500 BNB.BNB"},
                    {"pool_deduct": "37471896"},
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "37443813 BNB.CACAO-67C"},
                    {"pool_deduct": "0"},
                ],
            ),
            Event(
                "refund",
                [
                    {"code": "105"},
                    {"reason": "memo can't be empty"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # bad provision memo should refund
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 150000000), Coin(CACAO, 50000000000)],
            "ADD:",
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 2)

        # check refund event generated for provider with bad memo
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "112500 BNB.BNB"},
                    {"pool_deduct": "37443813"},
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "37387711 BNB.CACAO-67C"},
                    {"pool_deduct": "0"},
                ],
            ),
            Event(
                "refund",
                [{"code": "105"}, {"reason": "Invalid symbol"}, *tx.get_attributes()],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # mismatch asset and memo
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 150000000), Coin(CACAO, 50000000000)],
            "ADD:BNB.TCAN-014",
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 2)

        # check refund event generated for provider with mismatch asset and memo
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "112500 BNB.BNB"},
                    {"pool_deduct": "37387711"},
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "37331755 BNB.CACAO-67C"},
                    {"pool_deduct": "0"},
                ],
            ),
            Event(
                "refund",
                [
                    {"code": "105"},
                    {"reason": "did not find both coins: unknown request"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # cannot provide liquidity with cacao in memo
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 150000000), Coin(CACAO, 50000000000)],
            "ADD:" + CACAO,
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 2)

        # check refund event generated for provider with cacao in memo
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "112500 BNB.BNB"},
                    {"pool_deduct": "37331755"},
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "37275925 BNB.CACAO-67C"},
                    {"pool_deduct": "0"},
                ],
            ),
            Event(
                "refund",
                [
                    {"code": "105"},
                    {"reason": "asset cannot be cacao: unknown request"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # cannot provide liquidity with > 2 coins
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [
                Coin("BNB.BNB", 150000000),
                Coin(CACAO, 50000000000),
                Coin("BNB-LOK-3C0", 30000000000),
            ],
            "ADD:BNB.BNB:PROVIDER-1",
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 2)

        # check refund event generated for provider with > 2 coins
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "112500 BNB.BNB"},
                    {"pool_deduct": "37275925"},
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "37220220 BNB.CACAO-67C"},
                    {"pool_deduct": "0"},
                ],
            ),
            Event(
                "refund",
                [
                    {"code": "105"},
                    {"reason": "refund reason message"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # can provide liquidity with only asset
        tx = Transaction(
            Binance.chain,
            "PROVIDER-2",
            "VAULT",
            [Coin("BNB.BNB", 30000000)],
            "ADD:BNB.BNB",
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 0)
        self.assertEqual(pool.get_liquidity_provider(
            "PROVIDER-2").units, 4153686396)
        self.assertEqual(pool.lp_units, 54153686396)

        # check event generated for successful provide liquidity
        expected_events += [
            Event(
                "add_liquidity",
                [
                    {"pool": pool.asset},
                    {"liquidity_provider_units": "4153686396"},
                    {"cacao_address": "PROVIDER-2"},
                    {"cacao_amount": "0"},
                    {"asset_amount": "30000000"},
                    {"asset_address": "PROVIDER-2"},
                    {"BNB_txid": "TODO"},
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin(CACAO, 10000000000)],
            "ADD:BNB.BNB:PROVIDER-1",
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 0)

        expected_events += [
            Event(
                "add_liquidity",
                [
                    {"pool": pool.asset},
                    {"liquidity_provider_units": "4657084839"},
                    {"cacao_address": "PROVIDER-1"},
                    {"cacao_amount": "10000000000"},
                    {"asset_amount": "0"},
                    {"asset_address": "PROVIDER-1"},
                    {"BNB_txid": "TODO"},
                ],
            ),
        ]

        self.assertEqual(mayachain.events, expected_events)

        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin(CACAO, 30000000000), Coin("BNB.BNB", 90000000)],
            "ADD:BNB.BNB:PROVIDER-1",
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 0)

        # check event generated for successful provide liquidity
        if CACAO.get_chain() == "BNB":
            expected_events += [
                Event(
                    "add_liquidity",
                    [
                        {"pool": pool.asset},
                        {"liquidity_provider_units": "29374965503"},
                        {"cacao_address": "PROVIDER-1"},
                        {"cacao_amount": "30000000000"},
                        {"asset_amount": "90000000"},
                        {"asset_address": "PROVIDER-1"},
                        {"BNB_txid": "TODO"},
                    ],
                ),
            ]
            self.assertEqual(mayachain.events, expected_events)

    def test_liquidity_provider_native(self):
        if CACAO.get_chain() == "BNB":
            return

        mayachain = MayachainState()
        mayachain.network_fees = {"BNB": 37500}

        tx = Transaction(
            Mayachain.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin(CACAO, 50000000000)],
            "ADD:BNB.BNB:PROVIDER-1",
        )

        outbound = mayachain.handle(tx)
        self.assertEqual(outbound, [])

        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 150000000)],
            "ADD:BNB.BNB:PROVIDER-1",
        )

        outbound = mayachain.handle(tx)
        self.assertEqual(outbound, [])

        pool = mayachain.get_pool("BNB.BNB")
        self.assertEqual(pool.cacao_balance, 50000000000)
        self.assertEqual(pool.asset_balance, 150000000)
        self.assertEqual(pool.get_liquidity_provider(
            "PROVIDER-1").units, 50000000000)
        self.assertEqual(pool.lp_units, 50000000000)

        # check event generated for successful provide liquidity
        expected_events = [
            Event(
                "pending_liquidity",
                [
                    {"pool": pool.asset},
                    {"type": "add"},
                    {"cacao_address": tx.from_address},
                    {"cacao_amount": 50000000000},
                    {"asset_amount": 0},
                    {"asset_address": "PROVIDER-1"},
                    {"MAYA_txid": tx.id},
                ],
            ),
            Event("pool", [{"pool": pool.asset},
                  {"pool_status": "Available"}]),
            Event(
                "add_liquidity",
                [
                    {"pool": pool.asset},
                    {"liquidity_provider_units": pool.lp_units},
                    {"cacao_address": tx.from_address},
                    {"cacao_amount": "50000000000"},
                    {"asset_amount": "150000000"},
                    {"asset_address": "PROVIDER-1"},
                    {"BNB_txid": "TODO"},
                    {"MAYA_txid": "TODO"},
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # should refund if no memo
        tx = Transaction(
            Mayachain.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin(CACAO, 50000000000)],
            "",
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 1)

        # check refund event generated for provider with no memo
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "2000000000 MAYA.CACAO"},
                    {"pool_deduct": "0"},
                ],
            ),
            Event(
                "refund",
                [
                    {"code": "105"},
                    {"reason": "memo can't be empty"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 150000000)],
            "",
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 1)

        # check refund event generated for provider with no memo
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "75000 BNB.BNB"},
                    {"pool_deduct": "24987506"},
                ],
            ),
            Event(
                "refund",
                [
                    {"code": "105"},
                    {"reason": "memo can't be empty"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

    def test_withdraw_bep2(self):

        if CACAO.get_chain() == "MAYA":
            return

        mayachain = MayachainState()
        mayachain.network_fees = {"BNB": 37500}
        mayachain.pools = [Pool("BNB.BNB", 50 * Coin.ONE, 50 * Coin.ONE)]

        # provide some funds into a pool
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 1.5 * Coin.ONE), Coin(CACAO, 500 * Coin.ONE)],
            "ADD:BNB.BNB:PROVIDER-1",
        )
        outbounds = mayachain.handle(tx)
        self.assertEqual(outbounds, [])

        pool = mayachain.get_pool("BNB.BNB")
        self.assertEqual(pool.cacao_balance, 55000000000)
        self.assertEqual(pool.asset_balance, 5150000000)
        self.assertEqual(pool.get_liquidity_provider(
            "PROVIDER-1").units, 50000000000)
        self.assertEqual(pool.lp_units, 50000000000)

        expected_events = [
            Event("pool", [{"pool": "BNB.BNB"}, {"pool_status": "Available"}]),
            Event(
                "add_liquidity",
                [
                    {"pool": "BNB.BNB"},
                    {"liquidity_provider_units": "50000000000"},
                    {"cacao_address": "PROVIDER-1"},
                    {"cacao_amount": "50000000000"},
                    {"asset_amount": "150000000"},
                    {"asset_address": "PROVIDER-1"},
                    {"BNB_txid": "TODO"},
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin(CACAO, 1)],
            "WITHDRAW:BNB.BNB:100",
        )
        outbounds = mayachain.handle(tx)
        self.assertEqual(len(outbounds), 2)
        self.assertEqual(outbounds[0].coins[0], Coin("BNB.BNB", 51387500))
        self.assertEqual(outbounds[1].coins[0], Coin(CACAO, 548798597))

        pool = mayachain.get_pool("BNB.BNB")
        self.assertEqual(pool.cacao_balance, 54448798570)
        self.assertEqual(pool.asset_balance, 5098612500)
        self.assertEqual(pool.get_liquidity_provider(
            "PROVIDER-1").units, 49500000000)
        self.assertEqual(pool.lp_units, 49500000000)

        # check event generated for successful withdraw
        expected_events += [
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "112500 BNB.BNB"},
                    {"pool_deduct": "1201430"},
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "1201403 BNB.CACAO-67C"},
                    {"pool_deduct": "0"},
                ],
            ),
            Event(
                "withdraw",
                [
                    {"pool": "BNB.BNB"},
                    {"liquidity_provider_units": "500000000"},
                    {"basis_points": "100"},
                    {"asymmetry": "0.000000000000000000"},
                    {"emit_asset": "51500000"},
                    {"emit_cacao": "550000000"},
                    {"imp_loss_protection": "0"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # should error without a refund
        # but because 1 CACAO is not enough to pay the fee
        # nothing is returned
        tx.memo = "WITHDRAW:"
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 0)

        # check refund event not generated for withdraw with bad memo
        expected_events += [
            Event(
                "fee",
                [{"tx_id": "TODO"}, {"coins": "1 BNB.CACAO-67C"}, {"pool_deduct": "0"}],
            )
        ]
        self.assertEqual(mayachain.events, expected_events)

        # should error without a bad withdraw basis points, should be between 0
        # and 10,000
        tx.memo = "WITHDRAW::-4"
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 0)

        # check refund event not generated for withdraw with bad withdraw basis points
        expected_events += [
            Event(
                "fee",
                [{"tx_id": "TODO"}, {"coins": "1 BNB.CACAO-67C"}, {"pool_deduct": "0"}],
            )
        ]
        self.assertEqual(mayachain.events, expected_events)

        tx.memo = "WITHDRAW::1000000000"
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 0)

        # check refund event not generated for withdraw with bad memo
        expected_events += [
            Event(
                "fee",
                [{"tx_id": "TODO"}, {"coins": "1 BNB.CACAO-67C"}, {"pool_deduct": "0"}],
            )
        ]
        self.assertEqual(mayachain.events, expected_events)

        # check successful withdraw everything
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin(CACAO, 1)],
            "WITHDRAW:BNB.BNB",
        )
        outbounds = mayachain.handle(tx)
        self.assertEqual(len(outbounds), 2)
        self.assertEqual(outbounds[0].coins[0], Coin(CACAO, 54348798544))
        self.assertEqual(outbounds[1].coins[0], Coin("BNB.BNB", 5098537500))

        pool = mayachain.get_pool("BNB.BNB")
        self.assertEqual(pool.cacao_balance, 0)
        self.assertEqual(pool.asset_balance, 75000)
        self.assertEqual(pool.get_liquidity_provider("PROVIDER-1").units, 0)
        self.assertEqual(pool.lp_units, 0)

        # check event generated for successful withdraw
        expected_events += [
            Event("pool", [{"pool": "BNB.BNB"}, {"pool_status": "Staged"}]),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "100000000 BNB.CACAO-67C"},
                    {"pool_deduct": "0"},
                ],
            ),
            Event(
                "withdraw",
                [
                    {"pool": "BNB.BNB"},
                    {"liquidity_provider_units": "49500000000"},
                    {"basis_points": "10000"},
                    {"asymmetry": "0.000000000000000000"},
                    {"emit_asset": "5098537500"},
                    {"emit_cacao": "54448798570"},
                    {"imp_loss_protection": "0"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # check withdraw liquidity provider has 0 units
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin(CACAO, 1)],
            "WITHDRAW:BNB.BNB",
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 0)

        pool = mayachain.get_pool("BNB.BNB")
        self.assertEqual(pool.cacao_balance, 0)
        self.assertEqual(pool.get_liquidity_provider("PROVIDER-1").units, 0)
        self.assertEqual(pool.lp_units, 0)
        self.assertEqual(pool.asset_balance, 75000)

        # check refund event not generated for withdraw with 0 units left
        expected_events += [
            Event(
                "fee",
                [{"tx_id": "TODO"}, {"coins": "1 BNB.CACAO-67C"}, {"pool_deduct": "0"}],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

    def test_withdraw_native(self):

        # check event generated for successful stake
        # only if BNB.CACAO-67C as with native CACAO it would
        # be a cross chain stake and no event on first stake
        if CACAO.get_chain() == "BNB":
            return

        mayachain = MayachainState()
        mayachain.network_fees = {"BNB": 37500}
        mayachain.pools = [Pool("BNB.BNB", 50 * Coin.ONE, 50 * Coin.ONE)]

        # provide some funds into a pool
        tx = Transaction(
            Mayachain.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin(CACAO, 54950 * Coin.ONE)],
            "ADD:BNB.BNB:PROVIDER-1",
        )
        outbounds = mayachain.handle(tx)
        self.assertEqual(outbounds, [])
        tx = Transaction(
            Binance.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin("BNB.BNB", 1.5 * Coin.ONE)],
            "ADD:BNB.BNB:PROVIDER-1",
        )
        outbounds = mayachain.handle(tx)
        self.assertEqual(outbounds, [])

        pool = mayachain.get_pool("BNB.BNB")
        self.assertEqual(pool.cacao_balance, 5500000000000)
        self.assertEqual(pool.asset_balance, 5150000000)
        self.assertEqual(pool.get_liquidity_provider(
            "PROVIDER-1").units, 5495000000000)
        self.assertEqual(pool.lp_units, 5495000000000)

        expected_events = [
            Event(
                "pending_liquidity",
                [
                    {"pool": pool.asset},
                    {"type": "add"},
                    {"cacao_address": tx.from_address},
                    {"cacao_amount": 5495000000000},
                    {"asset_amount": 0},
                    {"asset_address": "PROVIDER-1"},
                    {"MAYA_txid": tx.id},
                ],
            ),
            Event("pool", [{"pool": pool.asset},
                  {"pool_status": "Available"}]),
            Event(
                "add_liquidity",
                [
                    {"pool": pool.asset},
                    {"liquidity_provider_units": "5495000000000"},
                    {"cacao_address": "PROVIDER-1"},
                    {"cacao_amount": "5495000000000"},
                    {"asset_amount": "150000000"},
                    {"asset_address": "PROVIDER-1"},
                    {"BNB_txid": "TODO"},
                    {"MAYA_txid": "TODO"},
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        tx = Transaction(
            Mayachain.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin(CACAO, 1)],
            "WITHDRAW:BNB.BNB:100",
        )
        outbounds = mayachain.handle(tx)
        self.assertEqual(len(outbounds), 2)
        self.assertEqual(outbounds[0].coins[0], Coin(CACAO, 53000000000))
        self.assertEqual(outbounds[1].coins[0], Coin("BNB.BNB", 51425000))

        pool = mayachain.get_pool("BNB.BNB")
        self.assertEqual(pool.cacao_balance, 5444919904091)
        self.assertEqual(pool.asset_balance, 5098575000)
        self.assertEqual(pool.get_liquidity_provider(
            "PROVIDER-1").units, 5440050000000)
        self.assertEqual(pool.lp_units, 5440050000000)

        # check event generated for successful withdraw
        expected_events += [
            Event(
                "withdraw",
                [
                    {"pool": "BNB.BNB"},
                    {"liquidity_provider_units": "54950000000"},
                    {"basis_points": "100"},
                    {"asymmetry": "0.000000000000000000"},
                    {"emit_asset": "51500000"},
                    {"emit_cacao": "55000000000"},
                    {"imp_loss_protection": "0"},
                    *tx.get_attributes(),
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "75000 BNB.BNB"},
                    {"pool_deduct": "80095909"},
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "2000000000 MAYA.CACAO"},
                    {"pool_deduct": "0"},
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # should error without a refund
        # but because 1 CACAO is not enough to pay the fee
        # nothing is returned
        tx.memo = "WITHDRAW:"
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 0)

        # check refund event not generated for withdraw with bad memo
        expected_events += [
            Event(
                "refund",
                [{"code": "105"}, {"reason": "Invalid symbol"}, *tx.get_attributes()],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # should error without a bad withdraw basis points, should be between 0
        # and 10,000
        tx.memo = "WITHDRAW::-4"
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 0)

        # check refund event not generated for withdraw with bad withdraw basis points
        expected_events += [
            Event(
                "refund",
                [{"code": "105"}, {"reason": "Invalid symbol"}, *tx.get_attributes()],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        tx.memo = "WITHDRAW::1000000000"
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 0)

        # check refund event not generated for withdraw with bad memo
        expected_events += [
            Event(
                "refund",
                [{"code": "105"}, {"reason": "Invalid symbol"}, *tx.get_attributes()],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # check successful withdraw everything
        tx = Transaction(
            Mayachain.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin(CACAO, 1)],
            "WITHDRAW:BNB.BNB",
        )
        outbounds = mayachain.handle(tx)
        self.assertEqual(len(outbounds), 2)
        self.assertEqual(outbounds[1].coins[0], Coin("BNB.BNB", 5098537500))
        self.assertEqual(outbounds[0].coins[0],
                         Coin("MAYA.CACAO", 5442919904091))

        pool = mayachain.get_pool("BNB.BNB")
        self.assertEqual(pool.cacao_balance, 0)
        self.assertEqual(pool.get_liquidity_provider("PROVIDER-1").units, 0)
        self.assertEqual(pool.lp_units, 0)
        self.assertEqual(pool.asset_balance, 37500)

        # check event generated for successful withdraw
        expected_events += [
            Event("pool", [{"pool": "BNB.BNB"}, {"pool_status": "Staged"}]),
            Event(
                "withdraw",
                [
                    {"pool": "BNB.BNB"},
                    {"liquidity_provider_units": "5440050000000"},
                    {"basis_points": "10000"},
                    {"asymmetry": "0.000000000000000000"},
                    {"emit_asset": "5098537500"},
                    {"emit_cacao": "5444919904091"},
                    {"imp_loss_protection": "0"},
                    *tx.get_attributes(),
                ],
            ),
            Event(
                "fee",
                [
                    {"tx_id": "TODO"},
                    {"coins": "2000000000 MAYA.CACAO"},
                    {"pool_deduct": "0"},
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

        # check withdraw liquidity provider has 0 units
        tx = Transaction(
            Mayachain.chain,
            "PROVIDER-1",
            "VAULT",
            [Coin(CACAO, 1)],
            "WITHDRAW:BNB.BNB",
        )
        outbound = mayachain.handle(tx)
        self.assertEqual(len(outbound), 0)

        pool = mayachain.get_pool("BNB.BNB")
        self.assertEqual(pool.cacao_balance, 0)
        self.assertEqual(pool.get_liquidity_provider("PROVIDER-1").units, 0)
        self.assertEqual(pool.lp_units, 0)
        self.assertEqual(pool.asset_balance, 37500)

        # check refund event not generated for withdraw with 0 units left
        expected_events += [
            Event(
                "refund",
                [
                    {"code": "105"},
                    {"reason": "refund reason message"},
                    *tx.get_attributes(),
                ],
            ),
        ]
        self.assertEqual(mayachain.events, expected_events)

    def test_withdraw_calc(self):
        pool = Pool("BNB.BNB", 112928660551, 257196272)
        pool.lp_units = 44611997190
        after, withdraw_cacao, withdraw_asset = pool._calc_withdraw_units(
            25075000000, 5000
        )
        self.assertEqual(withdraw_cacao, 31736823519)
        self.assertEqual(withdraw_asset, 72280966)
        self.assertEqual(after, 12537500000)

    def test_liquidity_provider_calc(self):
        pool = Pool("BNB.BNB", 112928660551, 257196272)
        liquidity_provider_units = pool._calc_liquidity_units(
            0, 0, 34500000000, 23400000000
        )
        self.assertEqual(liquidity_provider_units, 34500000000)
        pool.lp_units = 34500000000
        liquidity_provider_units = pool._calc_liquidity_units(
            50000000000, 40000000000, 50000000000, 40000000000
        )
        self.assertEqual(liquidity_provider_units, 34500000000)

    def test_calc_liquidity_fee(self):
        mayachain = MayachainState()
        fee = mayachain._calc_liquidity_fee(94382619747, 100001000, 301902607)
        self.assertEqual(fee, 338)
        fee = mayachain._calc_liquidity_fee(
            10000000000, 1000000000, 10000000000)
        self.assertEqual(fee, 82644628)

    def test_calc_swap_slip(self):
        mayachain = MayachainState()
        slip = mayachain._calc_swap_slip(10000000000, 1000000000)
        self.assertEqual(slip, 909)
        slip = mayachain._calc_swap_slip(94405967833, 10000000000)
        self.assertEqual(slip, 958)

    def test_get_asset_in_cacao(self):
        pool = Pool("BNB.BNB", 49900000000, 150225000)
        self.assertEqual(pool.get_asset_in_cacao(75000), 24912631)

        pool = Pool("BNB.BNB", 49824912631, 150450902)
        self.assertEqual(pool.get_asset_in_cacao(75000), 24837794)

    def test_get_asset_fee(self):
        pool = Pool("BNB.BNB", 49900000000, 150225000)
        self.assertEqual(pool.get_asset_fee(), 6021)

    def test_handle_rewards(self):
        mayachain = MayachainState()
        mayachain.pools.append(Pool("BNB.BNB", 94382620747, 301902605))
        mayachain.pools.append(Pool("BNB.LOKI", 50000000000, 100))
        mayachain.reserve = 40001517380253

        # test minus cacao from pools and add to bond rewards (too much rewards to pools)
        mayachain.liquidity["BNB.BNB"] = 105668
        mayachain.handle_rewards()
        self.assertEqual(mayachain.pools[0].cacao_balance, 94382515079)

        # test no swaps this block (no rewards)
        mayachain.handle_rewards()
        self.assertEqual(mayachain.pools[0].cacao_balance, 94382515079)

        # test add cacao to pools (not enough funds to pools)
        mayachain.liquidity["BNB.LOKI"] = 103
        mayachain.total_bonded = 5000000000000
        mayachain.handle_rewards()
        self.assertEqual(mayachain.pools[1].cacao_balance, 49999999897)


class TestEvent(unittest.TestCase):
    def test_get(self):
        swap = Event(
            "swap",
            [
                {"in_tx_id": "FAAFF"},
                {"id": "TODO"},
                {"chain": "BNB"},
                {"from": "tbnb1zge452mgjg9508edxqfpzfl3sfc7vakf2mprqj"},
                {"to": "tbnb189az9plcke2c00vns0zfmllfpfdw67dtv25kgx"},
                {"coin": "149700000 BNB.BNB"},
                {"memo": "REFUND:FAAFF"},
            ],
        )
        txid = swap.get("id")
        self.assertEqual(txid, "TODO")
        memo = swap.get("memo")
        self.assertEqual(memo, "REFUND:FAAFF")
        random = swap.get("random")
        self.assertEqual(random, None)

    def test_eq(self):
        outbound_sim = Event(
            "outbound",
            [
                {"in_tx_id": "FAAFF"},
                {"id": "TODO"},
                {"chain": "BNB"},
                {"from": "tbnb1zge452mgjg9508edxqfpzfl3sfc7vakf2mprqj"},
                {"to": "tbnb189az9plcke2c00vns0zfmllfpfdw67dtv25kgx"},
                {"coin": "149700000 BNB.BNB"},
                {"memo": "REFUND:FAAFF"},
            ],
        )
        outbound = Event(
            "outbound",
            [
                {"in_tx_id": "FAAFF"},
                {"id": "67672"},
                {"chain": "BNB"},
                {"from": "tbnb1zge452mgjg9508edxqfpzfl3sfc7vakf2mprqj"},
                {"to": "tbnb189az9plcke2c00vns0zfmllfpfdw67dtv25kgx"},
                {"coin": "149700000 BNB.BNB"},
                {"memo": "REFUND:FAAFF"},
            ],
        )
        self.assertEqual(outbound_sim, outbound)
        swap_sim = Event(
            "swap",
            [
                {"in_tx_id": "FAAFF"},
                {"id": "TODO"},
                {"chain": "BNB"},
                {"from": "tbnb1zge452mgjg9508edxqfpzfl3sfc7vakf2mprqj"},
                {"to": "tbnb189az9plcke2c00vns0zfmllfpfdw67dtv25kgx"},
                {"coin": "149700000 BNB.BNB"},
                {"memo": "REFUND:FAAFF"},
            ],
        )
        swap = Event(
            "swap",
            [
                {"in_tx_id": "FAAFF"},
                {"id": "67672"},
                {"chain": "BNB"},
                {"from": "tbnb1zge452mgjg9508edxqfpzfl3sfc7vakf2mprqj"},
                {"to": "tbnb189az9plcke2c00vns0zfmllfpfdw67dtv25kgx"},
                {"coin": "149700000 BNB.BNB"},
                {"memo": "REFUND:FAAFF"},
            ],
        )
        self.assertNotEqual(swap_sim, swap)
        swap_sim = Event(
            "swap",
            [
                {"pool": "ETH.ETH-0X0000000000000000000000000000000000000000"},
                {"liquidity_provider_units": "27000000000"},
                {"cacao_address": "tbnb1mkymsmnqenxthlmaa9f60kd6wgr9yjy9h5mz6q"},
                {"cacao_amount": "50000000000"},
                {"asset_amount": "4000000000"},
                {"BNB_txid": "9573683032CBEE28E1A3C01648F"},
                {"ETH_txid": "FBBB33A59B9AA3F787743EC4176"},
            ],
        )
        swap = Event(
            "swap",
            [
                {"pool": "ETH.ETH-0x0000000000000000000000000000000000000000"},
                {"liquidity_provider_units": "27000000000"},
                {"cacao_address": "tbnb1mkymsmnqenxthlmaa9f60kd6wgr9yjy9h5mz6q"},
                {"cacao_amount": "50000000000"},
                {"asset_amount": "4000000000"},
                {"ETH_txid": "FBBB33A59B9AA3F787743EC4176"},
                {"BNB_txid": "9573683032CBEE28E1A3C01648F"},
            ],
        )
        self.assertEqual(swap_sim, swap)

    def test_sorted(self):
        outbound_sim_1 = Event(
            "outbound",
            [
                {"in_tx_id": "FAAFF"},
                {"id": "TODO"},
                {"chain": "BNB"},
                {"from": "tbnb1zge452mgjg9508edxqfpzfl3sfc7vakf2mprqj"},
                {"to": "tbnb189az9plcke2c00vns0zfmllfpfdw67dtv25kgx"},
                {"coin": "149700000 BNB.BNB"},
                {"memo": "REFUND:FAAFF"},
            ],
        )
        outbound_sim_2 = Event(
            "outbound",
            [
                {"in_tx_id": "FAAFF"},
                {"id": "TODO"},
                {"chain": "BNB"},
                {"from": "tbnb1zge452mgjg9508edxqfpzfl3sfc7vakf2mprqj"},
                {"to": "tbnb189az9plcke2c00vns0zfmllfpfdw67dtv25kgx"},
                {"coin": "500000000 BNB.CACAO-67C"},
                {"memo": "REFUND:FAAFF"},
            ],
        )
        sim_events = [outbound_sim_1, outbound_sim_2]
        outbound_1 = Event(
            "outbound",
            [
                {"in_tx_id": "FAAFF"},
                {"id": "47AC6"},
                {"chain": "BNB"},
                {"from": "tbnb1zge452mgjg9508edxqfpzfl3sfc7vakf2mprqj"},
                {"to": "tbnb189az9plcke2c00vns0zfmllfpfdw67dtv25kgx"},
                {"coin": "149700000 BNB.BNB"},
                {"memo": "REFUND:FAAFF"},
            ],
        )
        outbound_2 = Event(
            "outbound",
            [
                {"in_tx_id": "FAAFF"},
                {"id": "E415A"},
                {"chain": "BNB"},
                {"from": "tbnb1zge452mgjg9508edxqfpzfl3sfc7vakf2mprqj"},
                {"to": "tbnb189az9plcke2c00vns0zfmllfpfdw67dtv25kgx"},
                {"coin": "500000000 BNB.CACAO-67C"},
                {"memo": "REFUND:FAAFF"},
            ],
        )
        sim_events = [outbound_sim_1, outbound_sim_2]
        events = [outbound_1, outbound_2]
        self.assertEqual(sim_events, events)
        events = [outbound_2, outbound_1]
        self.assertNotEqual(sim_events, events)
        self.assertEqual(sorted(sim_events), sorted(events))


if __name__ == "__main__":
    unittest.main()
