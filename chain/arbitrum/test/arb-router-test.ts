import { deployments, ethers, getNamedAccounts, network } from "hardhat";
import { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers";
import { expect } from "chai";
import { BigNumber, Contract, Signer } from "ethers";
import { USDT_ADDRESS, USDT_WHALE } from "./constants";
import ERC20 from "@openzeppelin/contracts/build/contracts/ERC20.json";
import { ArbRouter } from "../typechain-types";
import { Receipt } from "hardhat-deploy/dist/types";

describe("ArbRouter", function () {
  let accounts: SignerWithAddress[];
  let arbRouter: ArbRouter;
  let arbRouter2: ArbRouter;
  let usdtToken: Contract;
  const ETH = ethers.constants.AddressZero;

  beforeEach(async () => {
    const { wallet1, wallet2 } = await getNamedAccounts();

    accounts = await ethers.getSigners();
    await deployments.fixture();
    const arbRouterDeployment = await ethers.getContractFactory("ArbRouter");
    arbRouter = await arbRouterDeployment.deploy();
    arbRouter2 = await arbRouterDeployment.deploy();

    usdtToken = new ethers.Contract(USDT_ADDRESS, ERC20.abi, accounts[0]);

    // Transfer UCDCE to wallet1
    const usdtAmount = "150000000000"; // 6 dec

    await network.provider.request({
      method: "hardhat_impersonateAccount",
      params: [USDT_WHALE],
    });

    const whaleSigner = await ethers.getSigner(USDT_WHALE);
    const usdtWhale = usdtToken.connect(whaleSigner);
    await usdtWhale.transfer(wallet1, usdtAmount);
    await usdtWhale.transfer(wallet2, usdtAmount);

    let usdtBalance = await usdtWhale.balanceOf(wallet1);
    expect(usdtBalance).gt(0);
    usdtBalance = await usdtWhale.balanceOf(wallet2);
    expect(usdtBalance).gt(0);
  });

  describe("initialize", function () {
    it("Should init", async () => {
      expect(ethers.utils.isAddress(arbRouter.address)).eq(true);
      expect(arbRouter.address).to.not.eq(ethers.constants.AddressZero);
    });
  });

  describe("User Deposit Assets", function () {
    it("Should Deposit ETH To Asgard", async function () {
      const { asgard1 } = await getNamedAccounts();
      const amount = ethers.utils.parseEther("1000");

      const startBal = BigNumber.from(
        await ethers.provider.getBalance(asgard1),
      );
      const tx = await arbRouter.deposit(
        asgard1,
        ETH,
        amount,
        "SWAP:MAYA.CACAO",
        { value: amount },
      );
      const receipt = await tx.wait();

      expect(receipt?.events?.[0].event).to.equal("Deposit");
      expect(tx.value).to.equal(amount);
      expect(receipt?.events?.[0]?.args?.asset).to.equal(ETH);
      expect(receipt?.events?.[0]?.args?.memo).to.equal("SWAP:MAYA.CACAO");

      const endBal = BigNumber.from(await ethers.provider.getBalance(asgard1));
      const changeBal = BigNumber.from(endBal).sub(startBal);
      expect(changeBal).to.equal(amount);
    });
    it("Should revert expired Deposit ETH To Asgard1", async function () {
      const { asgard1 } = await getNamedAccounts();
      const amount = ethers.utils.parseEther("1000");

      await expect(
        arbRouter.depositWithExpiry(
          asgard1,
          ETH,
          amount,
          "SWAP:MAYA.CACAO:tthor1uuds8pd92qnnq0udw0rpg0szpgcslc9p8lluej",
          BigNumber.from(0),
          { value: amount },
        ),
      ).to.be.revertedWith("MAYAChain_Router: expired");
    });
    it("Should Deposit Token to Asgard1", async function () {
      const { wallet1, asgard1 } = await getNamedAccounts();
      const amount = "500000000";

      const wallet1Signer = accounts.find(
        (account) => account.address === wallet1,
      );
      const arbRouterWallet1 = arbRouter.connect(wallet1Signer as Signer);

      // approve usdt transfer
      const usdtTokenWallet1 = usdtToken.connect(wallet1Signer as Signer);
      await usdtTokenWallet1.approve(arbRouterWallet1.address, amount);

      const tx = await arbRouterWallet1.deposit(
        asgard1,
        usdtToken.address,
        amount,
        "SWAP:MAYA.CACAO",
      );
      const receipt: Receipt = await tx.wait();

      const event = receipt?.events?.find((event: any) => event.logIndex === 2);
      expect(event.event).to.equal("Deposit");
      expect(event.args?.asset.toLowerCase()).to.equal(USDT_ADDRESS);
      expect(event.args?.to).to.equal(asgard1);
      expect(event.args?.memo).to.equal("SWAP:MAYA.CACAO");
      expect(event.args?.amount).to.equal(amount);

      expect(await usdtToken.balanceOf(arbRouter.address)).to.equal(amount);
      expect(
        await arbRouterWallet1.vaultAllowance(asgard1, usdtToken.address),
      ).to.equal(amount);
    });
    it("Should revert Deposit Token to Asgard1", async function () {
      const { asgard1 } = await getNamedAccounts();
      const amount = "500000000";

      await expect(
        arbRouter.depositWithExpiry(
          asgard1,
          usdtToken.address,
          amount,
          "SWAP:MAYA.CACAO:tmaya1uuds8pd92qnnq0udw0rpg0szpgcslc9p8gps0z",
          BigNumber.from(0),
        ),
      ).to.be.revertedWith("MAYAChain_Router: expired");
    });
    it("Should revert when ETH sent during ERC20 Deposit", async function () {
      const { asgard1 } = await getNamedAccounts();
      const amount = ethers.utils.parseEther("1000");

      await expect(
        arbRouter.deposit(
          asgard1,
          usdtToken.address,
          amount,
          "SWAP:MAYA.CACAO",
          { value: amount },
        ),
      ).to.be.revertedWith("unexpected eth");
    });
  });
  describe("Fund Yggdrasil, Yggdrasil Transfer Out", function () {
    it("Should fund yggdrasil with ETH", async function () {
      const { asgard1, yggdrasil } = await getNamedAccounts();
      const amount400 = ethers.utils.parseEther("400");
      const amount300 = ethers.utils.parseEther("300");

      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1,
      );
      const arbRouterAsgard1 = arbRouter.connect(asgard1Signer as Signer);

      const startBal = BigNumber.from(
        await ethers.provider.getBalance(yggdrasil),
      );
      const tx = await arbRouterAsgard1.transferOut(
        yggdrasil,
        ETH,
        amount300,
        "ygg+:123",
        { value: amount400 },
      );
      const receipt: Receipt = await tx.wait();

      expect(receipt.events?.[0].event).to.equal("TransferOut");
      expect(receipt.events?.[0].args?.asset).to.equal(ETH);
      expect(receipt.events?.[0].args?.vault).to.equal(asgard1);
      expect(receipt.events?.[0].args?.amount).to.equal(amount400);
      expect(receipt.events?.[0].args?.memo).to.equal("ygg+:123");

      const endBal = BigNumber.from(
        await ethers.provider.getBalance(yggdrasil),
      );
      const changeBal = endBal.sub(startBal).toString();
      expect(changeBal).to.equal(amount400);
    });

    it("Should fund yggdrasil with tokens", async function () {
      const { wallet1, asgard1, yggdrasil } = await getNamedAccounts();
      const amount = "10000000000";

      // give asgard1 usdt
      const wallet1Signer = accounts.find(
        (account) => account.address === wallet1,
      );
      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1,
      );

      const usdtWallet1 = usdtToken.connect(wallet1Signer as Signer);
      await usdtWallet1.approve(arbRouter.address, amount);

      const arbRouterWallet1 = arbRouter.connect(wallet1Signer as Signer);

      let tx = await arbRouterWallet1.deposit(
        asgard1,
        usdtToken.address,
        amount,
        "SWAP:MAYA.CACAO",
      );

      const arbRouterAsgard1 = arbRouter.connect(asgard1Signer as Signer);

      // approve usdt transfer
      const usdtTokenAsgard1 = usdtToken.connect(asgard1Signer as Signer);
      await usdtTokenAsgard1.approve(arbRouter.address, amount);

      tx = await arbRouterAsgard1.transferAllowance(
        arbRouter.address,
        yggdrasil,
        usdtToken.address,
        amount,
        "yggdrasil+:1234",
      );
      const receipt: Receipt = await tx.wait();
      expect(receipt.events?.[0]?.event).to.equal("TransferAllowance");
      expect(receipt.events?.[0]?.args?.newVault).to.equal(yggdrasil);
      expect(receipt.events?.[0]?.args?.amount).to.equal(amount);

      expect(await usdtToken.balanceOf(arbRouter.address)).to.equal(amount);
      expect(
        await arbRouter.vaultAllowance(yggdrasil, usdtToken.address),
      ).to.equal(amount);
      expect(
        await arbRouter.vaultAllowance(asgard1, usdtToken.address),
      ).to.equal("0");
    });

    it("Should transfer ETH to Wallet2", async function () {
      const { wallet2, yggdrasil } = await getNamedAccounts();

      const amount = ethers.utils.parseEther("10");

      const yggdrasilSigner = accounts.find(
        (account) => account.address === yggdrasil,
      );
      const arbRouterYggdrasil = arbRouter.connect(yggdrasilSigner as Signer);

      const startBal = BigNumber.from(
        await ethers.provider.getBalance(wallet2),
      );
      const tx = await arbRouterYggdrasil.transferOut(
        wallet2,
        ETH,
        amount,
        "OUT:",
        { value: amount },
      );
      const receipt: Receipt = await tx.wait();

      expect(receipt.events?.[0]?.event).to.equal("TransferOut");
      expect(receipt.events?.[0]?.args?.to).to.equal(wallet2);
      expect(receipt.events?.[0]?.args?.asset).to.equal(ETH);
      expect(receipt.events?.[0]?.args?.memo).to.equal("OUT:");
      expect(receipt.events?.[0]?.args?.amount).to.equal(amount);

      const endBal = BigNumber.from(await ethers.provider.getBalance(wallet2));
      const changeBal = endBal.sub(startBal);
      expect(changeBal).to.equal(amount);
    });

    it("Should take ETH amount from the amount in transaction, instead of the amount parameter", async function () {
      const { wallet2, yggdrasil } = await getNamedAccounts();

      const amount20 = ethers.utils.parseEther("20");
      const amount10 = ethers.utils.parseEther("10");

      const yggdrasilSigner = accounts.find(
        (account) => account.address === yggdrasil,
      );
      const arbRouterYggdrasil = arbRouter.connect(yggdrasilSigner as Signer);

      const startBal = BigNumber.from(
        await ethers.provider.getBalance(wallet2),
      );
      const tx = await arbRouterYggdrasil.transferOut(
        wallet2,
        ETH,
        amount20,
        "OUT:",
        { value: amount10 },
      );
      const receipt: Receipt = await tx.wait();
      expect(receipt.events?.[0]?.event).to.equal("TransferOut");
      expect(receipt.events?.[0]?.args?.to).to.equal(wallet2);
      expect(receipt.events?.[0]?.args?.asset).to.equal(ETH);
      expect(receipt.events?.[0]?.args?.memo).to.equal("OUT:");
      expect(receipt.events?.[0]?.args?.amount).to.equal(amount10);

      const endBal = BigNumber.from(await ethers.provider.getBalance(wallet2));
      const changeBal = endBal.sub(startBal);
      expect(changeBal).to.equal(amount10);
    });

    it("Should transfer tokens to Wallet2", async function () {
      const { wallet2, yggdrasil, asgard1 } = await getNamedAccounts();
      const initialAmount = BigNumber.from("5000000000");
      const amount = initialAmount.div(2);

      const yggdrasilSigner = accounts.find(
        (account) => account.address === yggdrasil,
      );
      const arbRouterYggdrasilSigner = arbRouter.connect(
        yggdrasilSigner as Signer,
      );

      const wallet2Signer = accounts.find(
        (account) => account.address === wallet2,
      );

      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1,
      );
      const arbRouterAsgard1Signer = arbRouter.connect(asgard1Signer as Signer);

      const usdtWallet2 = usdtToken.connect(wallet2Signer as Signer);
      await usdtWallet2.approve(arbRouter.address, initialAmount);

      const arbRouterWallet2 = arbRouter.connect(wallet2Signer as Signer);

      await arbRouterWallet2.deposit(
        asgard1,
        usdtToken.address,
        initialAmount,
        "SWAP:MAYA.CACAO",
      );

      await arbRouterAsgard1Signer.transferAllowance(
        arbRouter.address,
        yggdrasil,
        usdtToken.address,
        initialAmount,
        "yggdrasil+:1234",
      );
      expect(
        await arbRouter.vaultAllowance(yggdrasil, usdtToken.address),
      ).to.equal(initialAmount);

      const usdtWallet1 = usdtToken.connect(wallet2Signer as Signer);
      await usdtWallet1.approve(arbRouter.address, initialAmount);

      const tx = await arbRouterYggdrasilSigner.transferOut(
        wallet2,
        usdtToken.address,
        amount,
        "OUT:",
      );
      const receipt: Receipt = await tx.wait();

      const event = receipt?.events?.find((event: any) => event.logIndex === 1);

      expect(event.event).to.equal("TransferOut");
      expect(event.args?.to).to.equal(wallet2);
      expect(event.args?.asset.toLowerCase()).to.equal(usdtToken.address);
      expect(event.args?.memo).to.equal("OUT:");
      expect(event.args?.amount).to.equal(amount);

      expect(
        await arbRouter.vaultAllowance(yggdrasil, usdtToken.address),
      ).to.equal(amount);
      expect(await usdtToken.balanceOf(arbRouter.address)).to.equal(amount);
    });
  });

  describe("Yggdrasil Returns Funds, Asgard Churns, Old Vaults can't spend", function () {
    it("Ygg returns", async function () {
      const { wallet1, asgard1, yggdrasil } = await getNamedAccounts();

      const arbBal = ethers.utils.parseEther("20");
      const tokenAmount = "200000000";

      const coins = {
        asset: usdtToken.address,
        amount: "200000000",
      };

      const yggdrasilSigner = accounts.find(
        (account) => account.address === yggdrasil,
      );
      const arbRouterYggdrasil = arbRouter.connect(yggdrasilSigner as Signer);

      const wallet1Signer = accounts.find(
        (account) => account.address === wallet1,
      );
      const arbRouterWallet1 = arbRouter.connect(wallet1Signer as Signer);

      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1,
      );
      const arbRouterAsgard1 = arbRouter.connect(asgard1Signer as Signer);

      const usdtWallet1 = usdtToken.connect(wallet1Signer as Signer);
      await usdtWallet1.approve(arbRouter.address, tokenAmount);

      let tx = await arbRouterWallet1.deposit(
        asgard1,
        usdtToken.address,
        tokenAmount,
        "SWAP:MAYA.CACAO",
      );

      // approve usdt transfer
      const usdtTokenAsgard1 = usdtToken.connect(asgard1Signer as Signer);
      await usdtTokenAsgard1.approve(arbRouter.address, tokenAmount);

      expect(await usdtToken.balanceOf(arbRouter.address)).to.equal(
        tokenAmount,
      );
      expect(
        await arbRouterWallet1.vaultAllowance(asgard1, usdtToken.address),
      ).to.equal(tokenAmount);

      tx = await arbRouterAsgard1.transferAllowance(
        arbRouter.address,
        yggdrasil,
        usdtToken.address,
        tokenAmount,
        "yggdrasil+:1234",
      );

      tx = await arbRouterYggdrasil.returnVaultAssets(
        arbRouter.address,
        asgard1,
        [coins],
        "yggdrasil-:1234",
        { from: yggdrasil, value: arbBal },
      );
      const receipt = await tx.wait();
      expect(receipt.events?.[0]?.event).to.equal("VaultTransfer");
      expect(receipt.events?.[0]?.args?.coins[0].asset.toLowerCase()).to.equal(
        usdtToken.address,
      );
      expect(receipt.events?.[0]?.args?.coins[0].amount).to.equal(tokenAmount);
      expect(receipt.events?.[0]?.args?.memo).to.equal("yggdrasil-:1234");

      expect(await usdtToken.balanceOf(arbRouter.address)).to.equal(
        tokenAmount,
      );
      expect(
        await arbRouter.vaultAllowance(yggdrasil, usdtToken.address),
      ).to.equal("0");
      expect(
        await arbRouter.vaultAllowance(asgard1, usdtToken.address),
      ).to.equal(tokenAmount);
    });
    it("Asgard Churns", async function () {
      const { wallet1, asgard1, asgard2 } = await getNamedAccounts();
      const amount = "10000000000";

      const wallet1Signer = accounts.find(
        (account) => account.address === wallet1,
      );
      const arbRouterWallet1 = arbRouter.connect(wallet1Signer as Signer);

      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1,
      );
      const arbRouterAsgard1 = arbRouter.connect(asgard1Signer as Signer);

      const usdtWallet1 = usdtToken.connect(wallet1Signer as Signer);
      await usdtWallet1.approve(arbRouter.address, amount);

      let tx = await arbRouterWallet1.deposit(
        asgard1,
        usdtToken.address,
        amount,
        "SWAP:MAYA.CACAO",
      );

      // approve usdt transfer
      const usdtTokenAsgard1 = usdtToken.connect(asgard1Signer as Signer);
      await usdtTokenAsgard1.approve(arbRouter.address, amount);

      tx = await arbRouterAsgard1.transferAllowance(
        arbRouter.address,
        asgard2,
        usdtToken.address,
        amount,
        "migrate:1234",
      );
      const receipt = await tx.wait();

      expect(receipt.events?.[0]?.event).to.equal("TransferAllowance");
      expect(receipt.events?.[0]?.args?.asset.toLowerCase()).to.equal(
        usdtToken.address,
      );
      expect(receipt.events?.[0]?.args?.amount).to.equal(amount);

      expect(await usdtToken.balanceOf(arbRouter.address)).to.equal(amount);
      expect(
        await arbRouter.vaultAllowance(asgard1, usdtToken.address),
      ).to.equal("0");
      expect(
        await arbRouter.vaultAllowance(asgard2, usdtToken.address),
      ).to.equal(amount);
    });
    it("Should fail to when old Asgard interacts", async function () {
      const { asgard1, asgard2, wallet2 } = await getNamedAccounts();
      const amount5k = ethers.utils.parseEther("5000");
      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1,
      );
      const arbRouterAsgard1 = arbRouter.connect(asgard1Signer as Signer);

      await expect(
        arbRouterAsgard1.transferAllowance(
          arbRouter.address,
          asgard2,
          usdtToken.address,
          amount5k,
          "migrate:1234",
        ),
      ).to.be.reverted;
      await expect(
        arbRouterAsgard1.transferOut(
          wallet2,
          usdtToken.address,
          amount5k,
          "OUT:",
        ),
      ).to.be.reverted;
    });
    it("Should fail to when old Yggdrasil interacts", async function () {
      const { yggdrasil, asgard2, wallet2 } = await getNamedAccounts();

      const yggdrasilSigner = accounts.find(
        (account) => account.address === yggdrasil,
      );
      const arbRouterYggdrasil = arbRouter.connect(yggdrasilSigner as Signer);

      const amount5k = ethers.utils.parseEther("5000");

      await expect(
        arbRouterYggdrasil.transferAllowance(
          arbRouter.address,
          asgard2,
          usdtToken.address,
          amount5k,
          "migrate:1234",
        ),
      ).to.be.reverted;
      await expect(
        arbRouterYggdrasil.transferOut(
          wallet2,
          usdtToken.address,
          amount5k,
          "OUT:",
        ),
      ).to.be.reverted;
    });
  });

  describe("Upgrade contract", function () {
    it("should return vault assets to new router", async function () {
      const { yggdrasil, asgard1, asgard3, wallet1 } = await getNamedAccounts();
      const amount5kusdt = "5000000000";

      const wallet1Signer = accounts.find(
        (account) => account.address === wallet1,
      );
      const yggdrasilSigner = accounts.find(
        (account) => account.address === yggdrasil,
      );
      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1,
      );
      const arbRouterYggdrasil = arbRouter.connect(yggdrasilSigner as Signer);
      const arbRouterAsgard1 = arbRouter.connect(asgard1Signer as Signer);
      const arbRouterWallet1 = arbRouter.connect(wallet1Signer as Signer);

      // approve usdt transfer
      const usdtTokenWallet1 = usdtToken.connect(wallet1Signer as Signer);
      await usdtTokenWallet1.approve(arbRouter.address, amount5kusdt);

      await arbRouterWallet1.deposit(
        asgard1,
        usdtToken.address,
        amount5kusdt,
        "SEED",
      );
      // await ROUTER1.deposit(yggdrasil, usdt.address, _50k, 'SEED', { from: USER1 });
      // await arbRouterWallet2.deposit(yggdrasil, ETH, '0', 'SEED ETH', { value: amount1 });
      const arbBal = ethers.utils.parseEther("20");

      // migrate _50k from asgard1 to asgard3 , to new arbRouter2 contract
      const coin1 = {
        asset: usdtToken.address,
        amount: amount5kusdt,
      };
      // let coin2 = {
      //     asset: usdt.address,
      //     amount: amount1
      // }

      const usdtTokenAsgard1 = usdtToken.connect(asgard1Signer as Signer);
      await usdtTokenAsgard1.approve(arbRouter.address, amount5kusdt);
      let tx = await arbRouterAsgard1.transferAllowance(
        arbRouter.address,
        yggdrasil,
        usdtToken.address,
        amount5kusdt,
        "yggdrasil+:1234",
      );

      tx = await arbRouterYggdrasil.returnVaultAssets(
        arbRouter2.address,
        asgard3,
        [coin1],
        "yggdrasil-:1234",
        { value: arbBal },
      );
      const receipt: Receipt = await tx.wait();

      const event = receipt?.events?.find((event) => event.logIndex === 3);
      expect(event?.event).to.equal("Deposit");
      expect(event?.args?.to).to.equal(asgard3);
      expect(event?.args?.asset.toLowerCase()).to.equal(usdtToken.address);
      expect(event?.args?.memo).to.equal("yggdrasil-:1234");
      expect(event?.args?.amount).to.equal(amount5kusdt);

      // make sure the token had been transfer to asgardex3 and arbRouter2
      expect(await usdtToken.balanceOf(arbRouter2.address)).to.equal(
        amount5kusdt,
      );
      expect(
        await arbRouter2.vaultAllowance(asgard3, usdtToken.address),
      ).to.equal(amount5kusdt);
      expect(
        await arbRouter.vaultAllowance(asgard1, usdtToken.address),
      ).to.equal("0");
    });

    it("should transfer all token and allowance to new contract", async function () {
      const { asgard1, asgard3, wallet1, wallet2 } = await getNamedAccounts();
      const amount5kusdt = "5000000000";
      const amount2k = ethers.utils.parseEther("2000");
      const amount1 = ethers.utils.parseEther("1");

      const wallet1Signer = accounts.find(
        (account) => account.address === wallet1,
      );
      const wallet2Signer = accounts.find(
        (account) => account.address === wallet2,
      );
      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1,
      );
      const arbRouterAsgard1 = arbRouter.connect(asgard1Signer as Signer);
      const arbRouterWallet1 = arbRouter.connect(wallet1Signer as Signer);
      const arbRouterWallet2 = arbRouter.connect(wallet2Signer as Signer);

      const asgard1StartBalance = BigNumber.from(
        await ethers.provider.getBalance(asgard1),
      );

      const usdtTokenWallet1 = usdtToken.connect(wallet1Signer as Signer);
      await usdtTokenWallet1.approve(arbRouter.address, amount5kusdt);

      await arbRouterWallet1.deposit(
        asgard1,
        usdtToken.address,
        amount5kusdt,
        "SEED",
      );
      // await arbRouterWallet1.deposit(asgard1, usdt.address, _50k, 'SEED');
      await arbRouterWallet2.deposit(asgard1, ETH, "0", "SEED ETH", {
        value: amount1,
      });

      const asgard1EndBalance = BigNumber.from(
        await ethers.provider.getBalance(asgard1),
      );
      expect(asgard1EndBalance.sub(asgard1StartBalance)).to.equal(amount1);

      // migrate _50k from asgard1 to asgard3 , to new Router3 contract
      const tx = await arbRouterAsgard1.transferAllowance(
        arbRouter2.address,
        asgard3,
        usdtToken.address,
        amount5kusdt,
        "MIGRATE:1",
      );
      const receipt: Receipt = await tx.wait();

      const event = receipt?.events?.find((event) => event.logIndex === 3);
      expect(event?.event).to.equal("Deposit");
      expect(event?.args?.to).to.equal(asgard3);
      expect(event?.args?.asset.toLowerCase()).to.equal(usdtToken.address);
      expect(event?.args?.memo).to.equal("MIGRATE:1");
      expect(event?.args?.amount).to.equal(amount5kusdt);

      // make sure the token had been transfer to ASGARD3 and Router3
      expect(await usdtToken.balanceOf(arbRouter2.address)).to.equal(
        amount5kusdt,
      );
      expect(
        await arbRouter2.vaultAllowance(asgard3, usdtToken.address),
      ).to.equal(amount5kusdt);
      expect(
        await arbRouter.vaultAllowance(asgard1, usdtToken.address),
      ).to.equal("0");

      //   let tx2 = await arbRouterAsgard1.transferAllowance(ROUTER3.address, ASGARD3, usdt.address, _50k, 'MIGRATE:1', { from: asgard1 });
      //   const receipt2 = await tx2.wait()

      //   expect(receipt2.events?.[0]?.event).to.equal('Deposit');
      //   expect(receipt2.events?.[0]?.args?.to).to.equal(asgard3);
      //   expect(receipt2.events?.[0]?.args?.asset).to.equal(usdt.address);
      //   expect(receipt2.events?.[0]?.args?.memo).to.equal('MIGRATE:1');
      //   expect(receipt2.events?.[0]?.args?.amount).to.equal(_50k);

      // make sure the token had been transfer to ASGARD3 and Router3
      //   expect((await usdt.balanceOf(ROUTER3.address))).to.equal(_100k);
      //   expect((await arbRouter2.vaultAllowance(asgard3, usdt.address))).to.equal(_100k); // router3
      //   expect((await arbRouter.vaultAllowance(asgard1, usdt.address))).to.equal('0');

      const asgard3StartBalance = BigNumber.from(
        await ethers.provider.getBalance(asgard3),
      );
      // this ignore the gas cost on ASGARD1
      // transfer out ARB.ETH
      const tx3 = await arbRouterAsgard1.transferOut(
        asgard3,
        ETH,
        "0",
        "MIGRATE:1",
        { value: amount2k },
      );
      const receipt3 = await tx3.wait();

      expect(receipt3.events?.[0]?.event).to.equal("TransferOut");
      expect(receipt3.events?.[0]?.args?.vault).to.equal(asgard1);
      expect(receipt3.events?.[0]?.args?.to).to.equal(asgard3);
      expect(receipt3.events?.[0]?.args?.asset).to.equal(ETH);
      expect(receipt3.events?.[0]?.args?.memo).to.equal("MIGRATE:1");

      const asgard3EndBalance = BigNumber.from(
        await ethers.provider.getBalance(asgard3),
      );
      expect(asgard3EndBalance.sub(asgard3StartBalance)).to.equal(amount2k);
    });
  });
});
