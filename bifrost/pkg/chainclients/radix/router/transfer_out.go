package router

import (
	"fmt"

	stypes "gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/radix/decimal"

	ret "github.com/radixdlt/radix-engine-toolkit-go/v2/radix_engine_toolkit_uniffi"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/radix/types"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	mem "gitlab.com/mayachain/mayanode/x/mayachain/memo"
)

var (
	TransactionValidityEpochs uint64 = 2
	TipPercentage             uint16 = 0
)

func BuildTransferOutTransaction(
	routerForWithdrawals string,
	routerForDeposits string,
	currentEpoch uint64,
	nonce uint32,
	resourceAddress string,
	resourceAmount *ret.Decimal,
	memo mem.Memo,
	maxFee *ret.Decimal,
	network types.Network,
	tx stypes.TxOutItem,
) (*ret.SignedIntent, error) {
	senderPubKeyBytes, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeAccPub, string(tx.VaultPubKey))
	if err != nil {
		return nil, fmt.Errorf("manifest creation error: %w", err)
	}
	recipientAddress := tx.ToAddress.String()
	retPubKey := ret.PublicKeySecp256k1{Value: senderPubKeyBytes.Bytes()}
	vaultAddress, err := ret.AddressVirtualAccountAddressFromPublicKey(retPubKey, network.Id)
	if err != nil {
		return nil, fmt.Errorf("manifest creation error: %w", err)
	}

	manifestBuilder, err := NewManifestBuilderWrapper(routerForWithdrawals, routerForDeposits)
	if err != nil {
		return nil, fmt.Errorf("manifest creation error: %w", err)
	}

	// We're always executing the following two manifest instructions:
	// LOCK_FEE
	err = manifestBuilder.routerLockFee(vaultAddress, maxFee)
	if err != nil {
		return nil, fmt.Errorf("manifest creation error: %w", err)
	}
	// CALL_METHOD "withdraw"
	err = manifestBuilder.routerWithdraw(vaultAddress, resourceAddress, recipientAddress, resourceAmount, memo, tx)
	if err != nil {
		return nil, fmt.Errorf("manifest creation error: %w", err)
	}

	// If an aggregator is being used, use it to swap the resources and put the result in "outBucket",
	// else put the withdrawn funds in "outBucket".
	outBucketName := "outBucket"

	if tx.Aggregator != "" {
		aggregatorInBucketName := "aggInBucket"
		// TAKE_ALL_FROM_WORKTOP into aggInBucket
		err = manifestBuilder.takeAllFromWorktop(resourceAddress, aggregatorInBucketName)
		if err != nil {
			return nil, fmt.Errorf("manifest creation error: %w", err)
		}

		// CALL_METHOD "swap_out"
		err = manifestBuilder.aggregatorSwapOut(aggregatorInBucketName, tx.Aggregator, tx.AggregatorTargetAsset, tx.AggregatorTargetLimit)
		if err != nil {
			return nil, fmt.Errorf("manifest creation error: %w", err)
		}

		// ASSERT_WORKTOP_CONTAINS at least tx.AggregatorTargetLimit of tx.AggregatorTargetAsset
		if tx.AggregatorTargetLimit != nil {
			err = manifestBuilder.assertWorktopContains(tx.AggregatorTargetAsset, tx.AggregatorTargetLimit)
			if err != nil {
				return nil, fmt.Errorf("manifest creation error: %w", err)
			}
		}

		// TAKE_ALL_FROM_WORKTOP and put into outBucket
		err = manifestBuilder.takeAllFromWorktop(tx.AggregatorTargetAsset, outBucketName)
		if err != nil {
			return nil, fmt.Errorf("manifest creation error: %w", err)
		}
	} else {
		// No additional actions needed if the transaction isn't using an aggregator,
		// so just put the withdrawn resource into the output bucket.
		err = manifestBuilder.takeAllFromWorktop(resourceAddress, outBucketName)
		if err != nil {
			return nil, fmt.Errorf("manifest creation error: %w", err)
		}
	}

	// At this point the resultant resource is in outBucket, and we just need to transfer it
	// to the end user or deposit back to the router under a new vault key, depending on the memo.

	switch memo.GetType() {
	case mem.TxOutbound, mem.TxRefund, mem.TxRagnarok:
		err = manifestBuilder.routerTransfer(recipientAddress, outBucketName)
		if err != nil {
			return nil, fmt.Errorf("manifest creation error: %w", err)
		}
	case mem.TxMigrate:
		err = manifestBuilder.routerDirectDeposit(recipientAddress, outBucketName)
		if err != nil {
			return nil, fmt.Errorf("manifest creation error: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported memo type %s", memo.GetType().String())
	}

	manifest := manifestBuilder.build()

	// We don't need any signatures here, the notary (that is `tx.VaultPubKey`) will act as a signatory.
	header := ret.TransactionHeader{
		NetworkId:           network.Id,
		StartEpochInclusive: currentEpoch,
		EndEpochExclusive:   currentEpoch + TransactionValidityEpochs,
		Nonce:               nonce,
		NotaryPublicKey:     retPubKey,
		NotaryIsSignatory:   true,
		TipPercentage:       TipPercentage,
	}
	message := ret.MessagePlainText{Value: ret.PlainTextMessage{Message: ret.MessageContentStr{Value: ""}}}
	intent := ret.NewIntent(header, manifest, message)
	signedIntent := ret.NewSignedIntent(intent, []ret.SignatureWithPublicKey{})
	return signedIntent, nil
}

