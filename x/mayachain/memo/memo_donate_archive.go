package mayachain

import (
	"gitlab.com/mayachain/mayanode/common"
)

func ParseDonateMemoV1(asset common.Asset) (DonateMemo, error) {
	return DonateMemo{
		MemoBase: MemoBase{TxType: TxDonate, Asset: asset},
	}, nil
}
