//go:build !mocknet && !stagenet
// +build !mocknet,!stagenet

package liquiditypools

import (
	"gitlab.com/mayachain/mayanode/common"
)

var LiquidityPoolsV104 = common.Assets{
	common.BTCAsset,
	common.BNBAsset,
	common.ETHAsset,
}
