from .version import __version__

import dashtx.core
import dashtx.core.script
import dashtx.wallet

from bitcointx import ChainParamsBase


# Declare chain params after frontend classes are regstered in wallet.py,
# so that issubclass checks in ChainParamsMeta.__new__() would pass
class DashMainnetParams(ChainParamsBase,
                            name=('dash', 'dash/mainnet')):
    RPC_PORT = 9998
    WALLET_DISPATCHER = dashtx.wallet.WalletDashClassDispatcher

    def __init__(self, allow_legacy_p2sh: bool = False) -> None:
        super().__init__()
        self.allow_legacy_p2sh = allow_legacy_p2sh


class DashTestnetParams(DashMainnetParams, name='dash/testnet'):
    RPC_PORT = 19998
    WALLET_DISPATCHER = dashtx.wallet.WalletDashTestnetClassDispatcher

    def get_datadir_extra_name(self) -> str:
        return 'testnet3'

    def get_network_id(self) -> str:
        return "test"


class DashRegtestParams(DashMainnetParams, name='dash/regtest'):
    RPC_PORT = 19898
    WALLET_DISPATCHER = dashtx.wallet.WalletDashRegtestClassDispatcher


__all__ = (
    '__version__',
    'DashMainnetParams',
    'DashTestnetParams',
    'DashRegtestParams'
)
