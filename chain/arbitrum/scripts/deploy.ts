import hre, { ethers } from "hardhat";

async function main() {
  const wethAddress = "0x82af49447d8a07e3bd95bd0d56f35241523fbab1";
  const sushiSwapRouter = "0x1b02dA8Cb0d097eB8D57A175b88c7D8b47997506";

  const ArbRouter = await ethers.getContractFactory("ArbRouter");
  const arbRouter = await ArbRouter.deploy();
  await arbRouter.deployed();

  console.log("ArbRouter deployed to:", arbRouter.address);
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
