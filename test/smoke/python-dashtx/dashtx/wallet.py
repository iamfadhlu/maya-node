from typing import Type, List

from bitcointx import get_current_chain_params
from bitcointx.util import dispatcher_mapped_list, ClassMappingDispatcher
from bitcointx.wallet import (
    WalletCoinClassDispatcher, WalletCoinClass,
    CCoinAddress, P2SHCoinAddress, P2WSHCoinAddress,
    P2PKHCoinAddress, P2WPKHCoinAddress,
    CBase58CoinAddress,
    CCoinKey, CCoinExtKey, CCoinExtPubKey,
    T_CBase58DataDispatched
)
from .core import CoreDashClassDispatcher


class WalletDashClassDispatcher(WalletCoinClassDispatcher,
                                    depends=[CoreDashClassDispatcher]):
    ...


class WalletDashTestnetClassDispatcher(WalletDashClassDispatcher):
    ...


class WalletDashRegtestClassDispatcher(WalletDashClassDispatcher):
    ...


class WalletDashClass(WalletCoinClass,
                          metaclass=WalletDashClassDispatcher):
    ...


class WalletDashTestnetClass(
    WalletDashClass, metaclass=WalletDashTestnetClassDispatcher
):
    ...


class WalletDashRegtestClass(
    WalletDashClass, metaclass=WalletDashRegtestClassDispatcher
):
    ...


class CDashAddress(CCoinAddress, WalletDashClass):
    ...


class CDashTestnetAddress(CCoinAddress, WalletDashTestnetClass):
    ...


class CDashRegtestAddress(CCoinAddress, WalletDashRegtestClass):
    ...


class CBase58DashAddress(CBase58CoinAddress, CDashAddress):
    @classmethod
    def base58_get_match_candidates(cls: Type[T_CBase58DataDispatched]
                                    ) -> List[Type[T_CBase58DataDispatched]]:
        assert isinstance(cls, ClassMappingDispatcher)
        candidates = dispatcher_mapped_list(cls)
        if P2SHDashAddress in candidates\
                and get_current_chain_params().allow_legacy_p2sh:
            return [P2SHDashLegacyAddress] + candidates
        return super(CBase58DashAddress, cls).base58_get_match_candidates()


class CBase58DashTestnetAddress(CBase58CoinAddress,
                                    CDashTestnetAddress):
    ...


class CBase58DashRegtestAddress(CBase58CoinAddress,
                                    CDashRegtestAddress):
    ...


class P2SHDashAddress(P2SHCoinAddress, CBase58DashAddress):
    base58_prefix = bytes([16])


class P2SHDashLegacyAddress(P2SHCoinAddress, CBase58DashAddress,
                                variant_of=P2SHDashAddress):
    base58_prefix = bytes([16])


class P2PKHDashAddress(P2PKHCoinAddress, CBase58DashAddress):
    base58_prefix = bytes([76])


class P2SHDashTestnetAddress(P2SHCoinAddress,
                                 CBase58DashTestnetAddress):
    base58_prefix = bytes([19])


class P2SHDashTestnetLegacyAddress(P2SHCoinAddress,
                                       CBase58DashTestnetAddress,
                                       variant_of=P2SHDashTestnetAddress):
    base58_prefix = bytes([19])


class P2PKHDashTestnetAddress(P2PKHCoinAddress,
                                  CBase58DashTestnetAddress):
    base58_prefix = bytes([140])


class P2SHDashRegtestAddress(P2SHCoinAddress,
                                 CBase58DashRegtestAddress):
    base58_prefix = bytes([19])


class P2SHDashRegtestLegacyAddress(P2SHCoinAddress,
                                       CBase58DashRegtestAddress,
                                       variant_of=P2SHDashRegtestAddress):
    base58_prefix = bytes([19])


class P2PKHDashRegtestAddress(P2PKHCoinAddress,
                                  CBase58DashRegtestAddress):
    base58_prefix = bytes([140])


class CDashKey(CCoinKey, WalletDashClass):
    base58_prefix = bytes([204])


class CDashTestnetKey(CCoinKey, WalletDashTestnetClass):
    base58_prefix = bytes([239])


class CDashRegtestKey(CCoinKey, WalletDashRegtestClass):
    base58_prefix = bytes([239])


class CDashExtPubKey(CCoinExtPubKey, WalletDashClass):
    base58_prefix = b'\x04\x88\xB2\x1E'


class CDashExtKey(CCoinExtKey, WalletDashClass):
    base58_prefix = b'\x04\x88\xAD\xE4'


class CDashTestnetExtPubKey(CCoinExtPubKey, WalletDashTestnetClass):
    base58_prefix = b'\x04\x35\x87\xCF'


class CDashTestnetExtKey(CCoinExtKey, WalletDashTestnetClass):
    base58_prefix = b'\x04\x35\x83\x94'


class CDashRegtestExtPubKey(CCoinExtPubKey, WalletDashRegtestClass):
    base58_prefix = b'\x04\x35\x87\xCF'


class CDashRegtestExtKey(CCoinExtKey, WalletDashRegtestClass):
    base58_prefix = b'\x04\x35\x83\x94'


__all__ = (
    'CDashAddress',
    'CDashTestnetAddress',
    'CBase58DashAddress',
    'P2SHDashAddress',
    'P2SHDashLegacyAddress',
    'P2PKHDashAddress',
    'P2WSHDashAddress',
    'P2WPKHDashAddress',
    'CBase58DashTestnetAddress',
    'P2SHDashTestnetAddress',
    'P2PKHDashTestnetAddress',
    'P2WSHDashTestnetAddress',
    'P2WPKHDashTestnetAddress',
    'CDashKey',
    'CDashExtKey',
    'CDashExtPubKey',
    'CDashTestnetKey',
    'CDashTestnetExtKey',
    'CDashTestnetExtPubKey',
    'CDashRegtestKey',
    'CDashRegtestExtKey',
    'CDashRegtestExtPubKey',
)
