//go:build !testnet && !stagenet && !regtest && !mocknet
// +build !testnet,!stagenet,!regtest,!mocknet

package mayachain

import (
	"fmt"
	"strconv"
	"strings"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
	"gitlab.com/mayachain/mayanode/x/mayachain/types"
)

func importPreRegistrationMAYANames(ctx cosmos.Context, mgr Manager) error {
	oneYear := mgr.Keeper().GetConfigInt64(ctx, constants.BlocksPerYear)
	names, err := getPreRegisterMAYANames(ctx, ctx.BlockHeight()+oneYear, mgr.GetVersion())
	if err != nil {
		return err
	}

	for _, name := range names {
		mgr.Keeper().SetMAYAName(ctx, name)
	}
	return nil
}

func migrateStoreV96(ctx cosmos.Context, mgr Manager) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v88", "error", err)
		}
	}()

	err := importPreRegistrationMAYANames(ctx, mgr)
	if err != nil {
		ctx.Logger().Error("fail to migrate store to v88", "error", err)
	}
}

func migrateStoreV102(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v102", "error", err)
		}
	}()

	if err := mgr.BeginBlock(ctx); err != nil {
		ctx.Logger().Error("fail to initialise block", "error", err)
		return
	}

	// Remove stuck txs
	hashes := []string{
		"D19F621FAD0AE81688E4AF40EA9D0CD132A8A4DBFF3EA56F443E2D9083915F17",
		"A03C0A41909D85B2DF2F7E9D5D13F6E0AF89F366F6B580C0CCC13F5CEC0A9872",
		"7B7CC323ED0AD04DCB26DF1DEB46DE02B85345499336D043CBB5582EB77D22DB",
		"CE29D8AD79314333E307265529256304E26FC0B538B19B2D07578BE3D6252CE4",
		"B6EBF457EB1817E852722CB9F51C26E45C35F58B2445048FE4BD38FD1A603894",
		"199838DB755A6199AB401AAB1D56D296C66B0972001CB033B9CDC4217E636270",
		"674BDD72DF068A95EFA5DD94C4691A1D492A3342DA368DF0799ADD4D344D694D",
		"6F9C3D5AD6221159191540CE55704BCBB446626B209C852DC29C5C0AC7A24A82",
		"F3A3041FD304B11B8EBB748C9BE964E1FCCE0004770B109F5F9B72114F7FB9B9",
		"4912D98B5C8D9D090CAD2732754F39FFB324DA7008A19A0235DD77A4AB8EF3E3",
		"C82029C6D3F7D8D226E9B13F09CD05CF30FEA15F6C96BB8D49E20A4E063F6E82",
		"46C783972218F50015281F28222F5DC46FA3926EABB93549A383180C43064F96",
		"081819200E3ACC82CC8D95DFA87A6F0D87704154922022F777FFA5AD82B1BEF0",
		"9C3B5774352256A37CC3B26B82287458C4D4DEDC988342E6A2088A1800ACE992",
		"8B651D92B0374FA4E97834E86D35601940F90E104B800BEA836685E28452A953",
		"EA57B9FC879E981598732F6112255D756593D354DC88712665FEDC354374AD41",
		"672D551F02D6030A77745E25C0C8768347BBDA35DD7AA61C02751C86799D7C18",
		"116237EDC4814A9F684D8FCBC58FB5ADED2A9386B5ED0F1E627BCEFA8246474C",
		"3EE6362906A180279B0B9470221017465A1AA25807EFDB5A7B9342A95E120E2B",
		"1B04FF39247F519BC01F88FB1AC6843223FF351C47DC1D96B0FEC782645463F1",
		"25BC9C71B8F4D071A684A327D6E2657DA1D01D241E419C5C705D3690B5653C2E",
		"758B2A8DF6BC62F1A922DEA5E75F585A9BDB39CFC01152E4C74FBF929C5B5777",
		"A502C210DE19555884464A27408E16C378D9327BDC155EE3076F7D3D8CC8B963",
		"95AD2B7B2EE2E2CBEF272B20AB271400CDF57EE8EC170F8B265554A9FC24542A",
		"279B076B50ADCF2DE06CF129DE6B4917754F56FFB7CF4644FF0429CFC49A0D23",
		"D427493DEF0DFE953194E2E3C633C7EF3AAEC38F77B06B9E1EFAEFFF2071D58C",
		"B6F9C4CB2ABC7FB80B336950E559DD3020CC44C8F92A6AD9D3449612A5A232CC",
		"DDB8A4FD768443BF36187EA6147469A3D4975ED0CD8B4DDB2140EF4B924C7817",
		"C6E972C90798E33317DAC162D7B419AF825540F352A7CF38A5AB1297EAA866E9",
		"313F924DC160D565573F3B9D0A47F378A099606FFE4D059947B5377AE98E9F65",
		"E8340277F7E2310DDD2435A52EC1CC7C07C6D33FCD1F4ABD513FF23B6B19990F",
		"CDFDEBF26E28789F7C272813524C7F3766A9B82AFF55CBAD9AF347121061171B",
		"9F10CA47145E9F6B6EB4297272D9DF9999A75937CD154D6C4DAEC3DBFE14C3D6",
		"1D843810B79E7ED1CDD5424B0FDBD6158ADE77479E1C006F2583E5263E26E667",
		"1A486C051F7478CE845D67E019EE30DEF58D61B8EE408FE43E6CD520DE45518F",
		"EC461F353F95D933723BDAE7945B970A7F45DCA68A671D06B0FE9AA206686EFC",
		"66F228BD65D1A82C6D78C234D1A86F1C7E118A1051D87FE6546F708E401720FA",
		"233D1D0FB660BDA2C3C13C9B6C2BD0E96E81E05EC93C43A526CE0B782CA4ADA1",
		"8C867538F1C5A564C1C82206CBF0B96277B66E630BF13473E51D27BAE8B1994E",
		"BEA5B29954A3634B37CC0D73EB30EB8427ABF58900521A413F0A66C73AD6742A",
		"F3FA329499C42BF258B4D79E43ADEEB1E9C56FB60D4A9390B12F4946A554642A",
		"B26E3D4D8458DD43DB3B6424F1310B457053BDA95DC22D3936FCC373B49C95AC",
		"972E49601D4BF9949C3B91162399249B4AC997ED1BA830DB6DBC7DF44ABBEF3D",
		"D4B8E0F61978046D1205B5DC857BBD887214BB7054113B499FAADF7105F4CFE0",
		"97C8E399272FD9C64C2E2F1E2E32804157BFBA71504B4B838850F2590F87D781",
		"50C0CCD601689011E88B54358001EA2C6B1E8C0AC6794D1A7D8C95A74256071C",
		"640702E326CB6B61CC7285B0ADCE6DEF0694E9CFB629FD32C34A5475B5391E9E",
		"C60AE6A164FE3BE8B2BE87543B25B0F36E199E1CE466CB09482D9ED7D2D78BA9",
		"A58A823D9E467B368713D65090DCCFAAD92D1C8D6F2B57E3933EB8ACB9946031",
		"8EA259B4E7D15FBB6703C0A1248A137BCBAA7255ABEC09CEFAC2FB34DF7BC2F5",
		"B20036A869329CA1CDF966F0443B8B524A2CF6AB4F4ECA7C359D61A0A167F36B",
		"05F77D6640AE44FCFCB30FBDB8E76F82C4FD75E05A6DC48271EC499A1A09C378",
		"8E70E838DCEAD4D1763B4B40E59942EEFE5492B631D9CFB303A1DB0F7075F835",
		"C268C6821C3A8C19B435B4591F216A20E8DAD9AD2C17EF59F7CF9BD4DA2B4536",
		"9E1DF502EE17709E267AEDF673CF94B188698C0CBB9A6FAAAF57EAE20D043495",
		"FA194EDF6818312E6B28AB1D228A44B8623415595ED1716E7B7A92CB3DFCDE36",
		"941D3F4B252B735C2D358A368724DC809ED9CC63D6ED4426E369E75175EDF0F0",
		"24B077C67D4F835D176B701EEC59FA1C14143A0E3ACFF64077632FB3CEBD2851",
		"C4E86318378C561AD16DF9697F09224D254A314BED36EC7AC6C0B7F35FAB5CDB",
		"BEFFC122704DB5525A9511411A942F7F06EECF6386C104BB05622EDAE94D8096",
		"5422336EF4134851F601A74AA30C5E47702CC08111775ADC3944F4F0B467CD4F",
		"308FDED05E0F39E103A6E3898A497A1F28806ED7DEAB2D88F825E95CC4942D53",
		"C41ADECBC9D85D956D3246CEFD350E54CCABDA2B315793FD2625D30BEA0763C4",
		"333C9BC7B7479D4A675307B63AB2372C89C9C21A75C379BB6FE8EA8FB83813A0",
		"A967482A359194C6B3E0045F68B2E11CD275B29FF7F3A7F6129902D90FFA7055",
		"9DD6CFA490E5ED47BAE45E1CEE141329C411D8BAF5642758CCF3749D13862076",
	}
	removeTransactions(ctx, mgr, hashes...)

	// Rebalance asgard vs real balance for RUNE
	vaultPubkey, err := common.NewPubKey("mayapub1addwnpepq0tgksv4kjn0ya5n4gt2546dnasw84nr3zdtdzfud9z984p8pvmnu5t3qsy")
	if err != nil {
		ctx.Logger().Error("fail to get vault pubkey", "error", err)
		return
	}
	vault, err := mgr.Keeper().GetVault(ctx, vaultPubkey)
	if err != nil {
		ctx.Logger().Error("fail to get vault", "error", err)
		return
	}

	vault.SubFunds(common.NewCoins(common.NewCoin(common.RUNEAsset, cosmos.NewUint(3947_32403277))))

	if err = mgr.Keeper().SetVault(ctx, vault); err != nil {
		ctx.Logger().Error("fail to set vault", "error", err)
		return
	}

	// Remove retiring vault
	vaults, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, RetiringVault)
	if err != nil {
		ctx.Logger().Error("fail to get retiring asgard vaults", "error", err)
		return
	}
	for _, v := range vaults {
		runeAsset := v.GetCoin(common.RUNEAsset)
		v.SubFunds(common.NewCoins(runeAsset))
		if err = mgr.Keeper().SetVault(ctx, v); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
		}
	}

	// Add LPs from unobserved txs
	lps := []struct {
		MayaAddress string
		ThorAddress string
		TxID        string
		Amount      cosmos.Uint
		Tier        int64
	}{
		// users which don't have an LP position yet
		{
			MayaAddress: "maya142m4adpj57hkrymqe5n8zzcxm5cqccpn3a6w6y",
			ThorAddress: "thor1jzzaw44tr0cxgxaah7h2sen2ck03lllw882wn2",
			TxID:        "73217ACF7F4061089236E29588825603FB4025E40AC5835586ED0B7959BE4A1F",
			Amount:      cosmos.NewUint(2_00000000),
			Tier:        3,
		},
		{
			MayaAddress: "maya15kg7dfew844rdh5esgkrdevp78yhf4fjryjcfu",
			ThorAddress: "thor1hd9p0fllkwkgj9epe3nynr253az7uclxs4g2uw",
			TxID:        "5594D2500BB36F70ADB4063B4D7A331DCE884D2C34373EDBD69022C33E31CD0F",
			Amount:      cosmos.NewUint(1_00000000),
			Tier:        3,
		},
		{
			MayaAddress: "maya17lllslx89rrxu0dh6y9ctz0aa2j82tljnuuy9s",
			ThorAddress: "thor1vmq7vwk8t6sxg730aps5vqetm905ndtmcvdq69",
			TxID:        "10376393CBF1C9E92CCBBDF582FFE9896FC04E82C2E9C641B4CB18A23559E43E",
			Amount:      cosmos.NewUint(2_00000000),
			Tier:        1,
		},
		{
			MayaAddress: "maya1f40wek6sj6uay6nplxpe2c6pj98c5uq78xspa4",
			ThorAddress: "thor1f40wek6sj6uay6nplxpe2c6pj98c5uq783wdt9",
			TxID:        "2A7297AD1EB1F1C53C90241264E78F067DE94F1C80588C208E8B7B5D86B3B9E7",
			Amount:      cosmos.NewUint(5388_00000000),
			Tier:        1,
		},
		{
			MayaAddress: "maya1j8pswr7vpf9jjmhrn0xlwvzla2f9yfxwcwtj0p",
			ThorAddress: "thor1y9h2yk95c6uqp29xglkgyf9kqxqnu28nn6vwwz",
			TxID:        "D5BEA6C8B3170B418ACD67B8C8A44A60CD0A66696B9B691E7A7471E393F5E8B4",
			Amount:      cosmos.NewUint(1_00000000),
			Tier:        3,
		},
		{
			MayaAddress: "maya1k83lm2nyrd7vgl8h9xcjhwu9kr2zecauslje79",
			ThorAddress: "thor1k83lm2nyrd7vgl8h9xcjhwu9kr2zecausgv4g4",
			TxID:        "DEF2BC77DFCDA774C81D921C8846886FFF804D462F0E6BFF78DBAA1ADDF72E68",
			Amount:      cosmos.NewUint(24_83513163),
		},
		{
			MayaAddress: "maya1p3hcnlfdla2647rpersykfatplvhkehd2duspa",
			ThorAddress: "thor1p3hcnlfdla2647rpersykfatplvhkehd26zuhd",
			TxID:        "C4F73CFBAC15565CCAED86B66EB405AE9E36F712F0457F8353D050FF37D636BB",
			Amount:      cosmos.NewUint(1_00000000),
		},
		{
			MayaAddress: "maya1pn03td7tzsftp6xz25r5fas43dgqynpf0lyan5",
			ThorAddress: "thor1pn03td7tzsftp6xz25r5fas43dgqynpf0g639y",
			TxID:        "A85BE46FFDD915D2074EC85C8E5B63B0407EFDD44CC6094CCC9A616A7FFB0494",
			Amount:      cosmos.NewUint(1_00000000),
			Tier:        3,
		},
		{
			MayaAddress: "maya1s0ry4c65c7k020vgpykjfy5rkqv8d7yn60lzx6",
			ThorAddress: "thor1s0ry4c65c7k020vgpykjfy5rkqv8d7yn6cpws2",
			TxID:        "4A1CA0E1D87869C5083F6BBD2042BF5DA5545B01ECE9CD7922F11D8AB715B261",
			Amount:      cosmos.NewUint(1_00000000),
			Tier:        3,
		},
		{
			MayaAddress: "maya1vwslytml73dclz0h4enc2xluf4z03esrt36n6r",
			ThorAddress: "thor1vwslytml73dclz0h4enc2xluf4z03esrtxylvn",
			TxID:        "237CBC3570DA3AE95D15F6E7C04A50EF3799A4106434A9A831A11BEDA8EB0FF6",
			Amount:      cosmos.NewUint(36_00000000),
			Tier:        1,
		},
		{
			MayaAddress: "maya1wlx25u0692nvxllg57tgt45h53hjsgggzlgavn",
			ThorAddress: "thor1cjlsyrzmfpldxhmz4j3yzyc0f6dp57lhv6cm2r",
			TxID:        "C8D1F65C6C6559D4A23E8BB47533E86CC25D8C41FA8382EC2C6FBF868953AB23",
			Amount:      cosmos.NewUint(1_50000000),
			Tier:        3,
		},
		{
			MayaAddress: "maya1zgtzwkd9qaagvwedgnmxeh9tsqc8wdsjwjxf6e",
			ThorAddress: "thor1zgtzwkd9qaagvwedgnmxeh9tsqc8wdsjw9c9vf",
			TxID:        "79A5288200EB347569B7E3707A822E72B2DB1CCD52BC035323DE2B1DC44273B3",
			Amount:      cosmos.NewUint(499_98000000),
			Tier:        1,
		},
		// users which already have an existing LP position
		{
			MayaAddress: "maya10nqg4w30e9dnm0qg7swa8qsyqevuemwx78dpdx",
			ThorAddress: "thor10nqg4w30e9dnm0qg7swa8qsyqevuemwx7sndmk",
			Amount:      cosmos.NewUint(5_58000000),
		},
		{
			MayaAddress: "maya14sanmhejtzxxp9qeggxaysnuztx8f5jra7vedl",
			ThorAddress: "thor14sanmhejtzxxp9qeggxaysnuztx8f5jrafj4m0",
			Amount:      cosmos.NewUint(958_08765797),
		},
		{
			MayaAddress: "maya17w5n2r7akuunq9e296y22qrljh3qqegf6usf5x",
			ThorAddress: "thor17w5n2r7akuunq9e296y22qrljh3qqegf6tw9zk",
			Amount:      cosmos.NewUint(1400_00000000),
		},
		{
			MayaAddress: "maya1a4v8ajttgx5u822k2s8zms3phvytz3at2a7mgj",
			ThorAddress: "thor1a4v8ajttgx5u822k2s8zms3phvytz3at22qh7z",
			Amount:      cosmos.NewUint(1_000000),
		},
		{
			MayaAddress: "maya1fdl7xga4sxhwlfs48fhkgwen88003g3hl006pn",
			ThorAddress: "thor1fdl7xga4sxhwlfs48fhkgwen88003g3hlc3khr",
			Amount:      cosmos.NewUint(1_00000000),
		},
		{
			MayaAddress: "maya1hh03993slyvggmvdl7q4xperg5n7l86pufhkwr",
			ThorAddress: "thor1wlzhcxs0r4yh4pswj8zfqlp7dnp95p4kxn0dcr",
			Amount:      cosmos.NewUint(4_30000000),
		},
		{
			MayaAddress: "maya1j42xpqgfdyagr57pxkxgmryzdfy2z4l65mjzf9",
			ThorAddress: "thor1j42xpqgfdyagr57pxkxgmryzdfy2z4l65vvwl4",
			Amount:      cosmos.NewUint(2_00000000),
		},
		{
			MayaAddress: "maya1j6ep9yljeswft03w2qunqx8my9e2efph5ywhls",
			ThorAddress: "thor1jj4xufkxrjd4d3uswh0ztgr0yan3mdcdxh3tgn",
			Amount:      cosmos.NewUint(2_00000000),
		},
		{
			MayaAddress: "maya1jwq4zu4v3tfktwemwh2lwwnlu3nvvrhuhs6k0h",
			ThorAddress: "thor1jwq4zu4v3tfktwemwh2lwwnlu3nvvrhuh8y6e8",
			Amount:      cosmos.NewUint(285_40743565),
		},
		{
			MayaAddress: "maya1ka2v9exy8ata00pch87wgzf9dsmyag94tq8mug",
			ThorAddress: "thor1ka2v9exy8ata00pch87wgzf9dsmyag94theh2c",
			Amount:      cosmos.NewUint(978_00000000),
		},
		{
			MayaAddress: "maya1mj8yhw3jqljfcggkjd77k9t7jlcw0uur0yfurh",
			ThorAddress: "thor1mj8yhw3jqljfcggkjd77k9t7jlcw0uur0nhs48",
			Amount:      cosmos.NewUint(341_00000000),
		},
		{
			MayaAddress: "maya1ppdzsyugtsdtd6dpvzzg2746qfdfmux7k2ydal",
			ThorAddress: "thor1z9xhmhtxn5gxd4ugfuxk7hg9hp03tw3qtqs3f3",
			Amount:      cosmos.NewUint(1_00000000),
		},
		{
			MayaAddress: "maya1qdhqqlg5kcn9hz7wf8wzw8hj8ujrjplnz669c9",
			ThorAddress: "thor1ru7upan5aj2hmzlevrztd6gn5r5z8jxrcjzmup",
			Amount:      cosmos.NewUint(1_00000000),
		},
		{
			MayaAddress: "maya1qtcst64ea585s7gtek3daj2xe59hgn8q7j0ccl",
			ThorAddress: "thor1qtcst64ea585s7gtek3daj2xe59hgn8q7935w0",
			Amount:      cosmos.NewUint(2998_00000000),
		},
	}

	pool, err := mgr.Keeper().GetPool(ctx, common.RUNEAsset)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return
	}

	var address common.Address
	for _, sender := range lps {
		address, err = common.NewAddress(sender.MayaAddress, mgr.GetVersion())
		if err != nil {
			ctx.Logger().Error("fail to parse address", "error", err)
			continue
		}

		var lp LiquidityProvider
		lp, err = mgr.Keeper().GetLiquidityProvider(ctx, common.RUNEAsset, address)
		if err != nil {
			ctx.Logger().Error("fail to get liquidity provider", "error", err)
			continue
		}

		pool.PendingInboundAsset = pool.PendingInboundAsset.Add(sender.Amount)
		lp.PendingAsset = lp.PendingAsset.Add(sender.Amount)
		lp.LastAddHeight = ctx.BlockHeight()
		if sender.TxID != "" {
			var txID common.TxID
			txID, err = common.NewTxID(sender.TxID)
			if err != nil {
				ctx.Logger().Error("fail to parse txID", "error", err)
				continue
			}
			lp.PendingTxID = txID
		}

		if lp.AssetAddress.IsEmpty() {
			var thorAdd common.Address
			thorAdd, err = common.NewAddress(sender.ThorAddress, mgr.GetVersion())
			if err != nil {
				ctx.Logger().Error("fail to parse address", "address", sender.MayaAddress, "error", err)
				continue
			}
			lp.AssetAddress = thorAdd
		}

		mgr.Keeper().SetLiquidityProvider(ctx, lp)
		if sender.Tier != 0 {
			if err = mgr.Keeper().SetLiquidityAuctionTier(ctx, lp.CacaoAddress, sender.Tier); err != nil {
				ctx.Logger().Error("fail to set liquidity auction tier", "address", lp.CacaoAddress, "error", err)
				continue
			}
		}

		if err = mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to set pool", "address", pool.Asset, "error", err)
			return
		}

		evt := NewEventPendingLiquidity(pool.Asset, AddPendingLiquidity, lp.CacaoAddress, cosmos.ZeroUint(), lp.AssetAddress, sender.Amount, common.TxID(""), common.TxID(sender.TxID))
		if err = mgr.EventMgr().EmitEvent(ctx, evt); err != nil {
			continue
		}
	}

	// Remove duplicated THOR address LP position
	// https://mayanode.mayachain.info/mayachain/liquidity_auction_tier/thor.rune/maya1dy6c9tmu7qgpd6cw2unumew3sknduwx7s0myr6?height=488436
	// https://mayanode.mayachain.info/mayachain/liquidity_auction_tier/thor.rune/maya1yf0sglxse7jkq0laddtve2fskkrv6vzclu3u6e?height=488436
	add1, err := common.NewAddress("maya1dy6c9tmu7qgpd6cw2unumew3sknduwx7s0myr6", mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to parse address", "error", err)
		return
	}

	lp1, err := mgr.Keeper().GetLiquidityProvider(ctx, common.RUNEAsset, add1)
	if err != nil {
		ctx.Logger().Error("fail to get liquidity provider", "error", err)
		return
	}

	add2, err := common.NewAddress("maya1yf0sglxse7jkq0laddtve2fskkrv6vzclu3u6e", mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to parse address", "error", err)
		return
	}

	lp2, err := mgr.Keeper().GetLiquidityProvider(ctx, common.RUNEAsset, add2)
	if err != nil {
		ctx.Logger().Error("fail to get liquidity provider", "error", err)
		return
	}
	lp2.PendingAsset = lp2.PendingAsset.Add(lp1.PendingAsset)

	mgr.Keeper().SetLiquidityProvider(ctx, lp2)
	if err = mgr.Keeper().SetLiquidityAuctionTier(ctx, lp2.CacaoAddress, 0); err != nil {
		ctx.Logger().Error("fail to set liquidity auction tier", "error", err)
	}
	mgr.Keeper().RemoveLiquidityProvider(ctx, lp1)

	// Mint cacao
	toMint := common.NewCoin(common.BaseAsset(), cosmos.NewUint(9_900_000_000_00000000))
	if err = mgr.Keeper().MintToModule(ctx, ModuleName, toMint); err != nil {
		ctx.Logger().Error("fail to mint cacao", "error", err)
		return
	}

	if err = mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, ReserveName, common.NewCoins(toMint)); err != nil {
		ctx.Logger().Error("fail to send cacao to reserve", "error", err)
		return
	}

	// 150214766379119 de BTC Asgard a reserve
	// 473657580023 de ETH Asgard a reserve
	// 24192844274670 de RUNE asgard a reserve
	for _, asset := range []common.Asset{common.BTCAsset, common.ETHAsset, common.RUNEAsset} {
		pool, err = mgr.Keeper().GetPool(ctx, asset)
		if err != nil {
			ctx.Logger().Error("fail to get pool", "error", err)
			return
		}
		switch asset {
		case common.BTCAsset:
			pool.BalanceCacao = pool.BalanceCacao.Sub(cosmos.NewUint(1_501_734_01773759))
		case common.ETHAsset:
			pool.BalanceCacao = pool.BalanceCacao.Sub(cosmos.NewUint(4736_57580023))
		case common.RUNEAsset:
			pool.BalanceCacao = pool.BalanceCacao.Sub(cosmos.NewUint(211_877_34242261))
		}

		if err = mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to set pool", "error", err)
			return
		}
	}

	// Sum of all the above will be sent
	asgardToReserve := common.NewCoin(common.BaseAsset(), cosmos.NewUint(1_717_347_93596043))
	if err = mgr.Keeper().SendFromModuleToModule(ctx, AsgardName, ReserveName, common.NewCoins(asgardToReserve)); err != nil {
		ctx.Logger().Error("fail to send asgard to reserve", "error", err)
		return
	}

	// 164293529917265 de itzamna a reserve
	itzamnaToReserve := common.NewCoin(common.BaseAsset(), cosmos.NewUint(1_642_935_29917265))
	itzamnaAcc, err := cosmos.AccAddressFromBech32("maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz")
	if err != nil {
		ctx.Logger().Error("fail to parse address", "error", err)
		return
	}

	if err := mgr.Keeper().SendFromAccountToModule(ctx, itzamnaAcc, ReserveName, common.NewCoins(itzamnaToReserve)); err != nil {
		ctx.Logger().Error("fail to send itzamna to reserve", "error", err)
		return
	}

	// FROM RESERVE TXS
	// 8_910_000_500_00000000 from reserve to itzamna
	reserveToItzamna := common.NewCoin(common.BaseAsset(), cosmos.NewUint(8_910_001_000_00000000))
	if err := mgr.Keeper().SendFromModuleToAccount(ctx, ReserveName, itzamnaAcc, common.NewCoins(reserveToItzamna)); err != nil {
		ctx.Logger().Error("fail to send reserve to itzamna", "error", err)
		return
	}

	// Remove Slash points from genesis nodes
	for _, genesis := range GenesisNodes {
		acc, err := cosmos.AccAddressFromBech32(genesis)
		if err != nil {
			ctx.Logger().Error("fail to parse address", "error", err)
			continue
		}

		mgr.Keeper().ResetNodeAccountSlashPoints(ctx, acc)
	}
}

func migrateStoreV104(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v104", "error", err)
		}
	}()

	// Select the least secure ActiveVault Asgard for all outbounds.
	// Even if it fails (as in if the version changed upon the keygens-complete block of a churn),
	// updating the voter's FinalisedHeight allows another MaxOutboundAttempts for LackSigning vault selection.
	activeAsgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil || len(activeAsgards) == 0 {
		ctx.Logger().Error("fail to get active asgard vaults", "error", err)
		return
	}
	if len(activeAsgards) > 1 {
		signingTransactionPeriod := mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)
		activeAsgards = mgr.Keeper().SortBySecurity(ctx, activeAsgards, signingTransactionPeriod)
	}
	vaultPubKey := activeAsgards[0].PubKey

	// Refund failed synth swaps back to users
	// These swaps were refunded because the target amount set by user was higher than the swap output
	// but because there were a bug in calculating the fee of synth swaps they were treated as zombie coins,
	// and thus we failed to generate the out tx of refund. (keep in mind that the refund event is emitted)
	// Since they are all inbound transactions, we can refund them back to users without deducting fee (see refundTransactions implementation)
	failedSwaps := []adhocRefundTx{
		{
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "ETH/USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48",
			amount:      300000000,
			inboundHash: "86AC0A216FA3138E3B1EE15D66DEBCBE46D8A62B45EA6D33E07DE044D4BD638E",
		}, {
			toAddr:      "maya1x5979k5wqgq58f4864glr7w2rtgyuqqm6l2zhx",
			asset:       "THOR/RUNE",
			amount:      26142918750,
			inboundHash: "9FC3C8886CD432338B4E4A388DF718B3EE03B257CA2D87792A9D3AFE4AC76DA6",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "ETH/USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48",
			amount:      5047059000,
			inboundHash: "86964E9623839AEBD7D4E74CC777F917AC5DACA850B322F07E7CD6F9A8ACEC1F",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "ETH/USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48",
			amount:      5273550000,
			inboundHash: "1A9D4E7000FE5EF4E292378F1EA075D69DE4DAF2FD5258AC5C2C6E495F28B843",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "ETH/USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48",
			amount:      5366524000,
			inboundHash: "7BADEBA845A889750BF9477B8A01870F109EAAE46E55EF032EF868540F6DB4C1",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "ETH/USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48",
			amount:      6347294000,
			inboundHash: "C71D1260FD4FE208CFA70440847250716F8C852674592B55EDA390FF840E1C8E",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "ETH/USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48",
			amount:      6347446000,
			inboundHash: "703E35046FC628B48CA06DD8FE9A95151ECC447C9A55FA7172CC6ED0F97540C4",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "ETH/USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48",
			amount:      5389962000,
			inboundHash: "B1F5B7C9B8AA46A96A72D1E10BD083172810669B912E46BFF2713B4D6237C42C",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "ETH/USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48",
			amount:      5390043000,
			inboundHash: "D55E1265E68D4605118EC02ADE7FB2FD2A91AD35878E701093D0C82B8D624A04",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      1271000,
			inboundHash: "A20F86A30BED39CFC4734EEA1C50680CC32002974E2FE5CE82BB22B26643D618",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      1271000,
			inboundHash: "F3FE1EFE4181E1F81048CCCD366A0E624A98C8C9ED9DC304E3DC32BF2FD3050D",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      1272000,
			inboundHash: "11BFC34721FEBE40CD080432B379F4D9C43DCA147653AFF82849B82838C1B4FD",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      621000,
			inboundHash: "31722DE22A5243DAB294529F3323B6708E8B3040C0205D5602F4F3F5D4218712",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      847000,
			inboundHash: "FFBBAE0420A1F7D1371F837BCA89D697EAC6E7D90835767C70D5B05584F95CD1",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      237000,
			inboundHash: "AD516C4C23A984336DC2BEE188CD7B607F31E342192BD0D05371A0B1AC127234",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      237000,
			inboundHash: "42C110B63651F47B10066C47334296E9E28E006A6481B675A8DAA27946843B81",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      429000,
			inboundHash: "EC192B4327CE11A03611FB5EFEEF3E133C8937040B36B4312A1743BACB4FFA88",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      384000,
			inboundHash: "A43B1F63D2B3092B80F0321DFFE81179BD3EE7209B1EA035D573A83F68EB7177",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      384000,
			inboundHash: "811035D9F7A177199F2BD84B90F82477AC68B2898D0F99B9EEB524766AC914DD",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      429000,
			inboundHash: "F5A01D066C001EB138E8DC4FA21B36917FCA8DCB07289B0E8575FD9B500C4C59",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      429000,
			inboundHash: "EB9F7D4AB93920447EC3423A8D4F1E92C43AFC42B9D18C1362EC5322DFB5ADDB",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      265000,
			inboundHash: "0C1359A466FDB89F450D02AB5C36F1073179D9333D05616FC33D8946058498BC",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      310000,
			inboundHash: "6AB46CD16A3BD9015570C2CD086CEF7BF75ADEBD98C7732149024C45F8458602",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      237000,
			inboundHash: "A45D65ADC584C687DBB696DD10E4E7E9FDC1E81FBA5525BEBF978A03EA2B93BD",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      310000,
			inboundHash: "4554C137395882EB69F01017D64993CCDCA7263AAFB755ADBFE5FDA6A00AB8A4",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      502000,
			inboundHash: "86567487D4E2B1E05B1A86EFE7A7A548849B8F82E17A8325484F855B92633D9B",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      502000,
			inboundHash: "83767F9F779657C0D770975ADFF3A92BCD1EE7A1C999985EA9DB066FBB44610F",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      502000,
			inboundHash: "34DE850E4F1AEFB5EE32C9FA2446D85B76F531E3006463B5F390570439FD96DE",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      502000,
			inboundHash: "18308912F70B0C58FF53BFC7617514C1279EDE4537FA29C822CA3801BBF7C82B",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      265000,
			inboundHash: "7C0EAF6ADE8B9A6DEF3CCF2F462826DDEEFD71C60B29023CE107115616B614BC",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      265000,
			inboundHash: "45B2F0E6338DCD7799026821D3F86A1C794F80A19ED5EE8CA3E5EC649A194B4C",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      265000,
			inboundHash: "2E22F59FC3B69CF871A411B2A057FDCA2DF00469819EFB2A946BE05E22373362",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      310000,
			inboundHash: "3DB087973B69B3A32EA4FA5B16579B3C2EEE6A0E070C03EF8DDE578A12B399FD",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      265000,
			inboundHash: "841A7B59C3E20A58A20C939BCD45800E69D2316F74B84F990B9DCC2E5D43D632",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      502000,
			inboundHash: "E394B2D44226421FC4FCCAD3C0F58D80EE6FE3F70B93E5FA1B699923EBF73588",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      502000,
			inboundHash: "79122693813DB28FC79D9454FA4327523ADAF71F4675BD36BF677301A568090F",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      502000,
			inboundHash: "34068637E11234A6AFC0C85A318CB58666623A8E36626D4E265B10551E1C7166",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      502000,
			inboundHash: "05AEE74CB1B9A2AD4CA3BAC63DB4D4ADA0ADF1EB345D6BC94CCCD0672669E1F8",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      502000,
			inboundHash: "BDBA06245E439EA80C1DEB8295449DF6EF3FC22D0F4D64FAFF3C0095D64413CB",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      502000,
			inboundHash: "A99BAE4BB38CA491D8457D16F2577144879285D97A44E3624848D70F1FD5963B",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      502000,
			inboundHash: "4541912B35D1D6A27A6263B6A7E608AFEB1687984765B2669DAE2612188AD4B9",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      846000,
			inboundHash: "59E5738F0DDB3B1D7F3DF8EFBD691C61480BDB909B57E971C5FFF03054A0EC3B",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      846000,
			inboundHash: "C48B393A050D7983F83C429BD3D17E2C300FAC2EFCB5B18F45E68E42952BC126",
		}, {
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "ETH/USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48",
			amount:      20021610000,
			inboundHash: "48D8788931772A5566C922C098C579FBBEE2B2793057B487FE3AE2AC2F3C8ED9",
		}, {
			toAddr:      "maya1x5979k5wqgq58f4864glr7w2rtgyuqqm6l2zhx",
			asset:       "ETH/USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7",
			amount:      82605080070,
			inboundHash: "E0320F7459B83A9F86695C9D0DB78B916F69FF5E408F94E521889F3F0C3CE086",
		},
	}
	refundTransactions(ctx, mgr, vaultPubKey.String(), failedSwaps...)

	// 1st user tier fix
	// User with address "maya1dy6c9tmu7qgpd6cw2unumew3sknduwx7s0myr6" and "maya1yf0sglxse7jkq0laddtve2fskkrv6vzclu3u6e" which had
	// should have been allocated an amount during the cacao donation in the last store migration but seems that
	// there was a problem with the migration and the amount was not allocated. So we change his/her tier to 1
	// and allocate the attribution amount manually from reserve.
	// The changes are as the following:
	// 1. Change Tier from 0 -> 1
	// 2. Overwrite LP Units from 0 -> 38089_5898484080 LP Units
	// 3. Pending Asset from 3210_34000000 -> 0
	// 4. Asset Deposit Value from 0 -> 3273_80071698
	// 5. Cacao Deposit Value from 0 -> 38089_5898484080 (Same as LP Units)
	// 6. Move 38827_9343263458 CACAO from Reserve to Asgard module (CACAO deposit value + Change difference between asset deposit value and pending asset with CACAO denom)
	// 7. Increase by 38827_9343263458 the CACAO on Asgard Pool for RUNE (CACAO deposit value + Change difference between asset deposit value and pending asset with CACAO denom)
	// 8. Increase by 38089_5898484080 the LP UNITS on Asgard Pool for RUNE
	// 9. Move 3210_34000000 Asset from Pending_Asset in Asgard Pool for RUNE to Balance_Asset in Asgard Pool for RUNE
	// 10. Emit Add Liquidity Event
	addr1, err := common.NewAddress("maya1yf0sglxse7jkq0laddtve2fskkrv6vzclu3u6e", mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to parse address", "error", err)
		return
	}
	lp1, err := mgr.Keeper().GetLiquidityProvider(ctx, common.RUNEAsset, addr1)
	if err != nil {
		ctx.Logger().Error("fail to get liquidity provider", "error", err)
		return
	}
	lp1.Units = cosmos.NewUint(38089_5898484080)
	lp1.PendingAsset = cosmos.ZeroUint()
	lp1.AssetDepositValue = cosmos.NewUint(3273_80071698)
	lp1.CacaoDepositValue = cosmos.NewUint(38089_5898484080)
	mgr.Keeper().SetLiquidityProvider(ctx, lp1)
	if err = mgr.Keeper().SetLiquidityAuctionTier(ctx, lp1.CacaoAddress, 1); err != nil {
		ctx.Logger().Error("fail to set liquidity auction tier", "error", err)
	}

	reserve2Asgard1 := common.NewCoin(common.BaseAsset(), cosmos.NewUint(38827_9343263458))
	if err = mgr.Keeper().SendFromModuleToModule(ctx, ReserveName, AsgardName, common.NewCoins(reserve2Asgard1)); err != nil {
		ctx.Logger().Error("fail to send reserve to asgard", "error", err)
		return
	}
	pool, err := mgr.Keeper().GetPool(ctx, common.RUNEAsset)
	if err != nil {
		ctx.Logger().Error("fail to get pool", "error", err)
		return
	}
	addedCacao1 := cosmos.NewUint(38827_9343263458)
	pool.BalanceCacao = pool.BalanceCacao.Add(addedCacao1)
	addedLPUnits := cosmos.NewUint(38089_5898484080)
	pool.LPUnits = pool.LPUnits.Add(addedLPUnits)
	pendingAsset2Balance := cosmos.NewUint(3210_34000000)
	pool.PendingInboundAsset = pool.PendingInboundAsset.Sub(pendingAsset2Balance)
	pool.BalanceAsset = pool.BalanceAsset.Add(pendingAsset2Balance)
	evt1 := NewEventAddLiquidity(
		pool.Asset,
		addedLPUnits,
		lp1.CacaoAddress,
		addedCacao1,
		pendingAsset2Balance,
		common.TxID(""),
		common.TxID(""),
		lp1.AssetAddress,
	)

	// 2nd user tier fix
	// 1. Change Tier from 3 -> 1
	// 2. Increase Asset Deposit Value from 333_46986565 to 507_5548724869
	// 3. Increase Cacao Deposit Value from 3879_8117483183 to 5905_2332716199
	// 4. Increase LP UNITS from 3879_8117483183 to 5905_2332716199
	// 5. Move 4050_84304660322 CACAO from Reserve module to Asgard module (twice as much on purpose, to account for asset side, will be armed away)
	// 6. Increase in 4050_84304660322 CACAO the balance_cacao of Asgard Pool for RUNE (twice as much on purpose, to account for asset side, will be arbed away)
	// 7. Increase by 2025_4215233016 the LP UNITS on Asgard Pool for RUNE
	// 8. Emit Add Liquidity Event
	addr2, err := common.NewAddress("maya1jwq4zu4v3tfktwemwh2lwwnlu3nvvrhuhs6k0h", mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to parse address", "error", err)
		return
	}
	lp2, err := mgr.Keeper().GetLiquidityProvider(ctx, common.RUNEAsset, addr2)
	if err != nil {
		ctx.Logger().Error("fail to get liquidity provider", "error", err)
		return
	}
	lp2.AssetDepositValue = cosmos.NewUint(507_5548724869)
	lp2.CacaoDepositValue = cosmos.NewUint(5905_2332716199)
	lp2.Units = cosmos.NewUint(5905_2332716199)
	mgr.Keeper().SetLiquidityProvider(ctx, lp2)
	if err = mgr.Keeper().SetLiquidityAuctionTier(ctx, lp2.CacaoAddress, 1); err != nil {
		ctx.Logger().Error("fail to set liquidity auction tier", "error", err)
	}

	reserve2Asgard2 := common.NewCoin(common.BaseAsset(), cosmos.NewUint(4050_84304660322))
	if err = mgr.Keeper().SendFromModuleToModule(ctx, ReserveName, AsgardName, common.NewCoins(reserve2Asgard2)); err != nil {
		ctx.Logger().Error("fail to send reserve to asgard", "error", err)
		return
	}
	addedCacao2 := cosmos.NewUint(4050_84304660322)
	pool.BalanceCacao = pool.BalanceCacao.Add(addedCacao2)
	addedLPUnits2 := cosmos.NewUint(2025_4215233016)
	pool.LPUnits = pool.LPUnits.Add(addedLPUnits2)
	evt2 := NewEventAddLiquidity(
		pool.Asset,
		addedLPUnits2,
		lp2.CacaoAddress,
		addedCacao2,
		cosmos.ZeroUint(),
		common.TxID(""),
		common.TxID(""),
		common.Address(""),
	)

	err = mgr.Keeper().SetPool(ctx, pool)
	if err != nil {
		ctx.Logger().Error("fail to set pool", "error", err)
		return
	}
	if err := mgr.EventMgr().EmitEvent(ctx, evt1); err != nil {
		ctx.Logger().Error("fail to emit event", "error", err)
		return
	}
	if err := mgr.EventMgr().EmitEvent(ctx, evt2); err != nil {
		ctx.Logger().Error("fail to emit event", "error", err)
		return
	}
}

