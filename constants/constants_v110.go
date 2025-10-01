package constants

// NewConstantValue110 get new instance of ConstantValue106
func NewConstantValue110() *ConstantVals {
	return &ConstantVals{
		int64values: map[ConstantName]int64{
			BlocksPerDay:                        14400,
			BlocksPerYear:                       5256000,
			OutboundTransactionFee:              20_00000000,         // A 20.0 Cacao fee on all swaps and withdrawals
			NativeTransactionFee:                20_00000000,         // A 20.0 Cacao fee on all on chain txs
			PoolCycle:                           43200,               // Make a pool available every 3 days
			StagedPoolCost:                      1000_00000000,       // amount of cacao to take from a staged pool on every pool cycle
			KillSwitchStart:                     0,                   // block height to start the kill switch of BEP2/ERC20 old CACAO
			KillSwitchDuration:                  0,                   // number of blocks until switch no longer works
			MinCacaoPoolDepth:                   1000_000_00000000,   // minimum cacao pool depth to be an available pool
			MaxAvailablePools:                   100,                 // maximum number of available pools
			MinimumNodesForYggdrasil:            6,                   // No yggdrasil pools if THORNode have less than 6 active nodes
			MinimumNodesForBFT:                  4,                   // Minimum node count to keep network running. Below this, Ragnarök is performed.
			DesiredValidatorSet:                 100,                 // desire validator set
			AsgardSize:                          40,                  // desired node operators in an asgard vault
			FundMigrationInterval:               360,                 // number of blocks THORNode will attempt to move funds from a retiring vault to an active one
			ChurnInterval:                       43200,               // How many blocks THORNode try to rotate validators
			ChurnRetryInterval:                  720,                 // How many blocks until we retry a churn (only if we haven't had a successful churn in ChurnInterval blocks
			BadValidatorRedline:                 3,                   // redline multiplier to find a multitude of bad actors
			BadValidatorRate:                    43200,               // rate to mark a validator to be rotated out for bad behavior
			OldValidatorRate:                    43200,               // rate to mark a validator to be rotated out for age
			LowBondValidatorRate:                43200,               // rate to mark a validator to be rotated out for low bond
			LackOfObservationPenalty:            2,                   // add two slash point for each block where a node does not observe
			SigningTransactionPeriod:            300,                 // how many blocks before a request to sign a tx by yggdrasil pool, is counted as delinquent.
			DoubleSignMaxAge:                    23,                  // number of blocks to limit double signing a block
			PauseBond:                           0,                   // pauses the ability to bond
			PauseUnbond:                         0,                   // pauses the ability to unbond
			MinimumBondInCacao:                  1_000_000_00000000,  // 1M cacao
			MaxBondProviders:                    2,                   // maximum number of bond providers
			MaxOutboundAttempts:                 0,                   // maximum retries to reschedule a transaction
			SlashPenalty:                        15000,               // penalty paid (in basis points) for theft of assets
			PauseOnSlashThreshold:               10_000_00000000,     // number of cacao to pause the network on the event a vault is slash for theft
			FailKeygenSlashPoints:               720,                 // slash for 720 blocks , which equals 1 hour
			FailKeysignSlashPoints:              2,                   // slash for 2 blocks
			LiquidityLockUpBlocks:               0,                   // the number of blocks LP can withdraw after their liquidity
			ObserveSlashPoints:                  1,                   // the number of slashpoints for making an observation (redeems later if observation reaches consensus
			ObservationDelayFlexibility:         10,                  // number of blocks of flexibility for a validator to get their slash points taken off for making an observation
			ForgiveSlashPeriod:                  17280,               // number of blocks a forgive slash request has to gain consensus. (24 hours)
			YggFundLimit:                        50,                  // percentage of the amount of funds a ygg vault is allowed to have.
			YggFundRetry:                        1000,                // number of blocks before retrying to fund a yggdrasil vault
			JailTimeKeygen:                      720 * 6,             // blocks a node account is jailed for failing to keygen. DO NOT drop below tss timeout
			JailTimeKeysign:                     60,                  // blocks a node account is jailed for failing to keysign. DO NOT drop below tss timeout
			NodePauseChainBlocks:                720,                 // number of blocks that a node can pause/resume a global chain halt
			NodeOperatorFee:                     3300,                // Node operator fee
			MinSwapsPerBlock:                    10,                  // process all swaps if queue is less than this number
			MaxSwapsPerBlock:                    100,                 // max swaps to process per block
			MaxSlashRatio:                       25,                  // max percentage a node can have its bond slashed before being banned.
			VirtualMultSynths:                   2,                   // pool depth multiplier for synthetic swaps
			VirtualMultSynthsBasisPoints:        10_000,              // pool depth multiplier for synthetic swaps (in basis points)
			MaxSynthPerAssetDepth:               2500,                // percentage (in basis points) of how many synths are allowed relative to asset depth of the related pool
			MaxSynthPerPoolDepth:                1700,                // percentage (in basis points) of how many synths are allowed relative to pool depth of the related pool
			MinSlashPointsForBadValidator:       100,                 // The minimum slash point
			FullImpLossProtectionBlocks:         2_160_000,           // number of blocks before a liquidity provider gets 100% impermanent loss protection times one  (150 days)
			FullImpLossProtectionBlocksTimes4:   6_480_000,           // number of blocks before a liquidity provider gets 100% impermanent loss protection times four (450 days)
			ZeroImpLossProtectionBlocks:         720_000,             // number of blocks before a liquidity provider gets 0% impermanent loss protection              (50 days)
			MinTxOutVolumeThreshold:             100_000_00000000,    // total txout volume (in cacao) a block needs to have to slow outbound transactions
			TxOutDelayRate:                      2500_00000000,       // outbound cacao per block rate for scheduled transactions (excluding native assets)
			TxOutDelayMax:                       17280,               // max number of blocks a transaction can be delayed
			MaxTxOutOffset:                      720,                 // max blocks to offset a txout into a future block
			TNSRegisterFee:                      1000_00000000,       // registration fee for new MAYAName
			TNSFeeOnSale:                        1000,                // fee for TNS sale in basis points
			TNSFeePerBlock:                      2000,                // per block cost for TNS, in cacao
			PermittedSolvencyGap:                100,                 // the setting is in basis points
			ValidatorMaxRewardRatio:             1,                   // the ratio to MinimumBondInCacao at which validators stop receiving rewards proportional to their bond
			PoolDepthForYggFundingMin:           50_000_000_00000000, // the minimum pool depth in CACAO required for ygg funding
			MaxNodeToChurnOutForLowVersion:      1,                   // the maximum number of nodes to churn out for low version per churn
			MayaFundPerc:                        10,                  // The percentage for the Maya Fund when the gas is distribute
			MinCacaoForMayaFundDist:             100_00000000,        // The minimum amount of tokens needed to distribute on MayaFund
			WithdrawLimitTier1:                  50,                  // Withdraw limit value for tier 1 (0.5%)
			WithdrawLimitTier2:                  150,                 // Withdraw limit value for tier 2 (1.5%)
			WithdrawLimitTier3:                  450,                 // Withdraw limit value for tier 3 (4.5%)
			WithdrawDaysTier1:                   200,                 // Days in which the withdraw limit is active for tier 1
			WithdrawDaysTier2:                   90,                  // Days in which the withdraw limit is active for tier 2
			WithdrawDaysTier3:                   30,                  // Days in which the withdraw limit is active for tier 3
			WithdrawTier1:                       1,                   // Value of withdraw tier 1
			WithdrawTier2:                       2,                   // Value of withdraw tier 2
			WithdrawTier3:                       3,                   // Value of withdraw tier 3
			InflationPercentageThreshold:        9000,                // Percentage threshold in which inflation should be 0 (90.00%)
			InflationPoolPercentage:             50,                  // Percentage that goes to the pools for the 100% minted (50%)
			InflationFormulaMulValue:            4000,                // Value for multiplying the dynamic inflation formula (40.00%)
			InflationFormulaSumValue:            100,                 // Value for adding the dynamic inflation formula (1.00%)
			POLMaxNetworkDeposit:                0,                   // Maximum amount of cacao deposited into the pools
			POLMaxPoolMovement:                  0,                   // Maximum amount of cacao to enter/exit a pool per iteration. This is in basis points of the pool cacao depth
			POLSynthUtilization:                 0,                   // target synth utilization for POL (basis points)
			POLBuffer:                           0,                   // buffer around the POL synth utilization (basis points added to/subtracted from POLSynthUtilization basis points)
			RagnarokProcessNumOfLPPerIteration:  200,                 // the number of LP to be processed per iteration during ragnarok pool
			SynthYieldBasisPoints:               6000,                // amount of the yield the capital earns the synth holder receives
			SynthYieldCycle:                     0,                   // number of blocks when the network pays out rewards to yield bearing synths
			MinimumL1OutboundFeeUSD:             1,                   // Minimum fee in USD to charge for LP swap, default to $0.01 , nodes need to vote it to a larger value
			MinimumPoolLiquidityFee:             0,                   // Minimum liquidity fee made by the pool,active pool fail to meet this within a PoolCycle will be demoted
			SubsidizeReserveMultiplier:          100,                 // Multiplier for the needed reserve amount to subsidize pools
			AllowWideBlame:                      0,                   // Allow multiple nodes to be blamed disregarding the majority that it represents
			TargetOutboundFeeSurplusRune:        100_000_00000000,    // Target amount of RUNE for Outbound Fee Surplus: the sum of the diff between outbound cost to user and outbound cost to network
			MaxOutboundFeeMultiplierBasisPoints: 30_000,              // Maximum multiplier applied to base outbound fee charged to user, in basis points
			MinOutboundFeeMultiplierBasisPoints: 15_000,              // Minimum multiplier applied to base outbound fee charged to user, in basis points
			SlipFeeAddedBasisPoints:             0,                   // Default slip fee for swap
			PayBPNodeRewards:                    1,                   // Enable/disable payment for the bond provider node rewards
			MaxSynthsForSaversYield:             1500,                // Maximum perc of synths that can be used for savers yield
			StreamingSwapPause:                  0,                   // pause streaming swaps from being processed or accepted
			StreamingSwapMinBPFee:               0,                   // min basis points for a streaming swap trade
			StreamingSwapMaxLength:              14400,               // max number of blocks a streaming swap can trade for
			StreamingSwapMaxLengthNative:        14400 * 365,         // max number of blocks native streaming swaps can trade over
			SaversStreamingSwapsInterval:        0,                   // For Savers deposits and withdraws, the streaming swaps interval to use for the Native <> Synth swap
			RescheduleCoalesceBlocks:            0,                   // number of blocks to coalesce rescheduled outbounds
		},
		boolValues: map[ConstantName]bool{
			StrictBondLiquidityRatio: false,
		},
		stringValues: map[ConstantName]string{
			DefaultPoolStatus: "Staged",
		},
	}
}
