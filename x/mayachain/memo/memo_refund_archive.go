package mayachain

import (
	"fmt"

	"gitlab.com/mayachain/mayanode/common"
)

func ParseRefundMemoV1(parts []string) (RefundMemo, error) {
	if len(parts) < 2 {
		return RefundMemo{}, fmt.Errorf("not enough parameters")
	}
	txID, err := common.NewTxID(parts[1])
	return NewRefundMemo(txID), err
}