type ManifestBuilderWrapper struct {
	builder              *ret.ManifestBuilder
	networkId            uint8
	routerForWithdrawals *ret.Address
	routerForDeposits    *ret.Address
}

func NewManifestBuilderWrapper(routerForWithdrawals string, routerForDeposits string) (ManifestBuilderWrapper, error) {
	routerForWithdrawalsRet, err := ret.NewAddress(routerForWithdrawals)
	if err != nil {
		return ManifestBuilderWrapper{}, fmt.Errorf("manifest creation error: %w", err)
	}
	routerForDepositsRet, err := ret.NewAddress(routerForDeposits)
	if err != nil {
		return ManifestBuilderWrapper{}, fmt.Errorf("manifest creation error: %w", err)
	}
	return ManifestBuilderWrapper{
		builder:              ret.NewManifestBuilder(),
		routerForWithdrawals: routerForWithdrawalsRet,
		routerForDeposits:    routerForDepositsRet,
	}, nil
}

func (w *ManifestBuilderWrapper) routerLockFee(vaultAddress *ret.Address, feeToLock *ret.Decimal) error {
	manifestFeeToLock := ret.ManifestBuilderValueDecimalValue{Value: feeToLock}
	manifestVaultAddress := ret.ManifestBuilderValueAddressValue{Value: ret.ManifestBuilderAddressStatic{Value: vaultAddress}}

	newBuilder, err := w.builder.CallMethod(
		ret.ManifestBuilderAddressStatic{Value: w.routerForWithdrawals},
		"lock_fee",
		[]ret.ManifestBuilderValue{
			manifestVaultAddress,
			manifestFeeToLock,
		})
	if err != nil {
		return fmt.Errorf("manifest creation error: %w", err)
	}
	w.builder = newBuilder
	return nil
}

