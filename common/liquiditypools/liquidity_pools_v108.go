//go:build !mocknet && !stagenet
// +build !mocknet,!stagenet

package liquiditypools

import (
	"gitlab.com/mayachain/mayanode/common"
)

var LiquidityPoolsV108 = common.Assets{
	common.BTCAsset,
	common.RUNEAsset,
	common.ETHAsset,
	common.DASHAsset,
	common.KUJIAsset,
	common.USDTAssetV1,
	common.USDCAssetV1,
	common.USKAsset,
	common.WSTETHAssetV1,
}