// migrateStoreV105 is complementory migration to migration v104
// it will refund another 17 failed synth swaps txs back to users
func migrateStoreV105(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v105", "error", err)
		}
	}()

	// Select the least secure ActiveVault Asgard for all outbounds.
	// Even if it fails (as in if the version changed upon the keygens-complete block of a churn),
	// updating the voter's FinalisedHeight allows another MaxOutboundAttempts for LackSigning vault selection.
	activeAsgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil || len(activeAsgards) == 0 {
		ctx.Logger().Error("fail to get active asgard vaults", "error", err)
		return
	}
	if len(activeAsgards) > 1 {
		signingTransactionPeriod := mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)
		activeAsgards = mgr.Keeper().SortBySecurity(ctx, activeAsgards, signingTransactionPeriod)
	}
	vaultPubKey := activeAsgards[0].PubKey

	// Refund failed synth swaps back to users
	// These swaps were refunded because the target amount set by user was higher than the swap output
	// but because there were a bug in calculating the fee of synth swaps they were treated as zombie coins,
	// and thus we failed to generate the out tx of refund. (keep in mind that the refund event is emitted)
	// Since they are all inbound transactions, we can refund them back to users without deducting fee (see refundTransactions implementation)
	failedSwaps := []adhocRefundTx{
		{
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "ETH/USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48",
			amount:      6175167000,
			inboundHash: "8EECEE5C27795B96E8465D3234DEC050219AC591D899D038D2F11A1EFCE00E72",
		},
		{
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      1271000,
			inboundHash: "E31EBA09AA7E64DE5F1209656956286C4883196B0E85A075764600ABC57ACDB6",
		},
		{
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      384000,
			inboundHash: "6777B04215485FC495A88FA5D76C1873E250756FFF5E23577CA3CEEB4E042B0C",
		},
		{
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      384000,
			inboundHash: "E34798700D6034A3D8C82F80E7FCC4AC0F68574FCB7FD018EFA7E90A2594A44F",
		},
		{
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      384000,
			inboundHash: "2D129E0E58A762263272DB2548B432912E995F2A09CFF4A6C06A4DF8534290C7",
		},
		{
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      846000,
			inboundHash: "0172C67339320D14E477DCEB64F9FC4FABEE67DF233F08A81EB4D061F1820AC1",
		},
		{
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "BTC/BTC",
			amount:      846000,
			inboundHash: "86E8363E44B4EF0B32A894FD3011AC6AB8EC7AAE3EA2F65ACD8D0D15DB1299C7",
		},
		{
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "ETH/USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48",
			amount:      5314334000,
			inboundHash: "DBBDA76A5315F25787041BF95A65FC19BD2464B637BA9ED322CD8A52C1CE447E",
		},
		{
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "ETH/USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48",
			amount:      5927862000,
			inboundHash: "8D40B6E45B676638764FB38A998FAD782514AF2DDDB840A809A6CB65C854DF70",
		},
		{
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "THOR/RUNE",
			amount:      8527629000,
			inboundHash: "A860164DFDC3B0E76B871FF93A509B80736486B622AE59D0EE77ECE5F0E39D6A",
		},
		{
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "THOR/RUNE",
			amount:      8527450000,
			inboundHash: "3B99680D1927C6A3D909B964378443E8D5C71F9DA2A3E7FF4AFF16C7B6E08FA5",
		},
		{
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "THOR/RUNE",
			amount:      20123240000,
			inboundHash: "53BA8317F50DFB97FF30235BA479F3E3F78E29FEFA90BB2C113891F121D79C04",
		},
		{
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "THOR/RUNE",
			amount:      7565885000,
			inboundHash: "DD862F71E427F5DE280F2CAA49E007B77A1B64E30896AA93B1EC782374CDAB04",
		},
		{
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "THOR/RUNE",
			amount:      20107080000,
			inboundHash: "62D7648EC776A7B68FFAB23844EB5AA2C967F7E7CA97379E03607682B312B33E",
		},
		{
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "THOR/RUNE",
			amount:      20103040000,
			inboundHash: "BD90238148001ADF4B485D98D549AB472F5AC1881E8F67DBA70CC5C80E979803",
		},
		{
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "ETH/USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48",
			amount:      5252251000,
			inboundHash: "D028821B72FD5A37C092771FF9F5039C7A7E04FFDDEF8793A0FFE0BD73156733",
		},
		{
			toAddr:      "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc",
			asset:       "ETH/USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48",
			amount:      5060695000,
			inboundHash: "89266DF89E689C79DE4ACCB7312FEC85CE57CF92852A222228C3388F6FBDDA57",
		},
		{
			toAddr:      "maya1x5979k5wqgq58f4864glr7w2rtgyuqqm6l2zhx",
			asset:       "THOR/RUNE",
			amount:      72494713125,
			inboundHash: "8671D17BFD6040531470C89D0412116EE2909396BB6C54E037535DFD529E67D2",
		},
	}
	refundTransactions(ctx, mgr, vaultPubKey.String(), failedSwaps...)

	// Refunding USDT coins that mistakenly got sent to the vault (mayapub1addwnpepqwuwsax7p3raecsn2k9uvqyykanlvhw47asz836se2h0nyg6knug6n9hklq) by "transfer" txs back to user
	// transaction hashes are: 0xda4306037c838dcaed92775ecd515441e4a932b1bcbeef1199bf37a29274575d and 0xa6d765192856e982deae51bfc817f612c30344402ca72fbe526e8c534b91d048 on eth mainnet
	maxGas, err := mgr.GasMgr().GetMaxGas(ctx, common.ETHChain)
	if err != nil {
		ctx.Logger().Error("fail to get max gas", "error", err)
		return
	}
	toi := TxOutItem{
		Chain:       common.ETHChain,
		InHash:      common.BlankTxID,
		ToAddress:   common.Address("0x2510d455bF4a9b829C0CfD579543918D793F7762"),
		Coin:        common.NewCoin(common.USDTAssetV1, cosmos.NewUint(191_970_000+96_448_216)),
		MaxGas:      common.Gas{maxGas},
		GasRate:     int64(mgr.GasMgr().GetGasRate(ctx, common.ETHChain).Uint64()),
		VaultPubKey: common.PubKey("mayapub1addwnpepqwuwsax7p3raecsn2k9uvqyykanlvhw47asz836se2h0nyg6knug6n9hklq"),
	}
	if err := mgr.TxOutStore().UnSafeAddTxOutItem(ctx, mgr, toi, ctx.BlockHeight()); err != nil {
		ctx.Logger().Error("fail to save tx out item for refund transfers", "error", err)
		return
	}
}

func migrateStoreV106(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v106", "error", err)
		}
	}()

	danglingInboundTxIDs := []common.TxID{
		"EE6C4711C360C09B88D399E2000F66EDBC9D88243E977E4DA386575801B6C7BD",
		"D870C04715093BA8180705324F4B5F7BBFAF24D2D9F6FD41825EC3DA0A4848D4",
		"625C4E707AC12244DD657CC0465A280E2B5C64DA37C1F61B70F2DC4269E66760",
		"BBF3652682882D05D0B2ACDF8A06ECD1F16CA95877B50AD9EA22A012F0CE22F2",
		"F5D933BC96464024C7B176A699C881D8C158D3019673D6E6F4156B1D5D1C2B92",
		"C02BEB8C8A35D3DEF148FD1BEEA1BE74A6E0C1E437CFCE0690342C0D988D7BDB",
		"E3BFDD6AAA01B1444DD43DD02F82485C4892FBA10620DD8CB3B8371EE98009E7",
		"77342E2C624EEC3A031EA541498401B60679A46CF793470879EEDE9D95E8B062",
		"DA80B9FA56213D8F4176D1D81B1FA056EA360CDC103408CF10946B3626E54DC0",
		"3733965B49CE9655EAA1AA0AFF7EECE6067448B9BC5C6EA39CDD03CBDF6210E5",
		"3495C1DD09808BE80048D1B169855A96509C07C1814FB0CCFB5F7B20314C33A4",
		"41DCE02268892610CFB0FF0C442C9B701F89A871F8BD1DE0973CF86BC539EC85",
		"3920E1FC6362656466D6BB9BF93D754BDFCAAABD0D83EB720D37A35D4483E696",
		"A0306B7FCB6A1E626E797E828C79EAD36EB64A1A382BA44444071F15B83F8601",
		"BB6735BE958AC4C92EF7FD828C39233504E3F4DF2E879AC028924C52A2373FDB",
		"2561147F935F6FB962ACE98019507BC3459F09BC3DE27B13FF7642375DE895B6",
		"8ABF335AC56A27F028435EC1633474E7CF5CC8549CA418E4B9C69E705E1745B8",
		"CB390FAD8D646D541046999A2347AABFE25FA73B00382E07C18F62A03C833420",
		"8BB5C19CD2CF12BA5FCC505BCC4727155DD5B12200AF81E85176CD3D894417B7",
		"C9AE69BB8982F14CDB7EA9135E0EEA71F9EADD5F9260346302547CC39BAC0ED6",
		"87766587E59B40ABBA29B3615B96B8A8743C0401B87E3F7B55C1FFBCBA2B70DF",
		"42987321CD58E6F174A1FB7703F720CE0B7D78EDD2F2D51FE6EDDD400A4EB881",
		"9E01E3CB16D655CF91FE45477C38265F73C06DEF9684BD91696BEB6635873B0E",
		"A51A34C375E093E5B4BA8D9F0330FCE0D959A3B1D237F3E40DFFE0A97A65414D",
		"210088D913A05BCD6166534FD78CE0472C57D8CD4DD6811911C3E4728AD8CC13",
		"F165CD890E63B782E61B497854F6C2E4F12CB1D5BEA22193352586239E513502",
		"2DA12C51226E5F7BDEF5FD9087C72957A71B2D9FF0068AB200CDC28CD590C5A4",
		"44A0A75C227C0B5C197071116EFF318DD913798F33CA602F3EE07B6B043ED7DD",
		"199EAFFEE1E83873C4A35539DB5A209F87C7796AF7BA47C05758C401B6AE72A8",
		"CB0B1034EB9C82D8FB5CEC349FE37C6BE9185EEDB17384294433D360A7A66202",
		"B6730601AF5CFB78E6A040F32BBBCE599F662067D7C3D1D3E825D94CB53FE95A",
		"6129474913FDBDDC7C0A40D260DAA581F162033962E617F5B047492917F4CF96",
		"3EA40E01E372FF97A42366C64198873EE42D2288F4056AF88A4288E8F6ADF16C",
		"D8039ACA5F751FD777BDDF046064B06962693B6964543FF5EFA65C12A2D76026",
		"63E7565BC422ED92693BEB6E4F43254D168199C49F0C154798E31DDEAAAF879A",
		"D4BD757812983E18C839F0C7C071545C53253697C6FF171E479EEDB71D44664C",
		"A30EA261E2ADC64D115A0FA59A98FF1F060543BFBE44A9DF30FE3E9D7A2097B4",
		"6AF92FB712157AD24E6369090FBF0EA80ED0096361BBE2167C5C3AD86253E8FB",
		"4755A67A2910562F047CFB3A22A68610B403B49F70BC63479525E92F861AB8C1",
		"09A26EFCBB2A7F7CFCA58EFFD9E33D6791E9184AB8E9E69C41CBAA55FA4E61C4",
		"3B471BFCB5D8E839F2DE293CE17F4A9C9C1756D72DD5C897A9FEB25B882DBA6E",
		"8096A1316D4BC86CACF76B1F9584A2FAE89660F5B1E5D78CB8716CC2F3C95D33",
		"F0E8EC18F3C2A264E00BF69139B57B4670735FCAEFD80E4AFDF867A586EEA1C5",
		"D2F0BBBE8D17A231A7EF39EF262F368B23C5206096CA1905870F188C0BDFA14B",
		"9AE32B3D9B7F4325C605B0166B04BF3DB805A54FAEA7180AFF01BAC713960FA2",
		"782304A4A0381EA89187D72958E4F12B4CEDEB0E1292538535834FE91EAB8301",
		"1EDF41B23F38B18C5178CB47CFA75D804F6C07CF2CA9E2453FABDE7E7DDDD8D8",
		"12A925AB62E42776FB9E483040B197AB7777A8353454A36733C76A1F5027AB18",
		"51EDA1F86D2A62121709BCE3BE12DD0E81D8A8CD61F5951CEA740FBDCCB82427",
		"9D73AA660DDDBCF2F376CAB3B2A45DB9210CB74828C1BA7F5403C4B95546DE34",
		"08F4B26D032202EEBE1DC5529C03D65A141DF27F87A2EE24844CACAF5380C442",
		"40B62AD6D4A64C2DABDE1649FFB2AC216583749CF71E531C8BFFEEF6A82A9D00",
		"E9EB28E234D15938F8CB13E7312FF7F3DA62CE0D81369FE2CF13B6A9E3B70C60",
		"998E6B367B43FBF729A702C297E0A89D17041610CA8EFD61A1A9D5B802C9D769",
		"47B49FEEE7E5C1C81F953D4445F8999545E640E6F9AF8D0930FA0D205F3238A0",
		"90BFD95971540E0704CB517D262F0A8C526C0242BCA49A80ACDA2EE4AA06C2A8",
		"D4C8299B5A537AE92716125F7706CDF8B7A7E4C8796E2EE6D3235847419957FE",
	}
	requeueDanglingActions(ctx, mgr, danglingInboundTxIDs)

	spentTxs := []common.TxID{
		"356D59F3211F03C15667470A1AC31255C14FC2840C099F5B5250612C7D07F9FE",
		"15FDE171F250356EBE416D203D5141702B0983ECAA0B270AC9DA2E5C95202C53",
	}
	for _, spentTx := range spentTxs {
		voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, spentTx)
		if err != nil {
			ctx.Logger().Error("fail to get observed tx voter", "error", err)
			continue
		}
		txOut, err := mgr.Keeper().GetTxOut(ctx, voter.OutboundHeight)
		if err != nil {
			ctx.Logger().Error("fail to get tx out array from key value store", "error", err)
			continue
		}
		outTxId := common.BlankTxID
		for _, outTx := range voter.OutTxs {
			if outTx.ID != common.BlankTxID {
				outTxId = outTx.ID
				break
			}
		}
		if outTxId != common.BlankTxID {
			for i := 0; i < len(txOut.TxArray); i++ {
				if txOut.TxArray[i].InHash.Equals(spentTx) {
					txOut.TxArray[i].OutHash = outTxId
				}
			}
		}
		err = mgr.Keeper().SetTxOut(ctx, txOut)
		if err != nil {
			ctx.Logger().Error("fail to save tx out item", "error", err)
			continue
		}
	}
}

func migrateStoreV107(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v106", "error", err)
		}
	}()

	toRemoveTxs := []string{
		"08F4B26D032202EEBE1DC5529C03D65A141DF27F87A2EE24844CACAF5380C442",
		"09A26EFCBB2A7F7CFCA58EFFD9E33D6791E9184AB8E9E69C41CBAA55FA4E61C4",
		"12A925AB62E42776FB9E483040B197AB7777A8353454A36733C76A1F5027AB18",
		"15FDE171F250356EBE416D203D5141702B0983ECAA0B270AC9DA2E5C95202C53",
		"199EAFFEE1E83873C4A35539DB5A209F87C7796AF7BA47C05758C401B6AE72A8",
		"1CD1FF94BD318864E6F5A50D44E4FB3E27378A15CF131EDCEC3367151DE4789C",
		"1EDF41B23F38B18C5178CB47CFA75D804F6C07CF2CA9E2453FABDE7E7DDDD8D8",
		"210088D913A05BCD6166534FD78CE0472C57D8CD4DD6811911C3E4728AD8CC13",
		"2561147F935F6FB962ACE98019507BC3459F09BC3DE27B13FF7642375DE895B6",
		"2DA12C51226E5F7BDEF5FD9087C72957A71B2D9FF0068AB200CDC28CD590C5A4",
		"32EFDD45DF865F5CA8219F78921B930314962DB014AF9FB8B7177339212349C7",
		"33F16A97389F1F91729A140BA2B7A03B30648D3E900674B1A6C09EBC5491F3D5",
		"3495C1DD09808BE80048D1B169855A96509C07C1814FB0CCFB5F7B20314C33A4",
		"356D59F3211F03C15667470A1AC31255C14FC2840C099F5B5250612C7D07F9FE",
		"3733965B49CE9655EAA1AA0AFF7EECE6067448B9BC5C6EA39CDD03CBDF6210E5",
		"391BD7F59BC700D7687F4FCB809D994EB4EAA33C5142115DAAE26623BEDDD801",
		"3920E1FC6362656466D6BB9BF93D754BDFCAAABD0D83EB720D37A35D4483E696",
		"3B471BFCB5D8E839F2DE293CE17F4A9C9C1756D72DD5C897A9FEB25B882DBA6E",
		"3EA40E01E372FF97A42366C64198873EE42D2288F4056AF88A4288E8F6ADF16C",
		"40B62AD6D4A64C2DABDE1649FFB2AC216583749CF71E531C8BFFEEF6A82A9D00",
		"41DCE02268892610CFB0FF0C442C9B701F89A871F8BD1DE0973CF86BC539EC85",
		"42987321CD58E6F174A1FB7703F720CE0B7D78EDD2F2D51FE6EDDD400A4EB881",
		"44A0A75C227C0B5C197071116EFF318DD913798F33CA602F3EE07B6B043ED7DD",
		"469474953796B1E5DE276DA1B41D2EF7D669D5EAAA68660DAF58ACF1CBB66FB8",
		"4755A67A2910562F047CFB3A22A68610B403B49F70BC63479525E92F861AB8C1",
		"47B49FEEE7E5C1C81F953D4445F8999545E640E6F9AF8D0930FA0D205F3238A0",
		"51EDA1F86D2A62121709BCE3BE12DD0E81D8A8CD61F5951CEA740FBDCCB82427",
		"56369450125799237380D7093635342BF99BC2278EABF7CABD8B5DACFB5DAEDC",
		"58F53279C0EB095B012369AF581C981DC79AB517E075204E31F7C4F453AE9159",
		"5968B058110932CA2B0EF181CEAD16B0008BDD2075E84EB2DBA8788C0C391FD6",
		"5AA2917E17476C6A40E67EC7D51B845433A97DB408BDFAB6AF0C2DF628D14CEF",
		"6129474913FDBDDC7C0A40D260DAA581F162033962E617F5B047492917F4CF96",
		"625C4E707AC12244DD657CC0465A280E2B5C64DA37C1F61B70F2DC4269E66760",
		"63E7565BC422ED92693BEB6E4F43254D168199C49F0C154798E31DDEAAAF879A",
		"6AF92FB712157AD24E6369090FBF0EA80ED0096361BBE2167C5C3AD86253E8FB",
		"70506FCE5BA295E5B8DCD2D38885EE1B717A099DF935640D5BA65A3512DE05DA",
		"770B1E9A1152DCED356AC69F421C4374AA81C883690CF1A7DF51E504973EC2B4",
		"77342E2C624EEC3A031EA541498401B60679A46CF793470879EEDE9D95E8B062",
		"77C77406C0EB550F0705838467BBDDA77638BAF2C2BDBE7FA27EAB7421BE32B5",
		"782304A4A0381EA89187D72958E4F12B4CEDEB0E1292538535834FE91EAB8301",
		"7A20AAC7EE7366CB82F46DE3470E7B2E6747AD13A38C8802B4BFC953C9CB58BA",
		"7B150F77ECD44232C242D1C6B0E568DB08E9BD5402701FA2FD9FB6021E687A1E",
		"7E441F57FE6D5297EE7F5E51B5FC7AAE232E015750F73C0B9255CC57D2185888",
		"8096A1316D4BC86CACF76B1F9584A2FAE89660F5B1E5D78CB8716CC2F3C95D33",
		"87766587E59B40ABBA29B3615B96B8A8743C0401B87E3F7B55C1FFBCBA2B70DF",
		"8ABF335AC56A27F028435EC1633474E7CF5CC8549CA418E4B9C69E705E1745B8",
		"8BB5C19CD2CF12BA5FCC505BCC4727155DD5B12200AF81E85176CD3D894417B7",
		"903504658A473094BC517A1435A73EC6331C6FBB9443241E441B29DD3B9C170D",
		"90A4B05A312D048586313CFDA6741B3848E5B7B2B22191DB0ABC1CB9037DE7A2",
		"90BFD95971540E0704CB517D262F0A8C526C0242BCA49A80ACDA2EE4AA06C2A8",
		"96166DE602F8491BB9FBFFDE443FC5A25810CF8B3C76EECB3708D4640F70277F",
		"998E6B367B43FBF729A702C297E0A89D17041610CA8EFD61A1A9D5B802C9D769",
		"9AE32B3D9B7F4325C605B0166B04BF3DB805A54FAEA7180AFF01BAC713960FA2",
		"9D73AA660DDDBCF2F376CAB3B2A45DB9210CB74828C1BA7F5403C4B95546DE34",
		"9E01E3CB16D655CF91FE45477C38265F73C06DEF9684BD91696BEB6635873B0E",
		"A0306B7FCB6A1E626E797E828C79EAD36EB64A1A382BA44444071F15B83F8601",
		"A19B4C3CCA0182EE6508243971D1310E88DA0292C855988B6402B945AF9943A5",
		"A30EA261E2ADC64D115A0FA59A98FF1F060543BFBE44A9DF30FE3E9D7A2097B4",
		"A51A34C375E093E5B4BA8D9F0330FCE0D959A3B1D237F3E40DFFE0A97A65414D",
		"A92CCD142F75EDB112BFBEE7F89AEA3813DFFC6DB54781F19EAD708A5B11557E",
		"AAE34BD6FCE944731AFA09E336B9677F01DAD688D8B16621D951EC874FAB3F52",
		"AFBDBC3D4039EE76A034ED40C5902BA1E887ABC16A3D5A757016AB7DA94FB158",
		"B6730601AF5CFB78E6A040F32BBBCE599F662067D7C3D1D3E825D94CB53FE95A",
		"BB6735BE958AC4C92EF7FD828C39233504E3F4DF2E879AC028924C52A2373FDB",
		"BBF3652682882D05D0B2ACDF8A06ECD1F16CA95877B50AD9EA22A012F0CE22F2",
		"C02BEB8C8A35D3DEF148FD1BEEA1BE74A6E0C1E437CFCE0690342C0D988D7BDB",
		"C3EB617E0DD25AE5AE1ABD5C91CDCDB51435CA515D5FE421462CBA0F37D11EDA",
		"C9AE69BB8982F14CDB7EA9135E0EEA71F9EADD5F9260346302547CC39BAC0ED6",
		"CB0B1034EB9C82D8FB5CEC349FE37C6BE9185EEDB17384294433D360A7A66202",
		"CB390FAD8D646D541046999A2347AABFE25FA73B00382E07C18F62A03C833420",
		"CE27C5A408DD4B30B170A90BB6EC2B04660A35F0ADA107B055812CB668AB6F8D",
		"D2F0BBBE8D17A231A7EF39EF262F368B23C5206096CA1905870F188C0BDFA14B",
		"D4BD757812983E18C839F0C7C071545C53253697C6FF171E479EEDB71D44664C",
		"D4C8299B5A537AE92716125F7706CDF8B7A7E4C8796E2EE6D3235847419957FE",
		"D8039ACA5F751FD777BDDF046064B06962693B6964543FF5EFA65C12A2D76026",
		"D870C04715093BA8180705324F4B5F7BBFAF24D2D9F6FD41825EC3DA0A4848D4",
		"DA80B9FA56213D8F4176D1D81B1FA056EA360CDC103408CF10946B3626E54DC0",
		"E31A6C3943E918C96638F5CDAF558EACE615E43F166D00EC0F97E117F98C46DA",
		"E3BFDD6AAA01B1444DD43DD02F82485C4892FBA10620DD8CB3B8371EE98009E7",
		"E56B1A5410DCA670268BA0EF262BC7C4A8D958E50987EF41A6116B9AE66FFD15",
		"E9EB28E234D15938F8CB13E7312FF7F3DA62CE0D81369FE2CF13B6A9E3B70C60",
		"EE6C4711C360C09B88D399E2000F66EDBC9D88243E977E4DA386575801B6C7BD",
		"F0E8EC18F3C2A264E00BF69139B57B4670735FCAEFD80E4AFDF867A586EEA1C5",
		"F165CD890E63B782E61B497854F6C2E4F12CB1D5BEA22193352586239E513502",
		"F5D933BC96464024C7B176A699C881D8C158D3019673D6E6F4156B1D5D1C2B92",
	}
	removeTransactions(ctx, mgr, toRemoveTxs...)

	// Take the inbound dash into account for the pool
	pool, err := mgr.Keeper().GetPool(ctx, common.DASHAsset)
	if err == nil {
		pool.BalanceAsset = pool.BalanceAsset.Add(cosmos.NewUint(438_32476664))
		err = mgr.Keeper().SetPool(ctx, pool)
		if err != nil {
			ctx.Logger().Error("fail to save pool", "error", err)
		}
	} else {
		ctx.Logger().Error("fail to get pool", "error", err)
	}

	// Sending the amount from reserve to pay out stuck txs
	address := "maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz"
	acc, err := cosmos.AccAddressFromBech32(address)
	if err != nil {
		ctx.Logger().Error("fail to parse address: %s", address, "error", err)
	}

	coins := common.NewCoins(common.NewCoin(common.BaseNative, cosmos.NewUint(130_000_0000000000)))
	if err := mgr.Keeper().SendFromModuleToAccount(ctx, ReserveName, acc, coins); err != nil {
		ctx.Logger().Error("fail to send provider reward: %s", address, "error", err)
	}

	// Send node rewards to each of the bond providers
	type providerReward struct {
		Provider string
		Amount   uint64
	}

	// Rewards getting paid out because first few churns they weren't distributed and BPs not able to claim.
	rewards := []providerReward{
		{Provider: "maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz", Amount: 27727_7226000606},
		{Provider: "maya1tndazzezsfka2wgqm52e5neej9q8jqrxv47h7m", Amount: 5298_8905701016},
		{Provider: "maya13yseu9un5f9gwqgzshjqvsqrxew0hhgm3wjh4l", Amount: 318_5469177912},
		{Provider: "maya1rzr9m407svj4jmc6rsxzsg75cx7gm3lsyyttyj", Amount: 1857_4794818020},
		{Provider: "maya1g70v5r9ujxrwewdn3w44pmqcygx49dx7ne82vr", Amount: 2841_3226310891},
		{Provider: "maya1a7gg93dgwlulsrqf6qtage985ujhpu068zllw7", Amount: 348_6034764209},
		{Provider: "maya1zvfwm65cmp9hufk3g800f7d2ejx7slrl4mgh07", Amount: 7803_7630550052},
		{Provider: "maya14udggus78e9hh2my7uxnn0l470dp9yj5u35l00", Amount: 399_8764937105},
		{Provider: "maya1v7gqc98d7d2sugsw5p4pshv0mm24mfmzgmj64n", Amount: 5992_3775801240},
		{Provider: "maya1qsynvzys9l63f0ljgr7vk028n4yk0eyvjakn80", Amount: 3000_6158005970},
		{Provider: "maya1fex4zs3psv8crn6dhx4y7kwwpuag3e6a3e4tc0", Amount: 330_8698303993},
		{Provider: "maya1gekecuwh3njjefpyk96lgjqhyg9mr6ry99nsjh", Amount: 65_5279824954},
		{Provider: "maya1j42xpqgfdyagr57pxkxgmryzdfy2z4l65mjzf9", Amount: 91_4013210909},
		{Provider: "maya1v7adg32vxmhhhmul98j23ut3ryr8r93sat4gkw", Amount: 159_4375777734},
		{Provider: "maya17lz0x3a58ew6qfc23ts68z7axyj7n8ymwqyxxh", Amount: 21_5222490083},
		{Provider: "maya189r94lmqg3hf6flgjdmjkemneruma38hugxqj5", Amount: 120_0202887348},
		{Provider: "maya14alj79vk3vfejtgjrgdjv38e23dd3vmrukqryx", Amount: 1288_7423785536},
		{Provider: "maya1qq30ur49s9fs2srkt6vfxq5hdl5q8f6e652q4y", Amount: 328_5224134319},
		{Provider: "maya109xtpvrzd3gmgjhrjzxjtkqg0veskh2jpg69p8", Amount: 53104058},
		{Provider: "maya1ay4u99j6mv7rtwl4nnv7er7fs67vpyrrangxl9", Amount: 136_4851533017},
		{Provider: "maya1q9v6r2g8lznw7ljp2tyv8wp8q2hrr37ms7trth", Amount: 25_2293461748},
		{Provider: "maya183frtejj5ay6wg5h5z9nll46z57hh35t3q8ssv", Amount: 728_4610283867},
		{Provider: "maya17pxhjm53l3du57wck0pr8jfjm38kx4xmyjw3em", Amount: 151_4680330032},
		{Provider: "maya1m0cza4vpan5sgtkz9yjsncl50e34k244c9wjct", Amount: 102_8997967973},
		{Provider: "maya1s2yw6uqyyaut3da8rrxtkufmy4pvysm93usc4j", Amount: 5_9299403199},
		{Provider: "maya1cpjhj27r04zz36gt5enl2jrhumhkc7eg4aqrk5", Amount: 99_7375922070},
		{Provider: "maya1hh03993slyvggmvdl7q4xperg5n7l86pufhkwr", Amount: 1090_1452789657},
		{Provider: "maya17cyy84n4x94upey4gg2cx0wtc3hf4uzuqsmyhh", Amount: 262_9731534176},
		{Provider: "maya18h4dzee602madsllymve86up6xj0s2n2tlwslm", Amount: 244_5755098764},
		{Provider: "maya192ynka6qjuprdfe040ynnlrzf26nyg38vckr2s", Amount: 73_7255366477},
		{Provider: "maya1guh3n0c84quc7szq9twmlxk9tk9fac3mmpeftt", Amount: 42_0294703033},
		{Provider: "maya1wgwrnw63tn7gxmh5j5eg057ey4ddeemzm4ws8w", Amount: 323_9881527618},
		{Provider: "maya13w6dqa772ndgpfv05sae7l4sue08eqcd8layc8", Amount: 12_1691150829},
		{Provider: "maya1xq3eaf70pdw4wl8esn0kyuxpnunprs05tgppzu", Amount: 44_4895759293},
		{Provider: "maya1y8a0lgl8r6pfwzu7apal07f75cquznvzl5kmea", Amount: 147_3384200708},
		{Provider: "maya1s89srqv03vuz9pacrtsdedqcdxjlkpsnxl8e8g", Amount: 417_2670401686},
		{Provider: "maya1fert275f6afn8hnjypzhq75f9vrwfy3uej2492", Amount: 62_6298239658},
		{Provider: "maya1lghvak02n32tlrgm4xvj9zmjr4s7fwx8wyethm", Amount: 14_5557440690},
		{Provider: "maya15n93tthvzldqykev5cs4l3utqhg8v0m2tn22z7", Amount: 10_0852951238},
		{Provider: "maya1smu8qs5dqrxuvqkyf5v9zrf7pa94gm7e2naq9v", Amount: 2_7552253200},
		{Provider: "maya1u40lr4a2fm9eftwj05wxx3v3nwejw4s7st8ufs", Amount: 417_2670401686},
		{Provider: "maya16f8kzx474xwu9rr9ah4mxrny5rq2nhy0yjkrme", Amount: 19_0164365924},
		{Provider: "maya1f5um8t8d68pk2np2vklpsxcnu799k5h4lj2667", Amount: 20_5120846911},
		{Provider: "maya1mfw8c2agx7tmdxt5ez3qsqfmyslagxny0sl7w8", Amount: 37_2591628642},
		{Provider: "maya1jzpntepl8ukadpejf5m2fccy6vygssn6llw98l", Amount: 36_5232494929},
		{Provider: "maya1gnl3j76rglvw3yfttl5vpgryl2gd6y9y2kmuld", Amount: 5_6707515898},
	}

	for _, reward := range rewards {
		providerAcc, err := cosmos.AccAddressFromBech32(reward.Provider)
		if err != nil {
			ctx.Logger().Error("fail to parse address: %s", reward.Provider, "error", err)
		}

		if err := mgr.Keeper().SendFromModuleToAccount(ctx, BondName, providerAcc, common.NewCoins(common.NewCoin(common.BaseNative, cosmos.NewUint(reward.Amount)))); err != nil {
			ctx.Logger().Error("fail to send provider reward: %s", reward.Provider, "error", err)
		}
	}
}

