package types

import "gitlab.com/mayachain/mayanode/common"

// FnLastScannedBlockHeight function signature for passing around the function call to get last_scanned_block_height from thorchain
type FnLastScannedBlockHeight func(chain common.Chain) (int64, error)
