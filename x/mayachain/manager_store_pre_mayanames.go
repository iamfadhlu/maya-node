package mayachain

import (
	"encoding/json"
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

type PreRegisterMAYAName struct {
	Name    string
	Address string
}

func getPreRegisterMAYANames(ctx cosmos.Context, blockheight int64, version semver.Version) ([]MAYAName, error) {
	var register []PreRegisterMAYAName
	if err := json.Unmarshal(preregisterMAYANames, &register); err != nil {
		return nil, fmt.Errorf("fail to load preregistation mayaname list,err: %w", err)
	}

	names := make([]MAYAName, 0)
	for _, reg := range register {
		addr, err := common.NewAddress(reg.Address, version)
		if err != nil {
			ctx.Logger().Error("fail to parse address", "address", reg.Address, "error", err)
			continue
		}
		name := NewMAYAName(reg.Name, blockheight, []MAYANameAlias{{Chain: common.BASEChain, Address: addr}}, common.EmptyAsset, nil, cosmos.ZeroUint(), nil)
		acc, err := cosmos.AccAddressFromBech32(reg.Address)
		if err != nil {
			ctx.Logger().Error("fail to parse acc address", "address", reg.Address, "error", err)
			continue
		}
		name.Owner = acc
		names = append(names, name)
	}
	return names, nil
}
