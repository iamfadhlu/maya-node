package mayachain

import (
	"strings"

	"github.com/blang/semver"
	cosmos "gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

// "pool+"

type CacaoPoolDepositMemo struct {
	MemoBase
}

func (m CacaoPoolDepositMemo) String() string {
	return m.string(false)
}

func (m CacaoPoolDepositMemo) ShortString() string {
	return m.string(true)
}

func (m CacaoPoolDepositMemo) string(short bool) string {
	return "pool+"
}

func NewCacaoPoolDepositMemo() CacaoPoolDepositMemo {
	return CacaoPoolDepositMemo{
		MemoBase: MemoBase{TxType: TxCacaoPoolDeposit},
	}
}

func (p *parser) ParseCacaoPoolDepositMemo() (CacaoPoolDepositMemo, error) {
	return NewCacaoPoolDepositMemo(), nil
}

// "pool-:<basis-points>:<affiliate>:<affiliate-basis-points>"

type CacaoPoolWithdrawMemo struct {
	MemoBase
	BasisPoints           cosmos.Uint
	AffiliateBasisPoints  cosmos.Uint
	Affiliates            []string
	AffiliatesBasisPoints []cosmos.Uint
}

func (m CacaoPoolWithdrawMemo) GetBasisPts() cosmos.Uint             { return m.BasisPoints }
func (m CacaoPoolWithdrawMemo) GetAffiliateBasisPoints() cosmos.Uint { return m.AffiliateBasisPoints }
func (m CacaoPoolWithdrawMemo) GetAffiliates() []string              { return m.Affiliates }
func (m CacaoPoolWithdrawMemo) GetAffiliatesBasisPoints() []cosmos.Uint {
	return m.AffiliatesBasisPoints
}

func (m CacaoPoolWithdrawMemo) String() string {
	affString, affBpsString := getAffiliatesMemoString(m.Affiliates, m.AffiliatesBasisPoints)
	args := []string{TxCacaoPoolWithdraw.String(), m.BasisPoints.String(), affString, affBpsString}
	return strings.Join(args, ":")
}

func NewCacaoPoolWithdrawMemo(basisPoints cosmos.Uint, affiliateBps cosmos.Uint, affiliates []string, affiliatesBasisPoints []cosmos.Uint) CacaoPoolWithdrawMemo {
	return CacaoPoolWithdrawMemo{
		MemoBase:              MemoBase{TxType: TxCacaoPoolWithdraw},
		BasisPoints:           basisPoints,
		AffiliateBasisPoints:  affiliateBps,
		Affiliates:            affiliates,
		AffiliatesBasisPoints: affiliatesBasisPoints,
	}
}

func (p *parser) ParseCacaoPoolWithdrawMemo() (CacaoPoolWithdrawMemo, error) {
	switch {
	case p.version.GTE(semver.MustParse("1.121.0")): // cacaopool-aff
		return p.ParseCacaoPoolWithdrawMemoV121()
	default:
		return p.ParseCacaoPoolWithdrawMemoV1()
	}
}

func (p *parser) ParseCacaoPoolWithdrawMemoV121() (CacaoPoolWithdrawMemo, error) {
	basisPoints := p.getUint(1, true, cosmos.ZeroInt().Uint64())
	maxBps := cosmos.NewUint(constants.MaxBasisPts) // 100%
	affiliates, affiliatesBps, totalAffBps := p.getMultipleAffiliatesAndBps(2, false, maxBps)
	return NewCacaoPoolWithdrawMemo(basisPoints, totalAffBps, affiliates, affiliatesBps), p.Error()
}
