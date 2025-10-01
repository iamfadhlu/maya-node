package types

import (
	"fmt"

	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

// MaxAffiliateFeeBasisPoints basis points for withdrawals
const LegacyMaxAffiliateFeeBasisPoints = 1_000

// ValidateBasicV63 runs stateless checks on the message
func (m *MsgSwap) ValidateBasicV63(version semver.Version) error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if err := m.Tx.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	if m.TargetAsset.IsEmpty() {
		return cosmos.ErrUnknownRequest("swap Target cannot be empty")
	}
	if len(m.Tx.Coins) > 1 {
		return cosmos.ErrUnknownRequest("not expecting multiple coins in a swap")
	}
	if m.Tx.Coins.IsEmpty() {
		return cosmos.ErrUnknownRequest("swap coin cannot be empty")
	}
	for _, coin := range m.Tx.Coins {
		if coin.Asset.Equals(m.TargetAsset) {
			return cosmos.ErrUnknownRequest("swap Source and Target cannot be the same.")
		}
	}
	if m.Tx.Coins.HasNoneNativeRune() {
		return cosmos.ErrUnknownRequest("only NATIVE RUNE can be used for swap")
	}
	if m.Destination.IsEmpty() {
		return cosmos.ErrUnknownRequest("swap Destination cannot be empty")
	}
	if m.AffiliateAddress.IsEmpty() && !m.AffiliateBasisPoints.IsZero() {
		return cosmos.ErrUnknownRequest("swap affiliate address is empty while affiliate basis points is non-zero")
	}
	if !m.AffiliateBasisPoints.IsZero() && m.AffiliateBasisPoints.GT(cosmos.NewUint(LegacyMaxAffiliateFeeBasisPoints)) {
		return cosmos.ErrUnknownRequest(fmt.Sprintf("affiliate fee basis points can't be more than %d", LegacyMaxAffiliateFeeBasisPoints))
	}
	if !m.Destination.IsNoop() && !m.Destination.IsChain(m.TargetAsset.GetChain(), version) {
		return cosmos.ErrUnknownRequest("swap destination address is not the same chain as the target asset")
	}
	if !m.AffiliateAddress.IsEmpty() && !m.AffiliateAddress.IsChain(common.BASEChain, version) {
		return cosmos.ErrUnknownRequest("swap affiliate address must be a MAYA address")
	}
	if len(m.Aggregator) != 0 && len(m.AggregatorTargetAddress) == 0 {
		return cosmos.ErrUnknownRequest("aggregator target asset address is empty")
	}
	if len(m.AggregatorTargetAddress) > 0 && len(m.Aggregator) == 0 {
		return cosmos.ErrUnknownRequest("aggregator is empty")
	}
	return nil
}
