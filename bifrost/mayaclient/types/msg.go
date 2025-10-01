package types

import (
	stypes "gitlab.com/mayachain/mayanode/x/mayachain/types"
)

type Msg struct {
	Type  string                 `json:"type"`
	Value stypes.MsgObservedTxIn `json:"value"`
}
