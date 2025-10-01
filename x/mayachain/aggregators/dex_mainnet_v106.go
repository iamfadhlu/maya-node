//go:build !testnet && !mocknet && !stagenet
// +build !testnet,!mocknet,!stagenet

package aggregators

import (
	"gitlab.com/mayachain/mayanode/common"
)

func DexAggregatorsV106() []Aggregator {
	return []Aggregator{
		// RangoDiamond Ethereum
		{common.ETHChain, `0x69460570c93f9DE5E2edbC3052bf10125f0Ca22d`, 400_000},
		// RangoDiamond Avax
		{common.AVAXChain, `0x69460570c93f9DE5E2edbC3052bf10125f0Ca22d`, 400_000},
		// RangoThorchainOutputAggUniV3_COMPACT_Fee500
		{common.ETHChain, `0x70F75937546fB26c6FD3956eBBfb285f41526186`, 400_000},
		// RangoThorchainOutputAggUniV3_COMPACT_Fee3000
		{common.ETHChain, `0xd1687354CBA0e56facd0c44eD0F69D97F5734Dc1`, 400_000},
		// RangoThorchainOutputAggUniV3_COMPACT_Fee10000
		{common.ETHChain, `0xaFa4cBA6db85515f66E3ed7d6784e8cf5b689E2D`, 400_000},
		// RangoThorchainOutputAggUniV2_COMPACT_SUSHI
		{common.ETHChain, `0x0964347B0019eb227c901220ce7d66BB01479220`, 400_000},
		// RangoThorchainOutputAggUniV2_COMPACT_UNI
		{common.ETHChain, `0x6f281993AB68216F8898c593C4578C8a4a76F063`, 400_000},
		// RangoThorchainOutputAggUniV2_COMPACT_TRADERJOE
		{common.AVAXChain, `0x892Fb7C2A23772f4A2FFC3DC82419147dC22021C`, 400_000},
		// RangoThorchainOutputAggUniV2_COMPACT_PANGOLIN
		{common.AVAXChain, `0xBd039a45e656221E28594d2761DDed8F6712AE46`, 400_000},
	}
}
