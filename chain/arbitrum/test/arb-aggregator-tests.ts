import { ethers, getNamedAccounts, network, deployments } from "hardhat";
import { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers";
import { expect } from "chai";
import { Contract, Signer } from "ethers";
import { sushiSwapRouterAbi } from "./abis/sushiSwapRouterAbi";
import { USDT_ADDRESS, USDT_WHALE, WETH_ADDRESS } from "./constants";
import { ArbAggregator, ArbRouter } from "../typechain-types";
import ERC20 from "@openzeppelin/contracts/build/contracts/ERC20.json";

describe("ArbAggregator", function () {
  let accounts: SignerWithAddress[];
  let arbAggregator: ArbAggregator;
  let arbRouter: ArbRouter;
  let usdtToken: Contract;
  let sushiSwap: any;
  const sushiSwapRouter = "0x1b02dA8Cb0d097eB8D57A175b88c7D8b47997506";

  beforeEach(async () => {
    const { admin, wallet1, wallet2 } = await getNamedAccounts();
    const usdtAmount = "150000000000"; // 6 dec

    accounts = await ethers.getSigners();
    await deployments.fixture();

    sushiSwap = new ethers.Contract(
      sushiSwapRouter,
      sushiSwapRouterAbi,
      accounts[0],
    );
    const arbRouterDeployment = await ethers.getContractFactory("ArbRouter");
    arbRouter = await arbRouterDeployment.deploy();

    const arbAggregatorDeployment =
      await ethers.getContractFactory("ArbAggregator");

    arbAggregator = await arbAggregatorDeployment.deploy(
      WETH_ADDRESS,
      sushiSwapRouter,
    );
    usdtToken = new ethers.Contract(USDT_ADDRESS, ERC20.abi, accounts[0]);

    await network.provider.request({
      method: "hardhat_impersonateAccount",
      params: [USDT_WHALE],
    });

    const whaleSigner = await ethers.getSigner(USDT_WHALE);
    const usdtWhale = usdtToken.connect(whaleSigner);
    await usdtWhale.transfer(admin, usdtAmount);
    await usdtWhale.transfer(wallet1, usdtAmount);
    await usdtWhale.transfer(wallet2, usdtAmount);
    let usdtBalance = await usdtWhale.balanceOf(wallet1);
    expect(usdtBalance).gt(0);
    usdtBalance = await usdtWhale.balanceOf(wallet2);
    expect(usdtBalance).gt(0);
  });

  describe("initialize", function () {
    it("Should init", async () => {
      expect(ethers.utils.isAddress(sushiSwap.address)).eq(true);
      expect(sushiSwap.address).to.not.eq(ethers.constants.AddressZero);
    });
  });

  describe("Swap In and Out", function () {
    it("Should swap ETH for USDT in sushiSwap", async () => {
      const { wallet1 } = await getNamedAccounts();

      const amountOutMin = "1000";

      const wallet1Signer = accounts.find(
        (account) => account.address === wallet1,
      );
      const sushiSwapWallet1 = sushiSwap.connect(wallet1Signer as Signer);
      const currentBlock = await ethers.provider.getBlockNumber();
      const currentTime = (await ethers.provider.getBlock(currentBlock))
        .timestamp;

      await sushiSwapWallet1.swapExactETHForTokens(
        amountOutMin,
        [WETH_ADDRESS, USDT_ADDRESS],
        wallet1,
        currentTime + 1000000000,
        { value: ethers.utils.parseEther("0.1") },
      );

      const usdtContract = await ethers.getContractAt("IERC20", USDT_ADDRESS);
      const balanceOfusdt = await usdtContract.balanceOf(wallet1);
      // Doesn't matter what the result from sushiSwap is
      expect(balanceOfusdt).gt(0);
    });
    it("Should Swap In Token for ETH", async function () {
      const { wallet2, asgard1 } = await getNamedAccounts();

      const transferAmount = "10000000000";
      const initialArbBalance = "10000000000000000000000";
      expect(await ethers.provider.getBalance(asgard1)).to.equal(
        initialArbBalance,
      );

      const wallet2Signer = accounts.find(
        (account) => account.address === wallet2,
      );

      // approve usdt transfer
      const usdtTokenWallet2 = usdtToken.connect(wallet2Signer as Signer);
      const arbAggregatorWallet2 = arbAggregator.connect(
        wallet2Signer as Signer,
      );
      await usdtTokenWallet2.approve(
        arbAggregator.address,
        "10000000000000000000",
      );

      const deadline = ~~(Date.now() / 1000) + 100;

      const tx = await arbAggregatorWallet2.swapIn(
        asgard1,
        arbRouter.address,
        "SWAP:THOR.RUNE:tthor1uuds8pd92qnnq0udw0rpg0szpgcslc9p8lluej",
        usdtToken.address,
        transferAmount,
        0,
        deadline,
      );
      tx.wait();

      expect(await usdtToken.balanceOf(wallet2)).to.equal("140000000000");
      expect(await ethers.provider.getBalance(wallet2)).lt(initialArbBalance);
      expect(await ethers.provider.getBalance(asgard1)).gt(initialArbBalance);
    });
    it("Should Swap In USDT for ETH", async function () {
      const { wallet2, asgard1 } = await getNamedAccounts();
      const transferAmount = "10000000000";
      const initialArbBalance = "10000000000000000000000";
      expect(await ethers.provider.getBalance(asgard1)).to.equal(
        initialArbBalance,
      );

      const wallet2Signer = accounts.find(
        (account) => account.address === wallet2,
      );

      // approve usdt transfer
      const usdtTokenWallet2 = usdtToken.connect(wallet2Signer as Signer);
      const arbAggregatorWallet2 = arbAggregator.connect(
        wallet2Signer as Signer,
      );
      await usdtTokenWallet2.approve(
        arbAggregator.address,
        "10000000000000000000",
      );

      const deadline = ~~(Date.now() / 1000) + 100;

      await arbAggregatorWallet2.swapIn(
        asgard1,
        arbRouter.address,
        "SWAP:BTC.BTC:bc1Address:",
        usdtToken.address,
        transferAmount,
        0,
        deadline,
      );

      expect(await usdtToken.balanceOf(wallet2)).to.equal("140000000000");
      expect(await ethers.provider.getBalance(wallet2)).eq(
        "9999936572500000000000",
      );
      expect(await ethers.provider.getBalance(asgard1)).eq(
        "10002683860015956866123",
      );
    });

    it("Should Swap Out using Aggregator", async function () {
      const { wallet2, asgard1 } = await getNamedAccounts();
      expect(await usdtToken.balanceOf(wallet2)).to.equal("150000000000");
      expect(await ethers.provider.getBalance(wallet2)).eq(
        "10000000000000000000000",
      );

      const wallet2Signer = accounts.find(
        (account) => account.address === wallet2,
      );
      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1,
      );

      // approve usdt transfer
      const usdtTokenWallet2 = usdtToken.connect(wallet2Signer as Signer);
      const arbRouterAsgard1 = arbRouter.connect(asgard1Signer as Signer);

      await usdtTokenWallet2.approve(
        arbAggregator.address,
        "10000000000000000000",
      );

      // Send 10 token to agg, which sends it to Sushi for 1 WETH,
      // Then unwraps to 1 ETH, then sends 1 ETH to Asgard vault
      await arbRouterAsgard1.transferOutAndCall(
        arbAggregator.address,
        usdtToken.address,
        wallet2,
        "0",
        "OUT:HASH",
        { value: ethers.utils.parseEther("1") },
      );

      expect(await ethers.provider.getBalance(asgard1)).eq(
        "9998963624700000000000",
      );
      expect(await usdtToken.balanceOf(wallet2)).to.equal("153515442262");
    });

    it("Should Fail Swap Out using Aggregator", async function () {
      const { wallet2, asgard1 } = await getNamedAccounts();
      expect(await usdtToken.balanceOf(wallet2)).to.equal("150000000000");
      expect(await ethers.provider.getBalance(wallet2)).eq(
        "10000000000000000000000",
      );

      const wallet2Signer = accounts.find(
        (account) => account.address === wallet2,
      );
      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1,
      );

      // approve usdt transfer
      const usdtTokenWallet2 = usdtToken.connect(wallet2Signer as Signer);
      const arbRouterAsgard1 = arbRouter.connect(asgard1Signer as Signer);

      await usdtTokenWallet2.approve(
        arbAggregator.address,
        "10000000000000000000",
      );

      // Send 10 token to agg, which sends it to Sushi for 1 WETH,
      // Then unwraps to 1 ETH, then sends 1 ETH to Asgard vault
      await arbRouterAsgard1.transferOutAndCall(
        arbAggregator.address,
        usdtToken.address,
        wallet2,
        "99999999999999999999999999999999999",
        "OUT:HASH",
        { value: ethers.utils.parseEther("1") },
      );

      expect(await ethers.provider.getBalance(asgard1)).eq(
        "9998982416475000000000",
      );
      expect(await ethers.provider.getBalance(wallet2)).eq(
        "10000987943600000000000",
      );
      expect(await usdtToken.balanceOf(wallet2)).to.equal("150000000000");
    });

    it("Should Fail Swap Out with ETH using Aggregator", async function () {
      const { wallet2, asgard1 } = await getNamedAccounts();
      expect(await usdtToken.balanceOf(wallet2)).to.equal("150000000000");
      expect(await ethers.provider.getBalance(wallet2)).eq(
        "10000000000000000000000",
      );
      expect(await ethers.provider.getBalance(ethers.constants.AddressZero)).eq(
        "17643903429625667365",
      );

      const wallet2Signer = accounts.find(
        (account) => account.address === wallet2,
      );
      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1,
      );

      // approve usdt transfer
      const usdtTokenWallet2 = usdtToken.connect(wallet2Signer as Signer);
      const arbRouterAsgard1 = arbRouter.connect(asgard1Signer as Signer);

      await usdtTokenWallet2.approve(
        arbAggregator.address,
        "10000000000000000000",
      );

      // Send 10 token to agg, which sends it to Sushi for 1 WETH,
      // Then unwraps to 1 ETH, then sends 1 ETH to Asgard vault
      await arbRouterAsgard1.transferOutAndCall(
        arbAggregator.address,
        ethers.constants.AddressZero,
        wallet2,
        "99999999999999999999999999999999999",
        "OUT:HASH",
        { value: ethers.utils.parseEther("1") },
      );

      expect(await ethers.provider.getBalance(asgard1)).eq(
        "9998984106000000000000",
      );
      expect(await ethers.provider.getBalance(ethers.constants.AddressZero)).eq(
        "17643903429625667365",
      );
      expect(await usdtToken.balanceOf(wallet2)).to.equal("150000000000");
    });
  });
});