func migrateStoreV108(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v108", "error", err)
		}
	}()

	danglingInboundTxIDs := []common.TxID{
		// KUJI queue tx's
		"35EA6C99A16E6334980BA7FDC6FC97A863D13A4B7E01D10EA9DFED5265726819",
		"C2EF8A782F23AFCCF0171C201FE0F70EE0E7A09C3C433EC82658013AC5D526C2",
		"07727D48C95A4F207892A27F723C94DE0643B84DA285C7DBAF47DFBB2F8D1FCF",
		"5C5040FBDCAE945F929A162792DCAA556DC5D76164B9E36F80E05D0A9133C27E",
		"B1568DAE825356B16994DDED2BE3F617F4FE511F48B31774F3D7A35DA2EC1634",
		"B2D81261ED12E2A0714FD049BA8CE1DA43480C49F3623F5CB1CCC32A261088A2",
		"CF998E8483AA9C7121F5300013A0EDE8707B9893456150F8CB2E06CB00454005",
		"0E0474BDB2AD1E9634AF38FAB0D24D379D8F60924E2271E6C2B84B52F2F929EA",
		"8D213C1E3C5ADC149D974905185831D5F66094362A34897894DB2AB8256CA91D",
		"BDD0EEC8BCC6043560EA8D3C85D221BE7C65EE8B4079B7CD0D430075734D8770",
		"11A773C27A30CF4217B137D0342D98CA24C8D5DDD8C60E1A5316D8176878617E",
		"C16BC39E7A63A6EABB202DF0834377FE9842F6CDFD088831E57784E8330F534C",
		"6E281DD6DB9BD448C166627D7AEECF3E72812CB571078E6254888A1E1C57AA5B",
		"426890501A1E5AD99E85B3AF846FB601E707130EAFAEDC2D3B7432E4843E549E",
		"855991ECA50787FB71EF9C2A2476674820CDF1A4A68C200D0345C50A8C9AC3A0",
		"0668A945199574EF3037C228D9AA3366DF9C09402613BC69D30CC0E06E7C1295",
		"5481BE9A943BA530CA0608D5BB8E82D9B5125AC61CE78C7931E6247C61B6C7CD",
		"E1B5BD245EBC6C1A568D08C94F8737FAC3898EEA9DAA80896D58C9D947E070B2",
		"F3C665942C66FDC4A3C29D293FEB5B053294C932F3716445F684F152C73987EB",
		"7916EC08954B2DA559E6A9F51FE8F6CB6C23D5F5E879FCAF3C6580BCCF785EA8",
		"A22C8BCBA695588ACF5E83283326F48CEEDDDF367B132F9DB4D5DB66B14BD038",
		"C79BD8A50DCD2CB0BC7F3EFB035B6D8FC4D75B813EBC36CB52B4C1CC7BF2306B",
		"67F93125F8726902AA17668745CA0F2070E923F28D49E5B9A73136FE2D8995D0",
		"876BEDC2D270C23589F1E8C143EFC5C86E1B0AED8B64A39C30223294A84CED1D",
		"73F2D3DB1BDB8D59ADBD275EA00E0FF6EB997FBA38C66EC291200E25D70318C6",
		"2BB72F4BED36EB707ADEB3BE295B8D69DD2001BD90B0A377587E84A56B6A257E",
		"FFA00E2F26988E51AE7937A723EB29DABD5F36F651BEE0E633A70517118E1402",
		"2F8A29325B2F262C1AEE11692BC5C34F662671377FF5EA8B433710114429D5AF",
		"41B24E943899F44E8DF66E8D12434439D36A548C840727E6A98EA4A258FA792F",
		"3B54CE1184C42991E286CF283942BFA2B190850E9C7F14A81E130580599BE567",
		"A6118444880F187C78283471E9C00B8387021F6C58CBA553B3F60A9A88CF994B",
		"F1983522FA5DA3C6EC9EDB513BB22CBBC469ED9086CA8F7CD1E78CF954C327C7",
		"05375C49190B17D76670A38B55D9DC8BBEC973F1C958B7063BE1439AF0589C35",
		"7533DE41CC57CD64A4337F3A2AA4EE755E47AC7F87748227ECA3D7EA6B60F306",
		"5C8264D030990E1AF18CB1E7C804C34E76D5C36A2892B81D3A1196C1B67EB8C2",
		"E8A7CAF2669D64AA7A53E56E3F9B65F606BA5DEE66B76E6B47DB4779CFBCFF57",
		"E27342795F75B8DCD803E5CE41EA8F431A7DC9C1FB0A1D57D8E31DF433BDC12C",
		"C77643F49ACE542D1E1FC15DA080AB61FCF5923405A0D456BC0C6832D123AE46",
		"C5C8CC605ACFA3CEB4E9D7C2AB7F02D4864E684EBEDFC559B730A579F27B09EB",
		"3812A75DD69D06D9E04A85F28DF32B94EAE1FB3786CA2029540CF5E6EFE749BA",
		"F2EF82E480A1B96EF15B4CB8CBE8C6C5CE2A6E3224BD5DDF27A924F7A7C5162C",
		"484A7B1BABCA27092442F6D45C3FE4E9D86E29857A2FF4FE51EE7FBFEB8D2843",
		"70675963939D19384AEC8BB88BBC3E05E28712691F907BCCDA0BBFA0995058EC",
		"220F7F531D372B182195050C3A409F14891C2872C98C7A10629B07C09C485AD4",
		"ADC8FBB736F45FD7B8E97B4ABB2210611A8E09562DD13A5989DA0E9CAC506490",
		// Refunds consensus failure
		"A07B4C55031F3B9FED8D3BF938740981149A41274CA2B33F54DB83091A75D635",
		"4AB40F14C011C733A0817E65EB5B6395D206C35D56C7912F9EB8207C49AEB20E",
		"E4AA05181989014F462A1BF969D77FCCA2D5E3C7106A895057D80B3E9733FBFA",
		"4C98B47D67ACEB5515BC39A3F053D8BCDD3FE469ABAD7AE042894261868A30E1",
		"FC27513B337E258CC9237638F3F22A370498B659862C39AE9A02E09E9674E8C1",
		"EC1BDC5295F6B98EBD9D472082D15E35319DF9A3FD0088F5CB4E528AA0062A74",
		"0BA546F56B63B131453F1F441D2B200511A6C88CFCD9F196A2759055727EC7A7",
		"5B1F7833601ED07F4DDA87B7A1CD306B981520AE6E934C0D2B60FAE790C22725",
		"A0702FDCB62879B183EC7D565C99BCBFE31653391E3AD181F6D46C84AE35E001",
		"DCC40487D914DB53E7B049D7F3A825743E97C39A6A07687087937AB24475E3EA",
		"C2D445428DCDB2CEEB66770B9E1E462FAC52979AC08E833696E13607E4CD6EAA",
		"8FCA78BD4B74416F00B5505F02EBAF83340F355FFD0729D68BF5E14F08B679E3",
		"3DC8D5E627AF4ACEADBD389EBEB7310CE868F152704B49073DF6C31ECD783E7E",
		// KUJI dropped txs on 08/Dec/2023
		"D7B7AC85DACF35B9FA3F912946A1977398C8F085FAD6C0FC2F13036C726A952E",
		"9F1E084FD5A7EFD9B6A7C304944DB266300C28D91D006ECB0C7E113755D574A0",
		"1A88CDB0AED6FE0047AD1CD27F08002039D584BD02BFE1CF3CB1FF7DACFD4C99",
		"EA33A352BAA5397AFB1F0EC1C30A8FC6B937D49704EFE46436BEF06B9224E99D",
		"99A7F15CF990E8B941A4960CD3E1FE3D8C6F1356891606A4F59DA3C221CA2412",
		"52A5093AF70F6B497E757AB9BE46CF24007C4DD28386EE1BB98AF8B6FD07B4E5",
		"5FB7014A1D750B8D33F22E74528028AD8EBCEC298378793FB99BB8282800BEAE",
		// KUJI dropped txs on 14/Dec/2023
		"2676A985928181376F147C5FEFFA5723B0E001A9C799256B0286AC97FDBED9C4",
		"DD410481BCFEDB5B54298948903EBCB61A55FC610A72DB9AFA92AB4430A43AAE",
		"10F87DEC374B1091C3F9B52735DDA6D0BD262284220952BAAE5FE8F5B007824E",
		"7702783493C209EE0E5879308D93523FBF84B857C39F35BCC05284CECE59ED52",
		"EE95B8AE9CB9D0413348A81C1B30FFA064B8B7CDCF9E9131BDF898707A9F44A1",
		"E395D6A8A748B2FCE7B4251ACADE8B9A4F298F2E93D3560D41C5BA28EE6032BE",
		"E9DD4695E613CE9AAEF2F7BF3BA17647961D2CBADFED08B0DEDA5537404F16C3",
		"0D02DD74026C1EA90FFAC809F811AA349207BEABF2454DA4576FEBD1F5EB7C44",
		"94B17D1B4A9F81E595C6515D037592971F872FFCEB1B525C7EFE1ABBC08EA789",
		"0CAF6DCAF0F9BB2583E639C4DC80B8DD67B0632917EB0DD3B75CA1A27483BE4F",
		"568905978966B1296AB21616479BB9C9C56BE84107F9AA081E04424A0E87DE4F",
		"1B015385AA71EEBE675E3046522E689C9A7F3CE09B1548573CB428A381CD5DCC",
		"46F1A6B6BB222F29DFDAB39BF08BE4357EB0A926CBD07B7252C6C0270A34ED85",
		"60660C31363CD87A6BED81F59D81035FFE274904E457B30CD464C0CF92BC0BC4",
		"C8DB11D3B0A92BF661370947BC23AC2CE2037C4470CD53713D9A0005A35A742A",
		"3892484F2D8F63979148678244622FB17C1F2C8C7324A0D51A95C07FF69B56CB",
		"6219176D387DF508DB80A5DA97F902B396041B936CAA99449F778D991D27E9F1",
		"059F759A98FC1B9AC8D088575EF34C1C718FB98094312FD723523456A2DB27C4",
		"8356489F25D45205C9894F26D4203FDE054E79EFB124272844FF677895A77B8F",
		"AFA4BB32162239BE0BE36C22870E7660C4AF6E6B95A92BE013D4D38BC6C6325C",
		"5EAFA060D7B171DAA2B5D718AFA119313A91C9CE5873CB1A793E18469BD8F591",
	}
	requeueDanglingActions(ctx, mgr, danglingInboundTxIDs)

	// Unbond bond providers from node account
	unbondAddresses := []unbondBondProvider{
		{bondProviderAddress: "maya1f5um8t8d68pk2np2vklpsxcnu799k5h4lj2667", nodeAccountAddress: "maya1v6lt70lqkhxhftlpx26d52ryzc6s3fl4adyzuv"},
		{bondProviderAddress: "maya1gnl3j76rglvw3yfttl5vpgryl2gd6y9y2kmuld", nodeAccountAddress: "maya1v6lt70lqkhxhftlpx26d52ryzc6s3fl4adyzuv"},
	}
	unbondBondProviders(ctx, mgr, unbondAddresses)

	// Send node rewards to each of the bond providers
	type providerReward struct {
		Provider string
		Amount   uint64
	}

	// Rewards getting paid out because first few churns they weren't distributed and BPs not able to claim.
	//
	// Address that have been already paid because of:
	// Store migration v107 or
	// BOND/UNBOND rewards distribution
	//
	// maya10sy79jhw9hw9sqwdgu0k4mw4qawzl7czewzs47: 0
	// maya12amvthg5jqv99j0w4pnmqwqnysgvgljxmazgnq: 0
	// maya1gv85v0jvc0rsjunku3qxempax6kmrg5jqh8vmg: 0
	// maya1q3jj8n8pkvl2kjv3pajdyju4hp92cmxnadknd2: 0
	// maya1vm43yk3jq0evzn2u6a97mh2k9x4xf5mzp62g23: 0
	// maya1gczk5e3slv35y35qyw0jc6jwudm2jg4ztscc5x: 0
	// maya1xfuxhzj2e63yd37z87vmca25v5n8486an9yde2: 0
	// maya1s89srqv03vuz9pacrtsdedqcdxjlkpsnxl8e8g: 0
	// maya189r94lmqg3hf6flgjdmjkemneruma38hugxqj5: 0
	// maya1cujc2sj8avcfnyrxj9grwlcfhyflpxchvq65cg: 0
	// maya14alj79vk3vfejtgjrgdjv38e23dd3vmrukqryx: 0
	// maya1jzpntepl8ukadpejf5m2fccy6vygssn6llw98l: 0
	// maya1gekecuwh3njjefpyk96lgjqhyg9mr6ry99nsjh: 0
	// maya178wqee3z5y9fyqxkdud4rxyldlytq9d6xcs823: 0
	// maya1hh03993slyvggmvdl7q4xperg5n7l86pufhkwr: 0
	// maya1rzr9m407svj4jmc6rsxzsg75cx7gm3lsyyttyj: 0
	// maya1v7gqc98d7d2sugsw5p4pshv0mm24mfmzgmj64n: 0
	// maya1tndazzezsfka2wgqm52e5neej9q8jqrxv47h7m: 0
	// maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz: 0

	rewards := []providerReward{
		{Provider: "maya1zvfwm65cmp9hufk3g800f7d2ejx7slrl4mgh07", Amount: 26725_3597627890},
		{Provider: "maya183frtejj5ay6wg5h5z9nll46z57hh35t3q8ssv", Amount: 19582_1211035418},
		{Provider: "maya1m0cza4vpan5sgtkz9yjsncl50e34k244c9wjct", Amount: 9633_7253828425},
		{Provider: "maya1mfw8c2agx7tmdxt5ez3qsqfmyslagxny0sl7w8", Amount: 9550_5124319582},
		{Provider: "maya1qq30ur49s9fs2srkt6vfxq5hdl5q8f6e652q4y", Amount: 7329_8887338551},
		{Provider: "maya1s7naj6kzxpudy64zka8h5w7uffnzmhzlue4w3p", Amount: 6488_7055306094},
		{Provider: "maya17pxhjm53l3du57wck0pr8jfjm38kx4xmyjw3em", Amount: 5759_6226579937},
		{Provider: "maya1fert275f6afn8hnjypzhq75f9vrwfy3uej2492", Amount: 5635_7251152517},
		{Provider: "maya1guh3n0c84quc7szq9twmlxk9tk9fac3mmpeftt", Amount: 5518_1294840549},
		{Provider: "maya1g70v5r9ujxrwewdn3w44pmqcygx49dx7ne82vr", Amount: 5097_3527408652},
		{Provider: "maya17cyy84n4x94upey4gg2cx0wtc3hf4uzuqsmyhh", Amount: 4397_8618563702},
		{Provider: "maya1cpjhj27r04zz36gt5enl2jrhumhkc7eg4aqrk5", Amount: 4323_0939760062},
		{Provider: "maya18h4dzee602madsllymve86up6xj0s2n2tlwslm", Amount: 3916_3506275542},
		{Provider: "maya14udggus78e9hh2my7uxnn0l470dp9yj5u35l00", Amount: 3883_8904268868},
		{Provider: "maya1qsynvzys9l63f0ljgr7vk028n4yk0eyvjakn80", Amount: 3290_5472396829},
		{Provider: "maya1ay4u99j6mv7rtwl4nnv7er7fs67vpyrrangxl9", Amount: 2326_7311031468},
		{Provider: "maya14dsp7ujkrxqzsv2h2x68ypaeevmg4r5z7500c9", Amount: 2273_5110342539},
		{Provider: "maya14sanmhejtzxxp9qeggxaysnuztx8f5jra7vedl", Amount: 2163_2951513050},
		{Provider: "maya1lghvak02n32tlrgm4xvj9zmjr4s7fwx8wyethm", Amount: 1783_8529169711},
		{Provider: "maya1v7adg32vxmhhhmul98j23ut3ryr8r93sat4gkw", Amount: 1352_3710961279},
		{Provider: "maya15n93tthvzldqykev5cs4l3utqhg8v0m2tn22z7", Amount: 1222_4490527092},
		{Provider: "maya192ynka6qjuprdfe040ynnlrzf26nyg38vckr2s", Amount: 1154_1855063351},
		{Provider: "maya1u40lr4a2fm9eftwj05wxx3v3nwejw4s7st8ufs", Amount: 1091_9836106714},
		{Provider: "maya1q9v6r2g8lznw7ljp2tyv8wp8q2hrr37ms7trth", Amount: 943_1758010293},
		{Provider: "maya1yk4xsaye2m37ytgzulzpr5ajvhvqhg68rpw7ff", Amount: 884_5660977299},
		{Provider: "maya1fex4zs3psv8crn6dhx4y7kwwpuag3e6a3e4tc0", Amount: 861_5494393883},
		{Provider: "maya16f8kzx474xwu9rr9ah4mxrny5rq2nhy0yjkrme", Amount: 835_7844832722},
		{Provider: "maya13yseu9un5f9gwqgzshjqvsqrxew0hhgm3wjh4l", Amount: 780_1025015833},
		{Provider: "maya1j42xpqgfdyagr57pxkxgmryzdfy2z4l65mjzf9", Amount: 727_1695244006},
		{Provider: "maya19jqjqnc7hmvrfez8p5z2tcfmfmq9k5z3wm0rq9", Amount: 629_3112263003},
		{Provider: "maya1a7gg93dgwlulsrqf6qtage985ujhpu068zllw7", Amount: 625_3970832338},
		{Provider: "maya1s2yw6uqyyaut3da8rrxtkufmy4pvysm93usc4j", Amount: 555_1752125982},
		{Provider: "maya1g7c6jt5ynd5ruav2qucje0vuaan0q5xwasswts", Amount: 525_8771743671},
		{Provider: "maya1smu8qs5dqrxuvqkyf5v9zrf7pa94gm7e2naq9v", Amount: 333_9146989413},
		{Provider: "maya1xkdt3ld8xtlfpztdp0k05tmf9g3q622lmahjr2", Amount: 311_7886647361},
		{Provider: "maya1y8a0lgl8r6pfwzu7apal07f75cquznvzl5kmea", Amount: 309_9920691612},
		{Provider: "maya1szmq6kkplsqn7k8lwsm6xajxzgvak0gjvm8c8w", Amount: 260_6258748096},
		{Provider: "maya1ewz79pg6qylpk0p98yzr6jhv23s4wrn0jcnard", Amount: 242_8304775521},
		{Provider: "maya1gnl3j76rglvw3yfttl5vpgryl2gd6y9y2kmuld", Amount: 239_3588509926},
		{Provider: "maya1pf7gg2h9kdq7zuj58r7wk8py99awwj9lwvchdx", Amount: 233_7043832402},
		{Provider: "maya1m7xnnkkrk7e6aa4eq3yndy4nlcre037xnf3zjz", Amount: 224_2073520353},
		{Provider: "maya1f5um8t8d68pk2np2vklpsxcnu799k5h4lj2667", Amount: 217_5743877338},
		{Provider: "maya1h64fpu5uwmzku4xynfc6sevqfpjxp4y36a4t00", Amount: 216_0778531297},
		{Provider: "maya14u40pul8pgpuk42k9502jq5r3wfrpnv9ly8e2j", Amount: 216_0385259062},
		{Provider: "maya1v54s0rwazm5k3ywhaz5rvwnneuccr7rtmqm5yz", Amount: 213_7449170907},
		{Provider: "maya17lz0x3a58ew6qfc23ts68z7axyj7n8ymwqyxxh", Amount: 185_9191116014},
		{Provider: "maya1sclplk79vvlakl8u54r0gr622jfuwar0vfl2l7", Amount: 173_4379838993},
		{Provider: "maya10sdhv0cn0fsfgax6vpzv9pwy8r5872hw3h4tuh", Amount: 169_4603734918},
		{Provider: "maya16k0al0fsslhx8j5cjsjsv4ntmq45sgew8waryj", Amount: 166_4425338415},
		{Provider: "maya1xq3eaf70pdw4wl8esn0kyuxpnunprs05tgppzu", Amount: 162_9076206105},
		{Provider: "maya1c6qrsnstl9l0wtc3fazd6jrfppshs6jk2myeky", Amount: 129_6562049931},
		{Provider: "maya1x64thscxsl39pun3lqzwhge890tvhvgd36c5gs", Amount: 122_3416961124},
		{Provider: "maya1hkqc78uhuc4z8qtt3qjsdn0u7348t2hhlgyzh9", Amount: 119_0618062398},
		{Provider: "maya1vu37n7h7mnk0uxakye2vhh2z2k5cehf6v2lk3r", Amount: 105_5831448383},
		{Provider: "maya18p22jfv43weeyznqg0h9f6dh3adnpj4nwch8hs", Amount: 100_2736412691},
		{Provider: "maya1v7jsyf94rnfdx5v0xjxn5c8vdsyvmym0aegl7k", Amount: 91_0781175605},
		{Provider: "maya1tdp957gs94j7ahgd6cemlunhrwd27l39e52l6l", Amount: 90_9706270720},
		{Provider: "maya1aderqdry6m6vr4qtzkpe5n36xefemfph79pv4a", Amount: 72_2430923098},
		{Provider: "maya18jtxyr7seydqq6q2enhq3c6hx6zc4s8y440swt", Amount: 72_0700051859},
		{Provider: "maya175dn4q74ztt7wzf2n5u0nqkmfvda5sc627vtvd", Amount: 70_2985614795},
		{Provider: "maya1xmn5ecq45fasyt7xqm8nefg8fvpf0w7zqtn2tq", Amount: 66_2528894056},
		{Provider: "maya1nv96km7hgmv76rsjcjj5qmx5ml53alf9r8fy22", Amount: 64_2201125917},
		{Provider: "maya1swcvf06tsytaalk7y6t3urnwyv435gu8fly77g", Amount: 58_0278367469},
		{Provider: "maya1ccf7rs4z6y2spvpmdf7v66v7xy2rd8dye7jhrr", Amount: 55_3287635573},
		{Provider: "maya138cjvf52rr7v5zp6s2gemu0m9wx593juprpgnl", Amount: 38_2125074036},
		{Provider: "maya1f8j08d6p7pqtuhjzcm9297gq4kvhv5lz4p5pma", Amount: 30_5106110499},
		{Provider: "maya19z4xlhxp6hkqe4mlfmqwsnjahrpa3ycjflqczc", Amount: 30_1740875619},
		{Provider: "maya13w6dqa772ndgpfv05sae7l4sue08eqcd8layc8", Amount: 29_8000796134},
		{Provider: "maya1xrn6rw99ncj0qxflwtmvjeuf4kkwuwja4xpwhv", Amount: 27_4001705664},
		{Provider: "maya1wgwrnw63tn7gxmh5j5eg057ey4ddeemzm4ws8w", Amount: 18_5817869273},
		{Provider: "maya1y6lk677q4gdy75qm5x3q4t0sxvx40r8n2kcc4s", Amount: 17_8549597986},
		{Provider: "maya1g286wstwf4vqmegj5324p58gxmy7mnmha80hgz", Amount: 17_1346960497},
		{Provider: "maya1vtzdhyl9sfxh965euupyawn2ql6aa3ee37wz8l", Amount: 9_3962917655},
		{Provider: "maya10n2xw02y4wvv64qulnhmgjdryktzq3nhd53f6x", Amount: 6_5444411103},
		{Provider: "maya1adkthl5cd6h4atrdvxt7tp9xnwu3xpn89c7flu", Amount: 9688364232},
		{Provider: "maya1z4dyge20n7c6g87txma7lv8qmmzvluv2crn8pl", Amount: 8270205310},
		{Provider: "maya109xtpvrzd3gmgjhrjzxjtkqg0veskh2jpg69p8", Amount: 5811384939},
		{Provider: "maya1ha4ypeghxhtdu63dqhhkspqcu4s7375kc3ch4u", Amount: 5735026761},
		{Provider: "maya1nwe0vs65myamknwehgr00r5t2afrlpn26du4vt", Amount: 4822957294},
		{Provider: "maya1kzd9fj58g9exxt44lj8sfzuvc94tsrr2v4gv6g", Amount: 1792051180},
		{Provider: "maya1jttfwrve7mcjfnhnsavpnfzeql4mr5mjns0wpj", Amount: 1250657295},
		{Provider: "maya1ajzlu2p2mnecl6q739fn7hsctlwxyqdulwsslg", Amount: 1210922557},
		{Provider: "maya1ngzyvjtr2xeh4gesxj4wtgl9jxgp2jf3fueah6", Amount: 1094496407},
		{Provider: "maya1kgma45rn0qs22pd45smp2pakng7qz42d2mxmch", Amount: 38386732},
		{Provider: "maya1gmf8lt6ddlq3f0skq77pdskfs4cjz4wcc2s82y", Amount: 14303230},
	}

	for _, reward := range rewards {
		providerAcc, err := cosmos.AccAddressFromBech32(reward.Provider)
		if err != nil {
			ctx.Logger().Error("fail to parse address: %s", reward.Provider, "error", err)
		}

		if err := mgr.Keeper().SendFromModuleToAccount(ctx, BondName, providerAcc, common.NewCoins(common.NewCoin(common.BaseNative, cosmos.NewUint(reward.Amount)))); err != nil {
			ctx.Logger().Error("fail to send provider reward: %s", reward.Provider, "error", err)
		}
	}

	// Manual Refunds done by Maya team, these are the in_hashes of all manually refunded txs:
	// A62B2036B49D3CA8F8A7FB8A5041BFE21AB4E0CFEA57A6FBAB383FAC84A98911
	// 49B54AD04019CAC6907242D687CA9ADF6BC4C5C69D4EA0C91CE0C9ED76225593
	// 6C0A39FEB57C750FA284D09709E0CCFB0DD30FFDED2067D380BEF8499EE23B51
	// E948D68957DDBEE943723F18DEC8A5A5E66358B0619B894A9C5C160B28EEBB3B
	// 7DC12CA54AC3BFC1EA5177D97C68D32C5C5F8FC344BC92299A151A92E1A6E3D0
	// 17454BAB890705F32C1706A696C16B05464EEAAA38F1BCA915CA7E31BE5FB55B
	// 282A9C93E2351CD22FFFD63C976E2938155F998E2A6BA560FE457A0C010983B4
	// 0AEEEA9FC6E615144C67DE33DEC518631616F58C3397F9C33A82542FB9F7F4CF
	// 548DA2CB06BEA4A7814A389C7E01E30CA7077CEB11B8B95FB1DE21C987C9CDED
	// 140CB4F389366B4A47ECD05EBBD2CB9C051D1AB9766B030F926C4C7A85F8009C
	// 40B0EDF61AA5B22500B2AF2BA80B91ED92633D4797E02E40CEE2D95E99A4CC11
	// E41E7AC9E4A2AE3EEB3B8096CF7B0C4044CF4972029EA68ECC883D50E79E4942
	// 7FA5741EF1D4A653ED3FDB87BC1C167674C17310B9A2B6686365791830F1A788
	// AC2406FDF0C3EB99C3D2D5404587670A86F92C9D9C3F643ED5C503EF5BF46E6E
	// 332E53DA379486730EBF44992AFB8EEBBA93D14F950C0D85AC414A61A692A283
	// F61FBE9EA534D7B5D0B88ED8DA9C926E5DA0DD85F045D8B04B4DE23B9D7D0993
	// FC27513B337E258CC9237638F3F22A370498B659862C39AE9A02E09E9674E8C1
	// 5DBEA8B5591593F03FD9A16D26246EE8DEE581A9798E7F0CA002060604B30AB8
	// 62193BBC0CF9559A8A115269A0903D1B0E9779C5AF9DB6E7FE9A1620BDA65563
	// 5A139A466370218C956C7B9E893E471F235BA6123D51FA8EAFC50BA78D965262
	// 9791E51AF7B54C204B8FE8BE50D98CE0046B4D6BD00A0634A696393A6D95AB27
	// 48856566752CF7082A1ABE3C4452C6496BEEBBC64719CF6222DB3BD5F1D41A5E
	// 49361D535CBDDC2DA954369F23D69147431ECCD3102AB0C0A1FB2651D63A3752
	// 2D3EAC94FB828518B38C7FF122C29B826BC03AF372568ABC3621A4C6A048531A
	// A07B4C55031F3B9FED8D3BF938740981149A41274CA2B33F54DB83091A75D635
	// 7032A84A54D38A4F3FA9A35875848798953472EC5DD24FF232DB5E5DFE906D46
	// 3EA5E2D4E3BA9F5FDF670BA44010AA10D3DD995A6A00EDCAAAA7490901EDA9D6
	// D237147312F23FA85FD207948E24594093AFB883BD6FF21CAC828F1734E7048B
	// B997BBE82DC1654267463CA03C49A195609DDC9D63CFED9608458A44EB808D45
	// 59AFDCDFA99A9194D76CA15957CF1676C11373AC67CEB600AA4FE239EE0CDE75
	// 17DB507BCA325199E4D0F5A238348073743984FE2578A395B0437449BDA2B5E4

	// These are the txid's of all manually refunded txs, or out_hashes
	// https://finder.kujira.network/kaiyo-1/tx/0063EF3F70AD7325AB8756F2CBEA39C4C387FF86EC8B07C00D9F5FDD95D63449
	// https://finder.kujira.network/kaiyo-1/tx/12A46622697E00E67140FEBC124E92A516A29768C844B3B79CFD6E41BE0FC3F9
	// https://finder.kujira.network/kaiyo-1/tx/171FF77979A923898E1648C9FCB0CC30459632AF8EC89175A8E97144D6C5F053
	// https://finder.kujira.network/kaiyo-1/tx/44F321B54C26F49C39D6E465025453238CCAAFCA7857C89BC3740EAD844A8169
	// https://finder.kujira.network/kaiyo-1/tx/4767322ACA41F871FAF4560FB26C4DEA964216ED5460AE789C27071F848C2400
	// https://finder.kujira.network/kaiyo-1/tx/84C62A4902DB6C7AD0BC456FDD4645482EBAB999687150C06967C1CF68F7036A
	// https://finder.kujira.network/kaiyo-1/tx/85E9644E6D6D321CD235D3F15B595E3AF7A28AEAEC81FCCC7F33C31261748765
	// https://finder.kujira.network/kaiyo-1/tx/8B791123AD76BD05F33F130AA197EF020B39BED0233F715D9D8B609E04964272
	// https://finder.kujira.network/kaiyo-1/tx/92DA2232C9D06C46354CFF616CB05CA5D27BA39214AF11A5111BA21C98828F17
	// https://finder.kujira.network/kaiyo-1/tx/C458CC31B6908D86271E3D1090B9590073BD57A901361DD8A1EEEBF11B0AFC0C
	// https://finder.kujira.network/kaiyo-1/tx/E6D61FEDF8925099B4D489BFC92CE52FE64C275208311D7367A774F047851754
	// https://finder.kujira.network/kaiyo-1/tx/E6D61FEDF8925099B4D489BFC92CE52FE64C275208311D7367A774F047851754
	// https://finder.kujira.network/kaiyo-1/tx/E6D61FEDF8925099B4D489BFC92CE52FE64C275208311D7367A774F047851754
	// https://finder.kujira.network/kaiyo-1/tx/E6D61FEDF8925099B4D489BFC92CE52FE64C275208311D7367A774F047851754
	// https://finder.kujira.network/kaiyo-1/tx/EA2294FCC7E1C8BFF8E2692011293EA02BD79FE73CE3617E79CF96049101A60B
	// https://finder.kujira.network/kaiyo-1/tx/F7EE8C3FF401C5F70E536F4255B815A2965F826975312F3DBC241587C630B137
	// https://finder.kujira.network/kaiyo-1/tx/F964F5C87EF1C97EBC0B9663A745BB1E8BB2005CC22F0028849E77EA6D5E812E
	// https://runescan.io/tx/6128A5AA0882F994E101CB7792EA6779AADA6C15AF6DE734670DA8144859176F
	// https://viewblock.io/thorchain/tx/136B9EB5B7FF520A62DAFCBEBEFAA262B42BF5520967970A580057418E55A84B
	// https://viewblock.io/thorchain/tx/136B9EB5B7FF520A62DAFCBEBEFAA262B42BF5520967970A580057418E55A84B
	// https://viewblock.io/thorchain/tx/136B9EB5B7FF520A62DAFCBEBEFAA262B42BF5520967970A580057418E55A84B

	manualRefunds := []RefundTxCACAO{
		// Total refunds:
		// 38,500.00	CACAO
		// 13,360.00	KUJI
		//  5,231.00	RUNE
		{sendAddress: "maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz", amount: cosmos.NewUint(151447_9545000000)},
		// Kayaba ImmuneFi bug bounty for TSS vulnerability: $38,500
		// https://etherscan.io/tx/0xdae2f4f89726f8d71a2802958dc80e30edf63aafcb725c8396c26f64d756793d
		// https://etherscan.io/tx/0x578024d6aac35ba5985989ee2b80ecde9484a80b189cc2649f52ed9c6d2a1633
		// https://etherscan.io/tx/0xf596f21b2717cc1b1730f3f52fa95af3aac1c9b607eaf2b63e17de484705476f
		// https://etherscan.io/tx/0x8980713f6dbe62b4a1c971ffd26806698603e7733bb1bc7f4c0373f5ee6a7df7
		{sendAddress: "maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz", amount: cosmos.NewUint(39772_7272700000)},
		// From Reserve
		{sendAddress: "maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz", amount: cosmos.NewUint(230000_0000000000)},
	}
	refundTxsCACAO(ctx, mgr, manualRefunds)

	// Send CACAO due to invalidad memo (not specifying kuji address or using maya address instead for non synth asset)
	swapKujiFail := []RefundTxCACAO{
		// Tx Hash: 2E9E6A8BC525635892E1809DC5BBAC9FE7906A41AA821B174DCFEDF130042C32 memo: =:kuji.kuji:
		{sendAddress: "maya1xv2tqx22t4666awa2eyu4uayucxwyy6jk05yxn", amount: cosmos.NewUint(60000000000)},
		// Tx Hash: D754704FEEAA474F27665AB7D19351543E7BC4AFC5AA1F07453B6ED36717535A memo: swap:kuji.kuji
		{sendAddress: "maya1a7gg93dgwlulsrqf6qtage985ujhpu068zllw7", amount: cosmos.NewUint(27700000000000)},
		// Tx Hash: E7E298C775F1480027907ACAD29F6A7E872D633F4EFBB1E28FE16577BA8D5257 memo: swap:kuji.kuji
		{sendAddress: "maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz", amount: cosmos.NewUint(46560000000000)},
		// Tx Hash: 2C64F982BB6FE20EEC54C3F05E277EE7EFC1762CB43E50FA3C047B80B34481B3 memo: swap:kuji.kuji
		{sendAddress: "maya1m3t5wwrpfylss8e9g3jvq5chsv2xl3uchjja6v", amount: cosmos.NewUint(79310000000000)},
		// Tx Hash: 59399A6F94707513314DA42B8D0490A2C9C7F4395FB60385F3C87B827A541484 memo: =:KUJI.KUJI:maya1kh5lvr8msnvwpgl4sdk8r572d29dtvw3wq5j69
		// Using WSTETH = 2577 and CACAO = 0.74
		// (0.20443 * 2577) / 0.74 = 711.9136621622
		{sendAddress: "maya1kh5lvr8msnvwpgl4sdk8r572d29dtvw3wq5j69", amount: cosmos.NewUint(711_9136621622)},
		// Tx Hash: 99339C24ACF9C1EE3239F3F0EA11A050CC1F8A39923407DAA1ECE0F3F55E369E memo: =:KUJI.KUJI:maya1sqj6zr2wy762rg9ugn7l3s599xczl4uvetv4z6
		// Using ETH = 2233 and CACAO = 0.74
		// (0.15 * 2233) / 0.74 = 452.6351351351
		{sendAddress: "maya1sqj6zr2wy762rg9ugn7l3s599xczl4uvetv4z6", amount: cosmos.NewUint(452_6351351351)},
	}

	refundTxsCACAO(ctx, mgr, swapKujiFail)
}

