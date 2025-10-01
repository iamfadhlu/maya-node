//go:build !mocknet
// +build !mocknet

package utxo

import (
	"fmt"

	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

func GetConfMulBasisPoint(chain string, bridge mayaclient.MayachainBridge) (cosmos.Uint, error) {
	confMulKey := fmt.Sprintf("ConfMultiplierBasisPoints-%s", chain)
	confMultiplier, err := bridge.GetMimir(confMulKey)
	// should never be negative
	if err != nil || confMultiplier <= 0 {
		return cosmos.NewUint(constants.MaxBasisPts), err
	}
	return cosmos.NewUint(uint64(confMultiplier)), nil
}

func MaxConfAdjustment(confirm uint64, chain string, bridge mayaclient.MayachainBridge) (uint64, error) {
	maxConfKey := fmt.Sprintf("MaxConfirmations-%s", chain)
	maxConfirmations, err := bridge.GetMimir(maxConfKey)
	if err != nil || maxConfirmations <= 0 {
		return confirm, err
	}
	if maxConfirmations > 0 && confirm > uint64(maxConfirmations) {
		confirm = uint64(maxConfirmations)
	}
	return confirm, nil
}
