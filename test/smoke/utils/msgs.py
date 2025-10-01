"""Bank module message types."""

from __future__ import annotations

from typing import Any

from bech32 import bech32_decode, convertbits
from terra_sdk.core import AccAddress, AccPubKey
from mayanode_proto.types import MsgDeposit as MsgDeposit_pb
from mayanode_proto.types import MsgSend as MsgSend_pb
from mayanode_proto.cosmos.base.v1beta1 import Coin as Thor_Coin_pb

from utils.common import Coins
from terra_sdk.core.msg import Msg

__all__ = ["MsgDeposit", "MsgSend"]

import attr


@attr.s
class MsgDeposit(Msg):
    """Deposit native assets on mayachain from ``signer`` to
     asgard module with ``coins`` and ``memo``.
    Args:
        coins (Coins): coins to deposit
        memo: memo
        signer (Coins): signer
    """

    type_amino = "mayachain/MsgDeposit"
    """"""
    type_url = "/types.MsgDeposit"
    """"""
    action = "deposit"
    """"""

    coins: Coins = attr.ib(converter=Coins)
    memo: str = attr.ib()
    signer: str = attr.ib()

    @classmethod
    def from_data(cls, data: dict) -> MsgDeposit:
        return cls(
            coins=Coins.from_data(data["coins"]),
            memo=data["memo"],
            signer=data["signer"],
        )

    def to_data(self) -> dict:
        return {
            "@type": self.type_url,
            "coins": self.coins.to_data(),
            "memo": self.memo,
            "signer": self.signer,
        }

    @classmethod
    def from_proto(cls, proto: MsgDeposit_pb) -> MsgDeposit:
        return cls(
            coins=Coins.from_proto(proto["coins"]),
            memo=proto["memo"],
            signer=proto["signer"],
        )

    def to_proto(self) -> MsgDeposit_pb:
        data = bech32_decode(self.signer)[1]
        signer = convertbits(data, 5, 8, False)
        proto = MsgDeposit_pb()
        proto.coins = self.coins.to_proto()
        proto.memo = self.memo
        proto.signer = bytes(signer)
        return proto

    @classmethod
    def unpack_any(cls, any: Any) -> MsgDeposit:
        return MsgDeposit.from_proto(any)

@attr.s
class MsgSend(Msg):
    """Send native assets on thorchain from ``from_address`` to
     ``to_address`` with ``coins`` and ``memo``.
    Args:
        from_address (bytes): sender address as bytes
        to_address (bytes): recipient address as bytes
        amount (Coins): amount to send
    """

    type_amino = "thorchain/MsgSend"
    """"""
    type_url = "/types.MsgSend"
    """"""
    action = "send"
    """"""

    from_address: str = attr.ib()
    to_address: str = attr.ib()
    amount: Coins = attr.ib(converter=Coins)

    def to_amino(self) -> dict:
        return {
            "type": self.type_amino,
            "value": {
                "from_address": self.from_address,
                "to_address": self.to_address,
                "amount": [{"denom": 'rune', "amount": 2000000} for coin in self.amount]
            },
        }

    @classmethod
    def from_data(cls, data: dict) -> MsgSend:
        return cls(
            from_address=data["from_address"],
            to_address=data["to_address"],
            amount=Coins.from_data(data["amount"]),
        )

    def to_data(self) -> dict:
        return {
            "@type": self.type_url,
            "from_address": self.from_address,
            "to_address": self.to_address,
            "amount": self.amount.to_data(),
        }

    @classmethod
    def from_proto(cls, proto: MsgSend_pb) -> MsgSend:
        amount = []
        for c in proto["amount"]:
            coin = Coins.from_proto(c)
            amount.append(coin)

        return cls(
            from_address=proto["from_address"],
            to_address=proto["to_address"],
            amount=amount,
        )

    def to_proto(self) -> MsgSend_pb:
        proto = MsgSend_pb()
        data = bech32_decode(self.from_address)[1]
        from_address = convertbits(data, 5, 8, False)
        proto.from_address = bytes(from_address)
        data = bech32_decode(self.to_address)[1]
        to_address = convertbits(data, 5, 8, False)
        proto.to_address = bytes(to_address)

        amount = []
        for c in self.amount:
            coin = Thor_Coin_pb()
            coin.denom = c.asset.get_symbol().lower()
            #coin.amount = '{:,}'.format(int(c.amount / 100)).replace(",", "")
            coin.amount = f"{int(c.amount)}"
            amount.append(coin)
        proto.amount.extend(amount)  # Use extend() to add multiple items to a repeated field

        return proto


    @classmethod
    def unpack_any(cls, any: Any) -> MsgSend:
        return MsgSend.from_proto(any)