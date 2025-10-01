package mayachain

import (
	"fmt"

	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common"
)

type DonateMemo struct{ MemoBase }

func (m DonateMemo) String() string {
	return fmt.Sprintf("DONATE:%s", m.Asset)
}

func (p *parser) ParseDonateMemo() (DonateMemo, error) {
	switch {
	case p.version.GTE(semver.MustParse("1.112.0")):
		return p.ParseDonateMemoV112()
	default:
		return ParseDonateMemoV1(p.getAsset(1, true, common.EmptyAsset))
	}
}

func (p *parser) ParseDonateMemoV112() (DonateMemo, error) {
	asset := p.getAsset(1, true, common.EmptyAsset)
	return DonateMemo{
		MemoBase: MemoBase{TxType: TxDonate, Asset: asset},
	}, p.Error()
}
