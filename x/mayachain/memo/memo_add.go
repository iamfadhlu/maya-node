package mayachain

import (
	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

// By default tier is set to 3
const DefaultTierValue = 3

type AddLiquidityMemo struct {
	MemoBase
	Address              common.Address
	AffiliateAddress     common.Address
	AffiliateBasisPoints cosmos.Uint
	Tier                 int64
}

func (m AddLiquidityMemo) GetDestination() common.Address { return m.Address }

func NewAddLiquidityMemo(asset common.Asset, addr, affAddr common.Address, affPts cosmos.Uint, tier int64) AddLiquidityMemo {
	return AddLiquidityMemo{
		MemoBase:             MemoBase{TxType: TxAdd, Asset: asset},
		Address:              addr,
		AffiliateAddress:     affAddr,
		AffiliateBasisPoints: affPts,
		Tier:                 tier,
	}
}

func (p *parser) ParseAddLiquidityMemo() (AddLiquidityMemo, error) {
	if p.keeper == nil {
		return ParseAddLiquidityMemoV1(p.ctx, p.keeper, p.getAsset(1, true, common.EmptyAsset), p.parts)
	}
	switch {
	case p.version.GTE(semver.MustParse("1.112.0")):
		return p.ParseAddLiquidityMemoV112()
	case p.version.GTE(semver.MustParse("1.110.0")):
		return ParseAddLiquidityMemoV110(p.ctx, p.keeper, p.getAsset(1, true, common.EmptyAsset), p.parts, p.version)
	default:
		return ParseAddLiquidityMemoV1(p.ctx, p.keeper, p.getAsset(1, true, common.EmptyAsset), p.parts)
	}
}

func (p *parser) ParseAddLiquidityMemoV112() (AddLiquidityMemo, error) {
	asset := p.getAsset(1, true, common.EmptyAsset)
	addr := p.getAddressWithKeeper(2, false, common.NoAddress, asset.Chain, p.version)
	affChain := common.BASEChain
	if asset.IsSyntheticAsset() {
		// For a Savers add, an Affiliate MAYAName must be resolved
		// to an address for the Layer 1 Chain of the synth to succeed.
		affChain = asset.GetLayer1Asset().GetChain()
	}
	affAddr := p.getAddressWithKeeper(3, false, common.NoAddress, affChain, p.version)
	maxAffiliateFeeBasisPoints := uint64(p.getConfigInt64(constants.MaxAffiliateFeeBasisPoints))
	affPts := p.getUintWithMaxValue(4, false, 0, maxAffiliateFeeBasisPoints)

	return NewAddLiquidityMemo(asset, addr, affAddr, affPts, 0), p.Error()
}
