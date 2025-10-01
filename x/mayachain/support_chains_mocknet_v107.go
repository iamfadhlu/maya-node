//go:build mocknet
// +build mocknet

package mayachain

import "gitlab.com/mayachain/mayanode/common"

// Supported chains for mainnet
var SUPPORT_CHAINS_V107 = common.Chains{
	common.BASEChain,
	common.BTCChain,
	common.DASHChain,
	common.ETHChain,
	common.KUJIChain,
	common.THORChain,
	// Smoke
	common.AVAXChain,
	common.BNBChain,
}
