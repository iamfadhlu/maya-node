package app

import (
	tmlog "github.com/tendermint/tendermint/libs/log"
	"gitlab.com/mayachain/mayanode/log"
)

var _ tmlog.Logger = (*log.TendermintLogWrapper)(nil)
