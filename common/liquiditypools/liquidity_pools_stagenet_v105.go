//go:build stagenet
// +build stagenet

package liquiditypools

import (
	"gitlab.com/mayachain/mayanode/common"
)

var LiquidityPoolsV105 = common.Assets{
	common.BTCAsset,
	common.RUNEAsset,
	common.ETHAsset,
}
