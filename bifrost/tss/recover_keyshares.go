package tss

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	ctypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/itchio/lzma"
	"github.com/rs/zerolog/log"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"

	"gitlab.com/mayachain/mayanode/app"
	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/config"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

func RecoverKeyShares(conf config.Bifrost, mayachain mayaclient.MayachainBridge) error {
	tctx := mayachain.GetContext()

	// fetch the node account
	na, err := mayachain.GetNodeAccount(tctx.FromAddress.String())
	if err != nil {
		return fmt.Errorf("fail to get node account: %w", err)
	}

	// skip recovery if the current node is not active
	if na.Status != types.NodeStatus_Active {
		log.Info().Msgf("%s is not active, skipping key shares recovery", na.NodeAddress)
		return nil
	}

	// the current vault is the last pub key in the signer membership list
	membership := na.GetSignerMembership()
	if len(membership) == 0 {
		log.Info().Msgf("no signer membership for %s, skipping key shares recovery", na.NodeAddress)
		return fmt.Errorf("fail to get signer membership")
	}
	vault := membership[len(membership)-1]
	keysharesPath := filepath.Join(app.DefaultNodeHome(), fmt.Sprintf("localstate-%s.json", vault))

	// skip recovery if keyshares for the nodes current vault already exist
	if _, err = os.Stat(keysharesPath); !os.IsNotExist(err) {
		log.Info().Msgf("ecdsa key shares for %s already exist, skipping recovery", vault)
		return nil
	}

	// get all vaults
	vaults, err := mayachain.GetAsgards()
	if err != nil {
		return fmt.Errorf("fail to get asgards: %w", err)
	}

	// get the creation height of the member vault
	var lastVaultHeight int64
	for _, v := range vaults {
		if v.PubKey.Equals(vault) {
			lastVaultHeight = v.BlockHeight
			break
		}
	}
	if lastVaultHeight == 0 {
		return fmt.Errorf("fail to get creation height of %s", vault)
	}

	// walk backward from the churn height until we find the TssPool message we sent
	var keysharesEncBytes []byte
	var keysharesEncBytesEddsa []byte
	var vaultEddsa common.PubKey
	dec := tx.DefaultTxDecoder(mayaclient.MakeCodec())
	for i := lastVaultHeight; i > lastVaultHeight-conf.TSS.MaxKeyshareRecoverScanBlocks; i-- {
		if i%1000 == 0 {
			log.Info().Msgf("scanning block %d for TssPool message to recover key shares", i)
		}

		var b *coretypes.ResultBlock
		b, err = mayachain.GetContext().Client.Block(context.Background(), &i)
		if err != nil {
			return fmt.Errorf("fail to get block: %w", err)
		}

		for _, txb := range b.Block.Txs {
			var tx ctypes.Tx
			tx, err = dec(txb)
			if err != nil {
				return fmt.Errorf("fail to decode tx: %w", err)
			}
			for _, msg := range tx.GetMsgs() {
				switch m := msg.(type) {
				case *types.MsgTssPool:
					if m.Signer.Equals(na.NodeAddress) {
						if m.KeysharesBackup == nil {
							log.Warn().Msgf("key shares backup not saved for %s", na.NodeAddress)
						}
						keysharesEncBytes = m.KeysharesBackup
						keysharesEncBytesEddsa = m.KeysharesBackupEddsa
						vaultEddsa = m.PoolPubKeyEddsa
						goto finish
					}
				default:
				}
			}
		}
	}

finish:
	// open ecdsa key shares file
	f, err := os.OpenFile(keysharesPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open keyshares file: %w", err)
	}
	defer f.Close()

	seedPhrase := os.Getenv("SIGNER_SEED_PHRASE")

	// decrypt and decompress into place
	var decrypted []byte
	decrypted, err = DecryptKeyshares(keysharesEncBytes, seedPhrase)
	if err != nil {
		return fmt.Errorf("failed to decrypt key shares: %w", err)
	}
	cmpDec := lzma.NewReader(bytes.NewReader(decrypted))
	if _, err = io.Copy(f, cmpDec); err != nil {
		return fmt.Errorf("failed to decompress key shares: %w", err)
	}

	// success
	log.Info().Str("path", keysharesPath).Msgf("recovered ecdsa key shares for %s", na.NodeAddress)

	// Now recover eddsa

	keysharesPath = filepath.Join(app.DefaultNodeHome(), fmt.Sprintf("localstate-%s.json", vaultEddsa))

	lastVaultHeight = 0
	for _, v := range vaults {
		if v.PubKeyEddsa.Equals(vaultEddsa) {
			lastVaultHeight = v.BlockHeight
			break
		}
	}
	if lastVaultHeight == 0 {
		return fmt.Errorf("fail to get creation height of %s", vaultEddsa)
	}

	// skip recovery if keyshares for the nodes current vault already exist
	if _, err = os.Stat(keysharesPath); !os.IsNotExist(err) {
		log.Info().Msgf("eddsa key shares for %s already exist, skipping recovery", vaultEddsa)
		return nil
	}

	// open eddsa key shares file
	f, err = os.OpenFile(keysharesPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open keyshares file: %w", err)
	}
	defer f.Close()

	// decrypt and decompress into place
	decrypted, err = DecryptKeyshares(keysharesEncBytesEddsa, seedPhrase)
	if err != nil {
		return fmt.Errorf("failed to decrypt key shares: %w", err)
	}
	cmpDec = lzma.NewReader(bytes.NewReader(decrypted))
	if _, err = io.Copy(f, cmpDec); err != nil {
		return fmt.Errorf("failed to decompress key shares: %w", err)
	}

	// success
	log.Info().Str("path", keysharesPath).Msgf("recovered key shares for %s", na.NodeAddress)

	return nil
}
