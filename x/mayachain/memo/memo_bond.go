package mayachain

import (
	"fmt"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"

	"github.com/blang/semver"
)

type BondMemo struct {
	MemoBase
	NodeAddress         cosmos.AccAddress
	BondProviderAddress cosmos.AccAddress
	NodeOperatorFee     int64
	Units               cosmos.Uint
}

func (m BondMemo) GetAmount() cosmos.Uint           { return m.Units }
func (m BondMemo) GetAccAddress() cosmos.AccAddress { return m.NodeAddress }

func NewBondMemo(asset common.Asset, addr, additional cosmos.AccAddress, units cosmos.Uint, operatorFee int64) BondMemo {
	return BondMemo{
		MemoBase: MemoBase{
			TxType: TxBond,
			Asset:  asset,
		},
		NodeAddress:         addr,
		BondProviderAddress: additional,
		NodeOperatorFee:     operatorFee,
		Units:               units,
	}
}

func (p *parser) ParseBondMemo() (BondMemo, error) {
	switch {
	case p.version.GTE(semver.MustParse("1.112.0")):
		return p.ParseBondMemoV112()
	case p.version.GTE(semver.MustParse("1.105.0")):
		return ParseBondMemoV105(p.parts, p.version)
	case p.version.GTE(semver.MustParse("1.88.0")):
		return ParseBondMemoV88(p.parts)
	case p.version.GTE(semver.MustParse("0.81.0")):
		return ParseBondMemoV81(p.parts)
	default:
		return BondMemo{}, fmt.Errorf("invalid version(%s)", p.version.String())
	}
}

func (p *parser) ParseBondMemoV112() (BondMemo, error) {
	asset := p.getAsset(1, false, common.EmptyAsset)
	units := p.getUint(2, false, 0)
	addr := p.getAccAddress(3, true, nil)
	additional := p.getAccAddress(4, false, nil)
	operatorFee := p.getInt64(5, false, -1)

	return NewBondMemo(asset, addr, additional, units, operatorFee), p.Error()
}
