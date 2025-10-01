package zcash

import (
	//	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	// trunk-ignore(golangci-lint/gosec)
	"golang.org/x/crypto/ripemd160"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcutil"

	"github.com/btcsuite/btcutil/base58"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/common/cosmos"

	// zecutil "gitlab.com/mayachain/chains/zcash/go/maya_zcash"

	stypes "gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/shared/utxo"
	"gitlab.com/mayachain/mayanode/bifrost/tss"
	mem "gitlab.com/mayachain/mayanode/x/mayachain/memo"

	"github.com/cosmos/cosmos-sdk/crypto/codec"

	"gitlab.com/mayachain/mayanode/chain/zec/go/zec"
	"gitlab.com/mayachain/mayanode/common"
	types2 "gitlab.com/mayachain/mayanode/x/mayachain/types"
)

const (
	regtestVaultPrivKeyHex = "8a74dce839bc2228428ed5de3c2edbabb5c9713f5e6eeb808f9c56640921c6c9" // value from maya_zcash regtest (func TestApplySignatures) - vault_sk
	regtestVaultPubKeyHex  = "03c622fa3be76cd25180d5a61387362181caca77242023be11775134fd37f403f7"
)

func createP2PKHScript(pubKey []byte) (string, error) {
	// Hash the public key using SHA-256
	sha256Hash := sha256.Sum256(pubKey)

	// hash the result using btcutil.Hash160 (RIPEMD-160)
	// This doesn't work: pubKeyHash := btcutil.Hash160(sha256Hash[:])
	// trunk-ignore(golangci-lint/gosec)
	ripemd160Hash := ripemd160.New()
	ripemd160Hash.Write(sha256Hash[:])
	pubKeyHash := ripemd160Hash.Sum(nil)

	// Construct the P2PKH script
	script := fmt.Sprintf("76a914%x88ac", pubKeyHash) // OP_DUP OP_HASH160 <pubKeyHash> OP_EQUALVERIFY OP_CHECKSIG
	return script, nil
}

func encodePrivateKeyToWIF(privateKey []byte, isTestnet bool, isCompressed bool) string {
	// version byte
	var version byte
	if isTestnet {
		version = 0xEF // Testnet version byte
	} else {
		version = 0x80 // Mainnet version byte
	}
	data := append([]byte{version}, privateKey...)

	// compression flag (if needed)
	if isCompressed {
		data = append(data, 0x01) // Compression flag
	}

	// calculate checksum (double SHA-256)
	firstSHA := sha256.Sum256(data)
	secondSHA := sha256.Sum256(firstSHA[:])
	checksum := secondSHA[:4] // first 4 bytes of the double SHA-256 hash

	// append checksum to data
	data = append(data, checksum...)

	// encode in Base58
	wif := base58.Encode(data)
	return wif
}

func getPrivKeyHexFromSecret(secret []byte) string {
	privKey := secp256k1.GenPrivKeyFromSecret(secret)
	privKeyHex := hex.EncodeToString(privKey.Bytes())
	return privKeyHex
}

func pubKeyFromBytes(pubKeyBytes []byte) (common.PubKey, error) {
	cosmosPubKey := &secp256k1.PubKey{Key: pubKeyBytes}
	pubKeyTm, err := codec.ToTmPubKeyInterface(cosmosPubKey)
	if err != nil {
		return "", err
	}
	pubKey, err := common.NewPubKeyFromCrypto(pubKeyTm)
	if err != nil {
		return "", err
	}
	return pubKey, nil
}

func pubKeyFromBtcecPubKey(btcecPubKey *btcec.PublicKey) (common.PubKey, error) {
	return pubKeyFromBytes(btcecPubKey.SerializeCompressed())
}

