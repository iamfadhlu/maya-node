package types

import (
	cosmos "gitlab.com/mayachain/mayanode/common/cosmos"
)

// NewProtocolOwnedLiquidity create a new instance ProtocolOwnedLiquidity it is empty though
func NewProtocolOwnedLiquidity() ProtocolOwnedLiquidity {
	return ProtocolOwnedLiquidity{
		CacaoDeposited: cosmos.ZeroUint(),
		CacaoWithdrawn: cosmos.ZeroUint(),
	}
}

func (pol ProtocolOwnedLiquidity) CurrentDeposit() cosmos.Int {
	deposited := cosmos.NewIntFromBigInt(pol.CacaoDeposited.BigInt())
	withdrawn := cosmos.NewIntFromBigInt(pol.CacaoWithdrawn.BigInt())
	return deposited.Sub(withdrawn)
}

// PnL - Profit and Loss
func (pol ProtocolOwnedLiquidity) PnL(value cosmos.Uint) cosmos.Int {
	deposited := cosmos.NewIntFromBigInt(pol.CacaoDeposited.BigInt())
	withdrawn := cosmos.NewIntFromBigInt(pol.CacaoWithdrawn.BigInt())
	v := cosmos.NewIntFromBigInt(value.BigInt())
	return withdrawn.Sub(deposited).Add(v)
}
