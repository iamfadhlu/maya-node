package mayachain

import (
	"fmt"

	"github.com/blang/semver"
)

type NoOpMemo struct {
	MemoBase
	Action string
}

// String implement fmt.Stringer
func (m NoOpMemo) String() string {
	if len(m.Action) == 0 {
		return "noop"
	}
	return fmt.Sprintf("noop:%s", m.Action)
}

// NewNoOpMemo create a new instance of NoOpMemo
func NewNoOpMemo(action string) NoOpMemo {
	return NoOpMemo{
		MemoBase: MemoBase{TxType: TxNoOp},
		Action:   action,
	}
}

func (p *parser) ParseNoOpMemo() (NoOpMemo, error) {
	switch {
	case p.version.GTE(semver.MustParse("1.112.0")):
		return p.ParseNoOpMemoV112()
	default:
		return ParseNoOpMemoV1(p.parts)
	}
}

func (p *parser) ParseNoOpMemoV112() (NoOpMemo, error) {
	return NewNoOpMemo(p.get(1)), p.Error()
}
