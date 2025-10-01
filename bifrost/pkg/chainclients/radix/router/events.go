package router

import (
	"errors"

	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/radix/decimal"

	ret "github.com/radixdlt/radix-engine-toolkit-go/v2/radix_engine_toolkit_uniffi"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/radix/types"

	"github.com/radixdlt/maya/radix_core_api_client/models"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

var (
	DepositEventName  = "MayaRouterDepositEvent"
	WithdrawEventName = "MayaRouterWithdrawEvent"
)

type DepositEvent struct {
	Sender                  string
	VaultAddress            string
	ResourceAddress         string
	AmountInDecimalSubunits cosmos.Uint
	Memo                    string
}

type WithdrawEvent struct {
	VaultAddress            string
	IntendedRecipient       string
	ResourceAddress         string
	AmountInDecimalSubunits cosmos.Uint
	Memo                    string
	AggregatorAddress       string
	AggregatorTargetAddress string
	AggregatorMinAmount     *cosmos.Uint
}

func DecodeDepositEventFromApiEvent(event models.Eventable, network types.Network) (DepositEvent, error) {
	fields, ok := event.GetData().GetProgrammaticJson().GetAdditionalData()["fields"].([]interface{})
	if !ok {
		return DepositEvent{}, errors.New("failed to decode Radix router deposit event (missing programmatic_json fields)")
	}

	if len(fields) != 5 {
		return DepositEvent{}, errors.New("failed to decode Radix router deposit event (expected exactly 5 fields)")
	}

	sender, ok := fields[0].(map[string]interface{})["value"].(*string)
	if !ok {
		return DepositEvent{}, errors.New("failed to decode Radix router deposit event (missing \"sender\" field value)")
	}

	vaultAddress, ok := fields[1].(map[string]interface{})["value"].(*string)
	if !ok {
		return DepositEvent{}, errors.New("failed to decode Radix router deposit event (missing \"vault_address\" field value)")
	}

	resourceAddress, ok := fields[2].(map[string]interface{})["value"].(*string)
	if !ok {
		return DepositEvent{}, errors.New("failed to decode Radix router deposit event (missing \"resource_address\" field value)")
	}

	amount, ok := fields[3].(map[string]interface{})["value"].(*string)
	if !ok {
		return DepositEvent{}, errors.New("failed to decode Radix router deposit event (missing \"amount\" field value)")
	}
	amountDecimal, err := ret.NewDecimal(*amount)
	if err != nil {
		return DepositEvent{}, err
	}
	amountUintSubunits, err := decimal.NonNegativeDecimalToUintSubunits(amountDecimal)
	if err != nil {
		return DepositEvent{}, err
	}

	memo, ok := fields[4].(map[string]interface{})["value"].(*string)
	if !ok {
		return DepositEvent{}, errors.New("failed to decode Radix router deposit event (missing \"memo\" field value)")
	}

	return DepositEvent{
		Sender:                  *sender,
		VaultAddress:            *vaultAddress,
		ResourceAddress:         *resourceAddress,
		AmountInDecimalSubunits: amountUintSubunits,
		Memo:                    *memo,
	}, nil
}

func DecodeWithdrawEventFromApiEvent(event models.Eventable, network types.Network) (WithdrawEvent, error) {
	fields, ok := event.GetData().GetProgrammaticJson().GetAdditionalData()["fields"].([]interface{})
	if !ok {
		return WithdrawEvent{}, errors.New("failed to decode Radix router withdraw event (missing programmatic_json fields)")
	}

	if len(fields) != 6 {
		return WithdrawEvent{}, errors.New("failed to decode Radix router withdraw event (expected exactly 6 fields)")
	}

	vaultAddress, ok := fields[0].(map[string]interface{})["value"].(*string)
	if !ok {
		return WithdrawEvent{}, errors.New("failed to decode Radix router withdraw event (missing \"vault_address\" field value)")
	}

	intendedRecipient, ok := fields[1].(map[string]interface{})["value"].(*string)
	if !ok {
		return WithdrawEvent{}, errors.New("failed to decode Radix router withdraw event (missing \"intended_recipient\" field value)")
	}

	resourceAddress, ok := fields[2].(map[string]interface{})["value"].(*string)
	if !ok {
		return WithdrawEvent{}, errors.New("failed to decode Radix router withdraw event (missing \"resource_address\" field value)")
	}

	amount, ok := fields[3].(map[string]interface{})["value"].(*string)
	if !ok {
		return WithdrawEvent{}, errors.New("failed to decode Radix router withdraw event (missing \"amount\" field value)")
	}
	amountDecimal, err := ret.NewDecimal(*amount)
	if err != nil {
		return WithdrawEvent{}, err
	}
	amountUintSubunits, err := decimal.NonNegativeDecimalToUintSubunits(amountDecimal)
	if err != nil {
		return WithdrawEvent{}, err
	}

	memo, ok := fields[4].(map[string]interface{})["value"].(*string)
	if !ok {
		return WithdrawEvent{}, errors.New("failed to decode Radix router withdraw event (missing \"memo\" field value)")
	}

	aggregator, ok := fields[5].(map[string]interface{})
	if !ok {
		return WithdrawEvent{}, errors.New("failed to decode Radix router withdraw event (missing \"aggregator\" field)")
	}

	aggregatorVariantId, ok := aggregator["variant_id"].(*string)
	if !ok {
		return WithdrawEvent{}, errors.New("failed to decode Radix router withdraw event (missing \"aggregator\" variant_id)")
	}

	emptyStr := ""
	aggregatorAddress := &emptyStr
	aggregatorTargetAddress := &emptyStr
	var aggregatorMinAmount *cosmos.Uint = nil

	switch *aggregatorVariantId {
	case "0":
		// None - no-op
	case "1":
		// Some - parse the fields
		aggregatorFields, ok := aggregator["fields"].([]interface{})
		if !ok {
			return WithdrawEvent{}, errors.New("failed to decode Radix router withdraw event (missing \"aggregator\" fields)")
		}

		if len(aggregatorFields) != 1 {
			return WithdrawEvent{}, errors.New("failed to decode Radix router withdraw event (expected exactly 1 aggregator field)")
		}

		aggregatorInfoFields, ok := aggregatorFields[0].(map[string]interface{})["fields"].([]interface{})
		if !ok {
			return WithdrawEvent{}, errors.New("failed to decode Radix router withdraw event (missing aggregator info fields)")
		}

		if len(aggregatorInfoFields) != 3 {
			return WithdrawEvent{}, errors.New("failed to decode Radix router withdraw event (expected exactly 3 aggregator info fields)")
		}

		aggregatorAddress, ok = aggregatorInfoFields[0].(map[string]interface{})["value"].(*string)
		if !ok {
			return WithdrawEvent{}, errors.New("failed to decode Radix router withdraw event (missing aggregator address field value)")
		}

		aggregatorTargetAddress, ok = aggregatorInfoFields[1].(map[string]interface{})["value"].(*string)
		if !ok {
			return WithdrawEvent{}, errors.New("failed to decode Radix router withdraw event (missing aggregator target address field value)")
		}

		aggregatorMinAmountStr, ok := aggregatorInfoFields[2].(map[string]interface{})["value"].(*string)
		if !ok {
			return WithdrawEvent{}, errors.New("failed to decode Radix router withdraw event (missing aggregator min amount field value)")
		}
		aggregatorMinAmountRet, err := ret.NewDecimal(*aggregatorMinAmountStr)
		if err != nil {
			return WithdrawEvent{}, err
		}
		aggregatorMinAmountUint, err := decimal.NonNegativeDecimalToUintSubunits(aggregatorMinAmountRet)
		if err != nil {
			return WithdrawEvent{}, err
		}
		aggregatorMinAmount = &aggregatorMinAmountUint
	default:
		return WithdrawEvent{}, errors.New("failed to decode Radix router withdraw event (invalid aggregator variant_id)")
	}

	return WithdrawEvent{
		VaultAddress:            *vaultAddress,
		IntendedRecipient:       *intendedRecipient,
		ResourceAddress:         *resourceAddress,
		AmountInDecimalSubunits: amountUintSubunits,
		Memo:                    *memo,
		AggregatorAddress:       *aggregatorAddress,
		AggregatorTargetAddress: *aggregatorTargetAddress,
		AggregatorMinAmount:     aggregatorMinAmount,
	}, nil
}
