package constants

import (
	"fmt"

	"github.com/blang/semver"
)

// ConstantName the name we used to get constant values
type ConstantName int

const (
	BlocksPerDay ConstantName = iota
	BlocksPerYear
	OutboundTransactionFee
	NativeTransactionFee
	KillSwitchStart
	KillSwitchDuration
	PoolCycle
	MinCacaoPoolDepth
	MaxAvailablePools
	StagedPoolCost
	MinimumNodesForYggdrasil
	MinimumNodesForBFT
	DesiredValidatorSet
	AsgardSize
	ChurnInterval
	ChurnRetryInterval
	ValidatorsChangeWindow
	LeaveProcessPerBlockHeight
	BadValidatorRedline
	BadValidatorRate
	OldValidatorRate
	LowBondValidatorRate
	LackOfObservationPenalty
	SigningTransactionPeriod
	DoubleSignMaxAge
	PauseBond
	PauseUnbond
	MinimumBondInCacao
	FundMigrationInterval
	ArtificialRagnarokBlockHeight
	MaximumLiquidityCacao
	StrictBondLiquidityRatio
	DefaultPoolStatus
	MaxOutboundAttempts
	SlashPenalty
	PauseOnSlashThreshold
	FailKeygenSlashPoints
	FailKeysignSlashPoints
	LiquidityLockUpBlocks
	ObserveSlashPoints
	DoubleBlockSignSlashPoints
	MissBlockSignSlashPoints
	ObservationDelayFlexibility
	ForgiveSlashPeriod
	YggFundLimit
	YggFundRetry
	JailTimeKeygen
	JailTimeKeysign
	NodePauseChainBlocks
	MinSwapsPerBlock
	MaxSwapsPerBlock
	MaxSlashRatio
	MaxSynthPerAssetDepth
	MaxSynthPerPoolDepth
	MaxSynthsForSaversYield
	VirtualMultSynths
	VirtualMultSynthsBasisPoints
	MinSlashPointsForBadValidator
	FullImpLossProtectionBlocks
	BondLockupPeriod
	MaxBondProviders
	NumberOfNewNodesPerChurn
	MinTxOutVolumeThreshold
	TxOutDelayRate
	TxOutDelayMax
	MaxTxOutOffset
	TNSRegisterFee
	TNSFeeOnSale
	TNSFeePerBlock
	PermittedSolvencyGap
	NodeOperatorFee
	ValidatorMaxRewardRatio
	PoolDepthForYggFundingMin
	MaxNodeToChurnOutForLowVersion
	MayaFundPerc
	MinCacaoForMayaFundDist
	WithdrawLimitTier1
	WithdrawLimitTier2
	WithdrawLimitTier3
	WithdrawDaysTier1
	WithdrawDaysTier2
	WithdrawDaysTier3
	WithdrawTier1
	WithdrawTier2
	WithdrawTier3
	InflationPercentageThreshold
	InflationPoolPercentage
	InflationFormulaMulValue
	InflationFormulaSumValue
	IBCReceiveEnabled
	IBCSendEnabled
	RagnarokProcessNumOfLPPerIteration
	SaverDepositDisabled
	SaverWithdrawalDisabled
	SwapOutDexAggregationDisabled
	POLMaxNetworkDeposit
	POLMaxPoolMovement
	POLSynthUtilization
	POLBuffer
	SynthYieldBasisPoints
	SynthYieldCycle
	MinimumL1OutboundFeeUSD
	MinimumPoolLiquidityFee
	SubsidizeReserveMultiplier
	LiquidityAuction
	IncentiveCurveControl
	FullImpLossProtectionBlocksTimes4
	ZeroImpLossProtectionBlocks
	AllowWideBlame
	MaxAffiliateFeeBasisPoints
	TargetOutboundFeeSurplusRune
	MaxOutboundFeeMultiplierBasisPoints
	MinOutboundFeeMultiplierBasisPoints
	SlipFeeAddedBasisPoints
	PayBPNodeRewards
	PendingLiquidityAgeLimit
	StreamingSwapPause
	StreamingSwapMinBPFee
	StreamingSwapMaxLength
	StreamingSwapMaxLengthNative
	SaversStreamingSwapsInterval
	KeygenRetryInterval
	RescheduleCoalesceBlocks
	PreferredAssetOutboundFeeMultiplier
	MAYANameGracePeriodBlocks
	MultipleAffiliatesMaxCount
	SignerConcurrency
	EVMDisableContractWhitelist
	AffiliateFeeTickGranularity
	CACAOPoolEnabled
	CACAOPoolRewardsEnabled
	CACAOPoolDepositMaturityBlocks
	ChurnMigrateRounds
	TradeAccountsEnabled
	TradeAccountsDepositEnabled
	TradeAccountsWithdrawEnabled

	// These are new implicitly-0 Constants undisplayed in the API endpoint (no explicit value set).
	BurnSynths
	ManualSwapsToSynthDisabled
	MintSynths
)