func migrateStoreV109(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v109", "error", err)
		}
	}()

	// Send node rewards to each of the bond providers
	type providerReward struct {
		Provider string
		Amount   uint64
	}

	// unpaid BPs from 2023-12-08 to 2024-01-25
	rewards := []providerReward{
		{Provider: "maya13yseu9un5f9gwqgzshjqvsqrxew0hhgm3wjh4l", Amount: 586_7249227958},
		{Provider: "maya1qq30ur49s9fs2srkt6vfxq5hdl5q8f6e652q4y", Amount: 1186539066},
		{Provider: "maya109xtpvrzd3gmgjhrjzxjtkqg0veskh2jpg69p8", Amount: 1162261524},
		{Provider: "maya1qsynvzys9l63f0ljgr7vk028n4yk0eyvjakn80", Amount: 2105_2928566590},
		{Provider: "maya1fex4zs3psv8crn6dhx4y7kwwpuag3e6a3e4tc0", Amount: 310_3463000064},
		{Provider: "maya1adkthl5cd6h4atrdvxt7tp9xnwu3xpn89c7flu", Amount: 1451832905},
		{Provider: "maya1ngzyvjtr2xeh4gesxj4wtgl9jxgp2jf3fueah6", Amount: 150846361},
		{Provider: "maya1z4dyge20n7c6g87txma7lv8qmmzvluv2crn8pl", Amount: 1287912860},
		{Provider: "maya1gekecuwh3njjefpyk96lgjqhyg9mr6ry99nsjh", Amount: 110_0299240344},
		{Provider: "maya1j42xpqgfdyagr57pxkxgmryzdfy2z4l65mjzf9", Amount: 362_2294826144},
		{Provider: "maya1v7adg32vxmhhhmul98j23ut3ryr8r93sat4gkw", Amount: 1019_6008344068},
		{Provider: "maya17lz0x3a58ew6qfc23ts68z7axyj7n8ymwqyxxh", Amount: 79_7492876313},
		{Provider: "maya1ay4u99j6mv7rtwl4nnv7er7fs67vpyrrangxl9", Amount: 1348_8535910543},
		{Provider: "maya1szmq6kkplsqn7k8lwsm6xajxzgvak0gjvm8c8w", Amount: 554_5526331001},
		{Provider: "maya16k0al0fsslhx8j5cjsjsv4ntmq45sgew8waryj", Amount: 383_0590203067},
		{Provider: "maya1q9v6r2g8lznw7ljp2tyv8wp8q2hrr37ms7trth", Amount: 127_0409512208},
		{Provider: "maya183frtejj5ay6wg5h5z9nll46z57hh35t3q8ssv", Amount: 2584_1288629325},
		{Provider: "maya17pxhjm53l3du57wck0pr8jfjm38kx4xmyjw3em", Amount: 779_1406136992},
		{Provider: "maya1zvfwm65cmp9hufk3g800f7d2ejx7slrl4mgh07", Amount: 2610_1017855368},
		{Provider: "maya1m0cza4vpan5sgtkz9yjsncl50e34k244c9wjct", Amount: 1173_7549946213},
		{Provider: "maya1s2yw6uqyyaut3da8rrxtkufmy4pvysm93usc4j", Amount: 67_6415045011},
		{Provider: "maya1guh3n0c84quc7szq9twmlxk9tk9fac3mmpeftt", Amount: 895_2277036614},
		{Provider: "maya1ha4ypeghxhtdu63dqhhkspqcu4s7375kc3ch4u", Amount: 1076643315},
		{Provider: "maya1nwe0vs65myamknwehgr00r5t2afrlpn26du4vt", Amount: 919619312},
		{Provider: "maya1y6lk677q4gdy75qm5x3q4t0sxvx40r8n2kcc4s", Amount: 193_9880334042},
		{Provider: "maya1swcvf06tsytaalk7y6t3urnwyv435gu8fly77g", Amount: 196_9506511639},
		{Provider: "maya1xmn5ecq45fasyt7xqm8nefg8fvpf0w7zqtn2tq", Amount: 557_1829671102},
		{Provider: "maya10n2xw02y4wvv64qulnhmgjdryktzq3nhd53f6x", Amount: 74_8255850119},
		{Provider: "maya1fert275f6afn8hnjypzhq75f9vrwfy3uej2492", Amount: 1468_3170205498},
		{Provider: "maya1lghvak02n32tlrgm4xvj9zmjr4s7fwx8wyethm", Amount: 218_6476571398},
		{Provider: "maya15n93tthvzldqykev5cs4l3utqhg8v0m2tn22z7", Amount: 153_1323322462},
		{Provider: "maya1smu8qs5dqrxuvqkyf5v9zrf7pa94gm7e2naq9v", Amount: 41_8405069148},
		{Provider: "maya1ccf7rs4z6y2spvpmdf7v66v7xy2rd8dye7jhrr", Amount: 130_4120103595},
		{Provider: "maya1x64thscxsl39pun3lqzwhge890tvhvgd36c5gs", Amount: 221_6570161033},
		{Provider: "maya1nv96km7hgmv76rsjcjj5qmx5ml53alf9r8fy22", Amount: 305_9885056479},
		{Provider: "maya1ewz79pg6qylpk0p98yzr6jhv23s4wrn0jcnard", Amount: 429_9454195178},
		{Provider: "maya1kzd9fj58g9exxt44lj8sfzuvc94tsrr2v4gv6g", Amount: 312366982},
		{Provider: "maya1ajzlu2p2mnecl6q739fn7hsctlwxyqdulwsslg", Amount: 190156170},
		{Provider: "maya1jttfwrve7mcjfnhnsavpnfzeql4mr5mjns0wpj", Amount: 184190874},
		{Provider: "maya14udggus78e9hh2my7uxnn0l470dp9yj5u35l00", Amount: 651_7631226817},
		{Provider: "maya19jqjqnc7hmvrfez8p5z2tcfmfmq9k5z3wm0rq9", Amount: 886_8425791225},
		{Provider: "maya1c6qrsnstl9l0wtc3fazd6jrfppshs6jk2myeky", Amount: 246_2729280723},
		{Provider: "maya1xkdt3ld8xtlfpztdp0k05tmf9g3q622lmahjr2", Amount: 439_3811075027},
		{Provider: "maya1sclplk79vvlakl8u54r0gr622jfuwar0vfl2l7", Amount: 244_4135469557},
		{Provider: "maya1v7jsyf94rnfdx5v0xjxn5c8vdsyvmym0aegl7k", Amount: 337_5376073612},
		{Provider: "maya1g286wstwf4vqmegj5324p58gxmy7mnmha80hgz", Amount: 179_4228504620},
		{Provider: "maya1xrn6rw99ncj0qxflwtmvjeuf4kkwuwja4xpwhv", Amount: 365_3826916897},
		{Provider: "maya16f8kzx474xwu9rr9ah4mxrny5rq2nhy0yjkrme", Amount: 237_7645982361},
		{Provider: "maya1mfw8c2agx7tmdxt5ez3qsqfmyslagxny0sl7w8", Amount: 969_6826822177},
		{Provider: "maya1s7naj6kzxpudy64zka8h5w7uffnzmhzlue4w3p", Amount: 746_0734312238},
		{Provider: "maya14sanmhejtzxxp9qeggxaysnuztx8f5jra7vedl", Amount: 263_1238239972},
		{Provider: "maya1g7c6jt5ynd5ruav2qucje0vuaan0q5xwasswts", Amount: 261_7814317799},
		{Provider: "maya1k3r9mtedeurcnjzfhgxzkqrum9f3yy2kkpgt34", Amount: 231_9382825684},
		{Provider: "maya175dn4q74ztt7wzf2n5u0nqkmfvda5sc627vtvd", Amount: 673_6157260911},
		{Provider: "maya1hkqc78uhuc4z8qtt3qjsdn0u7348t2hhlgyzh9", Amount: 1140_8754798383},
		{Provider: "maya1kpm9vz8cc2w984vghz40z0ekqef4xlglyx2yeg", Amount: 496_4215432024},
		{Provider: "maya1pf7gg2h9kdq7zuj58r7wk8py99awwj9lwvchdx", Amount: 670_4760996117},
		{Provider: "maya1tdp957gs94j7ahgd6cemlunhrwd27l39e52l6l", Amount: 186_6576821491},
		{Provider: "maya10sdhv0cn0fsfgax6vpzv9pwy8r5872hw3h4tuh", Amount: 548_2678887003},
		{Provider: "maya1vu37n7h7mnk0uxakye2vhh2z2k5cehf6v2lk3r", Amount: 203_4014261306},
		{Provider: "maya18p22jfv43weeyznqg0h9f6dh3adnpj4nwch8hs", Amount: 205_7460310253},
		{Provider: "maya19z4xlhxp6hkqe4mlfmqwsnjahrpa3ycjflqczc", Amount: 61_9125692167},
		{Provider: "maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz", Amount: 39891_0000000000},
	}

	for _, reward := range rewards {
		providerAcc, err := cosmos.AccAddressFromBech32(reward.Provider)
		if err != nil {
			ctx.Logger().Error("fail to parse address: %s", reward.Provider, "error", err)
		}

		if err := mgr.Keeper().SendFromModuleToAccount(ctx, BondName, providerAcc, common.NewCoins(common.NewCoin(common.BaseNative, cosmos.NewUint(reward.Amount)))); err != nil {
			ctx.Logger().Error("fail to send provider reward: %s", reward.Provider, "error", err)
		}
	}

	manualRefunds := []RefundTxCACAO{
		// Manual refund paid out by team https://www.mayascan.org/tx/A40EA9E982A794CE0ED9B813F553C218CAE975975A359326FEF9F9AABF749643
		{sendAddress: "maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz", amount: cosmos.NewUint(28000_0000000000)},
		// Manual refund by team (sent to old vault by TW due to mayanode public API endpoint downtime) CACAO (5,000)
		{sendAddress: "maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz", amount: cosmos.NewUint(5000_0000000000)},
		// Manual refund by team https://etherscan.io/tx/0xae68c0c59977087b340c5226f37c6e3c96b510caf151a13d06822a9061942138
		{sendAddress: "maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz", amount: cosmos.NewUint(3735_1430430000)},
		// Manual refund by team https://etherscan.io/tx/0xfe113a3d316cd4cccd2a79077628bffdc740bc1eecef5fa9408c76aed1f8dd44
		{sendAddress: "maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz", amount: cosmos.NewUint(1223_4625390000)},
		// Manual refund by team https://runescan.io/tx/210512AF36E7925AD80344CA86CFC3732359EA3708CE54E82A1BB33FC69F01AA
		{sendAddress: "maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz", amount: cosmos.NewUint(175_6608684000)},
		// Manual refund by team https://runescan.io/tx/9DC1F05BCCB1CEA628266FA3A02483074C20EEB40B3F22089B6FD1E9470ACA28
		{sendAddress: "maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz", amount: cosmos.NewUint(702_6434735000)},
		// Manual refund by team https://www.mayascan.org/tx/9216104566E91EEC694E90EDF7686B08164F26CF73E3B9E3EA959A4C1D248580
		{sendAddress: "maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz", amount: cosmos.NewUint(3366_0000000000)},

		// Refund Dropped ETH Outbounds during a stuck queue in January 2024
		// https://www.mayascan.org/tx/47CA1E330362982D79680583B11FA1AB7E5F64452BA21B39D6B244D09054925C
		{sendAddress: "maya1xq3eaf70pdw4wl8esn0kyuxpnunprs05tgppzu", amount: cosmos.NewUint(152_2000000000)},
		// https://www.mayascan.org/tx/A8745F4940662F648CA601B8D63A7EB25393072E87D3DAAE2BEA3EA51434ACE6 * 1.20
		{sendAddress: "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc", amount: cosmos.NewUint(6956_4662380000)},
		// https://www.mayascan.org/tx/9B8361F471C4CB8D803F92DF086DD4E30806EB2B9273C9A579A046AF218B7C0D * 1.20
		{sendAddress: "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc", amount: cosmos.NewUint(13000_0000000000)},

		// Refund 19.98 RUNE per tx (in CACAO) to the below addresses, bifröst was setting gas_rate at 20 RUNE instead of 0.02 RUNE, overcharging customers
		// https://www.mayascan.org/tx/7C5935C87A1F383E5F075249D511C1C19094323E6D36181DED6A2B736189DCC8
		{sendAddress: "maya1ym3vk67ldc2jwlwmgzpenq78kkln7naxcqy005", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/1F50570FAE74B4DE0FB57601FEA7657A750EFF1379FD9EF515984D14612ECDE9
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/3C44C9CC227C68AA5EA79511BDBFCB1A294A30576FD1099E66C7E8932F7ACC1B
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/8C0F22BD6B8C1D79A801F5C95F356EB73A4DB9BD8D52206A6B5101D496A36E25
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/9758A4E3E8F414A8C6B7670CC114819A42E225DC866E75E0B7EA1154EC670279
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/E57E2599F72E229CA77B2BCEF0EA632B2F77F3FEF9C04EEF8161A4214804C1FF
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/4B7571EF61561C8AE287BC83A4403786626F5F2E87A81413E04827BE833745F6
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/DEF1492B6322CFA6A46F521EB8864F894427512D35F601553F5C3E172F460514
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/9CE8B91AE846E558DD8F2294C17BB7932358EAD87F85B826E5C47D44BCE4695F
		{sendAddress: "maya1gsvhkjqe42z7h0hq589kfgd7xywwfud04awmwd", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/F23E27C695D88F6F91FB901C25DE6635025C48B7419C23B65C5715419EB24110
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/2476598E54F867E9498CCACAEED1E6E4FC0A8E0FA3B0BE028BAC30A5CAC92283
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/C760B881C06A46271DA845C5B74C231AEAE5FAE2C708F5DF9C2938148CA13D20
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/D2D2210402370E0E12BF828051CBFBBD6D840BDA61662403912198581A5B9696
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/EEBCB3D88A9C53820DF3FC1AD6498A374B04859CF01BE3880DCAE1F34C36E35F
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/D62641CB9FEF4E67EDADF0EA86A8185CD2EFA8D25BA778D1D3945F7A7E70D34B
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/CF340B8DD282D080BA515AF7D1B732462073BC04A3E47DD5D4D596D4E37185D2
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/0EC3288D9D7C43215E7058E0A3A997D42E07FDC4D067D108B66FDF099CFF6806
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/52B5931FDA4578A8B6FBA5D6D373EF3E923C415BE3EEBF0FD346835A2940FDD9
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/58C49A458A3E455260E5AAF9B322E76C8FAB5F06D7A510353C1014FD245C18BB
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/0CFD1055C2D0D7ED8C017DD24BA9023F6163FCB405D8A477EDD3EAF3100EB9E5
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/E52340F2D33CDC42EE38956848E9AC1E88F27D57A4173E9452006BAC7A952CDB
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/E91651E8B622190E15ED2CC7071A4A063B44DDB8CEF66A7D9A506578541F4466
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/1B5D64044B41D51FF8D2B0C41B01C41248A65848504E27388528148808548A3E
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/037B6F40CD59D3E548D1B2FD320F35CDBE907431770CAB1F186DF60F0588B50E
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/7365DF38557B0CC36D86C78D37475E6AFFB5AA63B5EE0AEB953E3DBF990BA9B4
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/8D526E6E39B00119078B7EE64A4656C1159941DB8456F44054D38F6CFE03D029
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/7EAD0D66085B6A278AB59CF3D92AA97CAB977AE8E8046CA8CC758033E8052E5E
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/B90F321681937CC2154C27E8E30723A180C1CFA8AC8EE428C144AC2DB2016455
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/9D8600866E0C6A0E1A1586A425CE723C00943AC618C6561BBE4116C1E686EEEA
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/9D793B7842FC6F57BFBE92E107FB24165C95E0B6FFEC43D6CF7128358F3F6DFB
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/D8A3AD9D6057AC27972378BFD3AD699AC330FDDEEC17BAE6B105B048D643E29A
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/5E60DD56A79749D2CF618EC54F466E473A97DB2659C5B8A6F65204524C2690A7
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/A96BF6FC8DF39DFBAEDF7CDAC917B10FD7D591F0E3D08AD90C534EEE081CEB27
		{sendAddress: "maya1ymvh7rg4a6thqsgmf9z3fa5u4alswkftml80sf", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/BCAD48DDD5645BEDCB8C9EEBA8D3FA4DF0277E07E955FC61F2B4E47543FA75F6
		{sendAddress: "maya16adcj4245hw5jdn023cg4nnt4f089l6vhgcpyt", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/DEC27A703D7F7F1E81D2AF4EE2A88959BB3C9C6A07CA8B7E31FA8DC94CD78011
		{sendAddress: "maya1j9hg086cp4kc79jhauqkqnmp6j6u402zsax94c", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/E007E360FCCF46A1D5F6780ECEFBB7EFF82D85B4038AF3EF0698FAA5BCF3ED15
		{sendAddress: "maya16adcj4245hw5jdn023cg4nnt4f089l6vhgcpyt", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/21E996872215D4961BDD8A16E20F886659724415EA3BAB670FAE8C42D01FB511
		{sendAddress: "maya16adcj4245hw5jdn023cg4nnt4f089l6vhgcpyt", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/21A7A0E345D144225938257A47096EC23A462D37FE5F2836319E78653B1FFE85
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/650B41AA0D096E5BD3DA801B06A26F7BB6B92E28B5A9D467098423AD60FE45DD
		{sendAddress: "maya187atpn8wgf45vah47hawkfhcrwyf4r7c8ersxs", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/F3DA3A3758A2F81749FF5ADFD8C4A81B58E03B6E5914BC115FE0593C879DE417
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/201F0EB6941A844C85C731F722D47255F1A5D3F608824510B4EE66A06A44BDA0
		{sendAddress: "maya19t4cjgcm6s47qshu2lt4d2yj4wy0rd8xs82vea", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/29A5009402A27423543E612988429D089C8FE879D56ED56F12B8022C0B21D7DC
		{sendAddress: "maya14sanmhejtzxxp9qeggxaysnuztx8f5jra7vedl", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/3A989A22D4A51791271D56535D079842FCF134D8F9602692C51B29C5183A86A2
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/B7B96D37D74A033612F11347B015EC21882BDFC104400CDDE2964406925684A6
		{sendAddress: "maya1qsynvzys9l63f0ljgr7vk028n4yk0eyvjakn80", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/C45B44E24379BEB601AF067E6F6A787D8DF5B6FCD7AE76B4F099789F16F7FB70
		{sendAddress: "maya1ahzf4jvc93j74m37ex3nfuvhupuevyanx5lfjc", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/5BA778806FF1D10BC96B619E1339D2C85C192C85DA3B705FC0E587AD6CC813DB
		{sendAddress: "maya1ym3vk67ldc2jwlwmgzpenq78kkln7naxcqy005", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/0C8D2B62D9C3FA906A947AF5457583DC81D96877A40D01D43C69879FA5AA2A88
		{sendAddress: "maya12qj3aw9e2ec45n4fv9jx56fl7lcrccupc7qj6p", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/523FDF05B87DC9EC2211F8AE3375A5EC054F4C41323F2D7D4CE0A175F781CFF4
		{sendAddress: "maya1mxes04w6mu9fy32w7zml20anux6smtajqprvfq", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/CA5FD4525A1C39BD3D66300EDBCD47332EEF516C30A57D61F66A55FE2B432455
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/DF471498F03AC94F8C2051174C4962DAC69A5F5FD111BA92B55134535A34C15D
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/3A91288B57B557FCC774CB2C9800240B72B891B8803D76A7943EFA73AF100A52
		{sendAddress: "maya1nxvlefnx27pqmj2q5trf4pzpgquhedxhxm0n4k", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/0A91AD07C15334F4AE78BA61FEC2C1D47407793DA8BE7D040AF2F7D9988261B3
		{sendAddress: "maya1ayzv7vga724m95g09unzcfkuxp6838md5tsy9k", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/D2D81ADC431C467E02D8EBFDEF90A442238680C26452797A980CEE4C5695C079
		{sendAddress: "maya1j0t0gwvz7untqwk8jv535hlxqrtzp8kyefp7n3", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/11C43A11A2F99375D8647D425D6E04722A970C5301E71043E4BA12DBB89C2FD9
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/3EBCE1628995DD0B8212CB11F76D6D37C51B53EE32A59B81E47D8417F77BDD4F
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/D7F91E02A7D6765B423F93B03565A078832CD267CC0C61608ED4C310687984BE
		{sendAddress: "maya12aetsuee7cyzxahn8u0xzpqftkdmyr3s3t3h88", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/004D4C4F2935C583F488EDBC5B93CD70DBC88060DCEFC0AB807A20F320F5071B
		{sendAddress: "maya13su7x39jun8mmup83c53lv5dqj58mg0nu24efp", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/A6467A6ABFC0B51591FECD7FE1B1AF272645D4EF128AAF1D3B91578E47B586EF
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/70694723EBAD82D6E4000A20B3EE9EA849C6E6C0A6CC2E8097D1F082BA613A06
		{sendAddress: "maya13su7x39jun8mmup83c53lv5dqj58mg0nu24efp", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/F8E958984DCEDFEC9EC5793012DEC0FE57EA04CEA149D7E1E2ABC39D170ADB91
		{sendAddress: "maya1tm0qxkqylv0xly82qykafvmg4ul8vj4yxma7h7", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/DA060D5149FEBE2242C968E977192153E7846B2F7A8F57AF045D58B38EFAF652
		{sendAddress: "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/A718298A90C08695D88CEBE1B20E3B6D1645D11DA6F9405C85DB9DEAC584039A
		{sendAddress: "maya1jmk9wak5xhjl7kljv636ayym9as8hvn7z667m4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/3A8068B1D8B43E5118B0F3B6A235D2F341D23A1676671AA7559A3D9342E1448B
		{sendAddress: "maya1hcaqh4ez3p24pr79gu226cvfw9473crxnyff9a", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/A898FBF21E878F69E4C5B74534CBDB50519083E2A71958DF6BE1C01C4618B911
		{sendAddress: "maya1vx2s0dqy0unedgmruc52sg0u2emq7rq0nvcm3u", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/736E667D6F0BD397C5D71500AB298AC945AFC2F6EF1A2EFBB81F65486901C5E5
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/8F1D44840B4D2CF6C60E0E8F6DD3BADB79783053099CB5522D16334547E60234
		{sendAddress: "maya1qefmyzkgkvu2kz57yv5x5r6k9tvr65v3u2dgvh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/2FC67EE02E2679687816A3A53438BF815AA0CC488BB58CF74034F8E381E6E8AC
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/65651525C93198E5114610049FFDDFABEA86FA2BC871A549298159E86A8B6327
		{sendAddress: "maya1d059pckjsfzv7wzn4wa5zfjm6snzen7mysmth2", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/E2BE7F1B12538B1C1933862584D0EFE422FB7B3F78A66C3330BF173D76DDF6C2
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/9412C099A738001BD2E22A2E9D595C49FC67CE4F7C12820A8E5C55D95995415F
		{sendAddress: "maya1605alvdhp7y990a9grnf2kvql8d66lxvwy9g0m", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/9659BCF686FF915188351BE4F39B281E07344F61BA93595E5EA18E4FDCB60C6A
		{sendAddress: "maya15n0nanaymr0aupvvwrxx3yde04nff9pxta682f", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/6072B245D02CCB0F47B96E8293033A3414B59B9C5244ECAE631927DEC5E3AAE4
		{sendAddress: "maya1sdnxgkh2te46ayhkj0au63k8klm6wj7tzzztnk", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/CBCDDA3779DBBBAA3B24C67DFE3F058AEB96D28D62C07397928D03E61CD5E271
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/499097246E2DE9EE4C718FDAD7EC83E7783D6F52A22349DB62FAF6792541CA6C
		{sendAddress: "maya1n0tp2rgc9pnl5r7nq705adpmk6lfvss6nhkh6y", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/AD0756F721D9442B6DECBF7353A77A8C6E3AE3895CBFA9B084DC535FC100E00D
		{sendAddress: "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/391A614C22F391A9F8C7A75E43527D26B7B681C3E04579776B0D7D61BDC19761
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/19CC3BB9019BB72B6EE2BD53439F89A090ACDF3C233BE29FDBA8FA0019DEB146
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/B2F1CD0AFA0FA306296FD68141CA0174D2080DD4A68E848720A954C2A0DA9FF2
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/4CA9E5BEC621135F76562D21B986C18D70F50AF5EDA7FFD5422EFF895249E800
		{sendAddress: "maya1n0tp2rgc9pnl5r7nq705adpmk6lfvss6nhkh6y", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/F959D7784EDD1048E60A1B4E38DA5DC3F135F111D5AF2D21DB20DB02C8EADCD7
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/1D00A91DFD3411D19495A678FBBB22BC23671FB7B2FF764B747107EB6A5C68C6
		{sendAddress: "maya1s7mga7ztwx8sf4ptuyspd4zyj8hac7ltulnegh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/510EA13B676027720F10FC1C8A33C7993DF5838339E2CC4C288A2DD747A9317C
		{sendAddress: "maya1n0tp2rgc9pnl5r7nq705adpmk6lfvss6nhkh6y", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/1E751FA1E6887647A061D3F76EE100F34F5C74A9521A3580A786B17139ED8D8F
		{sendAddress: "maya12fjzarhhc0f439yy5372xz4jwzgxg60un9gg9g", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/C52F328B736319D2DA5ACC20D486AF6F0B17256D615E86F5512EF6F18EF1B98C
		{sendAddress: "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/49294FF51BC204FE461CC2EA7A9DD970BA256F01E03B91792CF2FEAADE8B46D5
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/535E56823D5DF0C65C7F22FAD56025DD1EDFFE4223C8A652A46073232401FE16
		{sendAddress: "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/A90CD0FFD6B1500D854C29A2D37D55FC5F96FC7083B6A3D3C921488E545FECA4
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/10931011F0BD898D1EBB2C957332A74ADC782E293A0E6DB792DB351A8CAE7221
		{sendAddress: "maya1quefasy83stwlghdh79tpyr3hm9lapvc27ud5y", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/970FD9C8723A05BEC3F3FFA1B10B60C6C2E3F40C466240818BE4BE3576A9FD75
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/BFBEBC677BF333A624E23FE764B140616AA7BCFAC687F4406EB59EDC07472811
		{sendAddress: "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/E9C3D660F88FD29982B3F1FF7575CE4A5957341AF75D5FBCD06D14D93E3E3282
		{sendAddress: "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/12F9ABBBBE1FFCC15D5DD3DA4B3F6C9248A1FE15D205936E8FEFFAE7B2F0DB6F
		{sendAddress: "maya1quefasy83stwlghdh79tpyr3hm9lapvc27ud5y", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/3585AC665644FAFCBE104EB8679B2C00FE8B797BBA72D040E5D63BA34973CDC6
		{sendAddress: "maya1l42u3x2xr0srtvllupu4kgtnxkgvwvvmmpdh6u", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/9ED4FB15585F2CD6A0A4113673E4EDC067977282DD7866A76E6F295606B3CFDD
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/6F319008C963EA115FB7631D86F46CB060A8B40ED684C032B453FE852819099E
		{sendAddress: "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/98C5A92F45A9D1BB9A30BF39E54FF0E89739EE3FA5C415526FF8D8B923741C72
		{sendAddress: "maya1n0tp2rgc9pnl5r7nq705adpmk6lfvss6nhkh6y", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/9BC6DE77E4A1447BB8DB1AB374B197DC996143AA80E9FD9BF1C68D4889EC4AC2
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/1898ECD5449610F17998C5F2DE24286DDA5F6155DF28B8C19A1E74ECBE8C153E
		{sendAddress: "maya1ym3vk67ldc2jwlwmgzpenq78kkln7naxcqy005", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/AB31AC78F5BAFA0B91BADDEC1CE658F0D5F4C19C360B6C4FB1F815F42A4959FF
		{sendAddress: "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/142712796C854CC16787FA33C59A8B997F02AE911A82AFC77DD838B3DE55BF1B
		{sendAddress: "maya1ym3vk67ldc2jwlwmgzpenq78kkln7naxcqy005", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/3A70C367BEB973ACA2018A304857527998973D45F091D09C6C519AC550ACE343
		{sendAddress: "maya16adcj4245hw5jdn023cg4nnt4f089l6vhgcpyt", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/638EBEE98D7185736BCA369CC6277EAFB38C7DB02875367EBFA2BE405171AFCC
		{sendAddress: "maya1wx5av89rghsmgh2vh40aknx7csvs7xj2c5tjrr", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/5284F153C7D65D78188D1F477B6CDE7EBAF22BE52958028778150501548373A5
		{sendAddress: "maya12aetsuee7cyzxahn8u0xzpqftkdmyr3s3t3h88", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/D0B3263CC81D0C083525697AB4DDC8D5A68DA43CD610157B6FD6365488000466
		{sendAddress: "maya1ulzsr523wmx0gkndw029flkh3j9q5dljg8veen", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/D08B84BDADC7E56D170EAF2D4D027330F902CCBE39C5820763CA791A811F446B
		{sendAddress: "maya1mxes04w6mu9fy32w7zml20anux6smtajqprvfq", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/7EB3982BC22519C99E69A6D2E0F2097713A4BD531E1665C05875DDF718C317C4
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/E58AA905E06D33DCDED4D9EB073A9A535B5DEEA32DBBB6FFDADD2C9A0B1D2EC2
		{sendAddress: "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/1FCC63BDD9BBDF889EDA14D93A4B4653CF0CC5870D33FCB75DAB6FC07152CE60
		{sendAddress: "maya17cyy84n4x94upey4gg2cx0wtc3hf4uzuqsmyhh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/66A0A42BF830A5564546455FC8D819C6DC72F36E1A60A0F9FE5527592D9F03D0
		{sendAddress: "maya1vnalntj68qzv9mr0sftxgvucqlw2ht6yjqkr8d", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/0BDCD6A396F9074D7DD6686088D104FD19CC05E3730837AA3C116A8AF8BAD969
		{sendAddress: "maya17cyy84n4x94upey4gg2cx0wtc3hf4uzuqsmyhh", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/41E3F70245CC22AD4A237F4F68447E51457441DEE081973D27EB2ACE6A56B513
		{sendAddress: "maya1mxes04w6mu9fy32w7zml20anux6smtajqprvfq", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/F8BA206729778CC8EB13616193121743441331424163D755EE6295C76E7A6AB7
		{sendAddress: "maya18jxee48ah3vlkfrndlm32y6urwhgyjgpdfxw7e", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/CE8FE08CE5E7C31AC03C39EB35EC8AB5219760CA3B10C2644D3E7C319E53889D
		{sendAddress: "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/2C9B224C4E49D99EA13F3572BE31E3416D4484E4785BF8814D39DEFBCFFE302A
		{sendAddress: "maya12fjzarhhc0f439yy5372xz4jwzgxg60un9gg9g", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/CEE478CC58D8E28393DC1C7F7C3B3C616894C62A4CBB5ABC7A033D24786BF3F9
		{sendAddress: "maya1hklqwgqfe9dk43xg0lfz6zf3rsers3hslllms5", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/CFBB4EB812B706692830758CEEB92893955C45BEB0433B0D2ACEDF7D58EB1863
		{sendAddress: "maya12fjzarhhc0f439yy5372xz4jwzgxg60un9gg9g", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/03A4C470D4F360D6A4AF7D71131D87D60478D6E7CA9AB10E9EFA52974DA44D33
		{sendAddress: "maya17x5sz7lmayh4vw67nj2wynquf6vrt50f0u8lxx", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/87B910C7805F11902ABCE796141389101FEC3C217396495CC8D943A8C3914CA2
		{sendAddress: "maya17x5sz7lmayh4vw67nj2wynquf6vrt50f0u8lxx", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/486639334B1EA058DE2ACA87A8B8180DC8518C952C801F99661F61916FB5F413
		{sendAddress: "maya1wx5av89rghsmgh2vh40aknx7csvs7xj2c5tjrr", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/059024A6626F3AA6AA9B33CEFD97B48FE2B9E9B9715EC8D0B48037BC50277041
		{sendAddress: "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/B195D9EC30FCA896F93490645C1D167E05CFD2E150921AFABAE82697B5A71A0B
		{sendAddress: "maya1n0tp2rgc9pnl5r7nq705adpmk6lfvss6nhkh6y", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/C94DF785FFF232CDB43A9A6C8470BDCAF254C1301EC7725E30DAFA998016584C
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/D2E6B758E80C765D4F57403D5546C6503D86E36F69572AA976B0CC91F641523D
		{sendAddress: "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/B60AB62E3FC90E2FB3FCDE3B1F6A762B63432AFEFD6D587D0FC8A4621D5F202B
		{sendAddress: "maya1wx5av89rghsmgh2vh40aknx7csvs7xj2c5tjrr", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/61DF0A907F7C9AC310CAAEECE7179EAFB07E8130BB6F7548C9BD58EBF8252DF6
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/6CE11F3A150E65FED8F7A61159CEF1EB683A9203A44E7753F77DC15CCCA9A4EF
		{sendAddress: "maya1mxes04w6mu9fy32w7zml20anux6smtajqprvfq", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/E5AFBDEA0BF36EB7369770765C9056E435B61E14A0BA05FF3D2197DBD398B1E3
		{sendAddress: "maya1hqkucmh30dqq9lx4gtk4dx5m937s94cc5cxjh4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/134F5C10ACEEA2B427AD53E3DD94D5A01065BDDB8201F0C330FE5331EF3AAC8E
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/D38A09B04E117DBA4326C7020A6E3ED43713498F96DC13722EFD1E6A0C36A76C
		{sendAddress: "maya1x0uv04mglg439a8vw8q4mn83jm5s3uuj0vuuj4", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/D70FCFF3CB17FAA67F760FDC7EE2DCC2AC2977F07694C3351348483CC0B7509A
		{sendAddress: "maya1wx5av89rghsmgh2vh40aknx7csvs7xj2c5tjrr", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/0342DE3969CD3FDCDDB0203D46875A8EC244E17E962F3949AD936A1EDD3431D2
		{sendAddress: "maya1gyap83aenguyhce3a0y3gprap32ypuc99vtzlc", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/A3ADCEA068C52F73BA405BAD527A9FFE95C4E3B1F6C8E12DF1E7A20563E0EE50
		{sendAddress: "maya17hwqt302e5f2xm4h95ma8wuggqkvfzgvsyfc54", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/182F09F0E2DC9FF9AEAC8C6BFB974C79EC4F18D68D53C8399F50EEE3DE7AC8D1
		{sendAddress: "maya1tjw4nxtezank4kgwvupxfyc3r4shw4j86cjxpv", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/C7C09BEB97418AFA066A796F4DE5CE9D40D783411A011FDEB2C048286732FE8A
		{sendAddress: "maya1mxes04w6mu9fy32w7zml20anux6smtajqprvfq", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/67F34BFF74EAE24676A7CB573C9DACD9C271FC6B568FE9A332FB673A595F41AC
		{sendAddress: "maya1vx2s0dqy0unedgmruc52sg0u2emq7rq0nvcm3u", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/46932FDE36416702969BE329BF860AF6DA9A93A8906484A38D6455B7343BEAC8
		{sendAddress: "maya1dzawlfg28lp0lysxx3p6ucnwfy948xh7u0yjks", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/D69114D6B5989E11543625F1D52BC6BF457C9ED85474DB0D2DB6B0D24020054A
		{sendAddress: "maya18pd64frgh5eg4z374dsvsap3lazqhfxgcfe8d6", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/9D342F36518DAA0114D23241DC457598DFFA56E7988DEC2628CAB30C52C36A86
		{sendAddress: "maya12aetsuee7cyzxahn8u0xzpqftkdmyr3s3t3h88", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/287196CB09AE12FC0A67C5D997809CFCBEC4F0484DB779F4EB1FF7FF645C53D4
		{sendAddress: "maya1ehsjen3p9kfw93fasul299weujgl3pvp3hg7p2", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/B7CC19A8456339711473F35716D434D9D4B9D1A3679BF97DB3F72DFEC197EE88
		{sendAddress: "maya1rh4s4weg8ewvagnm27y3l99v342hxnda8z774j", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/BED0D529DD716A16ADF7C1C7FE33755818174E416BC7B3EBB202591F7CA4CA47
		{sendAddress: "maya13ucfs2scdcm5kusxt7khs6g4e9s2ycekk32q4u", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/5BC380417C867E9923C91786CE3AAFAF60778E948267FC429100056E84DAF4CB
		{sendAddress: "maya1lzwwgvdw6amvmt3en66tyhe64krkc6eul7d9m6", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/14029795AB815B00BFE18A038E825773D67A7443D442677F1DB9CACA80A46978
		{sendAddress: "maya1kxqckmd770ntr52qq8c4ryzhcrehsdqe38ydp2", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/3DE6C788FBC563801A8C9FECAD9EA9A5806AE20D78893BBCA2E617222A98927C
		{sendAddress: "maya13euhshrsncukjx6x0rpatrja4xvyjcmpwg08fj", amount: cosmos.NewUint(140_5286947000)},
		// https://www.mayascan.org/tx/14EDD9FDDA114FB1CF219CC1D668ADCD54E2ECF4F73FAECDF3F8BAB074A8F0DC
		{sendAddress: "maya1hwvf5pwa3379t2c9w3j6ys6h9djw7v2ldv4nlq", amount: cosmos.NewUint(140_5286947000)},

		// Ticket 1773 https://www.mayascan.org/tx/BBB7474251BED8D6CDFF6D8888678CA205A02F1912412D46B8D44AB63EDCD3B2
		{sendAddress: "maya16fpyktrqntc2q2hr98p366k6peme7mlp8eyu77", amount: cosmos.NewUint(141_7230769000)},
	}
	refundTxsCACAO(ctx, mgr, manualRefunds)

	// Move node op addr
	// https://www.mayascan.org/address/maya1lx06jmugq3s9n6rz6y5up2d0wh0vsj6gy5rh3e
	nodeOpAssets := common.NewCoin(common.BaseAsset(), cosmos.NewUint(347_7600000000))
	nodeOpMaya := common.NewCoin(common.MayaNative, cosmos.NewUint(1247_8600))
	nodeOpOriginAddr, err := cosmos.AccAddressFromBech32("maya1lx06jmugq3s9n6rz6y5up2d0wh0vsj6gy5rh3e")
	if err != nil {
		ctx.Logger().Error("failed to parse origin address", "error", err)
		return
	}

	nodeOpDestinationAddr, err := cosmos.AccAddressFromBech32("maya1jzpntepl8ukadpejf5m2fccy6vygssn6llw98l")
	if err != nil {
		ctx.Logger().Error("failed to parse destination address", "error", err)
		return
	}

	nodeOpAssetsCoin := cosmos.NewCoin(nodeOpAssets.Asset.Native(), cosmos.NewIntFromBigInt(nodeOpAssets.Amount.BigInt()))
	if err := mgr.Keeper().SendCoins(ctx, nodeOpOriginAddr, nodeOpDestinationAddr, cosmos.NewCoins(nodeOpAssetsCoin)); err != nil {
		ctx.Logger().Error("fail to send node op asset", "error", err)
		return
	}
	nodeOpMayaCoin := cosmos.NewCoin(nodeOpMaya.Asset.Native(), cosmos.NewIntFromBigInt(nodeOpMaya.Amount.BigInt()))
	if err := mgr.Keeper().SendCoins(ctx, nodeOpOriginAddr, nodeOpDestinationAddr, cosmos.NewCoins(nodeOpMayaCoin)); err != nil {
		ctx.Logger().Error("fail to send node op maya", "error", err)
		return
	}
}

func migrateStoreV110(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v110", "error", err)
		}
	}()

	manualRefunds := []RefundTxCACAO{
		// Refund team for other stuck txs
		// https://www.mayascan.org/tx/3630A7C5506D49C27F580B59E3C9E2308851E9A4A68DB62E4CCAFE5D8883D5B0
		// https://mayascan.org/tx/660A3EF28891B31963E0D7768F20058E80257C519C183EC46D45C4ABCF05A23D
		// https://runescan.io/tx/72EC71610AFF80028A71F91A74F481A46D471BE05290B71D8CBB5ABABC228255
		// https://www.mayascan.org/tx/D44DE3211E84DDD5F384700354F35A58DC07C7358C049E87AD4872F99A0F7F15

		// Refund team for Thorswap Grant of 35,294 CACAO

		// Refund for limbo RUNE saver’s add (7,284 CACAO worth of RUNE) to be store migrated to pool balance https://runescan.io/tx/C3D527932C11A8B59E7FE0E0586B660DDB39EBB289011EF2E8B0491D3371BA47

		// Refund team for manual Savers withdrawals
		//  XmwJn2ZAs59ZgCcH3epfsL2mTVpzps4ViE
		// thor12xdyyc9dep9ye7fzs8lnyaw7pk88w0f6hnvwus
		// thor1r08ls3k4k4fj84n27cryfmle9sdmp9jkpes3rj
		// Thor1xmwrh6626q7qvwnpyrw3fjh8q6f59r9mvkgcam
		// thor1a7gg93dgwlulsrqf6qtage985ujhpu0684pncw
		// https://viewblock.io/thorchain/tx/43232BDBFAF977A5DFE0D8C619D79055974D8D7AAD7E3D87B8DC2C957F058904
		// https://www.mayascan.org/tx/614D9F7AFBB0FD605E574F8428A57E2DBBA200AD42FA37B8C5517E845846D6AE
		// https://viewblock.io/thorchain/tx/BC6FF661171A1915BC98CE1358B11779770CE8A7BFD3975AA132A8734F38E65C
		// https://www.mayascan.org/tx/91826c0d44c41defffc7fcc179825f2b3a8189aadb91551f358300a11c73bdfc

		// Refunds from stuck RUNE when Thorchain halted its chain
		// 2508.40747108 A6E2C37C9BF658890657030393D7A3CCC1D2859169A0C37AA0FEEDAD4B377A52 done
		// 	53.31317576 B3AB2A81FB9624EFE2F64A3CF83EA87DE0F082DB79D02BFEB90BC3763674AE1C done
		// 3026.83465896 1A50CC9165B6F161632E185C34E198162D0C123AB5B8DFBF4015226E2B96595E double
		//  180.84873985 E2419D67A47818EBC45BB9EA6AF9D10D202D10973F25C2C133BD603EF8FA6803 double
		//  131.64379267 2ACC77FCB56FDA812BB1364721CD42A9B1B1F8DAC9C61C2C92AB7307D615F4D3 double
		// 	 1.78879370 58FDDB3DCBFC925BEC235D13B9F9B5B50904BDDA6781C8A3A51B25A9C84BA8EF double
		//  120.38799809 EF5E3B1CD8D54EAED01B3EE4F42E8E3304623D7C36E60BB77AC86620940C5770 double
		//  172.25939246 DD8D8FDE30D3ABBCC5D16DF627C8C50A7D435ECA858AB71870C8B0F5BFDB281E done
		// 		. 3175966 2B7A32B9C86E4E17F78D708F1FBEBDBBBFFFD4D437E4BE5B9F34799A6FE9854A done
		//  105.13197943 1927FB09E9382594CD794DEF489D2F95FFA4276D2709F8E87F517ACB9BB22F3A done
		// 	54.32749518 BF1CA13F217094EEECF19F04AAB86999469FA98BDB19A64249E56D9C3FC567AF done
		// 	 0.10919544 7B3A287932F6F119D93B8865FCEADD398D2F93B696879261D61F58005A1635F3 done
		// 1266.25643944 5B5171536EED1AEC6DB8A2BECBFC9C463490F5A5C9AF150D2E7D8A5952B49A84 done
		// 	77.95788519 AC189F12FCF832F6C1C50014C3607E77CC0D765B10FCCE70D7F05EA3C6DB11EE done
		//  113.81770482 FAF927B60DCB02220547B255D10C00B6E4C8D49853D64F79C1B0424CEA6EABA8 done
		//  104.97360247 695A0A2749B21A6043BDCC4C8AEBEF7ABA97A99FFEC6579ED124A35E02BCE3AB done
		// 	75.34835976 2907FDE0D445F142DA337DFD1626CD9710AA4E3BCC4621507F68C0A575F0FC3A done
		{sendAddress: "maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz", amount: cosmos.NewUint(112915_1100000000)},
	}
	refundTxsCACAO(ctx, mgr, manualRefunds)
}

func migrateStoreV111(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v111", "error", err)
		}
	}()

	// For any in-progress streaming swaps to non-RUNE Native coins,
	// mint the current Out amount to the Pool Module.
	var coinsToMint common.Coins

	iterator := mgr.Keeper().GetSwapQueueIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var msg MsgSwap
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &msg); err != nil {
			ctx.Logger().Error("fail to fetch swap msg from queue", "error", err)
			continue
		}

		if !msg.IsStreaming() || !msg.TargetAsset.IsNative() || msg.TargetAsset.IsBase() {
			continue
		}

		swp, err := mgr.Keeper().GetStreamingSwap(ctx, msg.Tx.ID)
		if err != nil {
			ctx.Logger().Error("fail to fetch streaming swap", "error", err)
			continue
		}

		if !swp.Out.IsZero() {
			mintCoin := common.NewCoin(msg.TargetAsset, swp.Out)
			coinsToMint = coinsToMint.Add(mintCoin)
		}
	}

	// The minted coins are for in-progress swaps, so keeping the "swap" in the event field and logs.
	var coinsToTransfer common.Coins
	for _, mintCoin := range coinsToMint {
		if err := mgr.Keeper().MintToModule(ctx, ModuleName, mintCoin); err != nil {
			ctx.Logger().Error("fail to mint coins during swap", "error", err)
		} else {
			// MintBurn event is not currently implemented, will ignore

			// mintEvt := NewEventMintBurn(MintSupplyType, mintCoin.Asset.Native(), mintCoin.Amount, "swap")
			// if err := mgr.EventMgr().EmitEvent(ctx, mintEvt); err != nil {
			// 	ctx.Logger().Error("fail to emit mint event", "error", err)
			// }
			coinsToTransfer = coinsToTransfer.Add(mintCoin)
		}
	}

	if err := mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, AsgardName, coinsToTransfer); err != nil {
		ctx.Logger().Error("fail to move coins during swap", "error", err)
	}

	manualRefunds := []RefundTxCACAO{
		// Manual refunds by team from stream to synth bug
		// https://viewblock.io/thorchain/tx/318C1D461D1097864E6BE497366614EFCF55F7E88B713491AE046E9FEE7090BC
		// https://mayascan.org/tx/7B8096E60D6B12C4478060E96D37C26115635961F39E45E54F2BDCD97A58916E
		// https://mayascan.org/tx/3BB20B5F6231D420A54E387C050DA6BBF4C5493CA6AD0266433C27DB0305D0DD
		// https://runescan.io/tx/589C08AD0254FFEB20D4A7A2BB43B6173AE7E11C3AEADB8FA6B2F7C4E37802B8
		{sendAddress: "maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz", amount: cosmos.NewUint(73584_0000000000)},
	}
	refundTxsCACAO(ctx, mgr, manualRefunds)
}

func migrateStoreV112(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v112", "error", err)
		}
	}()

	manualRefunds := []RefundTxCACAO{
		// Manual refunds by team
		// MAYA CACAO (79,000)   B638FDCFBFAE65F8AADC2714E70788CCA8E2FCCF71547E9D4EBD931F6CD8C369 https://mayascan.org/tx/4FAA61472BDB802F3B0530E20CACA12D44418FF5513DC6CA335ED082B24A428D
		// MAYA.CACAO (1823.12)  D56DC84DDDA6E4562F18D3221C3AC07B7A8F0DA4C147AE693FB48C29C5E94173 https://www.mayascan.org/tx/478481C688FB014D115F5862C0B8D76A18B2BC53D3140F8B15CCC5619A80AF51
		// MAYA.CACAO (1823.12)  D7A6A1039D6725E49C69F54E47D7FB3C51D96CD6B4A443CDCE52D0A67E41BFF7 https://www.mayascan.org/tx/781105878B6ACDD17C3091600ABEDD8040001B3AB83310536BB31D4DEC9E10E8
		// MAYA.CACAO (1823.12)  5FFAD375F156B98D0A9A39E6CB35AE95A818DA38E1E89B3A455E1EF7B7026B7C https://www.mayascan.org/tx/DD8E9216D22501E10B11F1A4214856AB8A7DF587F13706419F4A78B93E7A64F8
		// MAYA.CACAO (1385.59)  50E8E536423E019726A79CFDAF0AC2F57D4F9B93ABD5253E4BC36CA8DF989CBE https://www.mayascan.org/tx/4ADF4DB2E54B685FB5503D5FA79C2F56DAFD9D333A5EB80F1FA42A5A43B0E225
		// MAYA.CACAO (4937.47)  C71C052E89EF06D7847FAF7BA59E5728F94041C386C204F08699EF06AE17E184 https://www.mayascan.org/tx/8A1648FDBAE4AA214D0C3304FC6513B1F353DEA4AAEC44B6DCE0059A42414C96
		// MAYA.CACAO (478.84)   2E969B794F0EA9BB751B96C7EE88895DBBDD3C5C50F41CA28944F20B82B42331 https://www.mayascan.org/tx/49C72E199C297251969B9197C6C6C9D6A7122CEAF49F149198AC428CF36CC59D
		// MAYA.CACAO (275.82)   03D12C15AC926176F1CF4C505085DE9DD7C9E963A9A5A5A68F1F782B7D1E4C2E https://www.mayascan.org/tx/21E8D162F553EFA193706E2D14AFE666063BBC37CE9D00DCFA7276BEFA74773A
		// MAYA.CACAO (50.26)    45A3C22D57128A39E3630599844127A99A3A00D472FF72185392F35391E4D641 https://www.mayascan.org/tx/58E20229F50312D54252E5017F4144D00C96431BF97AE772BD13339337C3CED2
		// MAYA.CACAO (200)      BAEB9E00A146819A91F1ED5B9E92C75072DD868F9A1E866728F8FE6D9BE83F3E https://mayascan.org/tx/A8DA2E1E0CF37107B67A15883C43CF3FC3050DB4EFFAAA4F5C1ACBBCA07FA544
		// MAYA.CACAO (75)       2F83BFAFEAED2F5A52672B757B7CBA70CE1FC30ED9642FDB674CD76042DC8239 https://mayascan.org/tx/0A50ABAB56D6DFB91818B518056023B6A74DE6D0B88128B9288E0DA70877DED5
		// MAYA.CACAO (38)       A7B596B47C3F77BED99CB2D1BFBFA5FAA3D5F31595E3F3F47E89BC69CA616D2C https://www.mayascan.org/tx/B62FA8A3180F1E7609DD74972954890549A24BCBE9B7AA8F0CC4B7E6E40F32E3
		// MAYA.CACAO (11799.75) ""                                                               https://thorscanner.org/tx/4038F21FA63B34A65FE378051DC4F0DA3ECA2B8993A7C89532E38C0A7C027254
		// MAYA.CACAO (1792.25)  Surmanda                                                         https://thorscanner.org/tx/4FD636E318667117332583B131271D84251D7C9AA18512E9E81B5BE060A201E8
		// MAYA.CACAO (2471.06) https://www.mayascan.org/tx/04b6f780880967cf25c95955c7c95d81b78076d9dee85915fe29c41ca19f530b
		// MAYA.CACAO (549.23)  https://www.mayascan.org/tx/71684EB618D4EE9BC10B0712CD97E013892AF0EC5279B486840AFA8F0BDEAA08
		// MAYA.CACAO (1128.87) https://www.mayascan.org/tx/676666F6072C29BF9108F5D384CE689400555ABC0DB8A05FEBDEBD996F642DB1
		// MAYA.CACAO (2415)   	https://www.mayascan.org/tx/DC1B460AD72380305431570B4095FBE3E17506D7DAEDEA422293800A14EE2000 refund to XDEFI
		// MAYA.CACAO (845)   	https://www.mayascan.org/tx/82C4C9CF99205034B512D49A99FDD8A200AAA90E7C0F08022A279A02BE593A00 manual refund
		// MAYA.CACAO (590)   	https://www.mayascan.org/tx/0A1F40CA0D1294C3802048B726A1249F7CDF2E8D27A11657659C043DEC44710B manual refund
		// MAYA.CACAO (421)     https://mayanode.mayachain.info/mayachain/tx/details/3E8E12B707E9513211BD6E97A5C2F0D4D99D2909B7376783678B89AB8B97D0AB manual refund
		// MAYA.CACAO (28900)   https://mayanode.mayachain.info/mayachain/tx/details/CC478CC62C65C4F12583A0493BB72FBB6619A510DCC663FBAAAF0D0ECA82CB8A manual refund
		// MAYA.CACAO (1091)    https://runescan.io/tx/5A2E4F1192E58583E5AB099B88EBE6D6F632E02174EB4449F545802651C21A4C manual refund
		{sendAddress: "maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz", amount: cosmos.NewUint(143913_5000000000)},
	}
	refundTxsCACAO(ctx, mgr, manualRefunds)

	arbGLDAsset, err := common.NewAsset("ARB.GLD-0XAFD091F140C21770F4E5D53D26B2859AE97555AA")
	if err != nil {
		ctx.Logger().Error("fail to create ARB GDL asset", "error", err)
	}

	arbYumAsset, err := common.NewAsset("ARB.YUM-0X9F41B34F42058A7B74672055A5FAE22C4B113FD1")
	if err != nil {
		ctx.Logger().Error("fail to create ARB YUM asset", "error", err)
	}

	arbLeoAsset, err := common.NewAsset("ARB.LEO-0X93864D81175095DD93360FFA2A529B8642F76A6E")
	if err != nil {
		ctx.Logger().Error("fail to create ARB LEO asset", "error", err)
	}

	type txOutItemInfo struct {
		OriginalTxIDString string
		Chain              common.Chain
		ToAdressString     string
		VaultPubKeyString  string
		CoinAsset          common.Asset
		CoinAmount         int64
		Memo               string
	}

	txsNotProcessed := []txOutItemInfo{
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/80559CC3CCF2665531AAA7DD6B59F986721C6B76F1DD056DAE58DCC4878C5D56
			OriginalTxIDString: "80559CC3CCF2665531AAA7DD6B59F986721C6B76F1DD056DAE58DCC4878C5D56",
			Chain:              common.THORChain,
			ToAdressString:     "thor1sucvdnzcf4j6ynep4n4skjpq8tvqv8ags3a4ky",
			// vaults/asgard?height=8292361
			VaultPubKeyString: "mayapub1addwnpepqt2feen96mn88usq58q3ax0ueru5w97ms0ujqprp29dryz9y2d40v4dxyp8",
			CoinAsset:         common.RUNEAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/block?height=8292361
			CoinAmount: 199757582857,
			Memo:       "OUT:80559CC3CCF2665531AAA7DD6B59F986721C6B76F1DD056DAE58DCC4878C5D56",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/6F4F5801E5BEA96BC521BB00A7542EACB5FBDC161FD43431FEF3245D93F9C0AB
			OriginalTxIDString: "6F4F5801E5BEA96BC521BB00A7542EACB5FBDC161FD43431FEF3245D93F9C0AB",
			Chain:              common.ETHChain,
			ToAdressString:     "0xAa287489e76B11B56dBa7ca03e155369400f3d65",
			// vaults/asgard?height=8292990
			VaultPubKeyString: "mayapub1addwnpepqw0anseu8gqs52equc5phn980d78p2c8q7t2pwl92eg4lflr92hmu9xl2za",
			CoinAsset:         common.USDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/block?height=8292990
			CoinAmount: 9861109874700,
			Memo:       "OUT:6F4F5801E5BEA96BC521BB00A7542EACB5FBDC161FD43431FEF3245D93F9C0AB",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/9A823D4E90110A9306879865405E47F609ADCAAC1672223F403390F060121435
			OriginalTxIDString: "9A823D4E90110A9306879865405E47F609ADCAAC1672223F403390F060121435",
			Chain:              common.THORChain,
			ToAdressString:     "thor1qvlul0ujfrq27ja7uxrp8r7my9juegz0ug3nsg",
			// vaults/asgard?height=8442653
			VaultPubKeyString: "mayapub1addwnpepqw0anseu8gqs52equc5phn980d78p2c8q7t2pwl92eg4lflr92hmu9xl2za",
			CoinAsset:         common.RUNEAsset,
			// No swap generated
			CoinAmount: 10342158343,
			Memo:       "REFUND:9A823D4E90110A9306879865405E47F609ADCAAC1672223F403390F060121435",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/E8FECCA6F59FA38EDB6BCF9D25B087813FE2C649BD5887A11EC57E612317A185
			OriginalTxIDString: "E8FECCA6F59FA38EDB6BCF9D25B087813FE2C649BD5887A11EC57E612317A185",
			Chain:              common.THORChain,
			ToAdressString:     "thor1qvlul0ujfrq27ja7uxrp8r7my9juegz0ug3nsg",
			// vaults/asgard?height=8442653
			VaultPubKeyString: "mayapub1addwnpepqw0anseu8gqs52equc5phn980d78p2c8q7t2pwl92eg4lflr92hmu9xl2za",
			CoinAsset:         common.RUNEAsset,
			// No swap generated
			CoinAmount: 10742789577,
			Memo:       "REFUND:E8FECCA6F59FA38EDB6BCF9D25B087813FE2C649BD5887A11EC57E612317A185",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/66BC2704E1A4B7D1883702048BCF75A5D7B8E13B842C9C0E5E0443FFC8187A9E
			OriginalTxIDString: "66BC2704E1A4B7D1883702048BCF75A5D7B8E13B842C9C0E5E0443FFC8187A9E",
			Chain:              common.ARBChain,
			ToAdressString:     "0xA76423FEF8b5c71C5B520959Ce051d6e1EDF39Fd",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.ATGTAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/block?height=8562082
			CoinAmount: 229900499584,
			Memo:       "OUT:66BC2704E1A4B7D1883702048BCF75A5D7B8E13B842C9C0E5E0443FFC8187A9E",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/55686A4FA44A8D6DA496BD06F21FFF346F7A73425B368CABC9EAE8800482DA09
			OriginalTxIDString: "55686A4FA44A8D6DA496BD06F21FFF346F7A73425B368CABC9EAE8800482DA09",
			Chain:              common.ARBChain,
			ToAdressString:     "0xb710dfa34726ceb4f29e8b8360ecbec805166102",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.ATGTAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/55686A4FA44A8D6DA496BD06F21FFF346F7A73425B368CABC9EAE8800482DA09
			CoinAmount: 927094023332,
			Memo:       "OUT:55686A4FA44A8D6DA496BD06F21FFF346F7A73425B368CABC9EAE8800482DA09",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/66A15C26E6391504569248B5FCC5128C8748AE7732F1FF99EFD91161D51F2111
			OriginalTxIDString: "66A15C26E6391504569248B5FCC5128C8748AE7732F1FF99EFD91161D51F2111",
			Chain:              common.ARBChain,
			ToAdressString:     "0xdb1982f55cd138d0de4d9380a1b60b0b8014f2a7",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/block?height=8562093
			CoinAmount: 41324850700,
			Memo:       "OUT:66A15C26E6391504569248B5FCC5128C8748AE7732F1FF99EFD91161D51F2111",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/C49A37659A6FD8C2AFB6D398469038154D1AFEA55845C17F547C5DDA90C30973
			OriginalTxIDString: "C49A37659A6FD8C2AFB6D398469038154D1AFEA55845C17F547C5DDA90C30973",
			Chain:              common.ARBChain,
			ToAdressString:     "0xc74d832ac65683fd5d29fe1ffa40d30514198a13",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/C49A37659A6FD8C2AFB6D398469038154D1AFEA55845C17F547C5DDA90C30973
			CoinAmount: 10104197600,
			Memo:       "OUT:C49A37659A6FD8C2AFB6D398469038154D1AFEA55845C17F547C5DDA90C30973",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/11CCF1F62F9953DD332C9612FAD477BDF53BDE35973EAF1F73643F4B39A9C684
			OriginalTxIDString: "11CCF1F62F9953DD332C9612FAD477BDF53BDE35973EAF1F73643F4B39A9C684",
			Chain:              common.ARBChain,
			ToAdressString:     "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/11CCF1F62F9953DD332C9612FAD477BDF53BDE35973EAF1F73643F4B39A9C684
			CoinAmount: 17114233600,
			Memo:       "OUT:11CCF1F62F9953DD332C9612FAD477BDF53BDE35973EAF1F73643F4B39A9C684",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/F15890A463C15FDC8E8AAA974B79F79297C25FA358154EDC37271819D35EA464
			OriginalTxIDString: "F15890A463C15FDC8E8AAA974B79F79297C25FA358154EDC37271819D35EA464",
			Chain:              common.ARBChain,
			ToAdressString:     "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/F15890A463C15FDC8E8AAA974B79F79297C25FA358154EDC37271819D35EA464
			CoinAmount: 19125777800,
			Memo:       "OUT:F15890A463C15FDC8E8AAA974B79F79297C25FA358154EDC37271819D35EA464",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/2F433D1BBC7B3E546EC0FC1FFB0CA22A2D8A0002952B4AE45CDE52286935F325
			OriginalTxIDString: "2F433D1BBC7B3E546EC0FC1FFB0CA22A2D8A0002952B4AE45CDE52286935F325",
			Chain:              common.ARBChain,
			ToAdressString:     "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/2F433D1BBC7B3E546EC0FC1FFB0CA22A2D8A0002952B4AE45CDE52286935F325
			CoinAmount: 15085301400,
			Memo:       "OUT:2F433D1BBC7B3E546EC0FC1FFB0CA22A2D8A0002952B4AE45CDE52286935F325",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/0D86E563BD05A8EBFB9FF7E7A5525ED86E65474CB74E02AB8ACCB612EEB73F8B
			OriginalTxIDString: "0D86E563BD05A8EBFB9FF7E7A5525ED86E65474CB74E02AB8ACCB612EEB73F8B",
			Chain:              common.ARBChain,
			ToAdressString:     "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/0D86E563BD05A8EBFB9FF7E7A5525ED86E65474CB74E02AB8ACCB612EEB73F8B
			CoinAmount: 9041381100,
			Memo:       "OUT:0D86E563BD05A8EBFB9FF7E7A5525ED86E65474CB74E02AB8ACCB612EEB73F8B",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/EC6BC550F10BF8DF052E054E964D1F5FD0643201D4D16F25CAD1BFA6D5FF09D9
			OriginalTxIDString: "EC6BC550F10BF8DF052E054E964D1F5FD0643201D4D16F25CAD1BFA6D5FF09D9",
			Chain:              common.ARBChain,
			ToAdressString:     "0xC74D832ac65683FD5D29FE1fFA40D30514198a13",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/EC6BC550F10BF8DF052E054E964D1F5FD0643201D4D16F25CAD1BFA6D5FF09D9
			CoinAmount: 200600229500,
			Memo:       "OUT:EC6BC550F10BF8DF052E054E964D1F5FD0643201D4D16F25CAD1BFA6D5FF09D9",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/644E1E9D7D5FDBE0E842B95F25170B7C9F29101FE177205DFD24CC69BB932190
			OriginalTxIDString: "644E1E9D7D5FDBE0E842B95F25170B7C9F29101FE177205DFD24CC69BB932190",
			Chain:              common.ARBChain,
			ToAdressString:     "0xA76423FEF8b5c71C5B520959Ce051d6e1EDF39Fd",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDTAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/644E1E9D7D5FDBE0E842B95F25170B7C9F29101FE177205DFD24CC69BB932190
			CoinAmount: 5035483800,
			Memo:       "OUT:644E1E9D7D5FDBE0E842B95F25170B7C9F29101FE177205DFD24CC69BB932190",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/F6B8AFAC4406CDF5F84A9372E01683DAE7BAF84FDA0AB7B73352C9118C489628
			OriginalTxIDString: "F6B8AFAC4406CDF5F84A9372E01683DAE7BAF84FDA0AB7B73352C9118C489628",
			Chain:              common.ARBChain,
			ToAdressString:     "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/F6B8AFAC4406CDF5F84A9372E01683DAE7BAF84FDA0AB7B73352C9118C489628
			CoinAmount: 9047886400,
			Memo:       "OUT:F6B8AFAC4406CDF5F84A9372E01683DAE7BAF84FDA0AB7B73352C9118C489628",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/1A057D5D2E21FD650CB2892D44AEB324BD133622442DC4B79ED31C2711C7B814
			OriginalTxIDString: "1A057D5D2E21FD650CB2892D44AEB324BD133622442DC4B79ED31C2711C7B814",
			Chain:              common.ARBChain,
			ToAdressString:     "0xf30aa4f9adecb8bb209f764d300cbf78341d5e55",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/1A057D5D2E21FD650CB2892D44AEB324BD133622442DC4B79ED31C2711C7B814
			CoinAmount: 20415127000,
			Memo:       "OUT:1A057D5D2E21FD650CB2892D44AEB324BD133622442DC4B79ED31C2711C7B814",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/4BCEBA54F1B1351B7C5A465B2C8C7BB0CC86577B239E0BDC7CD52D20820B8819
			OriginalTxIDString: "4BCEBA54F1B1351B7C5A465B2C8C7BB0CC86577B239E0BDC7CD52D20820B8819",
			Chain:              common.ARBChain,
			ToAdressString:     "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/4BCEBA54F1B1351B7C5A465B2C8C7BB0CC86577B239E0BDC7CD52D20820B8819
			CoinAmount: 17143243400,
			Memo:       "OUT:4BCEBA54F1B1351B7C5A465B2C8C7BB0CC86577B239E0BDC7CD52D20820B8819",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/15277AFA4C19CC91B9AFE40BAC9FD004B64E5A2C5A78ADAAAB6D7ABEC4031924
			OriginalTxIDString: "15277AFA4C19CC91B9AFE40BAC9FD004B64E5A2C5A78ADAAAB6D7ABEC4031924",
			Chain:              common.ARBChain,
			ToAdressString:     "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/15277AFA4C19CC91B9AFE40BAC9FD004B64E5A2C5A78ADAAAB6D7ABEC4031924
			CoinAmount: 17149050200,
			Memo:       "OUT:15277AFA4C19CC91B9AFE40BAC9FD004B64E5A2C5A78ADAAAB6D7ABEC4031924",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/D69498D769850DD7D569DF3E31195DA4E3C9589FDFD5A8C91500489DCBE6633E
			OriginalTxIDString: "D69498D769850DD7D569DF3E31195DA4E3C9589FDFD5A8C91500489DCBE6633E",
			Chain:              common.ARBChain,
			ToAdressString:     "0xc74d832ac65683fd5d29fe1ffa40d30514198a13",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/D69498D769850DD7D569DF3E31195DA4E3C9589FDFD5A8C91500489DCBE6633E
			CoinAmount: 200558412300,
			Memo:       "OUT:D69498D769850DD7D569DF3E31195DA4E3C9589FDFD5A8C91500489DCBE6633E",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/10755839A260A260BC82CDA6AF2DDBEFC9E8D8AE30017957BF1DADC8520DA23D
			OriginalTxIDString: "10755839A260A260BC82CDA6AF2DDBEFC9E8D8AE30017957BF1DADC8520DA23D",
			Chain:              common.ARBChain,
			ToAdressString:     "0xC74D832ac65683FD5D29FE1fFA40D30514198a13",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/10755839A260A260BC82CDA6AF2DDBEFC9E8D8AE30017957BF1DADC8520DA23D
			CoinAmount: 15083257500,
			Memo:       "OUT:10755839A260A260BC82CDA6AF2DDBEFC9E8D8AE30017957BF1DADC8520DA23D",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/BC92063FA2FADAFD6913E55A22D73838A80D8E041AF59C79E397C388E8E508AF
			OriginalTxIDString: "BC92063FA2FADAFD6913E55A22D73838A80D8E041AF59C79E397C388E8E508AF",
			Chain:              common.ARBChain,
			ToAdressString:     "0xC74D832ac65683FD5D29FE1fFA40D30514198a13",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/BC92063FA2FADAFD6913E55A22D73838A80D8E041AF59C79E397C388E8E508AF
			CoinAmount: 10005751000,
			Memo:       "OUT:BC92063FA2FADAFD6913E55A22D73838A80D8E041AF59C79E397C388E8E508AF",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/B8AC39350B95EB97C942968792A228CDFD7BA10A6CA0E976086C46DBAB8F8898
			OriginalTxIDString: "B8AC39350B95EB97C942968792A228CDFD7BA10A6CA0E976086C46DBAB8F8898",
			Chain:              common.ARBChain,
			ToAdressString:     "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/B8AC39350B95EB97C942968792A228CDFD7BA10A6CA0E976086C46DBAB8F8898
			CoinAmount: 19125753200,
			Memo:       "OUT:B8AC39350B95EB97C942968792A228CDFD7BA10A6CA0E976086C46DBAB8F8898",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/6D18B4DBE9FE504A301E4FE4F2620AEDB8FAAB4B5CBDFE4B6CF7DE7B4EF1CFE1
			OriginalTxIDString: "6D18B4DBE9FE504A301E4FE4F2620AEDB8FAAB4B5CBDFE4B6CF7DE7B4EF1CFE1",
			Chain:              common.ARBChain,
			ToAdressString:     "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/6D18B4DBE9FE504A301E4FE4F2620AEDB8FAAB4B5CBDFE4B6CF7DE7B4EF1CFE1
			CoinAmount: 15104797600,
			Memo:       "OUT:6D18B4DBE9FE504A301E4FE4F2620AEDB8FAAB4B5CBDFE4B6CF7DE7B4EF1CFE1",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/31F1EB18E72F6A299BA8D9D6833106984B7774FE2FA20E0EF38DB653BBD7D111
			OriginalTxIDString: "31F1EB18E72F6A299BA8D9D6833106984B7774FE2FA20E0EF38DB653BBD7D111",
			Chain:              common.ARBChain,
			ToAdressString:     "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/31F1EB18E72F6A299BA8D9D6833106984B7774FE2FA20E0EF38DB653BBD7D111
			CoinAmount: 23081894100,
			Memo:       "OUT:31F1EB18E72F6A299BA8D9D6833106984B7774FE2FA20E0EF38DB653BBD7D111",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/121AB96A0DAD51664076BBFECBB546A1058CF39980379C839AE8198D6EC47F9D
			OriginalTxIDString: "121AB96A0DAD51664076BBFECBB546A1058CF39980379C839AE8198D6EC47F9D",
			Chain:              common.ARBChain,
			ToAdressString:     "0x62ef56f12bc003344cd3095499a2922e528b10ee",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/121AB96A0DAD51664076BBFECBB546A1058CF39980379C839AE8198D6EC47F9D
			CoinAmount: 36991502700,
			Memo:       "OUT:121AB96A0DAD51664076BBFECBB546A1058CF39980379C839AE8198D6EC47F9D",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/6C395853CFDE5846604CD53525A91FB0AA1006B339CF01476A2131452096F504
			OriginalTxIDString: "6C395853CFDE5846604CD53525A91FB0AA1006B339CF01476A2131452096F504",
			Chain:              common.ARBChain,
			ToAdressString:     "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/6C395853CFDE5846604CD53525A91FB0AA1006B339CF01476A2131452096F504
			CoinAmount: 17117410500,
			Memo:       "OUT:6C395853CFDE5846604CD53525A91FB0AA1006B339CF01476A2131452096F504",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/1BACEFBF8CF70671166819C28BC7B1AD265F86965CBA60264F75B0902903C30B
			OriginalTxIDString: "1BACEFBF8CF70671166819C28BC7B1AD265F86965CBA60264F75B0902903C30B",
			Chain:              common.ARBChain,
			ToAdressString:     "0x60b6dd58688e13a624f90d788f3c8c9167a3bb57",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.ATGTAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/1BACEFBF8CF70671166819C28BC7B1AD265F86965CBA60264F75B0902903C30B
			CoinAmount: 1311909227002,
			Memo:       "OUT:1BACEFBF8CF70671166819C28BC7B1AD265F86965CBA60264F75B0902903C30B",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/BF660A442512C93C70FA15672F0504046A1C7CEB6A29FCE403CF0EA6257587F7
			OriginalTxIDString: "BF660A442512C93C70FA15672F0504046A1C7CEB6A29FCE403CF0EA6257587F7",
			Chain:              common.ARBChain,
			ToAdressString:     "0xA76423FEF8b5c71C5B520959Ce051d6e1EDF39Fd",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDTAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/BF660A442512C93C70FA15672F0504046A1C7CEB6A29FCE403CF0EA6257587F7
			CoinAmount: 5036629300,
			Memo:       "OUT:BF660A442512C93C70FA15672F0504046A1C7CEB6A29FCE403CF0EA6257587F7",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/4184C2B9822F8DED13E6DFB93BB7E941CC051CDDB530861336A72D1C6C588843
			OriginalTxIDString: "4184C2B9822F8DED13E6DFB93BB7E941CC051CDDB530861336A72D1C6C588843",
			Chain:              common.ARBChain,
			ToAdressString:     "0xA76423FEF8b5c71C5B520959Ce051d6e1EDF39Fd",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.ATGTAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/4184C2B9822F8DED13E6DFB93BB7E941CC051CDDB530861336A72D1C6C588843
			CoinAmount: 222917824923,
			Memo:       "OUT:4184C2B9822F8DED13E6DFB93BB7E941CC051CDDB530861336A72D1C6C588843",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/B14D412920BE7073283036058BB70D3943F3956A617A8B4471C5748B8501BA8B
			OriginalTxIDString: "B14D412920BE7073283036058BB70D3943F3956A617A8B4471C5748B8501BA8B",
			Chain:              common.ARBChain,
			ToAdressString:     "0x592ee3398989f7a9c366e6a7f26f8f14f3666021",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.APEPEAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/B14D412920BE7073283036058BB70D3943F3956A617A8B4471C5748B8501BA8B
			CoinAmount: 767967999046691,
			Memo:       "OUT:B14D412920BE7073283036058BB70D3943F3956A617A8B4471C5748B8501BA8B",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/7F1F8116061A9241959D59F8803B76087F1C17A3DBE832330B3C94D4E930765B
			OriginalTxIDString: "7F1F8116061A9241959D59F8803B76087F1C17A3DBE832330B3C94D4E930765B",
			Chain:              common.ARBChain,
			ToAdressString:     "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/7F1F8116061A9241959D59F8803B76087F1C17A3DBE832330B3C94D4E930765B
			CoinAmount: 12963230100,
			Memo:       "OUT:7F1F8116061A9241959D59F8803B76087F1C17A3DBE832330B3C94D4E930765B",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/FF20D5AD64DD10318AC02B143A03EE36A5C92853EC1D027DAF3DCBD7B5527DB0
			OriginalTxIDString: "FF20D5AD64DD10318AC02B143A03EE36A5C92853EC1D027DAF3DCBD7B5527DB0",
			Chain:              common.ARBChain,
			ToAdressString:     "0xA76423FEF8b5c71C5B520959Ce051d6e1EDF39Fd",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.ATGTAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/FF20D5AD64DD10318AC02B143A03EE36A5C92853EC1D027DAF3DCBD7B5527DB0
			CoinAmount: 222385829417,
			Memo:       "OUT:FF20D5AD64DD10318AC02B143A03EE36A5C92853EC1D027DAF3DCBD7B5527DB0",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/76E0BEE65A1A074340D16E272D1A910515E61A2D1C7D0E9858F100A9BA79629F
			OriginalTxIDString: "76E0BEE65A1A074340D16E272D1A910515E61A2D1C7D0E9858F100A9BA79629F",
			Chain:              common.ARBChain,
			ToAdressString:     "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/76E0BEE65A1A074340D16E272D1A910515E61A2D1C7D0E9858F100A9BA79629F
			CoinAmount: 11011710700,
			Memo:       "OUT:76E0BEE65A1A074340D16E272D1A910515E61A2D1C7D0E9858F100A9BA79629F",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/6D976132C671838FFB7EDAF262E75D805AA9BD5CB2A13557EDBDFE32397409E2
			OriginalTxIDString: "6D976132C671838FFB7EDAF262E75D805AA9BD5CB2A13557EDBDFE32397409E2",
			Chain:              common.ARBChain,
			ToAdressString:     "0xBE4dE5797fe077b5FEE393243dCc2654ae8D8B62",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          arbGLDAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/6D976132C671838FFB7EDAF262E75D805AA9BD5CB2A13557EDBDFE32397409E2
			CoinAmount: 1537998614631,
			Memo:       "OUT:6D976132C671838FFB7EDAF262E75D805AA9BD5CB2A13557EDBDFE32397409E2",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/6CEF2B3E8CD88136D4241936ADA625AC0B09492E104309CE3321B32860589059
			OriginalTxIDString: "6CEF2B3E8CD88136D4241936ADA625AC0B09492E104309CE3321B32860589059",
			Chain:              common.ARBChain,
			ToAdressString:     "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/6CEF2B3E8CD88136D4241936ADA625AC0B09492E104309CE3321B32860589059
			CoinAmount: 11031041600,
			Memo:       "OUT:6CEF2B3E8CD88136D4241936ADA625AC0B09492E104309CE3321B32860589059",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/6A8C2CCFA3BBDE20AC94FA5654ECD644C31F93413104CA64DA90D5E802A187D2
			OriginalTxIDString: "6A8C2CCFA3BBDE20AC94FA5654ECD644C31F93413104CA64DA90D5E802A187D2",
			Chain:              common.ARBChain,
			ToAdressString:     "0xb710dfa34726ceb4f29e8b8360ecbec805166102",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          arbYumAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/6A8C2CCFA3BBDE20AC94FA5654ECD644C31F93413104CA64DA90D5E802A187D2
			CoinAmount: 585943110069,
			Memo:       "OUT:6A8C2CCFA3BBDE20AC94FA5654ECD644C31F93413104CA64DA90D5E802A187D2",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/492E5B36D21A6A905FC3BCD0C6B7285DE6C457B6A0057C5D7F3C7D81545CBF4E
			OriginalTxIDString: "492E5B36D21A6A905FC3BCD0C6B7285DE6C457B6A0057C5D7F3C7D81545CBF4E",
			Chain:              common.ARBChain,
			ToAdressString:     "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/492E5B36D21A6A905FC3BCD0C6B7285DE6C457B6A0057C5D7F3C7D81545CBF4E
			CoinAmount: 11023678600,
			Memo:       "OUT:492E5B36D21A6A905FC3BCD0C6B7285DE6C457B6A0057C5D7F3C7D81545CBF4E",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/11837E67CEFB51423B1BCF9F3FFAB4EF1DBF5209FD670AA5262EEBCC1738CB46
			OriginalTxIDString: "11837E67CEFB51423B1BCF9F3FFAB4EF1DBF5209FD670AA5262EEBCC1738CB46",
			Chain:              common.ARBChain,
			ToAdressString:     "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/11837E67CEFB51423B1BCF9F3FFAB4EF1DBF5209FD670AA5262EEBCC1738CB46
			CoinAmount: 12984442200,
			Memo:       "OUT:11837E67CEFB51423B1BCF9F3FFAB4EF1DBF5209FD670AA5262EEBCC1738CB46",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/C7D859113FD5090833A89A287E8871F84A994E5BFAC2A3A2B8D9174F06E5B759
			OriginalTxIDString: "C7D859113FD5090833A89A287E8871F84A994E5BFAC2A3A2B8D9174F06E5B759",
			Chain:              common.ARBChain,
			ToAdressString:     "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/C7D859113FD5090833A89A287E8871F84A994E5BFAC2A3A2B8D9174F06E5B759
			CoinAmount: 15107471600,
			Memo:       "OUT:C7D859113FD5090833A89A287E8871F84A994E5BFAC2A3A2B8D9174F06E5B759",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/F2DDAA0E84BEDB87D9E72825021AAD30451F4E3F305AAA1B73E0DAF7FFB5C550
			OriginalTxIDString: "F2DDAA0E84BEDB87D9E72825021AAD30451F4E3F305AAA1B73E0DAF7FFB5C550",
			Chain:              common.ARBChain,
			ToAdressString:     "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          common.AUSDCAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/F2DDAA0E84BEDB87D9E72825021AAD30451F4E3F305AAA1B73E0DAF7FFB5C550
			CoinAmount: 17049194700,
			Memo:       "OUT:F2DDAA0E84BEDB87D9E72825021AAD30451F4E3F305AAA1B73E0DAF7FFB5C550",
		},
		{
			// https://mayanode.mayachain.info/mayachain/tx/details/CE1CB29E21A33CFCE13342395CF8DDC895A11DC7FD929AEDA82A5D7ED95D12BE
			OriginalTxIDString: "CE1CB29E21A33CFCE13342395CF8DDC895A11DC7FD929AEDA82A5D7ED95D12BE",
			Chain:              common.ARBChain,
			ToAdressString:     "0x83ee188b5f8c0d947a3894c4054786f100ef9f93",
			VaultPubKeyString:  "mayapub1addwnpepqwzrjutpnck3j3dnhp68wqlm5fz2dn3jds8fh7s7agux0qpa8e9gv7u2usq",
			CoinAsset:          arbLeoAsset,
			// Check out value for https://mayanode.mayachain.info/mayachain/tx/details/CE1CB29E21A33CFCE13342395CF8DDC895A11DC7FD929AEDA82A5D7ED95D12BE
			CoinAmount: 474575200000,
			Memo:       "OUT:CE1CB29E21A33CFCE13342395CF8DDC895A11DC7FD929AEDA82A5D7ED95D12BE",
		},
	}

	for _, txNotProcessed := range txsNotProcessed {
		originalTxID := txNotProcessed.OriginalTxIDString
		maxGas, err := mgr.gasMgr.GetMaxGas(ctx, txNotProcessed.Chain)
		if err != nil {
			ctx.Logger().Error("unable to GetMaxGas while retrying", "txID:", originalTxID, "err", err)
		} else {
			gasRate := mgr.gasMgr.GetGasRate(ctx, txNotProcessed.Chain)
			droppedRescue := types.TxOutItem{
				Chain:       txNotProcessed.Chain,
				ToAddress:   common.Address(txNotProcessed.ToAdressString),
				VaultPubKey: common.PubKey(txNotProcessed.VaultPubKeyString),
				Coin: common.NewCoin(
					txNotProcessed.CoinAsset,
					cosmos.NewUint(uint64(txNotProcessed.CoinAmount)),
				),
				Memo:    txNotProcessed.Memo,
				InHash:  common.TxID(originalTxID),
				GasRate: int64(gasRate.Uint64()),
				MaxGas:  common.Gas{maxGas},
			}

			ok, err := mgr.txOutStore.TryAddTxOutItem(ctx, mgr, droppedRescue, cosmos.ZeroUint())
			if err != nil {
				ctx.Logger().Error("fail to retry rescue tx", "txID:", originalTxID, "error", err)
			}
			if !ok {
				ctx.Logger().Error("TryAddTxOutItem didn't success", "txID:", originalTxID)
			} else {
				ctx.Logger().Info("TryAddTxOutItem succeed", "txID:", originalTxID)
			}
		}
	}
}

func migrateStoreV113(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v113", "error", err)
		}
	}()

	// Txs to validate
	// toAddr:      "thor1sucvdnzcf4j6ynep4n4skjpq8tvqv8ags3a4ky",
	// asset:       "THOR.RUNE",
	// amount:      199757582857,
	// inboundHash: "80559CC3CCF2665531AAA7DD6B59F986721C6B76F1DD056DAE58DCC4878C5D56",
	//
	// toAddr:      "0xAa287489e76B11B56dBa7ca03e155369400f3d65",
	// asset:       "ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48",
	// amount:      199757582857,
	// inboundHash: "6F4F5801E5BEA96BC521BB00A7542EACB5FBDC161FD43431FEF3245D93F9C0AB",
	//
	// toAddr:      "thor1qvlul0ujfrq27ja7uxrp8r7my9juegz0ug3nsg",
	// asset:       "THOR.RUNE",
	// amount:      10342158343,
	// inboundHash: "9A823D4E90110A9306879865405E47F609ADCAAC1672223F403390F060121435",
	//
	// toAddr:      "thor1qvlul0ujfrq27ja7uxrp8r7my9juegz0ug3nsg",
	// asset:       "THOR.RUNE",
	// amount:      10742789577,
	// inboundHash: "E8FECCA6F59FA38EDB6BCF9D25B087813FE2C649BD5887A11EC57E612317A185",
	//
	// toAddr:      "0xA76423FEF8b5c71C5B520959Ce051d6e1EDF39Fd",
	// asset:       "ARB.TGT-0X429FED88F10285E61B12BDF00848315FBDFCC341",
	// amount:      229900499584,
	// inboundHash: "66BC2704E1A4B7D1883702048BCF75A5D7B8E13B842C9C0E5E0443FFC8187A9E",
	//
	// toAddr:      "0xb710dfa34726ceb4f29e8b8360ecbec805166102",
	// asset:       "ARB.TGT-0X429FED88F10285E61B12BDF00848315FBDFCC341",
	// amount:      927094023332,
	// inboundHash: "55686A4FA44A8D6DA496BD06F21FFF346F7A73425B368CABC9EAE8800482DA09",
	//
	// toAddr:      "0xdb1982f55cd138d0de4d9380a1b60b0b8014f2a7",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      41324850700,
	// inboundHash: "66A15C26E6391504569248B5FCC5128C8748AE7732F1FF99EFD91161D51F2111",
	//
	// toAddr:      "0xc74d832ac65683fd5d29fe1ffa40d30514198a13",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      10104197600,
	// inboundHash: "C49A37659A6FD8C2AFB6D398469038154D1AFEA55845C17F547C5DDA90C30973",
	//
	// toAddr:      "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      17114233600,
	// inboundHash: "11CCF1F62F9953DD332C9612FAD477BDF53BDE35973EAF1F73643F4B39A9C684",
	//
	// toAddr:      "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      19125777800,
	// inboundHash: "F15890A463C15FDC8E8AAA974B79F79297C25FA358154EDC37271819D35EA464",
	//
	// toAddr:      "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      15085301400,
	// inboundHash: "2F433D1BBC7B3E546EC0FC1FFB0CA22A2D8A0002952B4AE45CDE52286935F325",
	//
	// toAddr:      "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      9041381100,
	// inboundHash: "0D86E563BD05A8EBFB9FF7E7A5525ED86E65474CB74E02AB8ACCB612EEB73F8B",
	//
	// toAddr:      "0xC74D832ac65683FD5D29FE1fFA40D30514198a13",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      200600229500,
	// inboundHash: "EC6BC550F10BF8DF052E054E964D1F5FD0643201D4D16F25CAD1BFA6D5FF09D9",
	//
	// toAddr:      "0xA76423FEF8b5c71C5B520959Ce051d6e1EDF39Fd",
	// asset:       "ARB.USDT-0XFD086BC7CD5C481DCC9C85EBE478A1C0B69FCBB9",
	// amount:      5035483800,
	// inboundHash: "644E1E9D7D5FDBE0E842B95F25170B7C9F29101FE177205DFD24CC69BB932190",
	//
	// toAddr:      "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      9047886400,
	// inboundHash: "F6B8AFAC4406CDF5F84A9372E01683DAE7BAF84FDA0AB7B73352C9118C489628",
	//
	// toAddr:      "0xf30aa4f9adecb8bb209f764d300cbf78341d5e55",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      20415127000,
	// inboundHash: "1A057D5D2E21FD650CB2892D44AEB324BD133622442DC4B79ED31C2711C7B814",
	//
	// toAddr:      "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      17143243400,
	// inboundHash: "4BCEBA54F1B1351B7C5A465B2C8C7BB0CC86577B239E0BDC7CD52D20820B8819",
	//
	// toAddr:      "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      17149050200,
	// inboundHash: "15277AFA4C19CC91B9AFE40BAC9FD004B64E5A2C5A78ADAAAB6D7ABEC4031924",
	//
	// toAddr:      "0xc74d832ac65683fd5d29fe1ffa40d30514198a13",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      200558412300,
	// inboundHash: "D69498D769850DD7D569DF3E31195DA4E3C9589FDFD5A8C91500489DCBE6633E",
	//
	// toAddr:      "0xC74D832ac65683FD5D29FE1fFA40D30514198a13",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      15083257500,
	// inboundHash: "10755839A260A260BC82CDA6AF2DDBEFC9E8D8AE30017957BF1DADC8520DA23D",
	//
	// toAddr:      "0xC74D832ac65683FD5D29FE1fFA40D30514198a13",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      10005751000,
	// inboundHash: "BC92063FA2FADAFD6913E55A22D73838A80D8E041AF59C79E397C388E8E508AF",
	//
	// toAddr:      "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      19125753200,
	// inboundHash: "B8AC39350B95EB97C942968792A228CDFD7BA10A6CA0E976086C46DBAB8F8898",
	//
	// toAddr:      "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      15104797600,
	// inboundHash: "6D18B4DBE9FE504A301E4FE4F2620AEDB8FAAB4B5CBDFE4B6CF7DE7B4EF1CFE1",
	//
	// toAddr:      "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      36991502700,
	// inboundHash: "121AB96A0DAD51664076BBFECBB546A1058CF39980379C839AE8198D6EC47F9D",
	//
	// toAddr:      "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      17117410500,
	// inboundHash: "6C395853CFDE5846604CD53525A91FB0AA1006B339CF01476A2131452096F504",
	//
	// toAddr:      "0x60b6dd58688e13a624f90d788f3c8c9167a3bb57",
	// asset:       "ARB.TGT-0X429FED88F10285E61B12BDF00848315FBDFCC341",
	// amount:      1311909227002,
	// inboundHash: "1BACEFBF8CF70671166819C28BC7B1AD265F86965CBA60264F75B0902903C30B",
	//
	// toAddr:      "0xA76423FEF8b5c71C5B520959Ce051d6e1EDF39Fd",
	// asset:       "ARB.USDT-0XFD086BC7CD5C481DCC9C85EBE478A1C0B69FCBB9",
	// amount:      5036629300,
	// inboundHash: "BF660A442512C93C70FA15672F0504046A1C7CEB6A29FCE403CF0EA6257587F7",
	//
	// toAddr:      "0xA76423FEF8b5c71C5B520959Ce051d6e1EDF39Fd",
	// asset:       "ARB.TGT-0X429FED88F10285E61B12BDF00848315FBDFCC341",
	// amount:      222917824923,
	// inboundHash: "4184C2B9822F8DED13E6DFB93BB7E941CC051CDDB530861336A72D1C6C588843",
	//
	// toAddr:      "0x592ee3398989f7a9c366e6a7f26f8f14f3666021",
	// asset:       "ARB.PEPE-0X25D887CE7A35172C62FEBFD67A1856F20FAEBB00",
	// amount:      767967999046691,
	// inboundHash: "B14D412920BE7073283036058BB70D3943F3956A617A8B4471C5748B8501BA8B",
	//
	// toAddr:      "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      12963230100,
	// inboundHash: "7F1F8116061A9241959D59F8803B76087F1C17A3DBE832330B3C94D4E930765B",
	//
	// toAddr:      "0xA76423FEF8b5c71C5B520959Ce051d6e1EDF39Fd",
	// asset:       "ARB.TGT-0X429FED88F10285E61B12BDF00848315FBDFCC341",
	// amount:      222385829417,
	// inboundHash: "FF20D5AD64DD10318AC02B143A03EE36A5C92853EC1D027DAF3DCBD7B5527DB0",
	//
	// toAddr:      "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      11011710700,
	// inboundHash: "76E0BEE65A1A074340D16E272D1A910515E61A2D1C7D0E9858F100A9BA79629F",
	//
	// toAddr:      "0xBE4dE5797fe077b5FEE393243dCc2654ae8D8B62",
	// asset:       "ARB.GLD-0XAFD091F140C21770F4E5D53D26B2859AE97555AA1",
	// amount:      1537998614631,
	// inboundHash: "6D976132C671838FFB7EDAF262E75D805AA9BD5CB2A13557EDBDFE32397409E2",
	//
	// toAddr:      "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      11031041600,
	// inboundHash: "6CEF2B3E8CD88136D4241936ADA625AC0B09492E104309CE3321B32860589059",
	//
	// toAddr:      "0xb710dfa34726ceb4f29e8b8360ecbec805166102",
	// asset:       "ARB.YUM-0X9F41B34F42058A7B74672055A5FAE22C4B113FD1",
	// amount:      585943110069,
	// inboundHash: "6A8C2CCFA3BBDE20AC94FA5654ECD644C31F93413104CA64DA90D5E802A187D2",
	//
	// toAddr:      "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      11023678600,
	// inboundHash: "492E5B36D21A6A905FC3BCD0C6B7285DE6C457B6A0057C5D7F3C7D81545CBF4E",
	//
	// toAddr:      "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      12984442200,
	// inboundHash: "11837E67CEFB51423B1BCF9F3FFAB4EF1DBF5209FD670AA5262EEBCC1738CB46",
	//
	// toAddr:      "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      15107471600,
	// inboundHash: "C7D859113FD5090833A89A287E8871F84A994E5BFAC2A3A2B8D9174F06E5B759",
	//
	// toAddr:      "0xd1f7112354055160d58fa1b1e7cfd15c0bfee464",
	// asset:       "ARB.USDC-0XAF88D065E77C8CC2239327C5EDB3A432268E5831",
	// amount:      17049194700,
	// inboundHash: "F2DDAA0E84BEDB87D9E72825021AAD30451F4E3F305AAA1B73E0DAF7FFB5C550",
	//
	// toAddr:      "0x83ee188b5f8c0d947a3894c4054786f100ef9f93",
	// asset:       "ARB.LEO-0X93864D81175095DD93360FFA2A529B8642F76A6E",
	// amount:      474575200000,
	// inboundHash: "CE1CB29E21A33CFCE13342395CF8DDC895A11DC7FD929AEDA82A5D7ED95D12BE",
	//
	//  Txs from https://gitlab.com/mayachain/mayanode/-/issues/157
	// toAddr:      "0xa76423fef8b5c71c5b520959ce051d6e1edf39fd",
	// asset:       "ARB.TGT-0X429FED88F10285E61B12BDF00848315FBDFCC341",
	// amount:      206618358761,
	// inboundHash: "B51E75334F2A8E4351D5F4C14C72960C04B9170165FC2F9CD3836BFA70C334FF",
	//
	// toAddr:      "0xa76423fef8b5c71c5b520959ce051d6e1edf39fd",
	// asset:       "ARB.PEPE-0X25D887CE7A35172C62FEBFD67A1856F20FAEBB00",
	// amount:      500090951389095,
	// inboundHash: "F3F29996833DC30D9883CB6BCCFC24BC5D1BD47FCDEF56EFF18602AF73864E54",
	//
	// toAddr:      "0xa76423fef8b5c71c5b520959ce051d6e1edf39fd",
	// asset:       "ARB.PEPE-0X25D887CE7A35172C62FEBFD67A1856F20FAEBB00",
	// amount:      501218019660975,
	// inboundHash: "C2B6606E3D8D13717A626561FF735915E86BC86C8DCF641D1CC58953A598210A",
	//
	// toAddr:      "0xa76423fef8b5c71c5b520959ce051d6e1edf39fd",
	// asset:       "ARB.PEPE-0X25D887CE7A35172C62FEBFD67A1856F20FAEBB00",
	// amount:      501350768497079,
	// inboundHash: "1B57C24F843468BF53960964577CBA962C32B80EF9FCC48F08E6C05E3A49B7BA",
	//
	// toAddr:      "0xa76423fef8b5c71c5b520959ce051d6e1edf39fd",
	// asset:       "ARB.TGT-0X429FED88F10285E61B12BDF00848315FBDFCC341",
	// amount:      207055905229,
	// inboundHash: "F6D47F710860376C91B47428407C563AA28E70F14C3F9341C7406ED182A281A3",
	//
	// toAddr:      "account_rdx12xfczl7weps0v3dhcje648et2x32tsqu8expu9eapec8fzr48dum7a",
	// asset:       "XRD.XRD",
	// amount:      1211507479036,
	// inboundHash: "065B5709CCD9EBE6DA9E0FFCE1F638C0CABE1A3277E9796E1BC9540E6BF2EEC4",
	//
	// toAddr:      "thor1aqqn36tf54tjj6qtr79ke5methxvvx8g6g0tyu",
	// asset:       "THOR.RUNE",
	// amount:      50627786940,
	// inboundHash: "0A5A4A151F8C64B67E14CDB6040FAFA1DBBAF323F9D00C6231552092B8CA93DF",

	txIds := common.TxIDs{
		"065B5709CCD9EBE6DA9E0FFCE1F638C0CABE1A3277E9796E1BC9540E6BF2EEC4",
		"0A5A4A151F8C64B67E14CDB6040FAFA1DBBAF323F9D00C6231552092B8CA93DF",
		"0D86E563BD05A8EBFB9FF7E7A5525ED86E65474CB74E02AB8ACCB612EEB73F8B",
		"10755839A260A260BC82CDA6AF2DDBEFC9E8D8AE30017957BF1DADC8520DA23D",
		"11837E67CEFB51423B1BCF9F3FFAB4EF1DBF5209FD670AA5262EEBCC1738CB46",
		"11CCF1F62F9953DD332C9612FAD477BDF53BDE35973EAF1F73643F4B39A9C684",
		"121AB96A0DAD51664076BBFECBB546A1058CF39980379C839AE8198D6EC47F9D",
		"15277AFA4C19CC91B9AFE40BAC9FD004B64E5A2C5A78ADAAAB6D7ABEC4031924",
		"1A057D5D2E21FD650CB2892D44AEB324BD133622442DC4B79ED31C2711C7B814",
		"1B57C24F843468BF53960964577CBA962C32B80EF9FCC48F08E6C05E3A49B7BA",
		"1BACEFBF8CF70671166819C28BC7B1AD265F86965CBA60264F75B0902903C30B",
		"2F433D1BBC7B3E546EC0FC1FFB0CA22A2D8A0002952B4AE45CDE52286935F325",
		"4184C2B9822F8DED13E6DFB93BB7E941CC051CDDB530861336A72D1C6C588843",
		"492E5B36D21A6A905FC3BCD0C6B7285DE6C457B6A0057C5D7F3C7D81545CBF4E",
		"4BCEBA54F1B1351B7C5A465B2C8C7BB0CC86577B239E0BDC7CD52D20820B8819",
		"55686A4FA44A8D6DA496BD06F21FFF346F7A73425B368CABC9EAE8800482DA09",
		"644E1E9D7D5FDBE0E842B95F25170B7C9F29101FE177205DFD24CC69BB932190",
		"66A15C26E6391504569248B5FCC5128C8748AE7732F1FF99EFD91161D51F2111",
		"66BC2704E1A4B7D1883702048BCF75A5D7B8E13B842C9C0E5E0443FFC8187A9E",
		"6A8C2CCFA3BBDE20AC94FA5654ECD644C31F93413104CA64DA90D5E802A187D2",
		"6C395853CFDE5846604CD53525A91FB0AA1006B339CF01476A2131452096F504",
		"6CEF2B3E8CD88136D4241936ADA625AC0B09492E104309CE3321B32860589059",
		"6D18B4DBE9FE504A301E4FE4F2620AEDB8FAAB4B5CBDFE4B6CF7DE7B4EF1CFE1",
		"6D976132C671838FFB7EDAF262E75D805AA9BD5CB2A13557EDBDFE32397409E2",
		"6F4F5801E5BEA96BC521BB00A7542EACB5FBDC161FD43431FEF3245D93F9C0AB",
		"76E0BEE65A1A074340D16E272D1A910515E61A2D1C7D0E9858F100A9BA79629F",
		"7F1F8116061A9241959D59F8803B76087F1C17A3DBE832330B3C94D4E930765B",
		"80559CC3CCF2665531AAA7DD6B59F986721C6B76F1DD056DAE58DCC4878C5D56",
		"9A823D4E90110A9306879865405E47F609ADCAAC1672223F403390F060121435",
		"B14D412920BE7073283036058BB70D3943F3956A617A8B4471C5748B8501BA8B",
		"B51E75334F2A8E4351D5F4C14C72960C04B9170165FC2F9CD3836BFA70C334FF",
		"B8AC39350B95EB97C942968792A228CDFD7BA10A6CA0E976086C46DBAB8F8898",
		"BC92063FA2FADAFD6913E55A22D73838A80D8E041AF59C79E397C388E8E508AF",
		"BF660A442512C93C70FA15672F0504046A1C7CEB6A29FCE403CF0EA6257587F7",
		"C2B6606E3D8D13717A626561FF735915E86BC86C8DCF641D1CC58953A598210A",
		"C49A37659A6FD8C2AFB6D398469038154D1AFEA55845C17F547C5DDA90C30973",
		"C7D859113FD5090833A89A287E8871F84A994E5BFAC2A3A2B8D9174F06E5B759",
		"CE1CB29E21A33CFCE13342395CF8DDC895A11DC7FD929AEDA82A5D7ED95D12BE",
		"D69498D769850DD7D569DF3E31195DA4E3C9589FDFD5A8C91500489DCBE6633E",
		"E8FECCA6F59FA38EDB6BCF9D25B087813FE2C649BD5887A11EC57E612317A185",
		"EC6BC550F10BF8DF052E054E964D1F5FD0643201D4D16F25CAD1BFA6D5FF09D9",
		"F15890A463C15FDC8E8AAA974B79F79297C25FA358154EDC37271819D35EA464",
		"F2DDAA0E84BEDB87D9E72825021AAD30451F4E3F305AAA1B73E0DAF7FFB5C550",
		"F3F29996833DC30D9883CB6BCCFC24BC5D1BD47FCDEF56EFF18602AF73864E54",
		"F6B8AFAC4406CDF5F84A9372E01683DAE7BAF84FDA0AB7B73352C9118C489628",
		"F6D47F710860376C91B47428407C563AA28E70F14C3F9341C7406ED182A281A3",
		"FF20D5AD64DD10318AC02B143A03EE36A5C92853EC1D027DAF3DCBD7B5527DB0",
	}

	// Observed, but not paid txs
	observedTxIds := common.TxIDs{
		"94F84DD211C26A9801110E84F7BFA2EF23118FBB71E7EE52882A016A65BFDFD4",
		"431A6446957894DF77FA4220844990898F96BF46E1C8752910386A299AF72455",
	}

	for _, observedTxID := range observedTxIds {
		voter, err := mgr.K.GetObservedTxInVoter(ctx, observedTxID)
		if err != nil {
			ctx.Logger().Error("fail to get observed tx in voter", "error", err)
			continue
		}

		if len(voter.OutTxs) == 0 {
			continue
		}

		outboundTxID := voter.OutTxs[0].ID
		outVoter, err := mgr.K.GetObservedTxOutVoter(ctx, outboundTxID)
		if err != nil {
			ctx.Logger().Error("fail to get observed tx out voter", "error", err)
			continue
		}

		outVoter.SetReverted()
		voter.OutTxs = nil

		activeAsgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
		if err != nil || len(activeAsgards) == 0 {
			ctx.Logger().Error("fail to get active asgard vaults", "error", err)
			return
		}

		// we actually know there's only one active asgard
		if len(voter.Actions) > 0 {
			coin := voter.Actions[0].Coin
			gas := voter.Actions[0].MaxGas.ToCoins().GetCoin(coin.Asset).Amount
			coin.Amount = coin.Amount.Add(gas)
			activeAsgards[0].AddFunds(common.Coins{coin})

			// add the amount back to the vault
			if err := mgr.Keeper().SetVault(ctx, activeAsgards[0]); err != nil {
				ctx.Logger().Error("fail to save asgard vault", "error", err, "hash", observedTxID)
			}
			mgr.K.SetObservedTxOutVoter(ctx, outVoter)
			mgr.K.SetObservedTxInVoter(ctx, voter)

			txIds = append(txIds, observedTxID)
		}
	}

	requeueDanglingActions(ctx, mgr, txIds)
}

func migrateStoreV114(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v114", "error", err)
		}
	}()

	activeAsgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil || len(activeAsgards) == 0 {
		ctx.Logger().Error("fail to get active asgard vaults", "error", err)
		return
	}

	// https://mayanode.mayachain.info/mayachain/tx/details/83AEC95CE5BC2B4AE8835B23DE57ACDF14CC3B30B00095ADB9D7278840CABD2D
	coin := common.NewCoin(common.DASHAsset, cosmos.NewUint(616040018514))
	activeAsgards[0].AddFunds(common.Coins{coin})
	if err := mgr.Keeper().SetVault(ctx, activeAsgards[0]); err != nil {
		ctx.Logger().Error("fail to save asgard vault", "error", err)
	}
}

func migrateStoreV115(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v115", "error", err)
		}
	}()

	unobservedTxs := ObservedTxs{}

	// https://www.blockchain.com/explorer/transactions/btc/7470b7703cfba1909c7fde0133313d2dd9bf6ccf334abf55b021fc81f3db5710
	// Fake observation will use the from address as the sender for refund.
	fromAddr, err := common.NewAddress("bc1qdkjek65xwlxsc02h8nezpa69dglcz72aqerghg", mgr.Keeper().GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to create addr", "addr", fromAddr.String(), "error", err)
		return
	}
	toAddress, err := common.NewAddress("bc1qc9ccu7anhjvytcc4zc0qk5al9nlgyhqnq3nwfn", mgr.Keeper().GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to create addr", "addr", toAddress.String(), "error", err)
		return
	}
	toAddressPubKey, err := common.NewPubKey("mayapub1addwnpepqw0anseu8gqs52equc5phn980d78p2c8q7t2pwl92eg4lflr92hmu9xl2za")
	if err != nil {
		ctx.Logger().Error("fail to create pubkey for vault", "addr", toAddress.String(), "error", err)
		return
	}
	unobservedTxs = append(
		unobservedTxs,
		NewObservedTx(common.Tx{
			ID:          "7470b7703cfba1909c7fde0133313d2dd9bf6ccf334abf55b021fc81f3db5710",
			Chain:       common.BTCChain,
			FromAddress: fromAddr,
			ToAddress:   toAddress,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1244784),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: "",
		}, 867773, toAddressPubKey, 867773),
	)

	err = makeFakeTxInObservation(ctx, mgr, unobservedTxs)
	if err != nil {
		ctx.Logger().Error("failed to migrate v115", "error", err)
	}

	danglingInboundTxIDs := []common.TxID{
		// in resch-skipp
		"DD65729C655572F8AF294E0C5AC9FA564F363EA2B03ED487B47333A1C73B51AF",
		"93A4B71586277D2C69882C99B3048D3EEF5B148D5519C9ADE3F654F1E4D2E871",
		"023243E7C4B147517DC5E9BECF6DFEE3F3D10AAF67E9BB54409C1DE80959EE17",
		"99AE4F276E5D2979B89365D3FC0A8C5B3C4CE2A3379D270F9EC3D0CCBE177C19",
		"256E4618248EEB1A43FF64D85A3C39817F8FCDF96AB71E4C87ABBDD5406FDD3F",
		"DF4743FCA580FD485134BE953679DCE3E92346608985C84442AED67F4FE1A34E",
		"15921FEB3A6D294DA84E0D0F38A08CD0BC3BC8B80E01896C384296B00A179E08",
		"9C22D8A2DE61B308CADAFC1AEB4F7D2ABF5FEA3A0F0BFC45536EF50139A1AFB4",
		"3A0F86A05094B63CF54A6259A8D978CD47611E3E2A5D6C8F788303DDF0754558",
		"DC7CD3D2CB95252094E448442C801BF80DD56778D5ECF5B1F20233CECFDD228A",
		"77FA513D12520DC88BF19DD334CD436AD03AF4A70760310494D6A759B6A0DB96",
		"A8512017B0B81D7CF29E524CAC2C2A91098F82BFF7B2C749ACF813C9EBE3169D",
		"A15788EA7B0134845F4A7D4BE52127E695EAFDEE83C4D55486C5EFCA13DC8585",
		"4055CE5293A827F4980D1DA2F2A2DB94C0E1E19293F2FBA8B5E7C66B8F3A8D27",
		"0D7979A873EA2AEDAD50FC4CBBEDA9D4AFA89D2DED460EA90AF63D1EECA40800",
		"1078420D078B509A15E00A56547A0A0952DBBDDEC03D167D8228AEEF107FD2EE",
		"221F50EB88E4DD9A221246242184863B60EABFC2FE9037760071F347AAEB77D5",
		"6434810073E97A4552CEC17DA1743B040650FA1860C1E3C637A0FF9482D0E4CB",
		"1E6B1836F039CAC77E08C0382221779E380096C377C02406D5D74440A77BE506",
		"525A26BD97EDB94A140B5B36637195FA37A853C023B618E7EBE511F807EF6939",
		"56EA3453A3ED7B305B84A20E0B7E777F0D62F725DC4195218B0421870268778A",
		"650DCE16891A05C2D9EADEBE69DE32656EB10105C6584827F28E4723EED2DAFB",
		"70526C64896BA6E61268F97A8821AAA1028D29B268ABBB5054B3BAD46C89D7B9",
		"572A2EEC6DF34053534F6740362FBC409F3A1FBB296E3F468C95C1F3D96A9C54",
		"79E8839C87D6A200E079C714A80F647B9E313E87C3D1A77A3DE9798D555AE44D",
		"260714E80A3ED329F8462C489FE36B7DB7E89E190648121F7012C59E92727CBA",
		"9EA96071D52BC97465ED93B2635954837B7492596F686C3D0A08FBE7EC752D05",
		"8999827F5E4A013B6C40E01863E8F4AAD33E84693B33CB7CF5311EBDC197BFFE",
		"1F47E627C7E3F931FFF5CB95CBA2D501132B78F049E1F5268B5C16670AB17A55",
		"E62EB847CBC53E551D3D44CCFCC38CDECEA89BA2162D48D9E45787C51017DD61",
		"42756C3AB6D3B4698F7D8FDE4D22A3865DFD6174FD20E2FF4C95B3C5BD4A4A26",
		"84352BD190E8E8A1DE5D54FF38153FDE114CB3E61D4357323F8E5A60AB84A097",
		"A98539855A42C09B95C24D7683BE882CC4189BC1D2F7F955D8BEF2FE8FA5CE66",
		"A43BEBB13196B36980F84B11C1758FADF181E9662D510EC74984DE5274803830",
		"FD97E1CD576AA1DE4EF4E322DFC6D484040D246B305390566CC98D5831FBE75A",
		"6B9F1EB766E40FF14031B0E8B87F14E87DB28E5456F5D0B28B8E3D7191A77C45",
		"25505242C9429C62E3A8DF725EBCCFD6178848C2EEC397AA0B7F1BF322DAFE16",
		"C62BE07B5ABBFA94EA1B0C50502EB0D57A7FB88B2ED123E3AC74B065AADDC85E",
		"923A78808E45C777EADC1754A98FC837CF7D11C33DFDDF7C87B7780CB41861C9",
		"431A6446957894DF77FA4220844990898F96BF46E1C8752910386A299AF72455",
		"285C1A51DE9FB68B771AEAC45B709634045E136DA05FA6DF064FF83D6FC84080",
		"EEFC0811130E9457E79654E429C11D62FB9BDB00D80BFCEACCCF6EF3AA4EE590",
		"93813506F7378C3C61D1685B12920280F80D3F44144F19318C7A09066B32827C",
		"66159461B5AD6B4D88F6756C1684E5AD0ADE4FA39FF17668E23A9B98150CBE1F",
		"2F51220437D3C55F7990E1ACAF5C16C413C821C17BC4A3A9F22A38A72ECF38CB",
		"EBF67615C2B82A3CDF6909D23EF86357F893CC67B40E402AAB7B824EA9B75EF6",
		"788639DEB4821E98E89D4EAD5C13E4EF4941E957AC38F32108F1E57E96CC94F4",
		"16033390AB9EFE1A50F29DFADC73871CB7C3B177535D473076A84DAB27FADD02",
		"B733D5E6B926F4431A345F5E10DCA3F6C13C5FDDE335EEACDE6A2B3CA0774062",
		"9C15DCEB4A7E0D891B6ECFA4CB43121B626365F7EB5D089833C81DC578D36349",
		"B2E512BCE59D83A84C23572F9DBCF6A157E5E392A9084976169BA857D5DF9E1F",
		"8CB4F69E0E05E2C05803B04BE6A236C172FF6A6202D85029851C6B86347958BC",
		"FB060772BC572464EAE48CCEDB64C2DA873A5DF5E35FC6202F5C396BDEC8B710",
		"FFD2F54D12A2F2E3C660690C45D1B82526904509AF2C9C9A946F4EE69652FD30",
		"00DC92D65A91B775403923435FC70814F03B58D670D1DE9F2236320B945B9257",
		"082E9480478577F297B3931E0F6256BF5D31E0BE1ADA22C6625A45069D66029E",
		"880C5BCED5A77306534B4C6E4B4ACC6F00FDD0D5EC4C96439A6A72E35D93B1F0",
		"96659B209E6DAF38B352EF6AF84C4760827499F3AF51FDB694CA39B9106BD448",
		"29788CCAE9AEABFDB31E6B9FC396C71A574B6B1ADC65E5FCBD40DACAD6760E7A",
		"8C3F5334EFEAC924200A7D5CFF971AFAD562BED02B209516769F5FBC1D128121",
		"D460B420B90A398464B718F6AE3719D1645E6CD7C79A0BC5F2A28A97409AD78C",
		"225982D48A022CC53BCAEC38006E294BF403C4C596348A1477F645EFDCBC543B",
		"3E0A29483BB1BE4C78732E98BEDEE26E39C56F34505BA8A6CBD01E3B3181214B",
		"B7B0E90FA55D3359820D4E5974918CC8BC6DC7AE94FADDDDF8BC462E78DE40A1",
		"F2BD67761504D781D09912E663027C5169FA493E354731EA92A943F41607A0FE",
		"CE09A228D643841517C4E7B8958411F93D8A6237071C262B4762F7ECBBCFA67F",
		"4C3E2E8A99531EF8F8527C8D5B7F97F7F36270E0B335216EB369201BCAC45BA7",
		"AE340CCA6A898838903E0B78B7104B1C0475795BC5137395F242847288BB16FC",
		"1E8C34EF9C6D912C2D22DE61913EFEF069C67ABC0A06B171F9F1442AF7D88EF9",
		"3E21D9868AAF0B7FF0E6F09CF7F7AD2144632080C1DF4A23D1BFC84BC4D95847",
		"B18C61CAF023DFFCA5F2F85BFB4E46154C90088AF202D444C6BC50B8BA2EB153",
		"65FF8D8A0C7A5BD69D33CFAB6E2CF11F3CC061AF2003A9F7CDF39AA857034F66",
		"14CD2CA88EB6194EAF85860D7BADCA1BD1B481B2FFC162D7FD64F9BD79055605",
		"842BA0733AC1961545F53A5144DB176D6082BC1C4C02E552E899F3E8E97ACD4A",
		"181483A6B585DC33381A6B78357510CD5DB866834E44BF4336392EEEBCFAFE17",
		"915913174833D701BCF6D7D1DAAC3A4968DD6036DD9D3ECCC4590FDF3372DEF2",
		"5299E40797CAEA3CA3C5D7D44D7AC30723021F40980ED6B6F30EE3EB77054765",
		"DBB984CEC123B7A810C832781DAB5325511E8A2BA88610691597F9B26E7B8AF1",
		"8E77A7B0E2C07B408F5161D15D0CC7E61849FB1C1D84D400E31381C82F1243C7",
		"8017088481B4176AAFAA0DDE114E36BDF59DAF80B8062814DFA2B5B74BA46672",
		// arb
		"0FF900F3D412B0A7BC8E9714DF35E0F6DF2008F74EE8DCD3C512146719164FB4",
		"5CA0F973FB3545FC094ACCD78A115D5FB16F11E4232338DFEB08333C80893D56",
		"D1C1C7F11736F988239D81BEFEF14D458CE2C9EF60B53003DE5089FEF0A046C5",
		"09B45909DE70B5E186A0CFACC9E608ACAEE267B1DFDDD440BAE16257C112FEF7",
		"92C5D57B10FB8A043D461E1985A99534570CE626B19A8CF389DBC178C0BC9A36",
		"79060958426188EEBA06E744DFDCDB9EFB8314F6D77F7A8575A6B739258A86D8",
		"433D4EC400176CFB6A0189A3A77C2D0542BB6B8507FDE5C75A1FC85A3C322CFB",
		"AE3049BAA878BC6615863270938F39F5411E6403DFF60EB54CCC29CD3AABE092",
		"102BAB021328320F094BD5DCB919B76E483F9FE07BD2B3BBF39180B59BA06ACA",
		"F979E5E0332E3A56B4A543F644007AC60177D9844786409D657259636CA8B28F",
		"0AC7924F9DB47CF991A8F1008BE59B3279816EE457CB00A4A8299C3E39A9FB56",
		"0E15B2B612E64B03B58211AD35DE8B8E011BFCB398A502E771C2BEBB49F77A83",
		"51815733169AADA3AEA2D2038B8F647FFF33B03A902B1EF686D625FDF2B21E08",
		"E602A5FFEE449507B4A5865BE5B900A00603F1ED495D715653DDE47FF259DDCF",
		"A551FA2DFE1B2164018823681938627AEB6F57CC2375797ED6D779D14EACF87B",
		"A17D235E7FD4D96956BCDF7F28AAF1B6DDBC5D42F942FC0E48448E62CC3ED2DC",
		"CF3F70DA6A3F896917E5E99C1E8DC7E3AD69643078E504D8142B9568B9DDBA8F",
		"3C30C3473A8350D231A6B25AB6E858902919ED61F8B0F9840625E7789083309E",
		// eth
		"809D7F6774BD7DDDFC66FB25EC9C65EF428BE87BA160F18C53F0A82C406DFFFF",
		"46F784CBFA286B82963AAE6E05F58B9E32EACB702189FDE0765C10BDD4E9DBD8",
		"83857D6E619CB84F991B8EB579A96ED1E564D5C07A371DBC34C969F4F032B85C",
		// rune
		"C38FC6742E8E2DD64CFD21FC3C127B1E5C9162F2B427A42A4ED10C0468B3CB5F",
		"BC5B2A7D9FC424891F84C71FBF5DA204C157D2C33786B48E4C76F834A16DAA0A",
		"E1E685F926C2C2726381C30D3BF2E024A90C0614E69CFC37A371334150D56596",
		"B2FE49A810B96647E38D9EF7D8B2F05C628A83C96CDE5DD7571B31A6CAB7F88E",
		"ABCE8227A99E61CA12A6CEA32E414D95003445D71104FBD8FCFDA9DEBCED869A",
		"AACBD6C018222A17E2F7D92B25443DB8DE8AF40DBCF5D71BF1A957D7376A5B24",
		"EB0557F6253393E97D3E4ECCF08A38B2064A02209895832C2702997E1E109444",
		"7573997BFEF1C6C5EAA599A2F86F4D5E8D7BCCE6A1673F84BA380DE062499587",
		"F3554C59D366D07702064A3046E83DF61D5ED487FBF8123F03B1E50BAF8D16E9",
		"F6B88B58412BB8E320D94731D0CAA33CB8F2523782125C9F4E82E5FB5D9B09F4",
		"F2406FC81327E86CEB7B18CBB4461AD1079B536F34819C12F545B2CE90B9B109",
		"31AD6807C808CF685E0B8A462189A2BCC299EC1C910DD2A9C2BC58F1D61C3ED2",
		"81C6FA4224EEE1F5F4E51E55D62A89A93C988FE13C98A839184176C40C47A58E",
		"34CEB967DFCA0C990B0522BD48E2F18C142297358EEB79BD50050C8D833443AB",
		"B977082A01DE57C63D8CBEF842377D3F571B7C2C3E70B82B0218E0A83D5E8081",
		"71FF0DCD3EE2A8CC930D82ABC7C0C443665FADE5F64976DD231AC98573A937B3",
		"695B832978EF06BBAB7C465AEECF3370F617ED3CF5A50CEDF3F7FBDB0EE90027",
		"6A8CCCBAF1DEF416CE857DEC5106072469583936569BF3D674E4CD7A7AEA5E17",
		"0B3CC86FE285CC4A3EBACBC59E34713DB1E6D5511F805952F9C4F2C2133EB985",
		"76E6AB989AFF02655F5FF759FC5EE24A11852C3A821BA0A98770AFD0998DC67B",
		"15724C67A3401B2D5431813287BA3C07804EF71980B7EBA918396C8D74E0F435",
		"EB63857438C15FAD8C1960E546C63B754B86F059090FCCD0B027ABECD50DD761",
		"28E86F2DFA3C4C8F55E9E82D04C93A1B81706DC5073040028C3574EF1940DDC2",
		"14A644F7EF6A4753722222291D08CFF92ACA13A29C97D5F00BEFC7175C307D3C",
		"58EEE93D4325501C6D7D9B4E6D6A8EE97F2BFB95370E5593916265DCA4195935",
		"3C85FA4194DF6135D71F63C7A765DBD830DC510AF8E966927126989D4AC2A403",
		"3E2006B147B65C3526BA780666F151F9B99361EB49F4757A4B557913F3071559",
		"DE56A027CE733694AAE78D742A5CFFD40DE6291124DF20C57FDD0C3DFD9B24D3",
		"366541240BA7B8456298542AF4AC17A2F61D70210E90505729D5736D32ACADCA",
		"7629B29750003E4151417AC76BDF18A2F54BDFA10B86E38A713E11CB52052A9E",
		"4B3A09FCE075B6B5CDF531CFF3BE88FFA61AFCF343FDE72A920C5D1E9E5A8A9B",
		"DDAB76DF15997F2A99CE390F11CFD39B8914FDA99614EC35E9F69F94213C1BA0",
		"51341209129F32ECBCAE8536C4DEAFEB1525AED448D5C3C2AAE5AFEEB39AC049",
		"AB498BD5755FEDF28D3167A056681E1A5EB2424505D6CED9E4D5FFA70A9B970D",
		"779BB850AA7CD5BDEB7E02C5E9B641C02A292CE6EA776E28C075450F411F5047",
		// ticket 272
		"A45D605F1254469B566172A686E47E65FD036C04E9CEB1FCEDFE150844B6A932",
		"8A39735136C49113E87434361D7F820C3E97A4A2B078874A870892E9DC4AB713",
		"5F7ECFD8C1DE9606C49EAB663C4A1B131FC2CBA66547089DA41CEC26479AEC39",
		// kuji & btc
		"06495422B060C6ABD37686E421566BC22AFECA7023878929D120EDF290C66F45",
		"77BDA2727A415AD4BE5628DA8B10E8846A3BEDAB47755FB9158FC935C82FF0B2",
	}
	requeueDanglingActions(ctx, mgr, danglingInboundTxIDs)
}

func migrateStoreV116(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v116", "error", err)
		}
	}()

	txID := common.TxID("FD4CA0CEEE107E4A077BB178BC0A031EEB5BE6E55B6F529EE65C5A1A2487A621")

	newDestinationAddrString := "0xEf1C6F153afaf86424fd984728d32535902F1c3D"
	newDestinationAddr, err := common.NewAddress(newDestinationAddrString, mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to parse address", "error", err)
		return
	}

	voter, err := mgr.K.GetObservedTxInVoter(ctx, txID)
	if err != nil {
		ctx.Logger().Error("fail to get observed tx in voter", "error", err)
		return
	}

	newMemo := fmt.Sprintf("=:ETH.USDC:%s:0/3/0:wr:20", newDestinationAddrString)
	voter.Tx.Tx.Memo = newMemo

	for i, tx := range voter.Txs {
		tx.Tx.Memo = newMemo
		voter.Txs[i] = tx
	}

	mgr.K.SetObservedTxInVoter(ctx, voter)

	iterator := mgr.Keeper().GetSwapQueueIterator(ctx)
	defer iterator.Close()
	index := 0
	for ; iterator.Valid(); iterator.Next() {
		var msg MsgSwap
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &msg); err != nil {
			ctx.Logger().Error("fail to fetch swap msg from queue", "error", err)
			continue
		}

		if msg.IsStreaming() && msg.Tx.ID.Equals(txID) {
			msg.Destination = newDestinationAddr
			msg.Tx.Memo = newMemo
			if err := mgr.Keeper().SetSwapQueueItem(ctx, msg, index); err != nil {
				ctx.Logger().Error("fail to save swap msg to queue", "error", err)
			}
			return
		}
		index++
	}
}

func migrateStoreV117(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v116", "error", err)
		}
	}()

	txID := common.TxID("FD4CA0CEEE107E4A077BB178BC0A031EEB5BE6E55B6F529EE65C5A1A2487A621")

	newDestinationAddrString := "0xEf1C6F153afaf86424fd984728d32535902F1c3D"
	newDestinationAddr, err := common.NewAddress(newDestinationAddrString, mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to parse address", "error", err)
		return
	}

	newMemo := fmt.Sprintf("=:ETH.USDC:%s:0/3/0:wr:20", newDestinationAddrString)
	iterator := mgr.Keeper().GetSwapQueueIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var msg MsgSwap
		if err := mgr.Keeper().Cdc().Unmarshal(iterator.Value(), &msg); err != nil {
			ctx.Logger().Error("fail to fetch swap msg from queue", "error", err)
			continue
		}

		if msg.IsStreaming() && msg.Tx.ID.Equals(txID) {
			ss := strings.Split(string(iterator.Key()), "-")
			i, err := strconv.Atoi(ss[len(ss)-1])
			if err != nil {
				ctx.Logger().Error("fail to parse swap queue msg index", "key", iterator.Key(), "error", err)
				continue
			}

			if i != 0 {
				mgr.Keeper().RemoveSwapQueueItem(ctx, msg.Tx.ID, i)
				ctx.Logger().Info("Swap Queue Item Removed", "index", i)
			} else {
				oldMsg := msg
				msg.Destination = newDestinationAddr
				msg.Tx.Memo = newMemo
				if err := mgr.Keeper().SetSwapQueueItem(ctx, msg, 0); err != nil {
					ctx.Logger().Error("fail to save swap msg to queue", "error", err)
				}
				ctx.Logger().Info("Swap Queue Item Changed", "old msg", oldMsg, "new msg", msg)
			}
		}
	}
}

func migrateStoreV118(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v118", "error", err)
		}
	}()

	manualRefunds := []RefundTxCACAO{
		// 68BE10668462DD55357C6E07B76E1989150CDF96DA9189ECE0136A208B343EE7
		{sendAddress: "maya1tz5fcf4wy8a2dpd47p5mq92aq2j3h48n6xu7av", amount: cosmos.NewUint(300_0000000000)},
		// ticket 3374 | F5C78780E201485CC668EDD60E639E2B41ADD5276DDAEF00D96D42D41052E99A
		{sendAddress: "maya1tz5fcf4wy8a2dpd47p5mq92aq2j3h48n6xu7av", amount: cosmos.NewUint(1710_0000000000)},
		// ticket 2268 | 3F231BC06E0DF29C1BCE6C9F594F55F8C6A21E7D211D4F786E5D3A6B4A091BDB
		{sendAddress: "maya18k4zmua8l3q9fgpwuw23v5p6pfzmlg9srqkl48", amount: cosmos.NewUint(7920_8200000000)},
		// ticket 2278 | D1093D487E8D3D705DA9A7884A00829918DFA467749D40E0FED43CF642846711
		{sendAddress: "maya18k4zmua8l3q9fgpwuw23v5p6pfzmlg9srqkl48", amount: cosmos.NewUint(13217_4300000000)},
		// ticket 2296 | 74A78008689FCB3CF901F8EB14636C735D3ACAFEB525F345962F2238CFE018D7
		{sendAddress: "maya18k4zmua8l3q9fgpwuw23v5p6pfzmlg9srqkl48", amount: cosmos.NewUint(560_4000000000)},
		// ticket 2322 | EA45E9B0B52FED567FDAF5B6701EE9C8F93727C5C1E61123F620E912D82F7C77
		{sendAddress: "maya18k4zmua8l3q9fgpwuw23v5p6pfzmlg9srqkl48", amount: cosmos.NewUint(1618_5100000000)},
		// ticket 2325 | 2FE6A777E1D30100083A1BA3FC674CEF981157337503C236075E2D7C77E05270
		{sendAddress: "maya18k4zmua8l3q9fgpwuw23v5p6pfzmlg9srqkl48", amount: cosmos.NewUint(8715_0500000000)},
		// ticket 2327 | D212EA89ECFB26DA0D560A1902426DC77811B3414BE15BE0A3744D03C6150019
		{sendAddress: "maya18k4zmua8l3q9fgpwuw23v5p6pfzmlg9srqkl48", amount: cosmos.NewUint(652_8000000000)},
		// ticket 2330 | 773069E8F749F2B2D3D9E144B974AE8B5001C78934C17AEFCA451724FDF4C59D
		{sendAddress: "maya18k4zmua8l3q9fgpwuw23v5p6pfzmlg9srqkl48", amount: cosmos.NewUint(1972_0000000000)},
		// F577CBF8F0BA175AD7EF87E838E18FF72AF96D92BA4430DB18314E79BCC6FD03
		{sendAddress: "maya18k4zmua8l3q9fgpwuw23v5p6pfzmlg9srqkl48", amount: cosmos.NewUint(10465_2200000000)},
		// Immunefi Bug Bounty Payout
		{sendAddress: "maya18z343fsdlav47chtkyp0aawqt6sgxsh3vjy2vz", amount: cosmos.NewUint(54000_0000000000)},
	}
	refundTxsCACAO(ctx, mgr, manualRefunds)

	danglingInboundTxIDs := []common.TxID{
		// before churn at height 9311136
		"7470B7703CFBA1909C7FDE0133313D2DD9BF6CCF334ABF55B021FC81F3DB5710",

		// from height 9311136
		"1788DA46369CBB39192C4E8292108AC7E8E410A65C86A22E1D445A8CE77AF0B7",
		"1A46648C8F3A35D6BACF8CEBD235B98BCECB174087C0B39DB4B33A75884CB4B1",
		"4D92650A618DDC6EFAB70230F2E47FD7DDE83557714CD3F8F1C9247E400F4AEB",
		"57E586A076C16BFAF5C0A316DA287A35B47129B73B6E48D1AB5B87E3C53D763A",

		"B7171EED57914355E3985B7B83EDAF1DF356A9915539B1BD86DA4B520B5BE467",

		"05220C3B320305774E0223D15DA52EA9D75E3C6F8F892AA47C338D86C56CA583",
		"5D77D255F6352E2D7D7B81213609CB0E77B09986098829421AA284426CAC10A0",
		"71B5774FA613EAD125208CBAB4D1D9279991445872177996B717F108569DA98D",
		"0EBE87AFA1131D68AA39382CA3191D8211BD970CB6CA7C2F3A2E96085499FA8E",
		"11546FB1113FA741B0BE0BC47AB03AD0BCCD7B61E76F73E09097716B219B097A",
		"394224F486215D6332459A39737B617774C824BBDC6CDFD3B7678707DABA1E19",
		"71E4424C5196D20746157C8537F98816B0F89517347C278593581AB6F470D1CF",
		"B3E508B19524E16D7464E03F660CFA212F89D273E332B987D7FF3C33CA878527",
		"B8080E2AE5EAF02151F05D6CD578F01E90EA56F93599849E7C708F35287E8E48",
		"EF161A4195A1861D98A8E41BCE84D257A7A4867E4AD9756D56170D065B6C19B0",

		// ticket 2323
		"6DC7DC2AAB3CAFE12692AE8225942593CDDF61660BF40FB23946D9C2012BF285",
		"84B94E5972C168FAAF4C510438CA59477421B6D9C1ED2CE028549A0875223D35",

		// ticket 2377
		"26EFB08BE430A313ADDF9CF7CDEF0B72F4C64E122031F8165C50E3706AB8102D",
		"AAFB49BB42742D12D19BAD7BA3212E04CB4FAF9DF3FFF22D6F29DB6435C7D3B5",
		"915096240F078E003649AE384C3AB843C9E449516D4B79476CB27BEDDC1A4E24",
		"71C81A58D93B5C92B2FDFB5DA31D97F178F3D10BB74AAC4BD763A75FB11C09F9",
		"4C7ADBDC81EEB4C4610024FC55D2E8D835E53BA5D65A45B0AB8079866DCFFE54",
		"2AF854EF3EFFA7641F29EDBA4CD082C5E0D20185737D369C44DEB8C6E67584B8",
		"3C5A3511FD20A70EDA5FAB2C4AEE9E52C8463C34BF11274FF3C341F74AB60A5B",
		"2AD75D942DADE4B9D8E707A6CAB22247031C1BD115AAA9FA664993F93E8CAC5F",
		"541C3B7D04278A7336F88C8C75D0F60EB736B087F8D42F751A10B2293F2E5C2C",
		"394189EBB40657881D0DA20229C2042D835E7C4E5AA8BD283706B4F35327C000",
		"7D1E3B1FF2530CBA25DE99AFB3E874D082444E176D1A59CDF6E088E88E5095C8",
		"F74CF8590E80D8E947F729BB7369BFB0DEE28270B1D8DC59E6FB8B569C46D7FA",
		"87C4106AAFE1177D9D771BDF9170D4CFB0A1820D25D71A845E5D22AD71988AFE",
		"A45E3C3F97B2B53A3F3B53A325BC608827E6688ED3739A2C12AC8DE2D4CDD534",
		"A78FE99BC929DD1C586D1314A0137874416A192BFF1FFA8A8D7BA6C3316D0A31",
		"B12B045BE8DE5D3A2EA51F0C612E3C4A15B5F7D7796DF5CC47F2D2027DF1523F",
		"4C5CD591A48EB12C750B137C769F346DF62B3FC006093DD43628646619B6B6A7",

		// ticket 2384
		"389B22E31A6EAC4A53400D3536ADCD3A9D75C1AA08F3F198440961239C4272D8",
		"5CF645B49002A04BA48F429A441BFCEA90C25B7FEB6AF685F967427A85EFC906",

		// dropped queue
		"41D68FE63C0C77A7DBFE622FC58FC2EAE28B53155A517CB85DA8DB7A3458A2BC",
	}
	requeueDanglingActions(ctx, mgr, danglingInboundTxIDs)

	// Unbond bond providers from node account
	unbondAddresses := []unbondBondProvider{
		// ticket 2331
		{bondProviderAddress: "maya192ynka6qjuprdfe040ynnlrzf26nyg38vckr2s", nodeAccountAddress: "maya1rzr9m407svj4jmc6rsxzsg75cx7gm3lsyyttyj"},
	}
	unbondBondProviders(ctx, mgr, unbondAddresses)

	threeMillionCacao := cosmos.NewUint(uint64(3_000_000_0000000000))
	coinsToCacaoPool := common.NewCoin(common.BaseNative, threeMillionCacao)
	if err := mgr.Keeper().SendFromModuleToModule(ctx, ReserveName, CACAOPoolName, common.NewCoins(coinsToCacaoPool)); err != nil {
		ctx.Logger().Error("fail to move coins from Reserve to CACAOPool", "error", err)
	} else {
		cacaoPool, err := mgr.Keeper().GetCACAOPool(ctx)
		if err != nil {
			ctx.Logger().Error("fail to get cacao pool", "error", err)
		} else {
			cacaoPool.ReserveUnits = threeMillionCacao
			mgr.Keeper().SetCACAOPool(ctx, cacaoPool)
		}
	}

	notProcessedTxs := []adhocRefundTxV118{
		// ticket 2323
		// The attached url is the block where the streaming swap finished successfully and an outbound should've been emitted
		// https://mayanode.mayachain.info/mayachain/block\?height=9192251
		{
			toAddr:      "Xgi4EkCqj6v7uf4Dp5GhXXd678HGmGiQuS",
			asset:       common.DASHAsset,
			amount:      cosmos.NewUint(2265726193),
			inboundHash: "4EBBF14B944B67632D226B4DD78CA6BEF0EE940120593AE2567FDC2DFFCC316B",
		},
		// https://mayanode.mayachain.info/mayachain/block\?height=9192928
		{
			toAddr:      "Xgi4EkCqj6v7uf4Dp5GhXXd678HGmGiQuS",
			asset:       common.DASHAsset,
			amount:      cosmos.NewUint(2247963380),
			inboundHash: "3159C0B7237408338D43C17035DE89D7B51417BA5C32CC6C1E72047E9BA815B8",
		},
		// https://mayanode.mayachain.info/mayachain/block\?height=9209512
		{
			toAddr:      "Xgi4EkCqj6v7uf4Dp5GhXXd678HGmGiQuS",
			asset:       common.DASHAsset,
			amount:      cosmos.NewUint(2287040807),
			inboundHash: "E139A83442EDF690AC85034015F71B92D32ACCBF94EE25C99595D5A996CCEBBB",
		},
		{
			toAddr:      "Xgi4EkCqj6v7uf4Dp5GhXXd678HGmGiQuS",
			asset:       common.DASHAsset,
			amount:      cosmos.NewUint(1159125984),
			inboundHash: "138837EC26E9A34902D63DB16BE81A07BB72C7B9889F1E871E67726D5696F802",
		},
		// https://mayanode.mayachain.info/mayachain/block\?height=9315285
		{
			toAddr:      "Xgi4EkCqj6v7uf4Dp5GhXXd678HGmGiQuS",
			asset:       common.DASHAsset,
			amount:      cosmos.NewUint(1154682283),
			inboundHash: "C4A82E6A2FF2500D05A307C4A59DA782DC89F560B2CE5343181A6707011CAD51",
		},
		// https://mayanode.mayachain.info/mayachain/block\?height=9346087
		{
			toAddr:      "Xgi4EkCqj6v7uf4Dp5GhXXd678HGmGiQuS",
			asset:       common.DASHAsset,
			amount:      cosmos.NewUint(2656250623),
			inboundHash: "E2D164EE84E41700C7C6894B66169AF5A3C66D242A440A805B015770C820F4EE",
		},
		// https://mayanode.mayachain.info/mayachain/block\?height=9346089
		{
			toAddr:      "Xgi4EkCqj6v7uf4Dp5GhXXd678HGmGiQuS",
			asset:       common.DASHAsset,
			amount:      cosmos.NewUint(2652335794),
			inboundHash: "48AB8B7DD5DD33BE6F57B51FED3A293CC18F7093B69D0CB0077B8D9449F76DCD",
		},
		// https://mayanode.mayachain.info/mayachain/block\?height=9346095
		{
			toAddr:      "Xgi4EkCqj6v7uf4Dp5GhXXd678HGmGiQuS",
			asset:       common.DASHAsset,
			amount:      cosmos.NewUint(2629327963),
			inboundHash: "4EAA9C45E48D26A3A55E2E8D8286D2F2670FEC32BFCEAA6D23D4F7D07F0F8ECC",
		},
		// https://mayanode.mayachain.info/mayachain/block\?height=9346120
		{
			toAddr:      "Xgi4EkCqj6v7uf4Dp5GhXXd678HGmGiQuS",
			asset:       common.DASHAsset,
			amount:      cosmos.NewUint(2603202316),
			inboundHash: "734B45D4757BA95C5DFEEFA1084D6AEE014698B55ADF33FEB57360BFF12CB899",
		},
		// https://mayanode.mayachain.info/mayachain/block\?height=9346125
		{
			toAddr:      "Xgi4EkCqj6v7uf4Dp5GhXXd678HGmGiQuS",
			asset:       common.DASHAsset,
			amount:      cosmos.NewUint(2578647935),
			inboundHash: "FC64BEA7D52D3F645B1984CA2E5DCEFA066B4A836813620C11475EE34514FC7C",
		},
		// ticket 2278
		// https://mayanode.mayachain.info/mayachain/block\?height=9346036
		{
			toAddr:      "Xgi4EkCqj6v7uf4Dp5GhXXd678HGmGiQuS",
			asset:       common.DASHAsset,
			amount:      cosmos.NewUint(2730828052),
			inboundHash: "7AC14816A35D0EEFD9670D940218D727D2CDCAE80C0434A8EA220ADDE37F7E48",
		},
		// https://mayanode.mayachain.info/mayachain/block\?height=9342843
		{
			toAddr:      "Xgi4EkCqj6v7uf4Dp5GhXXd678HGmGiQuS",
			asset:       common.DASHAsset,
			amount:      cosmos.NewUint(2699474450),
			inboundHash: "A5A91E37A41D4B423FAB01C7737580FBC77A3189B1F17992CB4D70BB22E9F56C",
		},
		// ticket 2349
		// https://mayanode.mayachain.info/mayachain/block\?height=9319463
		{
			toAddr:      "Xgi4EkCqj6v7uf4Dp5GhXXd678HGmGiQuS",
			asset:       common.DASHAsset,
			amount:      cosmos.NewUint(802318801),
			inboundHash: "CEAADBC39F2CC1C9E630049051E5D8E54574246B66DB47E428EBAEE903150BD6",
		},
	}

	activeAsgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil || len(activeAsgards) == 0 {
		ctx.Logger().Error("fail to get active asgard vaults", "error", err)
		return
	}

	if len(activeAsgards) > 1 {
		signingTransactionPeriod := mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)
		activeAsgards = mgr.Keeper().SortBySecurity(ctx, activeAsgards, signingTransactionPeriod)
	}
	vaultPubKey := activeAsgards[0].PubKey

	refundTransactionsV118(ctx, mgr, vaultPubKey.String(), notProcessedTxs...)
}

func migrateStoreV119(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v118", "error", err)
		}
	}()

	sender, err := cosmos.AccAddressFromBech32("maya1g70v5r9ujxrwewdn3w44pmqcygx49dx7ne82vr")
	if err != nil {
		ctx.Logger().Error("fail to parse sender address", "error", err)
		return
	}
	recipient, err := cosmos.AccAddressFromBech32("maya16nmqmg5qrd6f2pkte4skyv04pye23us0ytxche")
	if err != nil {
		ctx.Logger().Error("fail to parse recipient address", "error", err)
		return
	}

	balances := mgr.Keeper().GetBalance(ctx, sender)

	if err := mgr.coinKeeper.SendCoinsFromAccountToModule(ctx, sender, ModuleName, balances); err != nil {
		ctx.Logger().Error("fail to send sender to module", "error", err)
		return
	}
	if err := mgr.coinKeeper.SendCoinsFromModuleToAccount(ctx, ModuleName, recipient, balances); err != nil {
		ctx.Logger().Error("fail to send module to recipient", "error", err)
		return
	}
}

func migrateStoreV120(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v120", "error", err)
		}
	}()

	// THOR Dangling Actions
	thorDanglingTxIDs := []string{
		"AB4F498280D3557FF93AF3AB9D170CB5867B82461220794256FD62D88D2DFED4",
		"4370E25192677DF80F4C05CE2ACCA74C3DE86334AE0E18F39686128108E59385",
		"48E2FE9F61F853A0ABE71B63B140BE3D84D1A059A20C1A337DBDDD4DC615E762",
		"BD31D5D9224CC85E152DE8714225484168B303BB98A0FC5BF8B5D5E888D98CBD",
		"3DE1C2F7AA1266D72C5BF21E52B0FF497648D131CE1950F4A293E11AF2E2566C",
	}

	// Convert string TxIDs to common.TxID
	var txIDs []common.TxID
	for _, txIDStr := range thorDanglingTxIDs {
		txID, err := common.NewTxID(txIDStr)
		if err != nil {
			ctx.Logger().Error("fail to parse txID", "error", err, "txID", txIDStr)
			continue
		}
		txIDs = append(txIDs, txID)
	}

	// Requeue dangling actions
	if len(txIDs) > 0 {
		requeueDanglingActions(ctx, mgr, txIDs)
	}
}

// /////////////////////
// -> Skipped tx, ticket-58
// https://www.blockchain.com/explorer/transactions/btc/dfe9d645f3a7822dd4b5678668f17d2561328954ddf770738111a75a85f4937a
// /////////////////////
func migrateStoreV121SkippedTx(ctx cosmos.Context, mgr *Mgrs) {
	activeAsgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil || len(activeAsgards) == 0 {
		ctx.Logger().Error("fail to get active asgard vaults", "error", err)
		return
	}
	vaultPubKey := activeAsgards[0].PubKey
	vaultBTCAddress, err := vaultPubKey.GetAddress(common.BTCChain)
	if err != nil {
		ctx.Logger().Error("fail to get vault BTC address", "error", err)
		return
	}

	fromAddr, err := common.NewAddress("bc1quw9jp968x2ngsvg3a5g5a3vxwkxvt4yxdy4hqn", mgr.Keeper().GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to create addr", "addr", fromAddr.String(), "error", err)
		return
	}

	unobservedTxs := ObservedTxs{}
	unobservedTxs = append(
		unobservedTxs,
		NewObservedTx(common.Tx{
			ID:          "DFE9D645F3A7822DD4B5678668F17D2561328954DDF770738111A75A85F4937A",
			Chain:       common.BTCChain,
			FromAddress: fromAddr,
			ToAddress:   vaultBTCAddress,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(9000000), // 0.09000000
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1060), // 0.00001060
			}},
			Memo: "=:e:0x458d76deff89674bb6e315fcd57d93f20adb5ab8:331575828/1/0:_/zak:20/20",
		}, ctx.BlockHeight(), vaultPubKey, ctx.BlockHeight()),
	)

	err = makeFakeTxInObservation(ctx, mgr, unobservedTxs)
	if err != nil {
		ctx.Logger().Error("failed to make txin observation for migrate v121", "error", err)
	}
}

// /////////////////////
// -> dropped txs
// /////////////////////
func migrateStoreV121DroppedTxs(ctx cosmos.Context, mgr *Mgrs) {
	droppedTxs := []common.TxID{
		"19A901A6408EE00E565F6642152A1F4339F538F895D883A75684B0FC3E43656C",
		"1BA806DA8A4BEA49E8E97961EB63DE0890489FF4CE1820D63646F45EE361AB4E",
		"E604FBEE71D0A1A6E43971B7C3DE81CD1E0094524FFC4C4A257A1D82A08DA91E",
		"02031E3C5D13D0D643AC661B76E3A73B58F62E6B4AA47697F9D4722628C6CAAD",
		"53233D76BEF65B38D2A652F205D95EC29AD4D71702F3001609FB566B5EB8CEBB",
		"389B22E31A6EAC4A53400D3536ADCD3A9D75C1AA08F3F198440961239C4272D8",
		"5CF645B49002A04BA48F429A441BFCEA90C25B7FEB6AF685F967427A85EFC906",
		"41D68FE63C0C77A7DBFE622FC58FC2EAE28B53155A517CB85DA8DB7A3458A2BC",
		"E573C3B03C09F55F96A1D2C6B6C4249499442D391DB3EA65B44E8FC6202A65DD",
		"E0EECEB47B1849D0911CE8DA28D294067873A23374DC8AF1AE5C29A9D2AF85D7",
		"F4E11AE2573827EFD1C08B84958B0BD6918F4B799358662B7619752F7C6316F2",
		"4104331CF4C1BFD3850D452A910E004403D34631F3FCB6AC1DA7CDAE07525B8D",
		"01E0A5C4AA29742C14D06AB56ADE3597C800E38DFAD9C3775F116D51DB7A9C26",
		"86946D783B4B4B012F47CAD28B1BBBED02C0355C43DAB28F7BB2F10EEA03E8EB",
		"DDF793E61FBBBD9572CF437922D3B66E871785C9D60CDC81B6E293BC23F208EF",
		"80E97FCD0B84791EBA7BD77BF2728BD87EFEFF5951F319B5A85BA13508D30AC6",
		"C6B49B177203F03870056A88C67A6B9D12D536A762E4ED2D473A3A0A973005B1",
		"61FC12CB073E12E9E7DE9EC5A8DF48801515CE196439C228FF688686B8A36816",
		"50AA42F2ED1052A7B5969F0BC1EBEAF1F5CE15F2E6D6D64BAD41182835026C86",
		"86BE62B9B9432FBB47259840BFC9A806BBCC5198DF3F8257335134090A1F8961",
		"2809EA569DE29BF94E1D8DB1C2642BD5DCC1211744ECC315464996BBE3D9867C",
		"9391AC2FE01E7B44A689AE593A246EA0C0AB601B1850AF42B77B3DD50EC15922",
		"C407D40C5E41C46507B04F8ADFEF140BCDB6ACE2A32EE848D7CA9CE850500D12",
		"8130F7D0E5E12B1C292034B1B3371BD2296F78C6D23F747B42CA1F984049A916",
		"60B26E36103B76D88625C1FAD7B1564FEA6B16AE8A7935D4246DFCA960B5A2A5",
		"ED1666C51D21316B3C974AE0FAA9830FC90761123AB4244B81487167D28DE411",
		"7FC90C5B373A2281F6CAFE6849DD78E67AEA9F08DF818D52FEC80A25FE6DCEEE",
		"D4099CD045B1F3424E460084FC8235C9A2BC4D7EEB8BC898F8AC1C09FB1DBAE4",
		"85B2037B09DAE58DAA063C22A2F33A62B761764C1B64EA117FDD175D56D441AB",
		"9AA3FFAEB43E8B9B0BD8B394EDA8FCDA04C6E843BDF89F0177043B232B5D3434",
		"F3C75E6B3A42A6D3E1F63567C1181096CCFDAB8251F90F7ADDB9C3BF316192A8",
		"E0CE732DE6882E34B2F34218C778088D85102A753FBB898C7147CD0B69D35A34",
		"B561294F0A077E727DC337D0EF7BA61D5245688EB347F1FC9DC419A28AED82A0",
		"083FBA01202D365C25FF2CE610ECE079D7E04B0924ED920660AEFDEA6CFEC890",
		"82E60EABA1B92731616C5ED226892C5FEEEB77130244AB4E196E8227E517CFDF",
		"CB97518259A2BD0B36AF68DD700A0ADB770BBF5B93B063DB949D62E7A2708F6B",
		"542ABD91D806D062C0FDB5ECE3CC4622C61A8E83E024E260AF5DB9596EFAA330",
		"2771B3F1880CF49C430CFCC302074F7C00AEE30F08FFE1959DF8F348495694E3",
		"7C71F253208DC7E717225E95119C9FF3F6409196AC7F2845F205C49472CFB171",
		"33B7F8CFCC62528623F9D9974DC11A76B818617801252CCD16E28A3F34FB1B82",
		"C533094A82076F1A90568246CED27ABC276F5426A26CDC36B825DA23A8C4111D",
		"D94A80EE6DE039849324217A2B188BCE923DB114EBC182BA8392CB6F8CFDFA64",
		"2C012BD1817AC88201A925D852B205DDF0738DAB5006B31217DB5ABBA15696A6",
		"6653123C7EBE64BCFA3C2A5C7F8FE954707FB23C8021A5BEA260D18D9F4D10FE",
		"5B454487D25E22112CBC54BD7AEE4C698A7C428E78D72F5B3471C4772F7060E8",
		"DCF7350CE6778636DF49D30030BB1792006067A0A49FD5F50B99A7A0DD726ACA",
	}
	requeueDanglingActions(ctx, mgr, droppedTxs)
}

// ARB.ETH liquidity provider maya192ynka6qjuprdfe040ynnlrzf26nyg38vckr2s has units bonded to
// node address maya1rzr9m407svj4jmc6rsxzsg75cx7gm3lsyyttyj but node doesn't have that address listed as providers
func migrateStoreV121RemoveLp(ctx cosmos.Context, mgr *Mgrs) {
	lpAddressToRemove, err := common.NewAddress("maya192ynka6qjuprdfe040ynnlrzf26nyg38vckr2s", mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail get address", "error", err)
		return
	}

	bondedNodeAddrToRemove, err := common.NewAddress("maya1rzr9m407svj4jmc6rsxzsg75cx7gm3lsyyttyj", mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail get address", "error", err)
		return
	}

	removed, err := removeBondedNodeFromLP(ctx, mgr, BondedNodeRemovalParams{
		LPAddress:         lpAddressToRemove,
		BondedNodeAddress: bondedNodeAddrToRemove,
		Asset:             common.AETHAsset,
	})
	if err != nil {
		ctx.Logger().Error("failed to remove bonded node from LP", "error", err)
	} else if !removed {
		ctx.Logger().Info("bonded node already absent from LP, no action required")
	}
}

// /////////////////////
// -> drop savers
// /////////////////////
func migrateStoreV121DropSavers(ctx cosmos.Context, mgr *Mgrs) {
	activeAsgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil || len(activeAsgards) == 0 {
		ctx.Logger().Error("fail to get active asgard vaults", "error", err)
		return
	}
	vaultPubKey := activeAsgards[0].PubKey

	asgardAddress, err := mgr.Keeper().GetModuleAddress(AsgardName)
	if err != nil {
		ctx.Logger().Error("fail to get module address", "error", err)
		return
	}

	asgardAcc, err := cosmos.AccAddressFromBech32(asgardAddress.String())
	if err != nil {
		ctx.Logger().Error("fail to get module address", "error", err)
		return
	}

	asgardCoins := mgr.Keeper().GetBalance(ctx, asgardAcc)
	for _, coin := range asgardCoins {
		var sAsset common.Asset
		sAsset, err = common.NewAsset(coin.Denom)
		if err != nil {
			ctx.Logger().Error("fail to parse asset", "asset", coin.Denom, "error", err)
			continue
		}

		if !sAsset.IsSyntheticAsset() {
			ctx.Logger().Info("skipping non-synthetic asset", "asset", sAsset.String())
			continue
		}

		iterator := mgr.Keeper().GetLiquidityProviderIterator(ctx, sAsset)
		for ; iterator.Valid(); iterator.Next() {
			var lp types.LiquidityProvider
			mgr.Keeper().Cdc().MustUnmarshal(iterator.Value(), &lp)

			var pool types.Pool
			pool, err = mgr.Keeper().GetPool(ctx, sAsset)
			if err != nil {
				ctx.Logger().Error("fail to get pool for asset", "asset", sAsset.String(), "error", err)
				continue
			}
			redeemAmount := lp.GetSaversAssetRedeemValue(pool)
			if redeemAmount.IsZero() {
				ctx.Logger().Info("Dropping empty saver", "address", lp.AssetAddress.String())
				mgr.Keeper().RemoveLiquidityProvider(ctx, lp)
				continue
			}

			moduleBalance := mgr.Keeper().GetBalanceOfModule(ctx, AsgardName, sAsset.Native())
			if moduleBalance.LT(redeemAmount) {
				deficit := redeemAmount.Sub(moduleBalance)

				maxSynthPerAssetDepth := mgr.GetConstants().GetInt64Value(constants.MaxSynthPerAssetDepth)
				poolAssetDepth := pool.BalanceAsset
				maxSupply := cosmos.NewUint(uint64(maxSynthPerAssetDepth)).Mul(poolAssetDepth).QuoUint64(10000)
				synthSupply := mgr.Keeper().GetTotalSupply(ctx, sAsset)

				if (synthSupply.Add(deficit)).GT(maxSupply) {
					ctx.Logger().Error("synth supply is more than max allowed supply, skipping.", "Asset", sAsset.String(), "synth supply", synthSupply.String(), "deficit", deficit.String(), "max supply", maxSupply.String())
					continue
				}

				err = mgr.Keeper().MintToModule(ctx, ModuleName, common.NewCoin(sAsset, deficit))
				if err != nil {
					ctx.Logger().Error("fail to mint to module", "error", err)
					continue
				}
				err = mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, AsgardName, common.NewCoins(common.NewCoin(sAsset, deficit)))
				if err != nil {
					ctx.Logger().Error("fail to send from module to asgard", "error", err)
					continue
				}
			}

			var addr common.Address
			addr, err = common.NewAddress(lp.AssetAddress.String(), mgr.Keeper().GetVersion())
			if err != nil {
				ctx.Logger().Error("fail to parse address", "address", lp.AssetAddress.String(), "error", err)
				continue
			}

			asset := sAsset.GetLayer1Asset()
			var maxGas common.Coin
			maxGas, err = mgr.GasMgr().GetMaxGas(ctx, asset.GetChain())
			if err != nil {
				ctx.Logger().Error("fail to get max gas", "error", err)
				continue
			}

			coin := common.NewCoin(sAsset, redeemAmount)

			txIDStr := makeTxID(asgardAddress, addr, common.Coins{coin}, "")
			var txID common.TxID
			txID, err = common.NewTxID(txIDStr)
			if err != nil {
				ctx.Logger().Error("fail to parse txID", "error", err, "txID", txID)
				continue
			}

			unobservedTxs := ObservedTxs{NewObservedTx(common.Tx{
				ID:          txID,
				Chain:       asset.GetChain(),
				FromAddress: asgardAddress,
				ToAddress:   addr,
				Coins: common.NewCoins(common.Coin{
					Asset:  asset,
					Amount: redeemAmount,
				}),
				Gas: common.Gas{common.Coin{
					Asset:  asset,
					Amount: maxGas.Amount,
				}},
			}, ctx.BlockHeight(), vaultPubKey, ctx.BlockHeight())}

			err = makeFakeTxInObservation(ctx, mgr, unobservedTxs)
			if err != nil {
				ctx.Logger().Error("failed to make tx in observation", "error", err)
			}

			memo := fmt.Sprintf("SWAP:%s:%s", asset.String(), addr.String())

			tx := common.NewTx(
				txID,
				asgardAddress,
				addr,
				common.Coins{coin},
				common.Gas{maxGas},
				memo,
			)

			swapMsg := NewMsgSwap(
				tx,
				asset,
				addr,
				cosmos.ZeroUint(),
				"",
				cosmos.ZeroUint(),
				"", "", nil,
				MarketOrder,
				0, 0,
				asgardAcc,
			)

			handler := NewSwapHandler(mgr)
			_, err = handler.Run(ctx, swapMsg)
			if err != nil {
				ctx.Logger().Error("Failed to run swap for saver refund", "error", err, "address", lp.AssetAddress.String())
				continue
			}

			mgr.Keeper().RemoveLiquidityProvider(ctx, lp)
		}
		iterator.Close()

		// deduct the remaining balance
		err = deductFromAsgardModule(ctx, mgr, sAsset)
		if err != nil {
			ctx.Logger().Error("fail to deduct from asgard module", "error", err)
			continue
		}
	}
}

func migrateStoreV121(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v121", "error", err)
		}
	}()

	migrateStoreV121SkippedTx(ctx, mgr)
	migrateStoreV121DroppedTxs(ctx, mgr)
	migrateStoreV121RemoveLp(ctx, mgr)
	migrateStoreV121DropSavers(ctx, mgr)
}

// migrateStoreV122 - force finalize stuck transactions with incorrect confirmation delays
func migrateStoreV122(ctx cosmos.Context, mgr *Mgrs) {
	ctx.Logger().Info("Starting v122 store migration")
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v122", "error", err)
		}
	}()

	// Fix the LastObserveHeight for affected chains first
	migrateStoreV122FixLastObserveHeight(ctx, mgr)

	// Fix stuck transactions
	migrateStoreV122FinalizeStuckTxs(ctx, mgr)

	// Process payouts last
	migrateStoreV122Payouts(ctx, mgr)

	ctx.Logger().Info("Completed v122 store migration")
}

// migrateStoreV122FinalizeStuckTxs - finalize stuck transactions with incorrect external_confirmation_delay_height
func migrateStoreV122FinalizeStuckTxs(ctx cosmos.Context, mgr *Mgrs) {
	ctx.Logger().Info("Starting v122 migration to finalize stuck transactions")

	// Transaction IDs that need to be finalized
	stuckTxIDs := []string{
		"13d5c6a1dbfd6d8d4eb76b20512bdcab58d08acef1ade50241ea12044109f105", // TRUST
		"d8766cafb07465557becf5f23592ba0260457ac675cdb2ea2bab9fd8039d3c84",
		"60965f801abd1bee82a09b814b21145c5950c168b74341ebb07f7e379dfa900b",
		"84f4354cee98a5e53c6606784da34d9e6b109210fcaaec5ccac0d1f113d5a03b",
		"6811b8f6070ab71b5cfffabf1fb20ca43630cdc18f6d45e51059bc411f563645",
		"9c70fba436e6a4242bf207567efdeb0d52a75ccded2670cfbc4812c80afb821b", // Hyperion
	}

	// Map of txIDs that need destination address updates since they were manually paid by Maya team
	destinationUpdates := map[string]string{
		"d8766cafb07465557becf5f23592ba0260457ac675cdb2ea2bab9fd8039d3c84": "0xef1c6f153afaf86424fd984728d32535902f1c3d",
		"60965f801abd1bee82a09b814b21145c5950c168b74341ebb07f7e379dfa900b": "0xef1c6f153afaf86424fd984728d32535902f1c3d",
		"84f4354cee98a5e53c6606784da34d9e6b109210fcaaec5ccac0d1f113d5a03b": "0xef1c6f153afaf86424fd984728d32535902f1c3d",
	}

	for _, txIDStr := range stuckTxIDs {
		ctx.Logger().Info("Processing stuck transaction", "txid", txIDStr)

		txID, err := common.NewTxID(txIDStr)
		if err != nil {
			ctx.Logger().Error("fail to parse tx id", "error", err, "tx_id", txIDStr)
			continue
		}

		// Get the voter
		voter, err := mgr.Keeper().GetObservedTxInVoter(ctx, txID)
		if err != nil {
			ctx.Logger().Error("fail to get observed tx voter", "error", err, "tx_id", txIDStr)
			continue
		}

		ctx.Logger().Info("Found transaction voter",
			"tx_id", txIDStr,
			"consensus_height", voter.Height,
			"finalized_height", voter.FinalisedHeight,
			"consensus_finalise_height", voter.Tx.FinaliseHeight,
			"num_observations", len(voter.Txs),
			"consensus_block_height", voter.Tx.BlockHeight)

		// Check if already finalized
		if voter.FinalisedHeight > 0 {
			ctx.Logger().Info("Transaction already finalized", "tx_id", txIDStr, "finalized_height", voter.FinalisedHeight)
			continue
		}

		// Force finalize the transaction by setting the correct confirmation delay
		voter.FinalisedHeight = ctx.BlockHeight()

		// If no observations yet, we can't fix the heights
		if len(voter.Txs) == 0 {
			ctx.Logger().Info("No observations yet for transaction, skipping height fixes", "tx_id", txIDStr)
			// Save the voter with just the FinalisedHeight set
			mgr.Keeper().SetObservedTxInVoter(ctx, voter)
			continue
		}

		// Check if this transaction needs destination address and memo updates
		newDestination, needsMemoUpdate := destinationUpdates[txIDStr]

		// Fix the external confirmation delay height to the correct value (should be external_observed_height)
		// Fix ALL observations, not just the consensus one
		fixedCount := 0
		for i := range voter.Txs {
			oldFinaliseHeight := voter.Txs[i].FinaliseHeight
			// This is just to correct the wrongfully reported transactions finalised heights. Since some of the observations reported we only want
			// the ones that are wrong in each fixed transactions
			if voter.Txs[i].FinaliseHeight == 11773983 || voter.Txs[i].FinaliseHeight == 11773978 || voter.Txs[i].FinaliseHeight == 11773976 {
				// Fix the incorrect finalise height - use same as block height for instant finalization
				voter.Txs[i].FinaliseHeight = voter.Txs[i].BlockHeight
				fixedCount++
				ctx.Logger().Info("Fixed observation confirmation delay",
					"tx_id", txIDStr,
					"observation_index", i,
					"observed_height", voter.Txs[i].BlockHeight,
					"old_finalise_height", oldFinaliseHeight,
					"new_finalise_height", voter.Txs[i].FinaliseHeight,
					"signers", len(voter.Txs[i].Signers))
			}

			// Update memo if needed
			if needsMemoUpdate && voter.Txs[i].Tx.Memo != "" {
				oldMemo := voter.Txs[i].Tx.Memo
				// Parse the memo and replace the destination address
				// Expected format: =:ASSET:DESTINATION:...
				memoParts := strings.Split(oldMemo, ":")
				if len(memoParts) >= 3 {
					memoParts[2] = newDestination
					newMemo := strings.Join(memoParts, ":")
					voter.Txs[i].Tx.Memo = newMemo
					ctx.Logger().Info("Updated transaction memo",
						"tx_id", txIDStr,
						"observation_index", i,
						"old_memo", oldMemo,
						"new_memo", newMemo)
				}
			}
		}

		// Also update the consensus Tx memo
		if needsMemoUpdate && voter.Tx.Tx.Memo != "" {
			oldMemo := voter.Tx.Tx.Memo
			memoParts := strings.Split(oldMemo, ":")
			if len(memoParts) >= 3 {
				memoParts[2] = newDestination
				newMemo := strings.Join(memoParts, ":")
				voter.Tx.Tx.Memo = newMemo
				ctx.Logger().Info("Updated consensus transaction memo",
					"tx_id", txIDStr,
					"old_memo", oldMemo,
					"new_memo", newMemo)
			}
		}

		// Fix the consensus transaction's FinaliseHeight
		// Use same height as block height for instant finalization
		oldConsensusHeight := voter.Tx.FinaliseHeight
		expectedHeight := voter.Tx.BlockHeight
		if voter.Tx.FinaliseHeight != expectedHeight {
			voter.Tx.FinaliseHeight = expectedHeight
			ctx.Logger().Info("Fixed consensus transaction finalise height",
				"tx_id", txIDStr,
				"observed_height", voter.Tx.BlockHeight,
				"old_finalise_height", oldConsensusHeight,
				"new_finalise_height", voter.Tx.FinaliseHeight)
		}

		if fixedCount > 0 {
			ctx.Logger().Info("Fixed confirmation delays for transaction",
				"tx_id", txIDStr,
				"total_observations", len(voter.Txs),
				"fixed_observations", fixedCount)
		}

		// Save the updated voter
		mgr.Keeper().SetObservedTxInVoter(ctx, voter)

		// If transaction is finalized but has no actions, process it
		if voter.FinalisedHeight > 0 && len(voter.Actions) == 0 && voter.Tx.Tx.Memo != "" {
			ctx.Logger().Info("Processing finalized transaction without actions", "tx_id", txIDStr)

			// Get signer address (use node account)
			nodeAccounts, err := mgr.Keeper().ListActiveValidators(ctx)
			if err != nil || len(nodeAccounts) == 0 {
				ctx.Logger().Error("Failed to get active validators", "error", err)
				continue
			}
			signer := nodeAccounts[0].NodeAddress

			// Process the transaction using the standard handler logic
			msg, txErr := processOneTxIn(ctx, mgr.GetVersion(), mgr.Keeper(), voter.Tx, signer)
			if txErr != nil {
				ctx.Logger().Error("fail to process inbound tx", "error", txErr.Error(), "tx_hash", voter.Tx.Tx.ID.String())
				if refundErr := refundTx(ctx, voter.Tx, mgr, CodeInvalidMemo, txErr.Error(), ""); refundErr != nil {
					ctx.Logger().Error("fail to refund", "error", refundErr.Error())
				}
				continue
			}

			// Handle the message based on its type
			if msg != nil {
				// If it's a swap, use addSwapDirect (all 6 transactions should be swaps)
				swapMsg, isSwap := msg.(*MsgSwap)
				if isSwap {
					ctx.Logger().Info("Adding swap to queue", "tx_id", txIDStr)
					addSwapDirect(ctx, mgr, *swapMsg)
				} else {
					ctx.Logger().Info("Transaction processed but not a swap", "tx_id", txIDStr)
				}
			}
		}

		// Log the final voter fields
		ctx.Logger().Info("Finalized stuck transaction",
			"tx_id", txIDStr,
			"consensus_height", voter.Height,
			"finalized_height", voter.FinalisedHeight,
			"consensus_tx_memo", voter.Tx.Tx.Memo,
			"consensus_tx_coins", voter.Tx.Tx.Coins.String(),
			"consensus_tx_gas", voter.Tx.Tx.Gas,
			"consensus_finalise_height", voter.Tx.FinaliseHeight,
			"num_observations", len(voter.Txs),
			"out_txs_count", len(voter.OutTxs))
	}

	ctx.Logger().Info("Completed v122 migration to finalize stuck transactions")

	// Fix the LastObserveHeight for nodes that observed these transactions
	migrateStoreV122FixLastObserveHeight(ctx, mgr)
}

// migrateStoreV122FixLastObserveHeight - fix incorrect LastObserveHeight that were set to MAYA heights instead of BTC/DASH heights
func migrateStoreV122FixLastObserveHeight(ctx cosmos.Context, mgr *Mgrs) {
	ctx.Logger().Info("Fixing LastObserveHeight for affected chains")

	// Reset BTC heights
	ctx.Logger().Info("Resetting BTC observation heights", "height", 903536)
	resetObservationHeights(ctx, mgr, 122, common.BTCChain, 903536)

	// Reset DASH heights
	ctx.Logger().Info("Resetting DASH observation heights", "height", 2297619)
	resetObservationHeights(ctx, mgr, 122, common.DASHChain, 2297619)

	ctx.Logger().Info("Completed fixing LastObserveHeight")
}

// migrateStoreV122Payouts - process payouts for v122 migration
func migrateStoreV122Payouts(ctx cosmos.Context, mgr *Mgrs) {
	ctx.Logger().Info("Processing v122 payouts")

	// Payout BTC - 0.86 BTC to bc1qztdn5395243l3zwskwdxaghgrgs8swy5fjrhls
	btcFromAddr, err := common.NewAddress("bc1qztdn5395243l3zwskwdxaghgrgs8swy5fjrhls", mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to parse BTC from address", "error", err)
		return
	}

	// Create streaming swap memo
	memo := "=:c:maya1a7gg93dgwlulsrqf6qtage985ujhpu068zllw7:0/1/0"
	btcCoins := common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(86000000))) // 0.86 BTC
	btcHash := makeTxID(btcFromAddr, btcFromAddr, btcCoins, memo)
	btcTxID, err := common.NewTxID(btcHash)
	if err != nil {
		ctx.Logger().Error("fail to create BTC tx id", "error", err)
		return
	}

	// Get vault for BTC
	activeAsgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil || len(activeAsgards) == 0 {
		ctx.Logger().Error("fail to get active asgard vaults", "error", err)
		return
	}
	vault := activeAsgards[0]
	vaultBTCAddress, err := vault.PubKey.GetAddress(common.BTCChain)
	if err != nil {
		ctx.Logger().Error("fail to get vault BTC address", "error", err)
		return
	}

	// Create fake BTC observation
	btcTx := common.Tx{
		ID:          btcTxID,
		Chain:       common.BTCChain,
		FromAddress: btcFromAddr,
		ToAddress:   vaultBTCAddress,
		Coins:       btcCoins,
		Gas: common.Gas{common.Coin{
			Asset:  common.BTCAsset,
			Amount: cosmos.NewUint(1000),
		}},
		Memo: memo,
	}

	// Get the last BTC chain height
	btcHeight, err := mgr.Keeper().GetLastChainHeight(ctx, common.BTCChain)
	if err != nil {
		ctx.Logger().Error("fail to get last BTC chain height", "error", err)
		btcHeight = 903536 // fallback to expected height
	}

	// Create and save the fake observation with same height for instant finalization
	btcObservedTx := NewObservedTx(btcTx, btcHeight, vault.PubKey, btcHeight)
	btcObservedTxs := ObservedTxs{btcObservedTx}
	if err = makeFakeTxInObservation(ctx, mgr, btcObservedTxs); err != nil {
		ctx.Logger().Error("fail to make BTC fake observation", "error", err)
	}

	// Payout RUNE - 25,422.69 RUNE to thor1a7gg93dgwlulsrqf6qtage985ujhpu0684pncw
	runeFromAddr, err := common.NewAddress("thor1a7gg93dgwlulsrqf6qtage985ujhpu0684pncw", mgr.GetVersion())
	if err != nil {
		ctx.Logger().Error("fail to parse RUNE from address", "error", err)
		return
	}

	// Create streaming swap memo
	runeCoins := common.NewCoins(common.NewCoin(common.RUNEAsset, cosmos.NewUint(2542269000000))) // 25,422.69 RUNE
	runeHash := makeTxID(runeFromAddr, runeFromAddr, runeCoins, memo)
	runeTxID, err := common.NewTxID(runeHash)
	if err != nil {
		ctx.Logger().Error("fail to create RUNE tx id", "error", err)
		return
	}

	// For THOR.RUNE we need to use the THOR chain
	vaultTHORAddress, err := vault.PubKey.GetAddress(common.THORChain)
	if err != nil {
		ctx.Logger().Error("fail to get vault THOR address", "error", err)
		return
	}

	// Create fake RUNE observation
	runeTx := common.Tx{
		ID:          runeTxID,
		Chain:       common.THORChain,
		FromAddress: runeFromAddr,
		ToAddress:   vaultTHORAddress,
		Coins:       runeCoins,
		Gas: common.Gas{common.Coin{
			Asset:  common.RUNEAsset,
			Amount: cosmos.NewUint(2000000),
		}},
		Memo: memo,
	}

	// Get the last THOR chain height
	thorHeight, err := mgr.Keeper().GetLastChainHeight(ctx, common.THORChain)
	if err != nil {
		ctx.Logger().Error("fail to get last THOR chain height", "error", err)
		thorHeight = 100000 // fallback to a reasonable height
	}

	// Create and save the fake observation with same height for instant finalization
	runeObservedTx := NewObservedTx(runeTx, thorHeight, vault.PubKey, thorHeight)
	runeObservedTxs := ObservedTxs{runeObservedTx}
	if err := makeFakeTxInObservation(ctx, mgr, runeObservedTxs); err != nil {
		ctx.Logger().Error("fail to make RUNE fake observation", "error", err)
	}

	ctx.Logger().Info("Completed v122 payouts",
		"btc_amount", "0.86 BTC",
		"btc_from", btcFromAddr,
		"btc_txid", btcTxID,
		"rune_amount", "25,422.69 RUNE",
		"rune_from", runeFromAddr,
		"rune_txid", runeTxID)
}

func migrateStoreV123RequeueDanglingActions(ctx cosmos.Context, mgr *Mgrs) {
	ctx.Logger().Info("Migrating store to v123 - requeueing dangling actions")
	// dropped ARB txs
	txIDs := common.TxIDs{
		"01E0A5C4AA29742C14D06AB56ADE3597C800E38DFAD9C3775F116D51DB7A9C26",
		"083FBA01202D365C25FF2CE610ECE079D7E04B0924ED920660AEFDEA6CFEC890",
		"2771B3F1880CF49C430CFCC302074F7C00AEE30F08FFE1959DF8F348495694E3",
		"2C012BD1817AC88201A925D852B205DDF0738DAB5006B31217DB5ABBA15696A6",
		"33B7F8CFCC62528623F9D9974DC11A76B818617801252CCD16E28A3F34FB1B82",
		"389B22E31A6EAC4A53400D3536ADCD3A9D75C1AA08F3F198440961239C4272D8",
		"4104331CF4C1BFD3850D452A910E004403D34631F3FCB6AC1DA7CDAE07525B8D",
		"4BA6CCA6BD447D38449D7891928C4AA55055F8A064FE89B2AC3BC0D1A2A4BDA6",
		"50AA42F2ED1052A7B5969F0BC1EBEAF1F5CE15F2E6D6D64BAD41182835026C86",
		"53233D76BEF65B38D2A652F205D95EC29AD4D71702F3001609FB566B5EB8CEBB",
		"5B454487D25E22112CBC54BD7AEE4C698A7C428E78D72F5B3471C4772F7060E8",
		"5CF645B49002A04BA48F429A441BFCEA90C25B7FEB6AF685F967427A85EFC906",
		"60B26E36103B76D88625C1FAD7B1564FEA6B16AE8A7935D4246DFCA960B5A2A5",
		"6653123C7EBE64BCFA3C2A5C7F8FE954707FB23C8021A5BEA260D18D9F4D10FE",
		"7FC90C5B373A2281F6CAFE6849DD78E67AEA9F08DF818D52FEC80A25FE6DCEEE",
		"80E97FCD0B84791EBA7BD77BF2728BD87EFEFF5951F319B5A85BA13508D30AC6",
		"82E60EABA1B92731616C5ED226892C5FEEEB77130244AB4E196E8227E517CFDF",
		"85B2037B09DAE58DAA063C22A2F33A62B761764C1B64EA117FDD175D56D441AB",
		"86946D783B4B4B012F47CAD28B1BBBED02C0355C43DAB28F7BB2F10EEA03E8EB",
		"86BE62B9B9432FBB47259840BFC9A806BBCC5198DF3F8257335134090A1F8961",
		"9AA3FFAEB43E8B9B0BD8B394EDA8FCDA04C6E843BDF89F0177043B232B5D3434",
		"B561294F0A077E727DC337D0EF7BA61D5245688EB347F1FC9DC419A28AED82A0",
		"C407D40C5E41C46507B04F8ADFEF140BCDB6ACE2A32EE848D7CA9CE850500D12",
		"C533094A82076F1A90568246CED27ABC276F5426A26CDC36B825DA23A8C4111D",
		"C6B49B177203F03870056A88C67A6B9D12D536A762E4ED2D473A3A0A973005B1",
		"D94A80EE6DE039849324217A2B188BCE923DB114EBC182BA8392CB6F8CFDFA64",
		"DCF7350CE6778636DF49D30030BB1792006067A0A49FD5F50B99A7A0DD726ACA",
		"DDF793E61FBBBD9572CF437922D3B66E871785C9D60CDC81B6E293BC23F208EF",
		"E0CE732DE6882E34B2F34218C778088D85102A753FBB898C7147CD0B69D35A34",
		"E0EECEB47B1849D0911CE8DA28D294067873A23374DC8AF1AE5C29A9D2AF85D7",
		"ED1666C51D21316B3C974AE0FAA9830FC90761123AB4244B81487167D28DE411",
		"F3C75E6B3A42A6D3E1F63567C1181096CCFDAB8251F90F7ADDB9C3BF316192A8",
		"F4E11AE2573827EFD1C08B84958B0BD6918F4B799358662B7619752F7C6316F2",
	}

	requeueDanglingActions(ctx, mgr, txIDs)
	ctx.Logger().Info("Completed v123 migration - requeued dangling actions")
}

func migrateStoreV123FixInsolvency(ctx cosmos.Context, mgr *Mgrs) {
	ctx.Logger().Info("Migrating store to v123 - fixing insolvency amounts in asgard vault")

	// Get active asgard vaults
	retiringAsgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, RetiringVault)
	if err != nil || len(retiringAsgards) == 0 {
		ctx.Logger().Error("fail to get active asgard vaults", "error", err)
		return
	}

	if len(retiringAsgards) > 1 {
		stp := mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)
		retiringAsgards = mgr.Keeper().SortBySecurity(ctx, retiringAsgards, stp)
	}

	btcAmount := cosmos.NewUint(56000000)       // 0.56 BTC
	runeAmount := cosmos.NewUint(3100000000000) // 31,000 RUNE

	var vault Vault
	for _, v := range retiringAsgards {
		vaultBTC := v.GetCoin(common.BTCAsset)
		vaultRUNE := v.GetCoin(common.RUNEAsset)
		if vaultBTC.Amount.GTE(btcAmount) && vaultRUNE.Amount.GTE(runeAmount) {
			vault = v
			break
		}
	}

	if vault.IsEmpty() {
		ctx.Logger().Error("no vault has sufficient funds to fix insolvency",
			"required_btc", btcAmount, "required_rune", runeAmount)
		return
	}

	fixInsolvency(ctx, mgr, &vault, common.BTCAsset, btcAmount)
	fixInsolvency(ctx, mgr, &vault, common.RUNEAsset, runeAmount)

	ctx.Logger().Info("Completed v123 migration - fixed insolvency amounts")
}

func migrateStoreV123(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v123", "error", err)
		}
	}()

	migrateStoreV123RequeueDanglingActions(ctx, mgr)
	migrateStoreV123FixInsolvency(ctx, mgr)
}
