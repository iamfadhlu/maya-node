package cmd

import (
	"bufio"
	"fmt"

	"github.com/cosmos/cosmos-sdk/client/input"
	bech32 "github.com/cosmos/cosmos-sdk/types/bech32/legacybech32" // nolint
	"github.com/spf13/cobra"

	"gitlab.com/mayachain/mayanode/app"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/common/crypto/ed25519"
)

const (
	DefaultEd25519KeyName           = `ed-mayachain`
	ThorchainDefaultBIP39PassPhrase = "thorchain"
	BIP44Prefix                     = "44'/931'/"
	PartialPath                     = "0'/0/0"
	FullPath                        = BIP44Prefix + PartialPath
)

func GetEd25519Keys() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ed25519",
		Short: "Generate an ed25519 key",
		Long:  ``,
		Args:  cobra.ExactArgs(0),
		RunE:  ed25519Keys,
	}
	return cmd
}

func ed25519Keys(cmd *cobra.Command, args []string) error {
	kb, err := cosmos.GetKeybase(app.DefaultNodeHome())
	if err != nil {
		return fmt.Errorf("fail to get keybase: %w", err)
	}

	edKey := ed25519.SignerNameEDDSA(kb.SignerName)
	r, err := kb.Keybase.Key(edKey)
	if err != nil {
		buf := bufio.NewReader(cmd.InOrStdin())
		var mnemonic string
		mnemonic, err = input.GetString("Enter mnemonic", buf)
		if err != nil {
			return fmt.Errorf("fail to get mnemonic: %w", err)
		}

		r, err = kb.Keybase.NewAccount(edKey, mnemonic, kb.SignerPasswd, ed25519.HDPath, ed25519.Ed25519)
		if err != nil {
			return fmt.Errorf("fail to create new key: %w", err)
		}
	}

	pubKey := r.GetPubKey()
	if pubKey == nil {
		return fmt.Errorf("public key not found in key info")
	}

	// trunk-ignore(golangci-lint/staticcheck): deprecated
	pubBech32, err := bech32.MarshalPubKey(bech32.AccPK, pubKey)
	if err != nil {
		return fmt.Errorf("fail to generate bech32 pubkey: %w", err)
	}
	fmt.Println(pubBech32)
	return nil
}
