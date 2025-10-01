package types

import (
	"fmt"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

func NewTradeAccount(addr cosmos.AccAddress, asset common.Asset) TradeAccount {
	return TradeAccount{
		Owner: addr,
		Asset: asset,
		Units: cosmos.ZeroUint(),
	}
}

func (tr TradeAccount) Key() string {
	return fmt.Sprintf("%s/%s", tr.Owner.String(), tr.Asset.String())
}