func (s *ZcashSuite) signTestCommon(c *C, expectedVaultPrivKey, expectedVaultPubKey, expectedScript string) ([]byte, error) {
	nodePubKeyBytes, err := s.client.nodePubKey.Bytes()
	c.Assert(err, IsNil)
	privKeyBytes := s.client.ksWrapper.privateKey.Serialize()

	var from common.Address
	from, err = s.client.nodePubKey.GetAddress(common.ZECChain)
	c.Assert(err, IsNil)

	// generate the script for the UTXO input
	var script string
	script, err = createP2PKHScript(nodePubKeyBytes)
	c.Assert(err, IsNil)

	// ### these tests are just to verify we calculate the private and public keys correctly
	// 1. verify privKey is ok and same as in regtest
	if expectedVaultPrivKey != "" {
		c.Assert(privKeyBytes, NotNil)
		pkBytesHex := hex.EncodeToString(privKeyBytes)
		c.Assert(pkBytesHex, Equals, expectedVaultPrivKey) // value from maya_zcash regtest (func TestApplySignatures)
	}
	// 2. verify pubKey is ok and same as in regtest
	if expectedVaultPubKey != "" {
		pubkBytesHex := hex.EncodeToString(nodePubKeyBytes)
		c.Assert(pubkBytesHex, Equals, expectedVaultPubKey)
	}
	// 3. verify script is ok and same as in regtest
	if expectedScript != "" {
		c.Assert(script, Equals, expectedScript)
	}
	// ### end of private and public keys verification tests

	// to := "tm9j9tS8nTnNQqoJuw8ToinJCapd3WdzGVu"
	// height := uint32(200)
	to := types2.GetRandomZECAddress().String()
	var height int64
	height, err = s.client.getBlockHeight()
	c.Assert(err, IsNil)

	memo := "MEMO OUT"
	inAmount := uint64(540000000)
	amount := uint64(500000)
	fee := getUsualGas()
	change := inAmount - amount - fee
	ptx := zec.PartialTx{
		Height: uint32(height),
		Inputs: []zec.Utxo{{
			Txid:   "3005b0052fcd17a91da4ab975fed5fa1dd518477b37dd95a2dc015ef04538577",
			Height: uint32(height - 39),
			Vout:   0,
			Script: script, // "76a9144fb7f7b9ea3859086b151cde4d3c75152e51547288ac",
			Value:  inAmount,
		}},
		Outputs: []zec.Output{
			{
				Address: to,
				Amount:  amount,
				Memo:    memo,
			},
			{
				Address: from.String(),
				Amount:  change,
				Memo:    "",
			},
		},
		Fee: fee,
		// Sighashes: [][]byte{hash},
	}

	// Calculate the REAL sighashes in Rust
	ptx, err = zec.BuildPtx(nodePubKeyBytes, ptx, s.client.getChainCfg().Net)
	c.Assert(err, IsNil)

	// Sign the sighashes returned from Rust
	var txb []byte
	signatures := make([][]byte, 0)
	for _, sigHash := range ptx.Sighashes {
		var signature []byte
		signature, err = signSighashWithPrivKey(s.client.ksWrapper.privateKey, sigHash)
		c.Assert(err, IsNil)
		signatures = append(signatures, signature)
	}
	txb, err = zec.ApplySignatures(nodePubKeyBytes, ptx, signatures, s.client.getChainCfg().Net)
	return txb, err
}

func signSighashWithPrivKey(privKeyBytes *btcec.PrivateKey, hash []byte) ([]byte, error) {
	pks := NewPrivateKeySignable(privKeyBytes)
	sig, err := pks.Sign(hash)
	if err != nil {
		return nil, err
	}
	// return serializeRawSignature(sig), nil
	return sig.Serialize(), nil
}

func (s *ZcashSuite) TestParseMemo(c *C) {
	_, err := mem.ParseMemo(common.LatestVersion, "=:ZEC.ZEC:tmCdeGhUu3Xcvq3GhxQvLEoCZc852wdPuHp")
	c.Assert(err, NotNil)
}

func (s *ZcashSuite) TestGetZecPrivateKey(c *C) {
	input := "YjQwNGM1ZWM1ODExNmI1ZjBmZTEzNDY0YTkyZTQ2NjI2ZmM1ZGIxMzBlNDE4Y2JjZTk4ZGY4NmZmZTkzMTdjNQ=="
	buf, err := base64.StdEncoding.DecodeString(input)
	c.Assert(err, IsNil)
	c.Assert(buf, NotNil)
	prikeyByte, err := hex.DecodeString(string(buf))
	c.Assert(err, IsNil)
	pk := secp256k1.GenPrivKeyFromSecret(prikeyByte)
	zecPrivateKey, err := getZECPrivateKey(pk)
	c.Assert(err, IsNil)
	c.Assert(zecPrivateKey, NotNil)
}

