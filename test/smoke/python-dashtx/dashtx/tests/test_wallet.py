  import unittest

from bitcointx.tests.test_wallet import test_address_implementations

from bitcointx import ChainParams
from bitcointx.wallet import CCoinAddress, CCoinAddressError

from bitcointx.core import Hash160
from bitcointx.core.script import CScript, OP_0

from dashtx import DashMainnetParams
from dashtx.wallet import P2PKHDashAddress, P2PKHDashTestnetAddress, P2PKHDashRegtestAddress


class Test_DashAddress(unittest.TestCase):

    def test_address_implementations(self, paramclasses=None):
        test_address_implementations(self, paramclasses=[DashMainnetParams])

    def test_p2pk(self):
        with ChainParams('dash'):
            a = CCoinAddress('XnaAwMFdebTu1W51q59zf2dzAT3MQRoWSB')
            self.assertIsInstance(a, P2PKHDashAddress)

        with ChainParams('dash/testnet'):
            a = CCoinAddress('yYCmxJL5697yMEzZPvUPh44LSjXitgWyBc')
            self.assertIsInstance(a, P2PKHDashTestnetAddress)

        with ChainParams('dash/regtest'):
            a = CCoinAddress('yYCmxJL5697yMEzZPvUPh44LSjXitgWyBc')
            self.assertIsInstance(a, P2PKHDashRegtestAddress)
