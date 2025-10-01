//go:build stagenet
// +build stagenet

package liquiditypools

import (
	"gitlab.com/mayachain/mayanode/common"
)

var LiquidityPoolsV111 = common.Assets{
	common.BTCAsset,
	common.RUNEAsset,
	common.ETHAsset,
	common.DASHAsset,
	common.KUJIAsset,
	common.USDTAsset,
	common.USDCAsset,
	common.USKAsset,
	common.WSTETHAsset,
	common.AETHAsset,
}
