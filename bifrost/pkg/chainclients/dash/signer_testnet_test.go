//go:build testnet
// +build testnet

package dash

import (
	dashec "gitlab.com/mayachain/dashd-go/btcec"
	"gitlab.com/mayachain/dashd-go/btcjson"
	stypes "gitlab.com/mayachain/mayanode/bifrost/mayaclient/types"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/shared/utxo"
	"gitlab.com/mayachain/mayanode/bifrost/tss"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	. "gopkg.in/check.v1"
)

func (s *BitcoinSuite) TestGetAddressesFromScriptPubKeyResult(c *C) {
	addresses := s.client.getAddressesFromScriptPubKeyBTC(btcjson.ScriptPubKeyResult{
		Asm:     "0 de4f4fce2642935d2b9fc7b28bcc9de20ebf2864",
		Hex:     "0014de4f4fce2642935d2b9fc7b28bcc9de20ebf2864",
		ReqSigs: 1,
		Type:    "witness_v0_keyhash",
		Addresses: []string{
			"tb1qme85ln3xg2f462ulc7eghnyaug8t72ryhwzs8f",
		},
	})
	c.Assert(addresses, HasLen, 1)
	c.Assert(addresses[0], Equals, "tb1qme85ln3xg2f462ulc7eghnyaug8t72ryhwzs8f")

	addresses = s.client.getAddressesFromScriptPubKeyBTC(btcjson.ScriptPubKeyResult{
		Asm:       "0 de4f4fce2642935d2b9fc7b28bcc9de20ebf2864",
		Hex:       "0014de4f4fce2642935d2b9fc7b28bcc9de20ebf2864",
		ReqSigs:   1,
		Type:      "witness_v0_keyhash",
		Addresses: nil,
	})
	c.Assert(addresses, HasLen, 1)
	c.Assert(addresses[0], Equals, "tb1qme85ln3xg2f462ulc7eghnyaug8t72ryhwzs8f")
}

func (s *DashSignerSuite) TestSignTxWithTSS(c *C) {
	pubkey, err := common.NewPubKey("tmayapub1addwnpepqwznsrgk2t5vn2cszr6ku6zned6tqxknugzw3vhdcjza284d7djp59sf99q")
	c.Assert(err, IsNil)
	addr, err := pubkey.GetAddress(common.DASHChain)
	c.Assert(err, IsNil)
	txOutItem := stypes.TxOutItem{
		Chain:       common.DASHChain,
		ToAddress:   addr,
		VaultPubKey: "tmayapub1addwnpepqw2k68efthm08f0f5akhjs6fk5j2pze4wkwt4fmnymf9yd463puru38eqd3",
		Coins: common.Coins{
			common.NewCoin(common.DASHAsset, cosmos.NewUint(10)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.DASHAsset, cosmos.NewUint(1000)),
		},
		InHash:  "",
		OutHash: "",
	}
	mayaKeyManager := &tss.MockMayachainKeyManager{}
	pkeyV2, _ := dashec.PrivKeyFromBytes(s.client.privateKey.Serialize())
	s.client.keySignWrapper, err = NewKeySignWrapper(pkeyV2, mayaKeyManager)
	txHash := "66d2d6b5eb564972c59e4797683a1225a02515a41119f0a8919381236b63e948"
	c.Assert(err, IsNil)
	// utxo := NewUnspentTransactionOutput(*txHash, 0, 0.00018, 100, txOutItem.VaultPubKey)
	blockMeta := utxo.NewBlockMeta("000000000000008a0da55afa8432af3b15c225cc7e04d32f0de912702dd9e2ae",
		100,
		"0000000000000068f0710c510e94bd29aa624745da43e32a1de887387306bfda")
	blockMeta.AddCustomerTransaction(txHash)
	c.Assert(s.client.temporalStorage.SaveBlockMeta(blockMeta.Height, blockMeta), IsNil)
	buf, _, _, err := s.client.SignTx(txOutItem, 1)
	c.Assert(err, IsNil)
	c.Assert(buf, NotNil)
}

func (s *BitcoinSuite) TestGetAccount(c *C) {
	acct, err := s.client.GetAccount("tmayapub1addwnpepqt7qug8vk9r3saw8n4r803ydj2g3dqwx0mvq5akhnze86fc536xcy7cau6l", nil)
	c.Assert(err, IsNil)
	c.Assert(acct.AccountNumber, Equals, int64(0))
	c.Assert(acct.Sequence, Equals, int64(0))
	c.Assert(acct.Coins[0].Amount.Uint64(), Equals, uint64(2502000000))

	acct1, err := s.client.GetAccount("", nil)
	c.Assert(err, NotNil)
	c.Assert(acct1.AccountNumber, Equals, int64(0))
	c.Assert(acct1.Sequence, Equals, int64(0))
	c.Assert(acct1.Coins, HasLen, 0)
}