func (w *ManifestBuilderWrapper) routerWithdraw(
	vaultAddress *ret.Address,
	resourceAddress string,
	recipientAddress string,
	resourceAmount *ret.Decimal,
	memo mem.Memo,
	tx stypes.TxOutItem,
) error {
	manifestVaultAddress := ret.ManifestBuilderValueAddressValue{Value: ret.ManifestBuilderAddressStatic{Value: vaultAddress}}

	recipientAddressRet, err := ret.NewAddress(recipientAddress)
	if err != nil {
		return fmt.Errorf("manifest creation error: %w", err)
	}
	resourceAddressRet, err := ret.NewAddress(resourceAddress)
	if err != nil {
		return fmt.Errorf("manifest creation error: %w", err)
	}

	recipientAddressManifest := ret.ManifestBuilderValueAddressValue{Value: ret.ManifestBuilderAddressStatic{Value: recipientAddressRet}}
	resourceAddressManifest := ret.ManifestBuilderValueAddressValue{Value: ret.ManifestBuilderAddressStatic{Value: resourceAddressRet}}
	amountManifest := ret.ManifestBuilderValueDecimalValue{Value: resourceAmount}
	memoManifest := ret.ManifestBuilderValueStringValue{Value: memo.String()}

	var aggregatorInfoManifest ret.ManifestBuilderValueEnumValue
	if tx.Aggregator != "" {
		var aggregatorTargetResourceAddressRet *ret.Address
		var aggregatorAddressRet *ret.Address

		aggregatorAddressRet, err = ret.NewAddress(tx.Aggregator)
		if err != nil {
			return fmt.Errorf("manifest creation error: %w", err)
		}
		aggregatorTargetResourceAddressRet, err = ret.NewAddress(tx.AggregatorTargetAsset)
		if err != nil {
			return fmt.Errorf("manifest creation error: %w", err)
		}

		var aggregatorMinAmount *ret.Decimal
		if tx.AggregatorTargetLimit != nil {
			aggregatorMinAmount, err = decimal.UintSubunitsToDecimal(*tx.AggregatorTargetLimit)
			if err != nil {
				return fmt.Errorf("manifest creation error: %w", err)
			}
		} else {
			aggregatorMinAmount, err = ret.NewDecimal("0")
			if err != nil {
				return fmt.Errorf("manifest creation error: %w", err)
			}
		}

		aggregatorInfoManifest = ret.ManifestBuilderValueEnumValue{
			Discriminator: 1,
			Fields: []ret.ManifestBuilderValue{
				ret.ManifestBuilderValueTupleValue{
					Fields: []ret.ManifestBuilderValue{
						ret.ManifestBuilderValueAddressValue{Value: ret.ManifestBuilderAddressStatic{Value: aggregatorAddressRet}},
						ret.ManifestBuilderValueAddressValue{Value: ret.ManifestBuilderAddressStatic{Value: aggregatorTargetResourceAddressRet}},
						ret.ManifestBuilderValueDecimalValue{Value: aggregatorMinAmount},
					},
				},
			},
		}
	} else {
		aggregatorInfoManifest = ret.ManifestBuilderValueEnumValue{Discriminator: 0}
	}

	newBuilder, err := w.builder.CallMethod(
		ret.ManifestBuilderAddressStatic{Value: w.routerForWithdrawals},
		"withdraw",
		[]ret.ManifestBuilderValue{
			manifestVaultAddress,
			resourceAddressManifest,
			recipientAddressManifest,
			aggregatorInfoManifest,
			amountManifest,
			memoManifest,
		})
	if err != nil {
		return fmt.Errorf("manifest creation error: %w", err)
	}
	w.builder = newBuilder
	return nil
}

func (w *ManifestBuilderWrapper) takeAllFromWorktop(resourceAddress string, bucketName string) error {
	resourceAddressRet, err := ret.NewAddress(resourceAddress)
	if err != nil {
		return fmt.Errorf("manifest creation error: %w", err)
	}
	bucketManifest := ret.ManifestBuilderBucket{Name: bucketName}
	newBuilder, err := w.builder.TakeAllFromWorktop(resourceAddressRet, bucketManifest)
	if err != nil {
		return fmt.Errorf("manifest creation error: %w", err)
	}
	w.builder = newBuilder
	return nil
}

