package constants

import (
	"regexp"

	"github.com/blang/semver"
	. "gopkg.in/check.v1"
)

type ConstantsTestSuite struct{}

var _ = Suite(&ConstantsTestSuite{})

func (ConstantsTestSuite) TestConstantName_String(c *C) {
	constantNames := []ConstantName{
		BlocksPerDay,
		BlocksPerYear,
		OutboundTransactionFee,
		NativeTransactionFee,
		KillSwitchStart,
		KillSwitchDuration,
		PoolCycle,
		MinCacaoPoolDepth,
		MaxAvailablePools,
		StagedPoolCost,
		MinimumNodesForYggdrasil,
		MinimumNodesForBFT,
		DesiredValidatorSet,
		AsgardSize,
		ChurnInterval,
		ChurnRetryInterval,
		BadValidatorRedline,
		BadValidatorRate,
		OldValidatorRate,
		LowBondValidatorRate,
		LackOfObservationPenalty,
		SigningTransactionPeriod,
		DoubleSignMaxAge,
		PauseBond,
		PauseUnbond,
		MinimumBondInCacao,
		FundMigrationInterval,
		ArtificialRagnarokBlockHeight,
		MaximumLiquidityCacao,
		StrictBondLiquidityRatio,
		DefaultPoolStatus,
		MaxOutboundAttempts,
		SlashPenalty,
		PauseOnSlashThreshold,
		FailKeygenSlashPoints,
		FailKeysignSlashPoints,
		LiquidityLockUpBlocks,
		ObserveSlashPoints,
		ObservationDelayFlexibility,
		ForgiveSlashPeriod,
		YggFundLimit,
		YggFundRetry,
		JailTimeKeygen,
		JailTimeKeysign,
		NodePauseChainBlocks,
		MinSwapsPerBlock,
		MaxSwapsPerBlock,
		MaxSlashRatio,
		MaxSynthPerAssetDepth,
		VirtualMultSynths,
		MinSlashPointsForBadValidator,
		FullImpLossProtectionBlocks,
		BondLockupPeriod,
		MaxBondProviders,
		NumberOfNewNodesPerChurn,
		MinTxOutVolumeThreshold,
		TxOutDelayRate,
		TxOutDelayMax,
		MaxTxOutOffset,
		TNSRegisterFee,
		TNSFeeOnSale,
		TNSFeePerBlock,
		PermittedSolvencyGap,
		NodeOperatorFee,
		ValidatorMaxRewardRatio,
		PoolDepthForYggFundingMin,
		MaxNodeToChurnOutForLowVersion,
		MayaFundPerc,
		MinCacaoForMayaFundDist,
		WithdrawLimitTier1,
		WithdrawLimitTier2,
		WithdrawLimitTier3,
		WithdrawDaysTier1,
		WithdrawDaysTier2,
		WithdrawDaysTier3,
		WithdrawTier1,
		WithdrawTier2,
		WithdrawTier3,
		InflationPercentageThreshold,
		InflationPoolPercentage,
		InflationFormulaMulValue,
		InflationFormulaSumValue,
		IBCReceiveEnabled,
		IBCSendEnabled,
		RagnarokProcessNumOfLPPerIteration,
		SaverDepositDisabled,
		SaverWithdrawalDisabled,
		SwapOutDexAggregationDisabled,
	}
	for _, item := range constantNames {
		c.Assert(item.String(), Not(Equals), "NA")
	}
}

func (ConstantsTestSuite) TestGetConstantValues(c *C) {
	ver := semver.MustParse("0.0.9")
	c.Assert(GetConstantValues(ver), IsNil)
	c.Assert(GetConstantValues(SWVersion), NotNil)
}

func (ConstantsTestSuite) TestAllConstantName(c *C) {
	keyRegex := regexp.MustCompile(MimirKeyRegex).MatchString
	for key := range nameToString {
		if !keyRegex(key.String()) {
			c.Errorf("key:%s can't be used to set mimir", key)
		}
	}
}
