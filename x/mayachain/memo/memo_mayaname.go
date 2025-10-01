package mayachain

import (
	"errors"

	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

type ManageMAYANameMemo struct {
	MemoBase
	Name             string
	Chain            common.Chain
	Address          common.Address
	PreferredAsset   common.Asset
	Expire           int64
	Owner            cosmos.AccAddress
	AffiliateBps     cosmos.Uint
	SubaffiliateName []string
	SubaffiliateBps  []cosmos.Uint
}

func (m ManageMAYANameMemo) GetName() string            { return m.Name }
func (m ManageMAYANameMemo) GetChain() common.Chain     { return m.Chain }
func (m ManageMAYANameMemo) GetAddress() common.Address { return m.Address }
func (m ManageMAYANameMemo) GetBlockExpire() int64      { return m.Expire }

func NewManageMAYANameMemo(name string, chain common.Chain, addr common.Address, expire int64, asset common.Asset, owner cosmos.AccAddress, affiliateBps cosmos.Uint, subaffiliateBps []cosmos.Uint, subaffiliateName []string) ManageMAYANameMemo {
	return ManageMAYANameMemo{
		MemoBase:         MemoBase{TxType: TxMAYAName},
		Name:             name,
		Chain:            chain,
		Address:          addr,
		PreferredAsset:   asset,
		Expire:           expire,
		Owner:            owner,
		AffiliateBps:     affiliateBps,
		SubaffiliateName: subaffiliateName,
		SubaffiliateBps:  subaffiliateBps,
	}
}

func (p *parser) ParseManageMAYANameMemo() (ManageMAYANameMemo, error) {
	switch {
	case p.version.GTE(semver.MustParse("1.112.0")):
		return p.ParseManageMAYANameMemoV112()
	default:
		return ParseManageMAYANameMemoV1(p.version, p.parts)
	}
}

// #######################################################################################
// MAYAName memo format:  ~:name:chain:address:?owner:?preferredAsset:?expiry:?affbps:?subaff:?subaffbps
// eg. "~:simple:MAYA:tmaya1t9n94ayyqq7xhdfp9ugey0fkxlfqy0efqrw4vc::::150"
// ########################################################################################
func (p *parser) ParseManageMAYANameMemoV112() (ManageMAYANameMemo, error) {
	chain := p.getChain(2, false, common.EmptyChain)
	addr := p.getAddress(3, false, common.NoAddress, p.version)
	if (chain.IsEmpty() && !addr.IsEmpty()) || (!chain.IsEmpty() && addr.IsEmpty()) {
		return ManageMAYANameMemo{}, errors.New("both chain and address must be provided, or neither")
	}
	owner := p.getAccAddress(4, false, nil)
	preferredAsset := p.getAsset(5, false, common.EmptyAsset)
	expire := p.getInt64(6, false, 0)
	maxAffiliateFeeBasisPoints := uint64(p.getConfigInt64(constants.MaxAffiliateFeeBasisPoints))
	// EmptyBps is 501, which is greater than MaxAffiliateFeeBasisPoints, so we can't use it as the default, therefore, we use 0
	affiliateBps := p.getUintWithMaxValue(7, false, 0, maxAffiliateFeeBasisPoints)
	// if the BPS part of the memo is empty, set the correct default value (EmptyBps) here
	if p.get(7) == "" {
		affiliateBps = types.EmptyBps
	}
	// if not empty both subaff names and subaff bps is required
	required := p.get(8) != ""
	if required {
		p.requiredFields++
	}
	subAffiliates, subAffiliateBps, _ := p.getMultipleAffiliatesAndBps(8, required, cosmos.NewUint(constants.MaxBasisPts))

	return NewManageMAYANameMemo(p.get(1), chain, addr, expire, preferredAsset, owner, affiliateBps, subAffiliateBps, subAffiliates), p.Error()
}
