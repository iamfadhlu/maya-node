package radix

import (
	"fmt"

	"gitlab.com/mayachain/mayanode/common/cosmos"

	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/radix/decimal"

	"github.com/radixdlt/maya/radix_core_api_client/models"
	ret "github.com/radixdlt/radix-engine-toolkit-go/v2/radix_engine_toolkit_uniffi"
	"gitlab.com/mayachain/mayanode/common"
)

var (
	XrdFeeEstimateInRadixSubunits = cosmos.NewUint(5000000000000000000) // 5 XRD
	XrdFeeEstimateInMayaSubunits  = DecimalSubunitsToMayaRoundingDown(XrdFeeEstimateInRadixSubunits)
)

func GetTotalTxnFee(receipt models.TransactionReceiptable) (common.Gas, error) {
	feeSummary := receipt.GetFeeSummary()

	total, err := ret.NewDecimal("0")
	if err != nil {
		return common.Gas{}, fmt.Errorf("could not create new decimal: %w", err)
	}

	xrdTotalExecutionCostStr := *feeSummary.GetXrdTotalExecutionCost()
	xrdTotalExecutionCost, err := ret.NewDecimal(xrdTotalExecutionCostStr)
	if err != nil {
		return common.Gas{}, fmt.Errorf("could not create Decimal from %s: %w", xrdTotalExecutionCostStr, err)
	}
	total, err = total.Add(xrdTotalExecutionCost)
	if err != nil {
		return common.Gas{}, fmt.Errorf("decimal addition failed: %w", err)
	}

	xrdTotalFinalizationCostStr := *feeSummary.GetXrdTotalFinalizationCost()
	xrdTotalFinalizationCost, err := ret.NewDecimal(xrdTotalFinalizationCostStr)
	if err != nil {
		return common.Gas{}, fmt.Errorf("could not create Decimal from %s: %w", xrdTotalFinalizationCostStr, err)
	}
	total, err = total.Add(xrdTotalFinalizationCost)
	if err != nil {
		return common.Gas{}, fmt.Errorf("decimal addition failed: %w", err)
	}

	xrdTotalRoyaltyCostStr := *feeSummary.GetXrdTotalRoyaltyCost()
	xrdTotalRoyaltyCost, err := ret.NewDecimal(xrdTotalRoyaltyCostStr)
	if err != nil {
		return common.Gas{}, fmt.Errorf("could not create Decimal from %s: %w", xrdTotalRoyaltyCostStr, err)
	}
	total, err = total.Add(xrdTotalRoyaltyCost)
	if err != nil {
		return common.Gas{}, fmt.Errorf("decimal addition failed: %w", err)
	}

	xrdTotalStorageCostStr := *feeSummary.GetXrdTotalStorageCost()
	xrdTotalStorageCost, err := ret.NewDecimal(xrdTotalStorageCostStr)
	if err != nil {
		return common.Gas{}, fmt.Errorf("could not create Decimal from %s: %w", xrdTotalStorageCostStr, err)
	}
	total, err = total.Add(xrdTotalStorageCost)
	if err != nil {
		return common.Gas{}, fmt.Errorf("decimal addition failed: %w", err)
	}

	xrdTotalTippingCostStr := *feeSummary.GetXrdTotalTippingCost()
	xrdTotalTippingCost, err := ret.NewDecimal(xrdTotalTippingCostStr)
	if err != nil {
		return common.Gas{}, fmt.Errorf("could not create Decimal from %s: %w", xrdTotalTippingCostStr, err)
	}
	total, err = total.Add(xrdTotalTippingCost)
	if err != nil {
		return common.Gas{}, fmt.Errorf("decimal addition failed: %w", err)
	}

	totalUintSubunits, err := decimal.NonNegativeDecimalToUintSubunits(total)
	if err != nil {
		return common.Gas{}, fmt.Errorf("failed to convert decimal to uint subunits: %w", err)
	}

	return common.Gas{
		{
			Asset:  common.XRDAsset,
			Amount: DecimalSubunitsToMayaRoundingUp(totalUintSubunits),
		},
	}, nil
}
