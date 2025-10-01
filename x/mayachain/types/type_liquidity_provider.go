package types

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/blang/semver"

	"github.com/cosmos/cosmos-sdk/codec"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

var _ codec.ProtoMarshaler = &LiquidityProvider{}

// LiquidityProviders a list of liquidity providers
type LiquidityProviders []LiquidityProvider

// Valid check whether lp represent valid information
func (m *LiquidityProvider) Valid() error {
	if m.LastAddHeight == 0 {
		return errors.New("last add liquidity height cannot be empty")
	}
	if m.AssetAddress.IsEmpty() && m.CacaoAddress.IsEmpty() {
		return errors.New("asset address and rune address cannot be empty")
	}
	return nil
}

func (lp LiquidityProvider) GetAddress() common.Address {
	if !lp.CacaoAddress.IsEmpty() {
		return lp.CacaoAddress
	}
	return lp.AssetAddress
}

// Key return a string which can be used to identify lp
func (lp LiquidityProvider) Key() string {
	return fmt.Sprintf("%s/%s", lp.Asset.String(), lp.GetAddress().String())
}

func (lp LiquidityProvider) GetLuviDepositValue(pool Pool) (error, cosmos.Uint) {
	if !lp.Asset.Equals(pool.Asset) {
		return fmt.Errorf("LP and Pool assets do not match (%s, %s)", lp.Asset.String(), pool.Asset.String()), cosmos.ZeroUint()
	}

	bigInt := &big.Int{}
	runeDeposit := lp.CacaoDepositValue.MulUint64(4).BigInt()
	assetDeposit := lp.AssetDepositValue.MulUint64(4).BigInt()
	num := bigInt.Mul(runeDeposit, assetDeposit)
	num = bigInt.Sqrt(num)
	denom := lp.Units.BigInt()
	if len(denom.Bits()) == 0 {
		return nil, cosmos.ZeroUint()
	}
	result := bigInt.Quo(num, denom)
	return nil, cosmos.NewUintFromBigInt(result)
}

func (lp LiquidityProvider) GetRuneRedeemValue(version semver.Version, pool Pool, synthSupply cosmos.Uint) (error, cosmos.Uint) {
	if !lp.Asset.Equals(pool.Asset) {
		return fmt.Errorf("LP and Pool assets do not match (%s, %s)", lp.Asset.String(), pool.Asset.String()), cosmos.ZeroUint()
	}

	bigInt := &big.Int{}
	lpUnits := lp.Units.BigInt()
	poolRuneDepth := pool.BalanceCacao.BigInt()
	num := bigInt.Mul(lpUnits, poolRuneDepth)

	pool.CalcUnits(version, synthSupply)
	denom := pool.GetPoolUnits().BigInt()
	if len(denom.Bits()) == 0 {
		return nil, cosmos.ZeroUint()
	}
	result := bigInt.Quo(num, denom)
	return nil, cosmos.NewUintFromBigInt(result)
}

func (lp LiquidityProvider) GetAssetRedeemValue(version semver.Version, pool Pool, synthSupply cosmos.Uint) (error, cosmos.Uint) {
	if !lp.Asset.Equals(pool.Asset) {
		return fmt.Errorf("LP and Pool assets do not match (%s, %s)", lp.Asset.String(), pool.Asset.String()), cosmos.ZeroUint()
	}

	bigInt := &big.Int{}
	lpUnits := lp.Units.BigInt()
	poolAssetDepth := pool.BalanceAsset.BigInt()
	num := bigInt.Mul(lpUnits, poolAssetDepth)

	pool.CalcUnits(version, synthSupply)
	denom := pool.GetPoolUnits().BigInt()
	if len(denom.Bits()) == 0 {
		return nil, cosmos.ZeroUint()
	}
	result := bigInt.Quo(num, denom)
	return nil, cosmos.NewUintFromBigInt(result)
}

