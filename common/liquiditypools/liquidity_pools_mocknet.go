//go:build mocknet
// +build mocknet

package liquiditypools

import (
	"gitlab.com/mayachain/mayanode/common"
)

var LiquidityPoolsVCUR = common.Assets{
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
	common.AETHAsset,
	common.AWBTCAsset,
	common.ATGTAsset,
	common.AUSDTAsset,
	common.AUSDCAsset,
	common.XRDAsset,
	common.ZECAsset,
}
