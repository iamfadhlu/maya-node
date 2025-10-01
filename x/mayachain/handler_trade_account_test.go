package mayachain

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	abci "github.com/tendermint/tendermint/abci/types"
	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	openapi "gitlab.com/mayachain/mayanode/openapi/gen"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
	. "gopkg.in/check.v1"
)

func (s *MultipleAffiliatesSuite) TestTradeAccountWithdrawal(c *C) {
	ctx, mgr := setupManagerForTest(c)
	asset := common.BTCAsset
	addr := GetRandomBech32Addr()
	bc1Addr := GetRandomBTCAddress()
	dummyTx := common.Tx{ID: "test"}

	{
		msg := NewMsgTradeAccountDeposit(asset, cosmos.NewUint(500), addr, addr, dummyTx)

		h := NewTradeAccountDepositHandler(mgr)
		_, err := h.Run(ctx, msg)
		c.Assert(err, IsNil)

		bal := mgr.TradeAccountManager().BalanceOf(ctx, asset, addr)
		c.Check(bal.String(), Equals, "500")

		vault := GetRandomVault()
		vault.Status = ActiveVault
		vault.Coins = common.Coins{
			common.NewCoin(asset, cosmos.NewUint(500*common.One)),
		}
		c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)
	}

	c.Assert(mgr.Keeper().SaveNetworkFee(ctx, common.BTCChain, NetworkFee{
		Chain: common.BTCChain, TransactionSize: 80000, TransactionFeeRate: 30,
	}), IsNil)

	msg := NewMsgTradeAccountWithdrawal(asset.GetTradeAsset(), cosmos.NewUint(350), bc1Addr, addr, dummyTx)

	h := NewTradeAccountWithdrawalHandler(mgr)
	_, err := h.Run(ctx, msg)
	c.Assert(err, IsNil)

	bal := mgr.TradeAccountManager().BalanceOf(ctx, asset, addr)
	c.Check(bal.String(), Equals, "150")

	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)
	c.Check(items[0].Coin.String(), Equals, "350 BTC.BTC")
	c.Check(items[0].ToAddress.String(), Equals, bc1Addr.String())
}

func (s *MultipleAffiliatesSuite) TestTradeAccountDeposit(c *C) {
	ctx, mgr := setupManagerForTest(c)
	h := NewTradeAccountDepositHandler(mgr)
	asset := common.BTCAsset
	addr := GetRandomBech32Addr()
	dummyTx := common.Tx{ID: "test"}

	msg := NewMsgTradeAccountDeposit(asset, cosmos.NewUint(350), addr, addr, dummyTx)

	_, err := h.Run(ctx, msg)
	c.Assert(err, IsNil)

	bal := mgr.TradeAccountManager().BalanceOf(ctx, asset, addr)
	c.Check(bal.String(), Equals, "350")
}

func (s *MultipleAffiliatesSuite) getTradeUnit(asset common.Asset, c *C) (resp openapi.TradeUnitResponse) {
	jsonData, err := queryTradeUnit(s.ctx, []string{asset.String()}, s.mgr)
	c.Assert(err, IsNil)
	err = json.Unmarshal(jsonData, &resp)
	c.Assert(err, IsNil)
	return resp
}

func (s *MultipleAffiliatesSuite) getTradeAccount(address common.Address, c *C) (resp []openapi.TradeAccountResponse) {
	jsonData, err := queryTradeAccount(s.ctx, []string{address.String()}, s.mgr)
	c.Assert(err, IsNil)
	err = json.Unmarshal(jsonData, &resp)
	c.Assert(err, IsNil)
	return resp
}

func (s *MultipleAffiliatesSuite) getTradeAccounts(asset common.Asset, c *C) (resp []openapi.TradeAccountResponse) {
	jsonData, err := queryTradeAccounts(s.ctx, []string{asset.String()}, s.mgr)
	c.Assert(err, IsNil)
	err = json.Unmarshal(jsonData, &resp)
	c.Assert(err, IsNil)
	return resp
}

