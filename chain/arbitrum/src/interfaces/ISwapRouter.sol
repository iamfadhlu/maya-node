// SPDX-License-Identifier: AGPL-3.0-or-later
pragma solidity 0.8.9;

// SushiSwap Interface
interface ISwapRouter {
  function swapExactTokensForETH(
    uint256 amountIn,
    uint256 amountOutMin,
    address[] calldata path,
    address to,
    uint256 deadline
  ) external;

  function swapExactETHForTokens(
    uint256 amountOutMin,
    address[] calldata path,
    address to,
    uint256 deadline
  ) external payable;
}
