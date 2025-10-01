# EVM Whitelist Procedure

## Overview

Ecosystem devs can ask for tokens/contracts to be added/removed from any MAYANode Whitelists using this procedure.

## Background

MAYANode maintains whitelists to prevent attacks on the network. There are a significant number of degrees of freedom when dealing with the EVM (event spoofing, re-entrancies, self-destructs), as well as economic attacks (zombie tokens, infinite mints etc). Maintaining a standard and whitelist nueters this attack surface.

There are 3 EVM Whitelists

1. Pool Token Whitelist - allows to be a pool on MAYAChain
2. DEX Token Whitelist - allows to be swapped to using DEX Aggregation
3. Aggregator Whitelist - allows to be an Aggregator to call into, or be called from, the router

## Procedure

Once a review cycle, the publisher will ask for new additions to be submitted for review and inclusion. There will be a 48hr cutoff. The publisher will follow the following checklist and will not include the token/contract if it does not meet the requirements.

"must" - unavoidable requirement
"should" - loose requirement

### Pool Token

- Must be ERC-20 compliant https://ethereum.org/en/developers/docs/standards/tokens/erc-20/
- Must not include token transfer fees (taxes)
- Must not be mintable
- Must be verified on Etherscan (or equivalent, eg Arbiscan for ARB)
- Should be economically valuable (greater than $100m mcap)
- Should be older than 4 years
- Should have a sponsor willing to provide $1m in bootstrap liquidity

### Dex Token

- Must be listed on an on-chain AMM
- Must be ERC-20 compliant https://ethereum.org/en/developers/docs/standards/tokens/erc-20/
- Must be verified on Etherscan

### Aggregator

Examples: https://gitlab.com/mayachain/mayanode/-/blob/develop/x/mayachain/aggregators/dex_mainnet.go

- Must be verified on Etherscan (or equivalent, eg Arbiscan for ARB)
- If support `swapIn(params)`, must call `router.depositWithExpiry(params, +15minsUNIXSeconds)`,
- If support `swapOut(params)`, must have a function exactly `swapOut(address,address,uint256)`
- Must have re-entrancy protection on all functions https://github.com/OpenZeppelin/openzeppelin-contracts/blob/master/contracts/security/ReentrancyGuard.sol
- Must not be proxied (https://docs.openzeppelin.com/upgrades-plugins/1.x/proxies)
