package mayaclient

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/crypto"
	ckeys "github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"

	cryptokeysed25519 "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptokeyssecp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"gitlab.com/mayachain/mayanode/common/crypto/ed25519"
)

const (
	// folder name for mayachain thorcli
	mayachainCliFolderName = `.mayanode`
)

type Keyring interface {
	ckeys.Keyring
}

// Keys manages all the keys used by mayachain
type Keys struct {
	signerName string
	password   string // TODO this is a bad way , need to fix it
	kb         Keyring
}

// NewKeysWithKeybase create a new instance of Keys
func NewKeysWithKeybase(kb ckeys.Keyring, name, password string) *Keys {
	return &Keys{
		signerName: name,
		password:   password,
		kb:         kb,
	}
}

// GetKeyringKeybase return keyring and key info
func GetKeyringKeybase(chainHomeFolder, signerName, password string) (ckeys.Keyring, ckeys.Info, error) {
	if len(signerName) == 0 {
		return nil, nil, fmt.Errorf("signer name is empty")
	}
	if len(password) == 0 {
		return nil, nil, fmt.Errorf("password is empty")
	}

	buf := bytes.NewBufferString(password)
	// the library used by keyring is using ReadLine , which expect a new line
	buf.WriteByte('\n')
	kb, err := getKeybase(chainHomeFolder, buf)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to get keybase,err:%w", err)
	}
	// the keyring library which used by cosmos sdk , will use interactive terminal if it detect it has one
	// this will temporary trick it think there is no interactive terminal, thus will read the password from the buffer provided
	oldStdIn := os.Stdin
	defer func() {
		os.Stdin = oldStdIn
	}()
	os.Stdin = nil
	si, err := kb.Key(signerName)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to get signer info(%s): %w", signerName, err)
	}
	return kb, si, nil
}

// getKeybase will create an instance of Keybase
func getKeybase(mayachainHome string, reader io.Reader) (ckeys.Keyring, error) {
	cliDir := mayachainHome
	if len(mayachainHome) == 0 {
		usr, err := user.Current()
		if err != nil {
			return nil, fmt.Errorf("fail to get current user,err:%w", err)
		}
		cliDir = filepath.Join(usr.HomeDir, mayachainCliFolderName)
	}

	return ckeys.New(sdk.KeyringServiceName(), ckeys.BackendFile, cliDir, reader)
}

// GetSignerInfo return signer info
func (k *Keys) GetSignerInfo() ckeys.Info {
	info, err := k.kb.Key(k.signerName)
	if err != nil {
		panic(err)
	}
	return info
}

// GetPrivateKey return the ecdsa private key
func (k *Keys) GetPrivateKey() (*cryptokeyssecp256k1.PrivKey, error) {
	// return k.kb.ExportPrivateKeyObject(k.signerName)
	privKeyArmor, err := k.kb.ExportPrivKeyArmor(k.signerName, k.password)
	if err != nil {
		return nil, err
	}
	priKey, _, err := crypto.UnarmorDecryptPrivKey(privKeyArmor, k.password)
	if err != nil {
		return nil, fmt.Errorf("fail to unarmor private key: %w", err)
	}
	secpKey, ok := priKey.(*cryptokeyssecp256k1.PrivKey)
	if !ok {
		return nil, fmt.Errorf("fail to cast private key to secp256k1 private key")
	}
	return secpKey, nil
}

// GetPrivateKeyEDDSA return the eddsa private key
// trunk-ignore(golangci-lint/staticcheck): deprecated
func (k *Keys) GetPrivateKeyEDDSA() (*cryptokeysed25519.PrivKey, error) {
	signerNameEDDSA := ed25519.SignerNameEDDSA(k.signerName)
	// return k.kb.ExportPrivateKeyObject(k.signerName)
	privKeyArmor, err := k.kb.ExportPrivKeyArmor(signerNameEDDSA, k.password)
	if err != nil {
		return nil, err
	}
	priKey, _, err := crypto.UnarmorDecryptPrivKey(privKeyArmor, k.password)
	if err != nil {
		return nil, fmt.Errorf("fail to unarmor private key: %w", err)
	}
	// trunk-ignore(golangci-lint/staticcheck): deprecated
	eddsaKey, ok := priKey.(*cryptokeysed25519.PrivKey)
	if !ok {
		return nil, fmt.Errorf("fail to cast private key to eddsa private key")
	}
	return eddsaKey, nil
}

// GetKeybase return the keybase
func (k *Keys) GetKeybase() ckeys.Keyring {
	return k.kb
}
