//go:build !testnet && !mocknet && !stagenet
// +build !testnet,!mocknet,!stagenet

package mayachain

import "gitlab.com/mayachain/mayanode/common"

// Supported chains for mainnet
var SUPPORT_CHAINS_V104 = common.Chains{
	common.BASEChain,
	common.BTCChain,
	common.ETHChain,
	common.THORChain,
}
