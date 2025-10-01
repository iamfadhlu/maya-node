from typing import List, cast

from bitcointx.core import (
    CoreCoinClassDispatcher, CoreCoinClass,

    CTransaction, CTxIn, CTxOut, CTxWitness, CTxInWitness,
    CMutableTransaction, CMutableTxIn, CMutableTxOut, CMutableTxWitness,
    CMutableTxInWitness, COutPoint, CMutableOutPoint
)
from .script import ScriptDashClassDispatcher


class CoreDashClassDispatcher(CoreCoinClassDispatcher,
                                  depends=[ScriptDashClassDispatcher]):
    ...


class CoreDashClass(CoreCoinClass, metaclass=CoreDashClassDispatcher):
    ...


class CDashOutPoint(COutPoint, CoreDashClass):
    """Dash COutPoint"""
    __slots__: List[str] = []


class CDashMutableOutPoint(CDashOutPoint, CMutableOutPoint,
                               CoreDashClass, mutable_of=CDashOutPoint):
    """A mutable Dash COutPoint"""
    __slots__: List[str] = []


class CDashTxIn(CTxIn, CoreDashClass):
    """An immutable Dash TxIn"""
    __slots__: List[str] = []


class CDashMutableTxIn(CDashTxIn, CMutableTxIn, CoreDashClass,
                           mutable_of=CDashTxIn):
    """A mutable Dash TxIn"""
    __slots__: List[str] = []


class CDashTxOut(CTxOut, CoreDashClass):
    """A immutable Dash TxOut"""
    __slots__: List[str] = []


class CDashMutableTxOut(CDashTxOut, CMutableTxOut, CoreDashClass,
                            mutable_of=CDashTxOut):
    """A mutable Dash CTxOut"""
    __slots__: List[str] = []


class CDashTxInWitness(CTxInWitness, CoreDashClass):
    """Immutable Dash witness data for a single transaction input"""
    __slots__: List[str] = []


class CDashMutableTxInWitness(CDashTxInWitness, CMutableTxInWitness,
                                  CoreDashClass,
                                  mutable_of=CDashTxInWitness):
    """Mutable Dash witness data for a single transaction input"""
    __slots__: List[str] = []


class CDashTxWitness(CTxWitness, CoreDashClass):
    """Immutable witness data for all inputs to a transaction"""
    __slots__: List[str] = []


class CDashMutableTxWitness(CDashTxWitness, CMutableTxWitness,
                                CoreDashClass,
                                mutable_of=CDashTxWitness):
    """Witness data for all inputs to a transaction, mutable version"""
    __slots__: List[str] = []


class CDashTransaction(CTransaction, CoreDashClass):
    """Dash transaction, mutable version"""
    __slots__: List[str] = []

    def to_mutable(self) -> 'CDashMutableTransaction':
        return cast('CDashMutableTransaction', super().to_mutable())

    def to_immutable(self) -> 'CDashTransaction':
        return cast('CDashTransaction', super().to_immutable())


class CDashMutableTransaction(CDashTransaction, CMutableTransaction,
                                  CoreDashClass,
                                  mutable_of=CDashTransaction):
    """Dash transaction"""
    __slots__: List[str] = []


__all__ = (
    'CDashOutPoint',
    'CDashMutableOutPoint',
    'CDashTxIn',
    'CDashMutableTxIn',
    'CDashTxOut',
    'CDashMutableTxOut',
    'CDashTransaction',
    'CDashMutableTransaction',
    'CDashTxWitness',
    'CDashMutableTxWitness',
    'CDashMutableTxInWitness',
    'CDashTxInWitness',
)
