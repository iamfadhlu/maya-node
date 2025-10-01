//go:build mocknet
// +build mocknet

package mayachain

import "gitlab.com/mayachain/mayanode/common"

// Supported chains for mocknet
var SUPPORT_CHAINS_V120 = common.Chains{
	common.ARBChain,
	common.BASEChain,
	common.BTCChain,
	common.DASHChain,
	common.ETHChain,
	common.KUJIChain,
	common.THORChain,
	common.XRDChain,
	common.ZECChain,
	// Smoke
	// common.AVAXChain,
	// common.BNBChain,
	// common.GAIAChain,
}
