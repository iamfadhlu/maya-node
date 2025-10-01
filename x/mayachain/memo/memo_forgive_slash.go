package mayachain

import (
	"fmt"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

type ForgiveSlashMemo struct {
	MemoBase
	Blocks         cosmos.Uint
	ForgiveAsset   common.Asset
	ForgiveAddress cosmos.AccAddress
}

func NewForgiveSlashMemo(blocks cosmos.Uint, addr cosmos.AccAddress) ForgiveSlashMemo {
	return ForgiveSlashMemo{
		MemoBase:       MemoBase{TxType: TxForgiveSlash},
		Blocks:         blocks,
		ForgiveAddress: addr,
	}
}

func ParseForgiveSlashMemo(parts []string) (ForgiveSlashMemo, error) {
	var err error
	if len(parts) < 3 {
		return ForgiveSlashMemo{}, fmt.Errorf("not enough parameters")
	}
	blocks, err := cosmos.ParseUint(parts[1])
	if err != nil {
		return ForgiveSlashMemo{}, fmt.Errorf("forgive amount: %s is invalid", parts[1])
	}

	// DESTADDR can be empty, if it is empty it will forgive all nodes.
	destination := cosmos.AccAddress{}
	if len(parts) > 2 {
		destination, err = cosmos.AccAddressFromBech32(parts[2])
		if err != nil {
			return ForgiveSlashMemo{}, err
		}
	}
	return NewForgiveSlashMemo(blocks, destination), nil
}
