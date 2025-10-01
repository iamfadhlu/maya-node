from utils.common import get_cacao_asset

CACAO = get_cacao_asset()


class Breakpoint:
    """
    This takes a snapshot picture of the chain(s) and generates json
    """

    def __init__(self, mayachain, bnb):
        self.bnb = bnb
        self.mayachain = mayachain

    def snapshot(self, txID, out):
        """
        Generate a snapshot picture of the bnb and mayachain balances to
        compare
        """
        snap = {
            "TX": txID,
            "OUT": out,
            "CONTRIB": {},
            "USER-1": {},
            "PROVIDER-1": {},
            "PROVIDER-2": {},
            "VAULT": {},
        }

        for name, acct in self.bnb.accounts.items():
            # ignore if is a new name
            if name not in snap:
                continue

            for coin in acct.balances:
                snap[name][str(coin.asset)] = coin.amount

        for pool in self.mayachain.pools:
            snap["POOL." + str(pool.asset)] = {
                str(pool.asset): int(pool.asset_balance),
                CACAO: int(pool.cacao_balance),
            }

        return snap
