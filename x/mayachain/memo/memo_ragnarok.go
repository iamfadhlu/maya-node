package mayachain

import (
	"fmt"

	"github.com/blang/semver"
)

type RagnarokMemo struct {
	MemoBase
	BlockHeight int64
}

func (m RagnarokMemo) String() string {
	return fmt.Sprintf("RAGNAROK:%d", m.BlockHeight)
}

func (m RagnarokMemo) GetBlockHeight() int64 {
	return m.BlockHeight
}

func NewRagnarokMemo(blockHeight int64) RagnarokMemo {
	return RagnarokMemo{
		MemoBase:    MemoBase{TxType: TxRagnarok},
		BlockHeight: blockHeight,
	}
}

func (p *parser) ParseRagnarokMemo() (RagnarokMemo, error) {
	switch {
	case p.version.GTE(semver.MustParse("1.112.0")):
		return p.ParseRagnarokMemoV112()
	default:
		return ParseRagnarokMemoV1(p.parts)
	}
}

func (p *parser) ParseRagnarokMemoV112() (RagnarokMemo, error) {
	blockHeight := p.getInt64(1, true, 0)
	err := p.Error()
	return NewRagnarokMemo(blockHeight), err
}