func (s *MultipleAffiliatesSuite) getSwapQuote(from, to common.Asset, destination common.Address, amount cosmos.Uint, c *C) (resp openapi.QuoteSwapResponse) {
	params := url.Values{}
	params.Add("from_asset", from.String())
	params.Add("to_asset", to.String())
	params.Add("destination", destination.String())
	params.Add("amount", amount.String())
	swapReq := abci.RequestQuery{Data: []byte("/mayachain/quote/swap?" + params.Encode())}
	jsonData, err := queryQuoteSwap(s.ctx, swapReq, s.mgr)
	c.Assert(err, IsNil)
	err = json.Unmarshal(jsonData, &resp)
	c.Assert(err, IsNil)
	return resp
}

func (s *MultipleAffiliatesSuite) TestTradeAccountRegressionTradeYaml(c *C) {
	fox := GetRandomBaseAddress()
	dogBtc := GetRandomBTCAddress()
	foxBtc := GetRandomBTCAddress()
	foxAcc, err := fox.AccAddress()
	c.Assert(err, IsNil)
	FundAccount(c, s.ctx, s.mgr.Keeper(), foxAcc, 100_00) // 100 cacao

	// deposit btc
	asset := common.BTCAsset
	gasFee := s.mgr.gasMgr.GetFee(s.ctx, asset.Chain, asset)
	s.tx.Gas = common.Gas{common.NewCoin(asset, gasFee)}
	amt := cosmos.NewUint(10000000)
	// deposit 0.1 btc
	s.tx.FromAddress = foxBtc
	s.tx.ToAddress = dogBtc
	s.tx.Coins = common.Coins{common.NewCoin(asset, amt)}
	s.tx.Memo = fmt.Sprintf("trade+:%s", fox)
	c.Assert(s.processTx(), IsNil)

	tasset := asset.GetTradeAsset()

	tu := s.getTradeUnit(tasset, c)
	c.Assert(tu.Asset, Equals, tasset.String())
	c.Assert(tu.Units, EqualUint, amt)
	c.Assert(tu.Depth, EqualUint, amt)

	ta := s.getTradeAccount(fox, c)
	c.Assert(ta, HasLen, 1)
	c.Assert(ta[0].Asset, Equals, tasset.String())
	c.Assert(ta[0].Units, EqualUint, amt)
	c.Assert(ta[0].Owner, Equals, fox.String())

	evCnt := 0
	var ev types.EventTradeAccountDeposit
	for i, e := range s.ctx.EventManager().Events() {
		if strings.EqualFold(e.Type, types.TradeAccountDepositEventType) {
			s.ctx.Logger().Debug("Trade Account Deposit Event", "i", i)
			evCnt++
			for j, a := range e.Attributes {
				s.ctx.Logger().Debug("Trade Account Deposit Event attribute", "i", i, "j", j, "key", string(a.Key), "value", string(a.Value))
				switch string(a.Key) {
				case "asset":
					ev.Asset, err = common.NewAsset(string(a.Value))
					c.Assert(err, IsNil)
				case "amount":
					ev.Amount = cosmos.NewUintFromString(string(a.Value))
				case "cacao_address":
					ev.CacaoAddress = common.Address(a.Value)
				case "asset_address":
					ev.AssetAddress = common.Address(a.Value)
				}
			}
		}
	}
	c.Assert(evCnt, Equals, 1)
	c.Assert(ev.Asset.String(), Equals, tasset.String())
	c.Assert(ev.Amount, EqualUint, amt)
	c.Assert(ev.CacaoAddress.String(), Equals, fox.String())
	c.Assert(ev.AssetAddress.String(), Equals, foxBtc.String())

	// swap trade asset to eth trade asset
	swapAmt := cosmos.NewUint(5000000)
	swapCoins := common.Coins{common.NewCoin(tasset, swapAmt)}
	memo := fmt.Sprintf("=:ETH~ETH:%s", fox)
	msg := NewMsgDeposit(swapCoins, memo, foxAcc)
	c.Assert(s.handleDeposit(msg), IsNil)
	c.Assert(s.queueEndBlock(), IsNil)
	c.Assert(s.mockTxOutStore.tois, HasLen, 1)
	s.mockTxOutStore.tois = nil

	tu = s.getTradeUnit(tasset, c)
	amt = amt.Sub(swapAmt)
	c.Assert(tu.Asset, Equals, tasset.String())
	c.Assert(tu.Units, EqualUint, amt)
	c.Assert(tu.Depth, EqualUint, amt)

	ta = s.getTradeAccount(fox, c)
	c.Assert(ta, HasLen, 2)
	c.Assert(ta[0].Asset, Equals, tasset.String())
	c.Assert(ta[0].Units, EqualUint, amt)
	c.Assert(ta[0].Owner, Equals, fox.String())
	c.Assert(ta[1].Asset, Equals, common.ETHAsset.GetTradeAsset().String())
	ethUnits := cosmos.NewUint(1995972016)
	c.Assert(ta[1].Units, EqualUint, ethUnits)
	c.Assert(ta[1].Owner, Equals, fox.String())

	s.mockTxOutStore.tois = nil
	s.clear()
}

