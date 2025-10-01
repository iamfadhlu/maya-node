package mayaclient

import (
	"encoding/json"
	"fmt"
	"time"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/constants"
	openapi "gitlab.com/mayachain/mayanode/openapi/gen"
)

// GetLastObservedInHeight returns the lastobservedin value for the chain past in
func (b *mayachainBridge) GetLastObservedInHeight(chain common.Chain) (int64, error) {
	lastblock, err := b.getLastBlock(chain)
	if err != nil {
		return 0, fmt.Errorf("failed to GetLastObservedInHeight: %w", err)
	}
	for _, item := range lastblock {
		if item.Chain == chain.String() {
			return item.LastObservedIn, nil
		}
	}
	return 0, fmt.Errorf("fail to GetLastObservedInHeight,chain(%s)", chain)
}

// GetLastSignedOutHeight returns the lastsignedout value for mayachain
func (b *mayachainBridge) GetLastSignedOutHeight(chain common.Chain) (int64, error) {
	lastblock, err := b.getLastBlock(chain)
	if err != nil {
		return 0, fmt.Errorf("failed to GetLastSignedOutHeight: %w", err)
	}
	for _, item := range lastblock {
		if item.Chain == chain.String() {
			return item.LastSignedOut, nil
		}
	}
	return 0, fmt.Errorf("fail to GetLastSignedOutHeight,chain(%s)", chain)
}

// GetBlockHeight returns the current height for mayachain blocks
func (b *mayachainBridge) GetBlockHeight() (int64, error) {
	if time.Since(b.lastBlockHeightCheck) < constants.MayachainBlockTime && b.lastMayachainBlockHeight > 0 {
		return b.lastMayachainBlockHeight, nil
	}
	latestBlocks, err := b.getLastBlock(common.EmptyChain)
	if err != nil {
		return 0, fmt.Errorf("failed to GetMayachainHeight: %w", err)
	}
	b.lastBlockHeightCheck = time.Now()
	for _, item := range latestBlocks {
		b.lastMayachainBlockHeight = item.Mayachain
		return item.Mayachain, nil
	}
	return 0, fmt.Errorf("failed to GetMayachainHeight")
}

// getLastBlock calls the /lastblock/{chain} endpoint and Unmarshal's into the QueryResLastBlockHeights type
func (b *mayachainBridge) getLastBlock(chain common.Chain) ([]openapi.LastBlock, error) {
	path := LastBlockEndpoint
	if !chain.IsEmpty() {
		path = fmt.Sprintf("%s/%s", path, chain.String())
	}
	buf, _, err := b.getWithPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get lastblock: %w", err)
	}
	var lastBlock []openapi.LastBlock
	if err := json.Unmarshal(buf, &lastBlock); err != nil {
		return nil, fmt.Errorf("failed to unmarshal last block: %w", err)
	}
	return lastBlock, nil
}
