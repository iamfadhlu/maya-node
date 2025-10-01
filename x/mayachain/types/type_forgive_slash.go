package types

import (
	"errors"

	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

// NewForgiveSlashVoter create a new instance of ForgiveSlashVoter
func NewForgiveSlashVoter(addr cosmos.AccAddress) ForgiveSlashVoter {
	return ForgiveSlashVoter{NodeAddress: addr}
}

// Valid return an error if the node address that need to be banned is empty
func (m *ForgiveSlashVoter) Valid() error {
	if m.NodeAddress.Empty() {
		return errors.New("node address is empty")
	}
	if m.BlockHeight <= 0 {
		return errors.New("block height cannot be equal to or less than zero")
	}
	return nil
}

// IsEmpty return true when the node address is empty
func (m *ForgiveSlashVoter) IsEmpty() bool {
	return m.NodeAddress.Empty()
}

func (m *ForgiveSlashVoter) String() string {
	return m.NodeAddress.String()
}

// HasSigned - check if given address has signed
func (m *ForgiveSlashVoter) HasSigned(signer cosmos.AccAddress) bool {
	for _, sign := range m.GetSigners() {
		if sign.Equals(signer) {
			return true
		}
	}
	return false
}

// Sign add the given signer to the signer list
func (m *ForgiveSlashVoter) Sign(signer cosmos.AccAddress) {
	if !m.HasSigned(signer) {
		m.Signers = append(m.Signers, signer.String())
	}
}

func (m *ForgiveSlashVoter) GetSigners() []cosmos.AccAddress {
	signers := make([]cosmos.AccAddress, 0)
	for _, str := range m.Signers {
		signer, err := cosmos.AccAddressFromBech32(str)
		if err != nil {
			continue
		}
		signers = append(signers, signer)
	}
	return signers
}

// HasConsensus return true if there are majority accounts sign off the ForgiveSlashVoter
func (m *ForgiveSlashVoter) HasConsensus(nodeAccounts NodeAccounts) bool {
	var count int
	for _, signer := range m.GetSigners() {
		if nodeAccounts.IsNodeKeys(signer) {
			count++
		}
	}
	return HasSuperMajority(count, len(nodeAccounts))
}

// HasExpired checks if forgiveslashvoter is no longer relevant since given time has passed.
func (m *ForgiveSlashVoter) HasExpired(ctx cosmos.Context) bool {
	return (ctx.BlockHeight() - m.ProposedBlockHeight) > int64(constants.ForgiveSlashPeriod)
}