func (s *MultipleAffiliatesSuite) TestTradeAccountRegressionTradeYamlWithdrawalAmountFee(c *C) {
	fox := GetRandomBaseAddress()
	dogBtc := GetRandomBTCAddress()
	foxBtc := GetRandomBTCAddress()
	foxAcc, err := fox.AccAddress()
	c.Assert(err, IsNil)
	FundAccount(c, s.ctx, s.mgr.Keeper(), foxAcc, 100_00) // 100 cacao

	// deposit btc
	btc := common.BTCAsset
	gasFee := s.mgr.gasMgr.GetFee(s.ctx, btc.Chain, btc)
	s.tx.Gas = common.Gas{common.NewCoin(btc, gasFee)}
	amt := cosmos.NewUint(9483211)
	// deposit 0.1 btc
	s.tx.FromAddress = foxBtc
	s.tx.ToAddress = dogBtc
	s.tx.Coins = common.Coins{common.NewCoin(btc, amt)}
	s.tx.Memo = fmt.Sprintf("trade+:%s", fox)
	c.Assert(s.processTx(), IsNil)

	btct := btc.GetTradeAsset()

	tu := s.getTradeUnit(btct, c)
	c.Assert(tu.Asset, Equals, btct.String())
	c.Assert(tu.Units, EqualUint, amt)
	c.Assert(tu.Depth, EqualUint, amt)

	ta := s.getTradeAccount(fox, c)
	c.Assert(ta, HasLen, 1)
	c.Assert(ta[0].Asset, Equals, btct.String())
	c.Assert(ta[0].Units, EqualUint, amt)
	c.Assert(ta[0].Owner, Equals, fox.String())

	btcOutFee := s.mgr.gasMgr.GetFee(s.ctx, btc.Chain, btc)

	// withdraw ~2/3, leave 3000000
	withdrawAmt := cosmos.NewUint(6483211)
	withdrawCoins := common.Coins{common.NewCoin(btct, withdrawAmt)}
	memo := fmt.Sprintf("trade-:%s", foxBtc)
	msg := NewMsgDeposit(withdrawCoins, memo, foxAcc)
	c.Assert(s.handleDeposit(msg), IsNil)
	c.Assert(s.queueEndBlock(), IsNil)

	newRemainingUints := amt.Sub(withdrawAmt)
	tu = s.getTradeUnit(btct, c)
	c.Assert(tu.Asset, Equals, btct.String())
	c.Assert(tu.Units, EqualUint, newRemainingUints)
	c.Assert(tu.Depth, EqualUint, newRemainingUints)
	ta = s.getTradeAccounts(btct, c)
	c.Assert(ta, HasLen, 1)
	ta = s.getTradeAccount(fox, c)
	c.Assert(ta, HasLen, 1)

	c.Assert(s.mockTxOutStore.tois, HasLen, 1)
	toi := s.mockTxOutStore.tois[0]
	c.Assert(toi.ToAddress, EqualAddress, foxBtc)
	c.Assert(toi.Coin.Asset.String(), Equals, "BTC.BTC")
	c.Assert(toi.Coin.Amount, EqualUint, withdrawAmt)

	block, err := s.mgr.Keeper().GetTxOut(s.ctx, s.ctx.BlockHeight())
	c.Assert(err, IsNil)
	c.Assert(block.TxArray, HasLen, 1)
	c.Assert(block.TxArray[0].Coin.Amount, EqualUint, withdrawAmt.Sub(btcOutFee))
	s.mockTxOutStore.tois = nil
	s.clear()

	teth := common.ETHAsset.GetTradeAsset()
	sq := s.getSwapQuote(btct, teth, fox, cosmos.NewUint(5000000), c)
	c.Assert(*sq.Memo, Equals, fmt.Sprintf("=:ETH~ETH:%s", fox))
	c.Assert(sq.ExpectedAmountOut, Equals, "1995971736")
}
