package common

import (
	"fmt"
	"math/big"
	"time"

	"github.com/rs/zerolog"

	ecommon "github.com/ethereum/go-ethereum/common"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/test/simulation/pkg/evm"
	"gitlab.com/mayachain/mayanode/test/simulation/pkg/mayanode"

	. "gitlab.com/mayachain/mayanode/test/simulation/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////////////
// Deposit L1
////////////////////////////////////////////////////////////////////////////////////////

func DepositL1(log *zerolog.Logger, client LiteChainClient, asset common.Asset, memo string, amount cosmos.Uint) (string, error) {
	// get inbound address
	inboundAddr, _, err := mayanode.GetInboundAddress(asset.Chain)
	if err != nil {
		log.Error().Err(err).Msg("failed to get inbound address")
		return "", err
	}

	// create tx out
	tx := SimTx{
		Chain:     asset.Chain,
		ToAddress: inboundAddr,
		Coin:      common.NewCoin(asset, amount),
		Memo:      memo,
	}

	// sign transaction
	signed, err := client.SignTx(tx)
	if err != nil {
		log.Error().Err(err).Msg("failed to sign tx")
		return "", err
	}

	// broadcast transaction
	txid, err := client.BroadcastTx(signed)
	if err != nil {
		log.Error().Err(err).Msg("failed to broadcast tx")
	}

	return txid, err
}

////////////////////////////////////////////////////////////////////////////////////////
// Deposit L1 Token
////////////////////////////////////////////////////////////////////////////////////////

func DepositL1Token(log *zerolog.Logger, client LiteChainClient, asset common.Asset, memo string, amount cosmos.Uint) (string, error) {
	// get router address
	inboundAddr, routerAddr, err := mayanode.GetInboundAddress(asset.Chain)
	if err != nil {
		log.Error().Err(err).Msg("failed to get inbound address")
		return "", err
	}
	if routerAddr == nil {
		log.Error().Msg("failed to get router address")
		return "", fmt.Errorf("failed to get router address")
	}
	token := evm.Tokens(asset.Chain)[asset]

	// convert amount to token decimals
	factor := big.NewInt(1).Exp(big.NewInt(10), big.NewInt(int64(token.Decimals)), nil)
	tokenAmount := amount.Mul(cosmos.NewUintFromBigInt(factor))
	tokenAmount = tokenAmount.QuoUint64(common.One)

	// approve the router
	eRouterAddr := ecommon.HexToAddress(routerAddr.String())
	tx := SimContractTx{
		Chain:    asset.Chain,
		Contract: common.Address(token.Address),
		ABI:      evm.ERC20ABI(),
		Method:   "approve",
		Args:     []interface{}{eRouterAddr, tokenAmount.BigInt()},
	}

	eClient, ok := client.(*evm.Client)
	if !ok {
		log.Fatal().Msg("failed to get evm client")
	}

	// sign approve transaction
	signed, err := eClient.SignContractTx(tx)
	if err != nil {
		log.Error().Err(err).Msg("failed to sign tx")
		return "", err
	}

	// broadcast approve transaction
	txid, err := client.BroadcastTx(signed)
	if err != nil {
		log.Error().Err(err).Msg("failed to broadcast tx")
		return "", err
	}
	log.Info().Str("txid", txid).Msg("broadcasted router approve tx")

	// call depositWithExpiry
	expiry := time.Now().Add(time.Hour).Unix()
	eInboundAddr := ecommon.HexToAddress(inboundAddr.String())
	eTokenAddr := ecommon.HexToAddress(token.Address)
	tx = SimContractTx{
		Chain:    asset.Chain,
		Contract: *routerAddr,
		ABI:      evm.RouterABI(),
		Method:   "depositWithExpiry",
		Args: []interface{}{
			eInboundAddr,
			eTokenAddr,
			tokenAmount.BigInt(),
			memo,
			big.NewInt(expiry),
		},
	}

	// sign deposit transaction
	signed, err = eClient.SignContractTx(tx)
	if err != nil {
		log.Error().Err(err).Msg("failed to sign tx")
		return "", err
	}

	// broadcast deposit transaction
	txid, err = client.BroadcastTx(signed)
	if err != nil {
		log.Error().Err(err).Msg("failed to broadcast tx")
	}

	return txid, err
}