func (s *ZcashSuite) TestApplySignaturesFromMayaZcashRegtest(c *C) {
	if common.CurrentChainNetwork.SoftEquals(common.MainNet) {
		return // this test is not supported on mainnet as the wallet addresses are tesnet addresses
	}
	vaultPrivKeyBytes, _ := hex.DecodeString(regtestVaultPrivKeyHex)
	vaultPubKeyBytes, _ := hex.DecodeString(regtestVaultPubKeyHex)
	// transpa: tm9j9tS8nTnNQqoJuw8ToinJCapd3WdzGVu
	// sapling: zregtestsapling18ywlqhk60zglax5drk3kwltkmcatf5eptxyrkrx20hcqma5nsvrgh63843seye923qk5wfvxpnr
	// orchard: uregtest1w7mhyq5xd5h8zrlqfdnf8kqrd0g8n8q9hg8502e63sr5xuenhyvama2jytdul0k2krj2kq86x86ch8x9eejxh4se8en4jpwdkse7l0gl
	var ptx zec.PartialTx
	jsonData := `{
			"Height": 200,
			"Inputs": [
				{
				"Txid": "3005b0052fcd17a91da4ab975fed5fa1dd518477b37dd95a2dc015ef04538577",
				"Height": 161,
				"Vout": 0,
				"Script": "76a9144fb7f7b9ea3859086b151cde4d3c75152e51547288ac",
				"Value": 540000000
				}
			],
			"Outputs": [
				{
				"Address": "tm9j9tS8nTnNQqoJuw8ToinJCapd3WdzGVu",
				"Amount": 500000,
				"Memo": "MEMO OUT"
				},
				{
				"Address": "tmGys6dBuEGjch5LFnhdo5gpSa7jiNRWse6",
				"Amount": 539485000,
				"Memo": ""
				}
			],
			"Fee": 15000,
			"Sighashes": [ "" ]
			}`
	// unmarshal the JSON from maya_zcash regtest
	err := json.Unmarshal([]byte(jsonData), &ptx)
	c.Assert(err, IsNil)

	// Calculate the REAL sighashes in Rust
	ptx, err = zec.BuildPtx(vaultPubKeyBytes, ptx, s.client.getChainCfg().Net)
	c.Assert(err, IsNil)

	sighashes := ptx.Sighashes
	signatures := make([][]byte, 0)
	for _, sighash := range sighashes {
		var signature []byte
		pkey, _ := btcec.PrivKeyFromBytes(btcec.S256(), vaultPrivKeyBytes)
		signature, err = signSighashWithPrivKey(pkey, sighash)
		c.Assert(err, IsNil)
		signatures = append(signatures, signature)
	}
	txb, err := zec.ApplySignatures(vaultPubKeyBytes, ptx, signatures, s.client.getChainCfg().Net)
	c.Assert(err, IsNil)
	c.Assert(txb, NotNil)
}

func (s *ZcashSuite) TestSignFromMayaZcash(c *C) {
	// this pubkey is for check only:
	regtestScript := "76a9144fb7f7b9ea3859086b151cde4d3c75152e51547288ac"
	privKeyBytes, err := hex.DecodeString(regtestVaultPrivKeyHex)
	c.Assert(err, IsNil)
	s.updateClientKeys(c, privKeyBytes)
	txb, err := s.signTestCommon(c, regtestVaultPrivKeyHex, regtestVaultPubKeyHex, regtestScript)
	c.Assert(err, IsNil)
	c.Assert(txb, NotNil)
}

func (s *ZcashSuite) TestSignRandomPK(c *C) {
	s.setRandomClientKeys(c)
	privKeyBytes := s.client.ksWrapper.privateKey.Serialize()
	privKeyHex := hex.EncodeToString(privKeyBytes)
	txb, err := s.signTestCommon(c, privKeyHex, "", "")
	c.Assert(err, IsNil)
	c.Assert(txb, NotNil)
}

func (s *ZcashSuite) TestSignClientPK(c *C) {
	txb, err := s.signTestCommon(c, "", "", "")
	c.Assert(err, IsNil)
	c.Assert(txb, NotNil)
}

