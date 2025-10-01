//go:build mocknet
// +build mocknet

package liquiditypools

import (
	"gitlab.com/mayachain/mayanode/common"
)

var LiquidityPoolsV105 = common.Assets{
	common.BTCAsset,
	common.BNBAsset,
	common.RUNEAsset,
	common.ETHAsset,
}
