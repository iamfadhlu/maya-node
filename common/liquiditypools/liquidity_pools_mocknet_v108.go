//go:build mocknet
// +build mocknet

package liquiditypools

import (
	"gitlab.com/mayachain/mayanode/common"
)

var LiquidityPoolsV108 = common.Assets{
	common.BTCAsset,
	common.BNBAsset,
	common.RUNEAsset,
	common.ETHAsset,
	common.DASHAsset,
	common.KUJIAsset,
	common.USDTAsset,
	common.USDCAsset,
	common.USKAsset,
	common.WSTETHAsset,
}