func (s *ZcashSuite) TestSignTx(c *C) {
	txOutItem := stypes.TxOutItem{
		Chain:       common.BTCChain, // invalid chain
		ToAddress:   types2.GetRandomZECAddress(),
		VaultPubKey: s.client.nodePubKey,
		Coins: common.Coins{
			common.NewCoin(common.ZECAsset, cosmos.NewUint(100)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.ZECAsset, cosmos.NewUint(25000)),
		},
		InHash:  "",
		OutHash: "",
	}
	// incorrect chain should return an error
	result, _, _, err := s.client.SignTx(txOutItem, 1)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	// invalid pubkey should return an error
	txOutItem.Chain = common.ZECChain
	txOutItem.VaultPubKey = common.PubKey("helloworld")
	result, _, _, err = s.client.SignTx(txOutItem, 2)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	// invalid to address should return an error
	txOutItem.ToAddress = types2.GetRandomBTCAddress()
	txOutItem.VaultPubKey = s.client.nodePubKey
	result, _, _, err = s.client.SignTx(txOutItem, 3)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	// happy path
	txOutItem.ToAddress = types2.GetRandomZECAddress()
	result, _, _, err = s.client.SignTx(txOutItem, 4)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	// release all utxos for spending again for testing
	c.Assert(s.client.temporalStorage.PruneBlockMeta(999999999999999, nil), IsNil)

	addr := types2.GetRandomZECAddress()
	// Test cases
	// Test 1: Empty to address
	signed1, signed2, obs, err := s.client.SignTx(stypes.TxOutItem{
		Chain:       common.ZECChain,
		ToAddress:   "",
		VaultPubKey: s.client.nodePubKey,
		Coins: common.Coins{
			common.NewCoin(common.ZECAsset, cosmos.NewUint(1e8)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.ZECAsset, cosmos.NewUint(50000)),
		},
		Memo: "OUT:ABCD",
	}, 1)
	c.Assert(err, NotNil)
	c.Assert(strings.Contains(err.Error(), "tx toAddress is invalid"), Equals, true)
	c.Assert(signed1, IsNil)
	c.Assert(signed2, IsNil)
	c.Assert(obs, IsNil)

	// Test 2: Empty vault pubkey
	signed1, signed2, obs, err = s.client.SignTx(stypes.TxOutItem{
		Chain:       common.ZECChain,
		ToAddress:   addr,
		VaultPubKey: "",
		Coins: common.Coins{
			common.NewCoin(common.ZECAsset, cosmos.NewUint(1e8)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.ZECAsset, cosmos.NewUint(50000)),
		},
		Memo: "OUT:ABCD",
	}, 1)
	c.Assert(err, NotNil)
	c.Assert(strings.Contains(err.Error(), "fail to get vault pubkey bytes: decoding Bech32 address failed: must provide a non empty address"), Equals, true)
	c.Assert(signed1, IsNil)
	c.Assert(signed2, IsNil)
	c.Assert(obs, IsNil)

	// Test 3: Invalid amount
	signed1, signed2, obs, err = s.client.SignTx(stypes.TxOutItem{
		Chain:       common.ZECChain,
		ToAddress:   addr,
		VaultPubKey: s.client.nodePubKey,
		Coins: common.Coins{
			common.NewCoin(common.ZECAsset, cosmos.NewUint(1e18)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.ZECAsset, cosmos.NewUint(50000)),
		},
		Memo: "",
	}, 1)
	c.Assert(err, NotNil)
	c.Assert(strings.Contains(err.Error(), "total utxo amount") && strings.Contains(err.Error(), "is less than out amount"), Equals, true)
	c.Assert(signed1, IsNil)
	c.Assert(signed2, IsNil)
	c.Assert(obs, IsNil)

	// Test 4: Valid outbound transaction
	signed1, signed2, obs, err = s.client.SignTx(stypes.TxOutItem{
		Chain:       common.ZECChain,
		ToAddress:   addr,
		VaultPubKey: s.client.nodePubKey,
		Coins: common.Coins{
			common.NewCoin(common.ZECAsset, cosmos.NewUint(1e8)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.ZECAsset, cosmos.NewUint(50000)),
		},
		GasRate: 1,
		Memo:    "OUT:4D91ADAFA69765E7805B5FF2F3A0BA1DBE69E37A1CFCD20C48B99C528AA3EE87",
	}, 1)
	c.Assert(err, IsNil)
	c.Assert(signed1, NotNil)
	c.Assert(signed2, NotNil)
	c.Assert(obs, NotNil)

	// release all utxos for spending again for testing
	c.Assert(s.client.temporalStorage.PruneBlockMeta(999999999999999, nil), IsNil)

	// Test 5: Valid refund transaction
	signed1, signed2, obs, err = s.client.SignTx(stypes.TxOutItem{
		Chain:       common.ZECChain,
		ToAddress:   addr,
		VaultPubKey: s.client.nodePubKey,
		Coins: common.Coins{
			common.NewCoin(common.ZECAsset, cosmos.NewUint(1e8)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.ZECAsset, cosmos.NewUint(50000)),
		},
		GasRate: 1,
		Memo:    "REFUND:t1My73pkNMDbmTfF9HjnPHenK1EQM7jbLfS",
	}, 1)
	c.Assert(err, IsNil)
	c.Assert(signed1, NotNil)
	c.Assert(signed2, NotNil)
	c.Assert(obs, NotNil)

	// release all utxos for spending again for testing
	c.Assert(s.client.temporalStorage.PruneBlockMeta(999999999999999, nil), IsNil)

	// Test 6: Valid migrate transaction
	signed1, signed2, obs, err = s.client.SignTx(stypes.TxOutItem{
		Chain:       common.ZECChain,
		ToAddress:   addr,
		VaultPubKey: s.client.nodePubKey,
		Coins: common.Coins{
			common.NewCoin(common.ZECAsset, cosmos.NewUint(6499835000)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.ZECAsset, cosmos.NewUint(165000)),
		},
		GasRate: 1,
		Memo:    "MIGRATE:1024",
	}, 1)
	c.Assert(err, IsNil)
	c.Assert(signed1, NotNil)
	c.Assert(signed2, NotNil)
	c.Assert(obs, NotNil)

	// release all utxos for spending again for testing
	c.Assert(s.client.temporalStorage.PruneBlockMeta(999999999999999, nil), IsNil)
}

