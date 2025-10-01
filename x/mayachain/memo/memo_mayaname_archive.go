package mayachain

import (
	"fmt"
	"strconv"

	"github.com/blang/semver"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

func ParseManageMAYANameMemoV1(version semver.Version, parts []string) (ManageMAYANameMemo, error) {
	var err error
	var name string
	var owner cosmos.AccAddress
	preferredAsset := common.EmptyAsset
	expire := int64(0)

	if len(parts) < 4 {
		return ManageMAYANameMemo{}, fmt.Errorf("not enough parameters")
	}

	name = parts[1]
	chain, err := common.NewChain(parts[2])
	if err != nil {
		return ManageMAYANameMemo{}, err
	}

	addr, err := common.NewAddress(parts[3], version)
	if err != nil {
		return ManageMAYANameMemo{}, err
	}

	if len(parts) >= 5 {
		owner, err = cosmos.AccAddressFromBech32(parts[4])
		if err != nil {
			return ManageMAYANameMemo{}, err
		}
	}

	if len(parts) >= 6 {
		preferredAsset, err = common.NewAssetWithShortCodes(version, parts[5])
		if err != nil {
			return ManageMAYANameMemo{}, err
		}
	}

	if len(parts) >= 7 {
		expire, err = strconv.ParseInt(parts[6], 10, 64)
		if err != nil {
			return ManageMAYANameMemo{}, err
		}
	}

	return NewManageMAYANameMemo(name, chain, addr, expire, preferredAsset, owner, cosmos.ZeroUint(), []cosmos.Uint{}, []string{}), nil
}
