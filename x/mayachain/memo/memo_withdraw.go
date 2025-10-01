package mayachain

import (
	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common"
	cosmos "gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

type WithdrawLiquidityMemo struct {
	MemoBase
	Amount          cosmos.Uint
	WithdrawalAsset common.Asset
	PairAddress     common.Address
}

func (m WithdrawLiquidityMemo) GetAmount() cosmos.Uint           { return m.Amount }
func (m WithdrawLiquidityMemo) GetWithdrawalAsset() common.Asset { return m.WithdrawalAsset }
func (m WithdrawLiquidityMemo) GetPairAddress() common.Address   { return m.PairAddress }

func NewWithdrawLiquidityMemo(asset common.Asset, amt cosmos.Uint, withdrawalAsset common.Asset, pairAddress common.Address) WithdrawLiquidityMemo {
	return WithdrawLiquidityMemo{
		MemoBase:        MemoBase{TxType: TxWithdraw, Asset: asset},
		Amount:          amt,
		WithdrawalAsset: withdrawalAsset,
		PairAddress:     pairAddress,
	}
}

func (p *parser) ParseWithdrawLiquidityMemo() (WithdrawLiquidityMemo, error) {
	switch {
	case p.version.GTE(semver.MustParse("1.112.0")):
		return p.ParseWithdrawLiquidityMemoV112()
	default:
		return ParseWithdrawLiquidityMemoV1(p.ctx, p.keeper, p.getAsset(1, true, common.EmptyAsset), p.parts, p.version)
	}
}

func (p *parser) ParseWithdrawLiquidityMemoV112() (WithdrawLiquidityMemo, error) {
	asset := p.getAsset(1, true, common.EmptyAsset)
	withdrawalBasisPts := p.getUintWithMaxValue(2, false, types.MaxWithdrawBasisPoints, types.MaxWithdrawBasisPoints)
	withdrawalAsset := p.getAsset(3, false, common.EmptyAsset)
	pairAddress := p.getAddressWithKeeper(4, false, common.NoAddress, asset.GetChain(), p.version)
	return NewWithdrawLiquidityMemo(asset, withdrawalBasisPts, withdrawalAsset, pairAddress), p.Error()
}