func (s *ZcashSuite) TestSignAndBroadcastTx(c *C) {
	if common.CurrentChainNetwork.SoftEquals(common.MainNet) {
		return // this test is not supported on mainnet as the wallet addresses are tesnet addresses
	}

	s.restartZecd(c)
	s.selectWallet(c, 0)
	utxos, err := s.client.client.GetAddressUTXOs(s.client.nodeAddress.String())
	c.Assert(err, IsNil)
	if len(utxos) == 0 {
		s.client.logger.Debug().Msgf("No UTXO")
		c.Assert(utxos, NotNil)
	}
	var testToAddress []string
	if common.CurrentChainNetwork.SoftEquals(common.MainNet) {
		testToAddress = testToAddressMainnet
	} else {
		testToAddress = testToAddressTestnet
	}
	for _, toAddressStr := range testToAddress {
		// clean spent utxos
		c.Assert(s.client.temporalStorage.PruneBlockMeta(999999999999999, nil), IsNil)
		var toAddress common.Address
		toAddress, err = common.NewAddress(toAddressStr, types2.GetCurrentVersion())
		c.Assert(err, IsNil)
		txOutItem := stypes.TxOutItem{
			Chain:       common.ZECChain,
			ToAddress:   toAddress,
			VaultPubKey: s.client.nodePubKey,
			Coins: common.Coins{
				common.NewCoin(common.ZECAsset, cosmos.NewUint(100)),
			},
			MaxGas: common.Gas{
				common.NewCoin(common.ZECAsset, cosmos.NewUint(25000)),
			},
			InHash:  "",
			OutHash: "",
			Memo:    "",
		}
		var payload []byte
		payload, _, _, err = s.client.SignTx(txOutItem, 4)
		c.Assert(err, IsNil)
		c.Assert(payload, NotNil)
		var computedTxId string
		computedTxId, err = s.client.client.GetRawTxID(payload)
		c.Assert(err, IsNil)
		c.Assert(computedTxId, HasLen, 64)
		var txId string
		generate := s.zecdSnapshot != "" // set to true if generating payloads for mock server
		if generate {
			txId = computedTxId
		} else {
			txId, err = s.client.BroadcastTx(txOutItem, payload)
		}
		if err != nil && (strings.Contains(err.Error(), "txn-mempool-conflict") ||
			strings.Contains(err.Error(), "txn-already-in-mempool") ||
			strings.Contains(err.Error(), "txn-already-known")) {
			// the same tx already broadcast
			s.client.logger.Debug().Msgf("tx already broadcast %s", computedTxId)
		} else {
			c.Assert(err, IsNil)
			c.Assert(txId, HasLen, 64)
			c.Assert(computedTxId, Equals, txId)
			if !generate {
				c.Assert(s.client.client.GenerateBlocks(5), IsNil)
			}
			s.client.logger.Debug().Msgf("tx successfully broadcast %s", txId)

			// remove from signer cache so we can broadcast again
			s.client.signerCacheManager.RemoveSigned(txId)
		}
	}
}

func (s *ZcashSuite) TestWifConversions(c *C) {
	vaultSk, err := hex.DecodeString(regtestVaultPrivKeyHex)
	c.Assert(err, IsNil)
	_, err = hex.DecodeString(regtestVaultPubKeyHex)
	c.Assert(err, IsNil)
	privKeyWIF := encodePrivateKeyToWIF(vaultSk, false, true)
	wif, err := btcutil.DecodeWIF(privKeyWIF)
	c.Assert(err, IsNil)
	privKeyBytes := wif.PrivKey.Serialize()
	privKeyHex := hex.EncodeToString(privKeyBytes)
	c.Assert(privKeyHex, Equals, regtestVaultPrivKeyHex)
	pubKey := btcec.PublicKey(wif.PrivKey.PublicKey)
	pubKeyBytes := pubKey.SerializeCompressed()
	pubKeyHex := hex.EncodeToString(pubKeyBytes)
	c.Assert(pubKeyHex, Equals, regtestVaultPubKeyHex)
}

