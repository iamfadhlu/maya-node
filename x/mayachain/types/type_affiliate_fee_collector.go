package types

import (
	"errors"

	ctypes "github.com/cosmos/cosmos-sdk/types"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

// NewAffiliateFeeCollector create a new instance of AffiliateFeeCollector
func NewAffiliateFeeCollector(owner ctypes.AccAddress, cacaoAmount cosmos.Uint) AffiliateFeeCollector {
	return AffiliateFeeCollector{
		OwnerAddress: owner,
		CacaoAmount:  cacaoAmount,
	}
}

// Valid - check whether AffiliateFeeCollector struct represent valid information
func (m *AffiliateFeeCollector) Valid() error {
	if m.OwnerAddress.Empty() {
		return errors.New("affiliate fee collector owner address can't be empty")
	}
	if err := cosmos.VerifyAddressFormat(m.OwnerAddress); err != nil {
		return cosmos.ErrInvalidAddress(m.OwnerAddress.String())
	}
	return nil
}