var nameToString = map[ConstantName]string{
	BlocksPerDay:                        "BlocksPerDay",
	BlocksPerYear:                       "BlocksPerYear",
	OutboundTransactionFee:              "OutboundTransactionFee",
	NativeTransactionFee:                "NativeTransactionFee",
	PoolCycle:                           "PoolCycle",
	MinCacaoPoolDepth:                   "MinRunePoolDepth", // Can't change the string value, because we would have to account for the version change when mimir is used
	MaxAvailablePools:                   "MaxAvailablePools",
	StagedPoolCost:                      "StagedPoolCost",
	KillSwitchStart:                     "KillSwitchStart",
	KillSwitchDuration:                  "KillSwitchDuration",
	MinimumNodesForYggdrasil:            "MinimumNodesForYggdrasil",
	MinimumNodesForBFT:                  "MinimumNodesForBFT",
	DesiredValidatorSet:                 "DesiredValidatorSet",
	AsgardSize:                          "AsgardSize",
	ChurnInterval:                       "ChurnInterval",
	ChurnRetryInterval:                  "ChurnRetryInterval",
	BadValidatorRedline:                 "BadValidatorRedline",
	BadValidatorRate:                    "BadValidatorRate",
	OldValidatorRate:                    "OldValidatorRate",
	LowBondValidatorRate:                "LowBondValidatorRate",
	LackOfObservationPenalty:            "LackOfObservationPenalty",
	SigningTransactionPeriod:            "SigningTransactionPeriod",
	DoubleSignMaxAge:                    "DoubleSignMaxAge",
	PauseBond:                           "PauseBond",
	PauseUnbond:                         "PauseUnbond",
	MinimumBondInCacao:                  "MinimumBondInRune", // Can't change the string value, because we would have to account for the version change when mimir is used
	MaxBondProviders:                    "MaxBondProviders",
	FundMigrationInterval:               "FundMigrationInterval",
	ArtificialRagnarokBlockHeight:       "ArtificialRagnarokBlockHeight",
	MaximumLiquidityCacao:               "MaximumLiquidityRune", // Can't change the string value, because we would have to account for the version change when mimir is used
	StrictBondLiquidityRatio:            "StrictBondLiquidityRatio",
	DefaultPoolStatus:                   "DefaultPoolStatus",
	MaxOutboundAttempts:                 "MaxOutboundAttempts",
	SlashPenalty:                        "SlashPenalty",
	PauseOnSlashThreshold:               "PauseOnSlashThreshold",
	FailKeygenSlashPoints:               "FailKeygenSlashPoints",
	FailKeysignSlashPoints:              "FailKeysignSlashPoints",
	LiquidityLockUpBlocks:               "LiquidityLockUpBlocks",
	ObserveSlashPoints:                  "ObserveSlashPoints",
	MissBlockSignSlashPoints:            "MissBlockSignSlashPoints",
	DoubleBlockSignSlashPoints:          "DoubleBlockSignSlashPoints",
	ObservationDelayFlexibility:         "ObservationDelayFlexibility",
	ForgiveSlashPeriod:                  "ForgiveSlashPeriod",
	YggFundLimit:                        "YggFundLimit",
	YggFundRetry:                        "YggFundRetry",
	JailTimeKeygen:                      "JailTimeKeygen",
	JailTimeKeysign:                     "JailTimeKeysign",
	NodePauseChainBlocks:                "NodePauseChainBlocks",
	MinSwapsPerBlock:                    "MinSwapsPerBlock",
	MaxSwapsPerBlock:                    "MaxSwapsPerBlock",
	VirtualMultSynths:                   "VirtualMultSynths",
	VirtualMultSynthsBasisPoints:        "VirtualMultSynthsBasisPoints",
	MaxSynthPerAssetDepth:               "MaxSynthPerAssetDepth",
	MaxSynthPerPoolDepth:                "MaxSynthPerPoolDepth",
	MaxSynthsForSaversYield:             "MaxSynthsForSaversYield",
	MinSlashPointsForBadValidator:       "MinSlashPointsForBadValidator",
	MaxSlashRatio:                       "MaxSlashRatio",
	FullImpLossProtectionBlocks:         "FullImpLossProtectionBlocks",
	BondLockupPeriod:                    "BondLockupPeriod",
	NumberOfNewNodesPerChurn:            "NumberOfNewNodesPerChurn",
	MinTxOutVolumeThreshold:             "MinTxOutVolumeThreshold",
	TxOutDelayRate:                      "TxOutDelayRate",
	TxOutDelayMax:                       "TxOutDelayMax",
	MaxTxOutOffset:                      "MaxTxOutOffset",
	TNSRegisterFee:                      "TNSRegisterFee",
	TNSFeeOnSale:                        "TNSFeeOnSale",
	TNSFeePerBlock:                      "TNSFeePerBlock",
	PermittedSolvencyGap:                "PermittedSolvencyGap",
	ValidatorMaxRewardRatio:             "ValidatorMaxRewardRatio",
	NodeOperatorFee:                     "NodeOperatorFee",
	PoolDepthForYggFundingMin:           "PoolDepthForYggFundingMin",
	MaxNodeToChurnOutForLowVersion:      "MaxNodeToChurnOutForLowVersion",
	MayaFundPerc:                        "MayaFundPerc",
	MinCacaoForMayaFundDist:             "MinRuneForMayaFundDist", // Can't change the string value, because we would have to account for the version change when mimir is used
	WithdrawLimitTier1:                  "WithdrawLimitTier1",
	WithdrawLimitTier2:                  "WithdrawLimitTier2",
	WithdrawLimitTier3:                  "WithdrawLimitTier3",
	WithdrawDaysTier1:                   "WithdrawDaysTier1",
	WithdrawDaysTier2:                   "WithdrawDaysTier2",
	WithdrawDaysTier3:                   "WithdrawDaysTier3",
	WithdrawTier1:                       "WithdrawTier1",
	WithdrawTier2:                       "WithdrawTier2",
	WithdrawTier3:                       "WithdrawTier3",
	InflationPercentageThreshold:        "InflationPercentageThreshold",
	InflationPoolPercentage:             "InflationPoolPercentage",
	InflationFormulaMulValue:            "InflationFormulaMulValue",
	InflationFormulaSumValue:            "InflationFormulaSumValue",
	IBCReceiveEnabled:                   "IBCReceiveEnabled",
	IBCSendEnabled:                      "IBCSendEnabled",
	SaverDepositDisabled:                "SaverDepositDisabled",
	SaverWithdrawalDisabled:             "SaverWithdrawalDisabled",
	SwapOutDexAggregationDisabled:       "SwapOutDexAggregationDisabled",
	POLMaxNetworkDeposit:                "POLMaxNetworkDeposit",
	POLMaxPoolMovement:                  "POLMaxPoolMovement",
	POLSynthUtilization:                 "POLSynthUtilization",
	POLBuffer:                           "POLBuffer",
	RagnarokProcessNumOfLPPerIteration:  "RagnarokProcessNumOfLPPerIteration",
	SynthYieldBasisPoints:               "SynthYieldBasisPoints",
	SynthYieldCycle:                     "SynthYieldCycle",
	MinimumL1OutboundFeeUSD:             "MinimumL1OutboundFeeUSD",
	MinimumPoolLiquidityFee:             "MinimumPoolLiquidityFee",
	SubsidizeReserveMultiplier:          "SubsidizeReserveMultiplier",
	LiquidityAuction:                    "LiquidityAuction",
	IncentiveCurveControl:               "IncentiveCurveControl",
	FullImpLossProtectionBlocksTimes4:   "FullImpLossProtectionBlocksTimes4",
	ZeroImpLossProtectionBlocks:         "ZeroImpLossProtectionBlocks",
	AllowWideBlame:                      "AllowWideBlame",
	MaxAffiliateFeeBasisPoints:          "MaxAffiliateFeeBasisPoints",
	TargetOutboundFeeSurplusRune:        "TargetOutboundFeeSurplusRune",
	MaxOutboundFeeMultiplierBasisPoints: "MaxOutboundFeeMultiplierBasisPoints",
	MinOutboundFeeMultiplierBasisPoints: "MinOutboundFeeMultiplierBasisPoints",
	SlipFeeAddedBasisPoints:             "SlipFeeAddedBasisPoints", // TODO: remove on hardfork
	PayBPNodeRewards:                    "PayBPNodeRewards",
	PendingLiquidityAgeLimit:            "PendingLiquidityAgeLimit",
	StreamingSwapPause:                  "StreamingSwapPause",
	StreamingSwapMinBPFee:               "StreamingSwapMinBPFee",
	StreamingSwapMaxLength:              "StreamingSwapMaxLength",
	StreamingSwapMaxLengthNative:        "StreamingSwapMaxLengthNative",
	SaversStreamingSwapsInterval:        "SaversStreamingSwapsInterval",
	KeygenRetryInterval:                 "KeygenRetryInterval",
	RescheduleCoalesceBlocks:            "RescheduleCoalesceBlocks",
	PreferredAssetOutboundFeeMultiplier: "PreferredAssetOutboundFeeMultiplier",
	MAYANameGracePeriodBlocks:           "MAYANameGracePeriodBlocks",
	MultipleAffiliatesMaxCount:          "MultipleAffiliatesMaxCount",
	SignerConcurrency:                   "SignerConcurrency",
	EVMDisableContractWhitelist:         "EVMDisableContractWhitelist",
	AffiliateFeeTickGranularity:         "AffiliateFeeTickGranularity",
	CACAOPoolEnabled:                    "CACAOPoolEnabled",
	CACAOPoolRewardsEnabled:             "CACAOPoolRewardsEnabled",
	CACAOPoolDepositMaturityBlocks:      "CACAOPoolDepositMaturityBlocks",
	ChurnMigrateRounds:                  "ChurnMigrateRounds",
	TradeAccountsEnabled:                "TradeAccountsEnabled",
	TradeAccountsDepositEnabled:         "TradeAccountsDepositEnabled",
	TradeAccountsWithdrawEnabled:        "TradeAccountsWithdrawEnabled",
}