func (s *ZcashSuite) TestBuildPtx(c *C) {
	txOutItem := stypes.TxOutItem{
		Chain:       common.ZECChain,
		VaultPubKey: s.client.nodePubKey,
		ToAddress:   types2.GetRandomZECAddress(),
		Coins: common.Coins{
			common.NewCoin(common.ZECAsset, cosmos.NewUint(100)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.ZECAsset, cosmos.NewUint(30000)),
		},
		InHash:  "",
		OutHash: "",
		Memo:    "=:c",
	}
	_, _, err := s.client.buildPtx(txOutItem)
	c.Assert(err, IsNil)
}

func (s *ZcashSuite) TestSignTxWithTSS(c *C) {
	if common.CurrentChainNetwork.SoftEquals(common.MainNet) {
		return // this test is not supported on mainnet as the wallet addresses are tesnet addresses
	}

	s.restartZecd(c)
	// generated by zcashd:
	testData := []struct {
		wif, pub, to string
		h            int64
		msg, r, s    string
	}{
		{
			wif: "cQ24hdAZXPXSP5DHmRADhLEYQ67NsTSpZS57AsDNM2wiho4n1CvQ", // wallet02, tmLqsSEy6cDjcTnRUd3ENKu1wWAckuiVUFx
			pub: "03a340c6b85f4a85c7d63c7c604237a94b388be547728b02ac9bd944c893afa55b",
			to:  "tmNUFAr71YAW3eXetm8fhx7k8zpUJYQiKZP",
			h:   185, // zec height
			msg: "Pp6jMCAp1HYk/cuo25TeOM5l2s2VSHgG5ifNtyxOm1U=",
			r:   "XP0JIoWF4mtVOf4HHMWkvhr7E7P8kxC36tCcC0s9QxI=",
			s:   "JhMCf/120lU6ZVhDu323tgoou86BuXEJGvysCpPr+6A=",
		},
		{
			wif: "cQ24hdAZXPXSP5DHmRADhLEYQ67NsTSpZS57AsDNM2wiho4n1CvQ", // wallet02, tmLqsSEy6cDjcTnRUd3ENKu1wWAckuiVUFx
			pub: "03a340c6b85f4a85c7d63c7c604237a94b388be547728b02ac9bd944c893afa55b",
			to:  "tmWJQXMC6h9hh3ohjxnnS25o3pEFe1HR8zr",
			h:   185, // zec height
			msg: "2aDQkD3ufWiB01FIbukC8PW4mAlMNzbRxgQaxOZp6Fw=",
			r:   "pAuqW4FWbaaSZyZMNT3rOTCUGfiOvmT2iruefBM6/DI=",
			s:   "U/+n9IollkSXZQ82E6fvL7YdpyBMn6MQVr4i+VBW6Bs=",
		},
		{
			wif: "cT5r3oWRA7aezz7EA9h68QzscJkL2P69faJ5hrZN35RRp5C5eZYK", // wallet03, tmAYgqddmQhUY6GnbUmCMyRwC7HcVnVZgWU
			pub: "021c85731e9c8318ec8d29247ccfd0ec5695953b4df978e8507e40bbedef72fcbb",
			to:  "tmNUFAr71YAW3eXetm8fhx7k8zpUJYQiKZP",
			h:   185, // zec height
			msg: "ysWTuQPtKXQYB/gli0jBtxDhPD7xeaShw5Q02rIyCY4=",
			r:   "YBO4JDm9VS8A+w7Ixt5mCAQ82LY4b0jZlAQfz4FDPTU=",
			s:   "MnJI16whz2m4KAqAZtT985sYgGYoU14rbBFSgVw/rlE=",
		},
		{
			wif: "cT5r3oWRA7aezz7EA9h68QzscJkL2P69faJ5hrZN35RRp5C5eZYK", // wallet03, tmAYgqddmQhUY6GnbUmCMyRwC7HcVnVZgWU
			pub: "021c85731e9c8318ec8d29247ccfd0ec5695953b4df978e8507e40bbedef72fcbb",
			to:  "tmWJQXMC6h9hh3ohjxnnS25o3pEFe1HR8zr",
			h:   185, // zec height
			msg: "uIOMFMYcAe174QOKP02UmzkTWVDMfwgCuGnlMEjzqNs=",
			r:   "bMAaJXNSTI4bBzBgOUvTN0Icb/ORRECwwjIuIyIP1a0=",
			s:   "drkS44+9c42+W5Am6RjGGCmhKql5Voqf3GOkIJK8CGU=",
		},
	}

	s.setRandomClientKeys(c)
	s.client.ksWrapper.tssKeyManager = &tss.MockMayachainKeyManager{}

	for _, td := range testData {
		c.Assert(s.client.temporalStorage.PruneBlockMeta(999999999999999, nil), IsNil)
		wif, err := btcutil.DecodeWIF(td.wif)
		c.Assert(err, IsNil)
		pubKey := btcec.PublicKey(wif.PrivKey.PublicKey)
		pubKeyBytes := pubKey.SerializeCompressed()
		pubKeyHex := fmt.Sprintf("%x", pubKeyBytes)
		c.Assert(pubKeyHex, Equals, td.pub)
		// convert pubKeyHex to common.PubKey to use in transaction
		// 1. convert btcec.PublicKey to Cosmos SDK secp256k1.PubKey
		cosmosPubKey := &secp256k1.PubKey{Key: pubKeyBytes}
		// 2. convert to Bech32 string
		bech32PubKey, err := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeAccPub, cosmosPubKey)
		c.Assert(err, IsNil)
		var txNodePubKey common.PubKey
		txNodePubKey, err = common.NewPubKey(bech32PubKey)
		c.Assert(err, IsNil)

		toAddress, err := common.NewAddress(td.to, types2.GetCurrentVersion())
		c.Assert(err, IsNil)
		txOutItem := stypes.TxOutItem{
			Chain:       common.ZECChain,
			ToAddress:   toAddress,
			VaultPubKey: txNodePubKey,
			Coins: common.Coins{
				common.NewCoin(common.ZECAsset, cosmos.NewUint(100)),
			},
			MaxGas: common.Gas{
				common.NewCoin(common.ZECAsset, cosmos.NewUint(25000)),
			},
			InHash:  "",
			OutHash: "",
		}
		var height int64
		height, err = s.client.getBlockHeight()
		c.Assert(err, IsNil)
		var result []byte
		if height == td.h {
			c.Assert(s.client.signerCacheManager.HasSigned(txOutItem.CacheHash()), Equals, false, Commentf("Tx already in signer cache"))
			result, _, _, err = s.client.SignTx(txOutItem, 69)
		}
		// if sign failed regenerate msgs and signatures
		if height != td.h || err != nil {
			// find and select the appropriate wallet for signing
			for i, w := range testSnapshotWallets {
				if td.wif == w.privKeyWIF {
					s.selectWallet(c, i)
					break
				}
			}
			var ptx zec.PartialTx
			ptx, _, err = s.client.buildPtx(txOutItem)
			c.Assert(err, IsNil)
			var signatures [][]byte
			signatures, err = s.client.signPtxParallel(ptx, txOutItem, 69, false)
			c.Assert(err, IsNil)
			var sig *btcec.Signature
			sig, err = btcec.ParseSignature(signatures[0], btcec.S256())
			c.Assert(err, IsNil)
			ok := sig.Verify(ptx.Sighashes[0], (*btcec.PublicKey)(&s.client.ksWrapper.privateKey.PublicKey))
			c.Assert(ok, Equals, true)
			for _, sb := range signatures {
				sig, err = btcec.ParseSignature(sb, btcec.S256())
				c.Assert(err, IsNil)
				msg := base64.StdEncoding.EncodeToString(ptx.Sighashes[0])
				sigR := base64.StdEncoding.EncodeToString(sig.R.Bytes())
				sigS := base64.StdEncoding.EncodeToString(sig.S.Bytes())
				s.client.logger.Error().Msgf("New sig data for %s: height=%d, msg=%s, r,s=(%s,%s)]", bech32PubKey, height, msg, sigR, sigS)
			}
			// set the default wallet
			s.selectWallet(c, 0)
		} else {
			c.Assert(err, IsNil)
			c.Assert(result, NotNil)
		}
	}
}

