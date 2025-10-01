//go:build stagenet
// +build stagenet

package mayachain

import "gitlab.com/mayachain/mayanode/common"

// Supported chains for stagenet
var SUPPORT_CHAINS_V111 = common.Chains{
	common.BASEChain,
	common.BTCChain,
	common.DASHChain,
	common.ETHChain,
	common.KUJIChain,
	common.THORChain,
	common.ARBChain,
	common.XRDChain,
}
