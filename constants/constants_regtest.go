//go:build regtest
// +build regtest

// For internal testing and mockneting
package constants

func init() {
	int64Overrides = map[ConstantName]int64{
		// ArtificialRagnarokBlockHeight: 200,
		DesiredValidatorSet:                 12,
		ChurnInterval:                       60, // 5 min
		ChurnRetryInterval:                  30,
		BadValidatorRate:                    60,          // 5 min
		OldValidatorRate:                    60,          // 5 min
		MinimumBondInCacao:                  100_000_000, // 1 cacao
		MaxBondProviders:                    6,           // maximum number of bond providers
		ValidatorMaxRewardRatio:             3,
		FundMigrationInterval:               2,
		LiquidityLockUpBlocks:               0,
		JailTimeKeygen:                      10,
		JailTimeKeysign:                     10,
		AsgardSize:                          6,
		MinimumNodesForYggdrasil:            4,
		VirtualMultSynthsBasisPoints:        20_000,
		MinTxOutVolumeThreshold:             2000000_00000000,
		TxOutDelayRate:                      2000000_00000000,
		PoolDepthForYggFundingMin:           500_000_00000000,
		MayaFundPerc:                        10, // The percentage for the Maya Fund when the gas is distribute
		SubsidizeReserveMultiplier:          1,
		AllowWideBlame:                      1,
		TargetOutboundFeeSurplusRune:        10_000_00000000,
		MaxOutboundFeeMultiplierBasisPoints: 20_000,
		MinOutboundFeeMultiplierBasisPoints: 15_000,
		MaxSynthPerAssetDepth:               3300,
		MaxSynthPerPoolDepth:                3_500,
		StreamingSwapMinBPFee:               100,
		SigningTransactionPeriod:            3,
		CACAOPoolDepositMaturityBlocks:      0,
		TradeAccountsEnabled:                1,
		TradeAccountsDepositEnabled:         1,
		TradeAccountsWithdrawEnabled:        1,
	}
	boolOverrides = map[ConstantName]bool{
		StrictBondLiquidityRatio: false,
	}
	stringOverrides = map[ConstantName]string{
		DefaultPoolStatus: "Available",
	}
}