func (s *ZcashSuite) TestBroadcastTx(c *C) {
	if common.CurrentChainNetwork.SoftEquals(common.MainNet) {
		return // this test is not supported on mainnet as the wallet addresses are tesnet addresses
	}

	s.restartZecd(c)
	toAddress, err := common.NewAddress(testToAddressForGeneratingPayloadForTestBroadcastTx, types2.GetCurrentVersion())
	c.Assert(err, IsNil)
	txOutItem := stypes.TxOutItem{
		Chain:       common.ZECChain,
		ToAddress:   toAddress,
		VaultPubKey: s.client.nodePubKey,
		Coins: common.Coins{
			common.NewCoin(common.ZECAsset, cosmos.NewUint(100)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.ZECAsset, cosmos.NewUint(25000)),
		},
		InHash:  "",
		OutHash: "",
	}
	input := []byte("hello world")
	_, err = s.client.BroadcastTx(txOutItem, input)
	c.Assert(err, NotNil)
	input, err = hex.DecodeString("050000800a27a7265510e7c800000000d300000001d9cff0f746eae15e3f59eb448f1e0d95785ce7f213a26922ba5168407970a3ea000000006b483045022100b32d055ad1926f452e1ceeb012491653ed2d55f872d2a8d378cb7ff27cb91bde022047af55d2907559c0b18ea86c973029ef4dbe5d22a8eb35e5ecac1fbf8d8108cd01210390fe79d1719ab60369c9e710739aefb9d7db411733aa711b467b60232f47d81fffffffff0264000000000000001976a91493ce48570b55c42c2af816aeaba06cfee1224fae88ac1cf06759000000001976a91422db56f1d0f603ca56d85c2cb7b65d18c2802d1488ac000000")
	c.Assert(err, IsNil)
	var txId string
	txId, err = s.client.BroadcastTx(txOutItem, input)
	c.Assert(err, IsNil)
	c.Assert(txId, Not(Equals), "")
	c.Assert(s.client.client.GenerateBlocks(5), IsNil)
	s.client.logger.Debug().Msgf("tx successfully broadcast %s", txId)

	// broadcast once again and check the returned hash is the same
	var txId2 string
	txId2, err = s.client.BroadcastTx(txOutItem, input)
	c.Assert(err, IsNil)
	c.Assert(txId2, Equals, txId)

	// remove from signer cache so we can broadcast again
	s.client.signerCacheManager.RemoveSigned(txId)
}