func (lp LiquidityProvider) GetLuviRedeemValue(runeRedeemValue, assetRedeemValue cosmos.Uint) (error, cosmos.Uint) {
	bigInt := &big.Int{}
	runeValue := runeRedeemValue.MulUint64(1e8).BigInt()
	assetValue := assetRedeemValue.MulUint64(1e8).BigInt()
	num := bigInt.Mul(runeValue, assetValue)
	num = bigInt.Sqrt(num)
	denom := lp.Units.BigInt()
	if len(denom.Bits()) == 0 {
		return nil, cosmos.ZeroUint()
	}
	result := bigInt.Quo(num, denom)
	return nil, cosmos.NewUintFromBigInt(result)
}

func (lp LiquidityProvider) GetSaversAssetRedeemValue(pool Pool) cosmos.Uint {
	bigInt := &big.Int{}
	lpUnits := lp.Units.BigInt()
	saversDepth := pool.BalanceAsset.BigInt()
	num := bigInt.Mul(lpUnits, saversDepth)
	denom := pool.LPUnits.BigInt()
	if len(denom.Bits()) == 0 {
		return cosmos.ZeroUint()
	}
	result := bigInt.Quo(num, denom)
	return cosmos.NewUintFromBigInt(result)
}

// Deprecated: do not use
func (lp LiquidityProvider) IsLiquidityBondProvider() bool {
	return !lp.NodeBondAddress.Empty()
}

// Deprecated: do not use
func (lps LiquidityProviders) SetNodeAccount(na cosmos.AccAddress) {
	for i := range lps {
		lps[i].NodeBondAddress = na
	}
}

// Bond creates a new bond record for the given node or increase its units if it already exists
func (lp *LiquidityProvider) Bond(nodeAddr cosmos.AccAddress, units cosmos.Uint) {
	for i := range lp.BondedNodes {
		if lp.BondedNodes[i].NodeAddress.Equals(nodeAddr) {
			lp.BondedNodes[i].Units = lp.BondedNodes[i].Units.Add(units)
			return
		}
	}

	lp.BondedNodes = append(lp.BondedNodes, LPBondedNode{
		NodeAddress: nodeAddr,
		Units:       units,
	})
}

// Unbond removes a bond record for the given node or decrease its units if it already exists
func (lp *LiquidityProvider) Unbond(nodeAddr cosmos.AccAddress, units cosmos.Uint) {
	// Soft migration to new bond model
	if !lp.NodeBondAddress.Empty() {
		lp.NodeBondAddress = nil

		if lp.Units.GT(units) {
			lp.BondedNodes = []LPBondedNode{
				{
					NodeAddress: nodeAddr,
					Units:       common.SafeSub(lp.Units, units),
				},
			}
		}

		return
	}

	for i := range lp.BondedNodes {
		if lp.BondedNodes[i].NodeAddress.Equals(nodeAddr) {
			lp.BondedNodes[i].Units = common.SafeSub(lp.BondedNodes[i].Units, units)
			if lp.BondedNodes[i].Units.IsZero() {
				lp.BondedNodes = append(lp.BondedNodes[:i], lp.BondedNodes[i+1:]...)
			}
			return
		}
	}
}

// GetRemainingUnits returns the number of LP units that are not bonded to a node
func (lp *LiquidityProvider) GetRemainingUnits() cosmos.Uint {
	if !lp.NodeBondAddress.Empty() {
		return cosmos.ZeroUint()
	}

	bondedUnits := cosmos.ZeroUint()
	for _, bond := range lp.BondedNodes {
		bondedUnits = bondedUnits.Add(bond.Units)
	}

	return common.SafeSub(lp.Units, bondedUnits)
}

// GetUnitsBondedToNode returns the number of LP units that are bonded to the given node
func (lp LiquidityProvider) GetUnitsBondedToNode(nodeAddr cosmos.AccAddress) cosmos.Uint {
	if lp.NodeBondAddress.Equals(nodeAddr) {
		return lp.Units
	}

	for _, bond := range lp.BondedNodes {
		if bond.NodeAddress.Equals(nodeAddr) {
			return bond.Units
		}
	}

	return cosmos.ZeroUint()
}

// IsEmpty returns true when the LP is empty
func (bn LPBondedNode) IsEmpty() bool {
	return bn.NodeAddress.Empty()
}
