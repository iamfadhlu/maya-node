//go:build mocknet
// +build mocknet

package liquiditypools

import (
	"gitlab.com/mayachain/mayanode/common"
)

var LiquidityPoolsV109 = common.Assets{
	common.BTCAsset,
	common.BNBAsset,
	common.RUNEAsset,
	common.ETHAsset,
	common.DASHAsset,
	common.KUJIAsset,
	common.USDTAssetV1,
	common.USDCAssetV1,
	common.USKAsset,
	common.WSTETHAssetV1,
	common.AETHAsset,
}
