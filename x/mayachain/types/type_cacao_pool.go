package types

import (
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

func NewCACAOPool() CACAOPool {
	return CACAOPool{
		ReserveUnits:   cosmos.ZeroUint(),
		PoolUnits:      cosmos.ZeroUint(),
		CacaoDeposited: cosmos.ZeroUint(),
		CacaoWithdrawn: cosmos.ZeroUint(),
	}
}

func (rp CACAOPool) CurrentDeposit() cosmos.Int {
	deposited := cosmos.NewIntFromBigInt(rp.CacaoDeposited.BigInt())
	withdrawn := cosmos.NewIntFromBigInt(rp.CacaoWithdrawn.BigInt())
	return deposited.Sub(withdrawn)
}

func (rp CACAOPool) TotalUnits() cosmos.Uint {
	return rp.ReserveUnits.Add(rp.PoolUnits)
}
