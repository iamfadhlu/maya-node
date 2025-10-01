// SPDX-License-Identifier: AGPL-3.0-or-later
pragma solidity 0.8.9;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "../interfaces/ISwapRouter.sol";

interface IRouter {
  function depositWithExpiry(
    address payable vault,
    address asset,
    uint256 amount,
    string memory memo,
    uint256 expiration
  ) external payable;
}

// MAYAChain_Aggregator is permissionless
contract ArbAggregator {
  using SafeERC20 for IERC20;

  uint256 private constant _NOT_ENTERED = 1;
  uint256 private constant _ENTERED = 2;
  uint256 private _status;

  address private ETH = address(0);
  address public WETH;
  ISwapRouter public swapRouter;

  modifier nonReentrant() {
    require(_status != _ENTERED, "ReentrancyGuard: reentrant call");
    _status = _ENTERED;
    _;
    _status = _NOT_ENTERED;
  }

  constructor(address _weth, address _swapRouter) {
    _status = _NOT_ENTERED;
    WETH = _weth;
    swapRouter = ISwapRouter(_swapRouter);
  }

  receive() external payable {}

  /**
   * @notice Calls deposit with an expiration
   * @param mcVault address - MAYAchain vault address
   * @param mcRouter address - MAYAchain vault address
   * @param tcMemo string - MAYAchain memo
   * @param amount uint - amount to swap
   * @param amountOutMin uint - minimum amount to receive
   * @param deadline string - timestamp for expiration
   */
  function swapIn(
    address mcVault,
    address mcRouter,
    string calldata tcMemo,
    address token,
    uint256 amount,
    uint256 amountOutMin,
    uint256 deadline
  ) public nonReentrant {
    uint256 startBal = IERC20(token).balanceOf(address(this));

    IERC20(token).safeTransferFrom(msg.sender, address(this), amount); // Transfer asset
    IERC20(token).safeApprove(address(swapRouter), amount);
    uint256 safeAmount = (IERC20(token).balanceOf(address(this)) - startBal);

    address[] memory path = new address[](2);
    path[0] = token;
    path[1] = WETH;

    ISwapRouter(swapRouter).swapExactTokensForETH(
      safeAmount,
      amountOutMin,
      path,
      address(this),
      deadline
    );

    safeAmount = address(this).balance;
    IRouter(mcRouter).depositWithExpiry{value: safeAmount}(
      payable(mcVault),
      ETH,
      safeAmount,
      tcMemo,
      deadline
    );
  }

  /**
   * @notice Calls deposit with an expiration
   * @param token address - ERC20 asset or zero address for ETH
   * @param to address - address to receive swap
   * @param amountOutMin uint - minimum amount to receive
   */
  function swapOut(
    address token,
    address to,
    uint256 amountOutMin
  ) public payable nonReentrant {
    address[] memory path = new address[](2);
    path[0] = WETH;
    path[1] = token;
    ISwapRouter(swapRouter).swapExactETHForTokens{value: msg.value}(
      amountOutMin,
      path,
      to,
      type(uint256).max
    );
  }
}
