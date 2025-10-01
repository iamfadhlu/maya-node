//go:build mocknet
// +build mocknet

package utxo

import (
	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

func GetConfMulBasisPoint(chain string, bridge mayaclient.MayachainBridge) (cosmos.Uint, error) {
	return cosmos.NewUint(1), nil
}

func MaxConfAdjustment(confirm uint64, chain string, bridge mayaclient.MayachainBridge) (uint64, error) {
	return 1, nil
}
