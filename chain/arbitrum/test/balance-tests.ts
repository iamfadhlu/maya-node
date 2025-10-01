import { ethers, getNamedAccounts, network, deployments } from "hardhat";
import { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers";
import { expect } from "chai";
import { Contract, Signer } from "ethers";
import { sushiSwapRouterAbi } from "./abis/sushiSwapRouterAbi";
import { USDT_ADDRESS, WETH_ADDRESS } from "./constants";
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
    accounts = await ethers.getSigners();
    await deployments.fixture();

    sushiSwap = new ethers.Contract(
      sushiSwapRouter,
      sushiSwapRouterAbi,
      accounts[0],
    );
    usdtToken = new ethers.Contract(USDT_ADDRESS, ERC20.abi, accounts[0]);
  });

  describe("Check Balances", function () {
    it("Balance of USDT", async () => {
      const { admin, asgard1 } = await getNamedAccounts();

      const usdtContract = await ethers.getContractAt("IERC20", USDT_ADDRESS);
      const balanceOfUsdt = await usdtContract.balanceOf(admin);
      console.log("usdtbal", balanceOfUsdt);

      let arbBal = await ethers.provider.getBalance(admin);
      console.log("adminarbbal", arbBal);

      arbBal = await ethers.provider.getBalance(asgard1);
      console.log("routerarbbal", arbBal);
    });
  });
});