func (s *ZcashSuite) TestIsSelfTransaction(c *C) {
	hash := "66d2d6b5eb564972c59e4797683a1225a02515a41119f0a8919381236b63e948"
	isSelf, _ := s.client.isSelfTransactionAndSpentUtxo(hash, 0)
	c.Check(isSelf, Equals, false)
	bm := utxo.NewBlockMeta("", 1024, "")
	bm.AddSelfTransaction(hash)
	c.Assert(s.client.temporalStorage.SaveBlockMeta(1024, bm), IsNil)
	isSelf, _ = s.client.isSelfTransactionAndSpentUtxo(hash, 0)
	c.Check(isSelf, Equals, true)
}

func (s *ZcashSuite) TestIsSpentUtxo(c *C) {
	txidSomeTxID, _ := common.NewTxID(common.RandHexString(64))
	txidSomeHash, _ := common.NewTxID(common.RandHexString(64))
	txidTxID, _ := common.NewTxID(common.RandHexString(64))
	txidHash, _ := common.NewTxID(common.RandHexString(64))
	someTxID := string(txidSomeTxID)
	someHash := string(txidSomeHash)
	txID := string(txidTxID)
	hash := string(txidHash)

	_, isSpent := s.client.isSelfTransactionAndSpentUtxo(hash, 0)
	c.Check(isSpent, Equals, false)
	bm := utxo.NewBlockMeta("", 1024, "")
	bm.AddPendingSpentUtxo(txID, hash, 0)
	bm.AddPendingSpentUtxo(someTxID, someHash, 0)
	c.Assert(s.client.temporalStorage.SaveBlockMeta(1024, bm), IsNil)
	// not yet committed as spent
	_, isSpent = s.client.isSelfTransactionAndSpentUtxo(hash, 0)
	c.Check(isSpent, Equals, false)
	// not yet committed as spent
	_, isSpent = s.client.isSelfTransactionAndSpentUtxo(someHash, 0)
	c.Check(isSpent, Equals, false)

	var err error
	bm, err = s.client.temporalStorage.GetBlockMeta(1024)
	c.Assert(err, IsNil)
	// our tx + some other tx
	c.Assert(bm.PendingSpentUtxos, HasLen, 2)
	bm.CommitPendingUtxoSpent(txID)
	c.Assert(s.client.temporalStorage.SaveBlockMeta(1024, bm), IsNil)
	// now it should be flagged as spent
	_, isSpent = s.client.isSelfTransactionAndSpentUtxo(hash, 0)
	c.Check(isSpent, Equals, true)
	// the other utxo from some other tx should not be committed
	_, isSpent = s.client.isSelfTransactionAndSpentUtxo(someHash, 0)
	c.Check(isSpent, Equals, false)
	// pending utxos should be cleared
	c.Assert(bm.PendingSpentUtxos, HasLen, 0)
}