func (w *ManifestBuilderWrapper) aggregatorSwapOut(bucketName string, aggregatorAddress string, targetResourceAddress string, targetLimit *cosmos.Uint) error {
	bucketManifest := ret.ManifestBuilderBucket{Name: bucketName}
	aggregatorAddressRet, err := ret.NewAddress(aggregatorAddress)
	if err != nil {
		return fmt.Errorf("manifest creation error: %w", err)
	}
	aggregatorTargetResourceAddressRet, err := ret.NewAddress(targetResourceAddress)
	if err != nil {
		return fmt.Errorf("manifest creation error: %w", err)
	}
	var aggregatorLimitDecimal *ret.Decimal
	if targetLimit != nil {
		aggregatorLimitDecimal, err = decimal.UintSubunitsToDecimal(*targetLimit)
		if err != nil {
			return fmt.Errorf("manifest creation error: %w", err)
		}
	} else {
		aggregatorLimitDecimal, err = ret.NewDecimal("0")
		if err != nil {
			return fmt.Errorf("manifest creation error: %w", err)
		}
	}
	newBuilder, err := w.builder.CallMethod(
		ret.ManifestBuilderAddressStatic{Value: aggregatorAddressRet},
		"swap_out",
		[]ret.ManifestBuilderValue{
			ret.ManifestBuilderValueBucketValue{Value: bucketManifest},
			ret.ManifestBuilderValueAddressValue{Value: ret.ManifestBuilderAddressStatic{Value: aggregatorTargetResourceAddressRet}},
			ret.ManifestBuilderValueDecimalValue{Value: aggregatorLimitDecimal},
		})
	if err != nil {
		return fmt.Errorf("manifest creation error: %w", err)
	}
	w.builder = newBuilder
	return nil
}

func (w *ManifestBuilderWrapper) assertWorktopContains(resourceAddress string, amount *cosmos.Uint) error {
	resourceAddressRet, err := ret.NewAddress(resourceAddress)
	if err != nil {
		return fmt.Errorf("manifest creation error: %w", err)
	}
	amountDecimal, err := decimal.UintSubunitsToDecimal(*amount)
	if err != nil {
		return fmt.Errorf("manifest creation error: %w", err)
	}
	newBuilder, err := w.builder.AssertWorktopContains(resourceAddressRet, amountDecimal)
	if err != nil {
		return fmt.Errorf("manifest creation error: %w", err)
	}
	w.builder = newBuilder
	return nil
}

func (w *ManifestBuilderWrapper) routerTransfer(recipientAddress string, bucketName string) error {
	recipientAddressRet, err := ret.NewAddress(recipientAddress)
	if err != nil {
		return fmt.Errorf("manifest creation error: %w", err)
	}
	recipientAddressManifest := ret.ManifestBuilderValueAddressValue{Value: ret.ManifestBuilderAddressStatic{Value: recipientAddressRet}}
	bucketManifest := ret.ManifestBuilderBucket{Name: bucketName}

	newBuilder, err := w.builder.CallMethod(
		ret.ManifestBuilderAddressStatic{Value: w.routerForWithdrawals},
		"transfer",
		[]ret.ManifestBuilderValue{
			recipientAddressManifest,
			ret.ManifestBuilderValueBucketValue{Value: bucketManifest},
		})
	if err != nil {
		return fmt.Errorf("manifest creation error: %w", err)
	}
	w.builder = newBuilder
	return nil
}

func (w *ManifestBuilderWrapper) routerDirectDeposit(recipientAddress string, bucketName string) error {
	recipientAddressRet, err := ret.NewAddress(recipientAddress)
	if err != nil {
		return fmt.Errorf("manifest creation error: %w", err)
	}
	recipientAddressManifest := ret.ManifestBuilderValueAddressValue{Value: ret.ManifestBuilderAddressStatic{Value: recipientAddressRet}}
	bucketManifest := ret.ManifestBuilderBucket{Name: bucketName}

	newBuilder, err := w.builder.CallMethod(
		ret.ManifestBuilderAddressStatic{Value: w.routerForDeposits},
		"direct_deposit",
		[]ret.ManifestBuilderValue{
			recipientAddressManifest,
			ret.ManifestBuilderValueBucketValue{Value: bucketManifest},
		})
	if err != nil {
		return fmt.Errorf("manifest creation error: %w", err)
	}
	w.builder = newBuilder
	return nil
}

func (w *ManifestBuilderWrapper) build() *ret.TransactionManifest {
	return w.builder.Build(w.networkId)
}
