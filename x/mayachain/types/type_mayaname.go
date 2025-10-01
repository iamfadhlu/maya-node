package types

import (
	"errors"
	"fmt"
	"strings"

	b64 "encoding/base64"

	ctypes "github.com/cosmos/cosmos-sdk/types"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

var EmptyMAYAName = MAYAName{}

// NewMAYAName create a new instance of MAYAName
func NewMAYAName(name string, exp int64, aliases []MAYANameAlias, preferredAsset common.Asset, owner ctypes.AccAddress, affiliateBps cosmos.Uint, subaffiliates []MAYANameSubaffiliate) MAYAName {
	mn := MAYAName{
		Name:              name,
		ExpireBlockHeight: exp,
		Aliases:           aliases,
		PreferredAsset:    preferredAsset,
		Subaffiliates:     subaffiliates,
		Owner:             owner,
	}
	mn.SetAffiliateBps(affiliateBps)
	return mn
}

// Valid - check whether MAYAName struct represent valid information
func (m *MAYAName) Valid() error {
	if len(m.Name) == 0 {
		return errors.New("name can't be empty")
	}
	if len(m.Aliases) == 0 {
		return errors.New("aliases can't be empty")
	}
	for _, a := range m.Aliases {
		if a.Chain.IsEmpty() {
			return errors.New("chain can't be empty")
		}
		if a.Address.IsEmpty() {
			return errors.New("address cannot be empty")
		}
	}
	sumSubAffiliateBps := cosmos.ZeroUint()
	for _, a := range m.Subaffiliates {
		if len(a.Name) == 0 {
			return errors.New("subaffiliate name can't be empty")
		}
		if a.Bps.GT(cosmos.NewUint(constants.MaxBasisPts)) {
			return fmt.Errorf("subaffiliates basis points must not exceed %d", constants.MaxBasisPts)
		}
		sumSubAffiliateBps = sumSubAffiliateBps.Add(a.Bps)
	}
	if sumSubAffiliateBps.GT(cosmos.NewUint(constants.MaxBasisPts)) {
		return fmt.Errorf("the total basis points for subaffiliates must not exceed %d", constants.MaxBasisPts)
	}
	return nil
}

func (m *MAYAName) GetAlias(chain common.Chain) common.Address {
	for _, a := range m.Aliases {
		if a.Chain.Equals(chain) {
			return a.Address
		}
	}
	return common.NoAddress
}

func (m *MAYAName) SetAlias(chain common.Chain, addr common.Address) {
	for i, a := range m.Aliases {
		if a.Chain.Equals(chain) {
			m.Aliases[i].Address = addr
			return
		}
	}
	m.Aliases = append(m.Aliases, MAYANameAlias{Chain: chain, Address: addr})
}

func (m *MAYAName) GetAffiliateBps() cosmos.Uint {
	if m.AffiliateBps == nil {
		return cosmos.ZeroUint()
	}
	return *m.AffiliateBps
}

func (m *MAYAName) SetAffiliateBps(affiliateBps cosmos.Uint) {
	if affiliateBps.IsZero() {
		m.AffiliateBps = nil
	}
	m.AffiliateBps = &affiliateBps
}

func (m *MAYAName) SetSubaffiliate(name string, bps cosmos.Uint) error {
	sumSubAffiliateBps := cosmos.ZeroUint()
	found := -1
	for i, s := range m.Subaffiliates {
		if s.Name == name {
			sumSubAffiliateBps = sumSubAffiliateBps.Add(bps)
			found = i
		} else {
			sumSubAffiliateBps = sumSubAffiliateBps.Add(m.Subaffiliates[i].Bps)
		}
	}
	if found >= 0 {
		sumSubAffiliateBps = sumSubAffiliateBps.Add(bps)
	}
	if sumSubAffiliateBps.GT(cosmos.NewUint(constants.MaxBasisPts)) {
		return fmt.Errorf("the total basis points for subaffiliates must not exceed %d", constants.MaxBasisPts)
	}

	if found >= 0 {
		m.Subaffiliates[found].Bps = bps
		return nil
	}
	m.Subaffiliates = append(m.Subaffiliates, MAYANameSubaffiliate{Name: name, Bps: bps})
	return nil
}

func (m *MAYAName) RemoveSubaffiliate(name string) {
	for i, s := range m.Subaffiliates {
		if s.Name == name {
			m.Subaffiliates = append(m.Subaffiliates[:i], m.Subaffiliates[i+1:]...)
			return
		}
	}
}

func (m *MAYAName) Key() string {
	// key is Base64 endoded
	return b64.StdEncoding.EncodeToString([]byte(strings.ToLower(m.Name)))
}

// CanReceiveAffiliateFee - returns true if the MAYAName can receive an affiliate fee.
// Conditions: - Must have an owner
//   - If no preferred asset, must have an alias for MAYAChain (since fee will be sent in CACAO)
//   - If preferred asset, can receive affiliate fee (since fee is collected in AC module)
func (m *MAYAName) CanReceiveAffiliateFee() bool {
	if m.Owner.Empty() {
		return false
	}
	// If no preferred asset set, fees will be sent to owner
	//
	// if m.PreferredAsset.IsEmpty() {
	// 	return !m.GetAlias(common.BASEChain).IsEmpty()
	// }

	// If preferred asset set, must have an alias for the preferred asset chain
	if !m.PreferredAsset.IsEmpty() {
		return !m.GetAlias(m.PreferredAsset.GetChain()).IsEmpty()
	}
	return true
}
