//go:build testnet || (mocknet && !regtest)
// +build testnet mocknet,!regtest

package mayachain

import (
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

// migrateStoreV86 remove all LTC asset from the retiring vault
func migrateStoreV86(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v86", "error", err)
		}
	}()
	vaults, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, RetiringVault)
	if err != nil {
		ctx.Logger().Error("fail to get retiring asgard vaults", "error", err)
		return
	}
	for _, v := range vaults {
		ethCoin := v.GetCoin(common.ETHAsset)
		v.SubFunds(common.NewCoins(ethCoin))
		if err := mgr.Keeper().SetVault(ctx, v); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
		}
	}
}

func migrateStoreV88(ctx cosmos.Context, mgr Manager)  {}
func migrateStoreV90(ctx cosmos.Context, mgr Manager)  {}
func migrateStoreV96(ctx cosmos.Context, mgr Manager)  {}
func migrateStoreV102(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV104(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV105(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV106(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV107(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV108(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV109(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV110(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV111(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV112(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV113(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV114(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV115(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV116(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV117(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV118(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV119(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV120(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV121(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV122(ctx cosmos.Context, mgr Manager) {}
func migrateStoreV123(ctx cosmos.Context, mgr Manager) {}