// String implement fmt.stringer
func (cn ConstantName) String() string {
	val, ok := nameToString[cn]
	if !ok {
		return "NA"
	}
	return val
}

// ConstantValues define methods used to get constant values
type ConstantValues interface {
	fmt.Stringer
	GetInt64Value(name ConstantName) int64
	GetBoolValue(name ConstantName) bool
	GetStringValue(name ConstantName) string
}

// GetConstantValues will return an  implementation of ConstantValues which provide ways to get constant values
func GetConstantValues(ver semver.Version) ConstantValues {
	switch {
	case ver.GTE(semver.MustParse("1.118.0")):
		return NewConstantValueVCUR()
	case ver.GTE(semver.MustParse("1.113.0")):
		return NewConstantValue113()
	case ver.GTE(semver.MustParse("1.112.0")):
		return NewConstantValue112()
	case ver.GTE(semver.MustParse("1.110.0")):
		return NewConstantValue110()
	case ver.GTE(semver.MustParse("1.108.0")):
		return NewConstantValue108()
	case ver.GTE(semver.MustParse("1.107.0")):
		return NewConstantValue107()
	case ver.GTE(semver.MustParse("1.106.0")):
		return NewConstantValue106()
	case ver.GTE(semver.MustParse("1.102.0")):
		return NewConstantValue102()
	case ver.GTE(semver.MustParse("0.1.0")):
		return NewConstantValue010()
	default:
		return nil
	}
}
