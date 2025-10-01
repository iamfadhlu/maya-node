package mayaclient

import (
	"fmt"
	"sync/atomic"
	"time"

	clienttx "github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	flag "github.com/spf13/pflag"

	stypes "github.com/cosmos/cosmos-sdk/types"

	"gitlab.com/mayachain/mayanode/bifrost/metrics"
	"gitlab.com/mayachain/mayanode/common"
)

// Broadcast Broadcasts tx to thorchain
func (b *mayachainBridge) Broadcast(msgs ...stypes.Msg) (common.TxID, error) {
	b.broadcastLock.Lock()
	defer b.broadcastLock.Unlock()

	noTxID := common.TxID("")

	start := time.Now()
	defer func() {
		b.m.GetHistograms(metrics.SendToMayachainDuration).Observe(time.Since(start).Seconds())
	}()

	blockHeight, err := b.GetBlockHeight()
	if err != nil {
		return noTxID, err
	}
	if blockHeight > b.blockHeight {
		var seqNum uint64
		b.accountNumber, seqNum, err = b.getAccountNumberAndSequenceNumber()
		if err != nil {
			return noTxID, fmt.Errorf("fail to get account number and sequence number from mayachain : %w", err)
		}
		b.blockHeight = blockHeight
		if seqNum > b.seqNumber {
			b.seqNumber = seqNum
		}
	}

	b.logger.Info().Uint64("account_number", b.accountNumber).Uint64("sequence_number", b.seqNumber).Msg("account info")

	flags := flag.NewFlagSet("mayachain", 0)

	ctx := b.GetContext()
	factory := clienttx.NewFactoryCLI(ctx, flags)
	factory = factory.WithAccountNumber(b.accountNumber)
	factory = factory.WithSequence(b.seqNumber)
	factory = factory.WithSignMode(signing.SignMode_SIGN_MODE_DIRECT)

	builder, err := clienttx.BuildUnsignedTx(factory, msgs...)
	if err != nil {
		return noTxID, err
	}
	builder.SetGasLimit(4000000000)
	err = clienttx.Sign(factory, ctx.GetFromName(), builder, true)
	if err != nil {
		return noTxID, err
	}

	txBytes, err := ctx.TxConfig.TxEncoder()(builder.GetTx())
	if err != nil {
		return noTxID, err
	}

	// broadcast to a Tendermint node
	commit, err := ctx.BroadcastTx(txBytes)
	if err != nil {
		return noTxID, fmt.Errorf("fail to broadcast tx: %w", err)
	}

	b.m.GetCounter(metrics.TxToMayachainSigned).Inc()
	// b.logger.Debug().Str("body", string(body)).Msg("broadcast response from BASEChain")
	txHash, err := common.NewTxID(commit.TxHash)
	if err != nil {
		return common.BlankTxID, fmt.Errorf("fail to convert txhash: %w", err)
	}
	// Code will be the tendermint ABICode , it start at 1 , so if it is an error , code will not be zero
	if commit.Code > 0 {
		if commit.Code == 32 {
			// bad sequence number, fetch new one
			_, seqNum, _ := b.getAccountNumberAndSequenceNumber()
			if seqNum > 0 {
				b.seqNumber = seqNum
			}
		}
		b.logger.Info().Int("bytes", len(txBytes)).Uint32("code", commit.Code).Interface("messages", msgs).Msg("failed tx")
		// commit code 6 means `unknown request` , which means the tx can't be accepted by mayachain
		// if that's the case, let's just ignore it and move on
		if commit.Code != 6 {
			return txHash, fmt.Errorf("fail to broadcast to BASEChain,code:%d, log:%s", commit.Code, commit.RawLog)
		}
	}
	b.m.GetCounter(metrics.TxToMayachain).Inc()
	b.logger.Info().Msgf("Received a TxHash of %v from the mayachain", commit.TxHash)

	// increment seqNum
	atomic.AddUint64(&b.seqNumber, 1)

	return txHash, nil
}
