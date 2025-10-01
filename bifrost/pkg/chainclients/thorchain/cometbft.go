package thorchain

import (
	"context"

	cometbfttypes "github.com/cometbft/cometbft/rpc/core/types"
)

type CometBFTRPC interface {
	Block(ctx context.Context, height *int64) (*cometbfttypes.ResultBlock, error)
	BlockResults(ctx context.Context, height *int64) (*cometbfttypes.ResultBlockResults, error)
}
