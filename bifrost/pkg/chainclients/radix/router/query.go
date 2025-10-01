package router

import (
	"fmt"

	"gitlab.com/mayachain/mayanode/common/cosmos"

	ret "github.com/radixdlt/radix-engine-toolkit-go/v2/radix_engine_toolkit_uniffi"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/radix/coreapi"
	"gitlab.com/mayachain/mayanode/common"
)

func GetVaultBalanceInRouter(
	coreApiWrapper *coreapi.CoreApiWrapper,
	routerAddress string,
	vaultPubKey common.PubKey,
	resourceAddress string,
	networkId uint8,
) (*ret.Decimal, error) {
	pk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeAccPub, string(vaultPubKey))
	if err != nil {
		return nil, fmt.Errorf("failed to decode vault public key: %v", err)
	}
	retPubKey := ret.PublicKeySecp256k1{Value: pk.Bytes()}

	vaultAddress, err := ret.AddressVirtualAccountAddressFromPublicKey(retPubKey, networkId)
	if err != nil {
		return nil, fmt.Errorf("failed to encode vault address: %v", err)
	}

	output, err := coreApiWrapper.MethodPreview(
		routerAddress,
		"get_vault_balance",
		[]string{vaultAddress.AsStr(), resourceAddress})
	if err != nil {
		return nil, fmt.Errorf("method call preview failed: %v", err)
	}

	outputValue, ok := output.GetProgrammaticJson().GetAdditionalData()["value"].(*string)
	if !ok {
		return nil, fmt.Errorf("missing output value")
	}

	retDecimal, err := ret.NewDecimal(*outputValue)
	if err != nil {
		return nil, fmt.Errorf("failed to convert output value to RET decimal: %v", err)
	}

	return retDecimal, nil
}
