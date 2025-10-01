package mayachain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

type UnbondMemo struct {
	MemoBase
	NodeAddress         cosmos.AccAddress
	BondProviderAddress cosmos.AccAddress
	Units               cosmos.Uint
}

func (m UnbondMemo) GetAccAddress() cosmos.AccAddress { return m.NodeAddress }

func NewUnbondMemo(asset common.Asset, addr, additional cosmos.AccAddress, units cosmos.Uint) UnbondMemo {
	return UnbondMemo{
		MemoBase: MemoBase{
			TxType: TxUnbond,
			Asset:  asset,
		},
		NodeAddress:         addr,
		BondProviderAddress: additional,
		Units:               units,
	}
}

func (p *parser) ParseUnbondMemo() (UnbondMemo, error) {
	switch {
	case p.version.GTE(semver.MustParse("1.112.0")):
		return p.ParseUnbondMemoV112()
	case p.version.GTE(semver.MustParse("1.105.0")):
		return ParseUnbondMemoV105(p.parts)
	case p.version.GTE(semver.MustParse("0.81.0")):
		return ParseUnbondMemoV81(p.parts)
	}
	return UnbondMemo{}, fmt.Errorf("invalid version(%s)", p.version.String())
}

func (p *parser) ParseUnbondMemoV112() (UnbondMemo, error) {
	asset := p.getAsset(1, false, common.EmptyAsset)
	amt := p.getUint(2, false, 0)
	addr := p.getAccAddress(3, true, nil)
	additional := p.getAccAddress(4, false, nil)

	return NewUnbondMemo(asset, addr, additional, amt), p.Error()
}
