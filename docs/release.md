<!-- markdownlint-disable MD024 -->

# Versioning

MAYANode is following semantic version. MAJOR.MINOR.PATCH(0.77.1)

The MAJOR version currently is updated per soft-fork.

Minor version need to update when the network introduce some none backward compatible changes.

Patch version, is backward compatible, usually changes only in bifrost

## Prepare for release ?

1. Create a milestone using the release version
2. Tag issues & PRs using the milestone, so we can identify which PR is on which version
3. PRs need to get approved by @maya-ah-kin, once get approved, merge to `develop` branch
4. Once all PRs for a version have been merged, create a release branch from `develop` such as: `release-1.92.0`. This allows future PRs to still land into `develop` while the release is happening.

## Test release candidate

1. Create a 5 nodes mock net, follow [private_mock_chain.md](private_mock_chain.md)
2. Build image from `develop` branch, sanity check the following features work

- [ ] Genesis node start up successfully
- [ ] Bifrost startup correctly, and start to observe all chains
- [ ] Create pools for RUNE/BTC/ETH/DASH/USDT/USDC/KUJI
- [ ] Add liquidity to RUNE/BTC/ETH/DASH/USDT/USDC/KUJI pools
- [ ] Bond new validator
- [ ] Set version
- [ ] Set node keys
- [ ] Set IP Address
- [ ] Churn successful, cluster grow from 1 genesis node to 4 nodes
- [ ] Fund migration successfully
- [ ] Some swaps, CACAO -> BTC, BTC -> DASH etc.
- [ ] Mocknet grow from four nodes -> five nodes, which include keygen, migration
- [ ] Node can leave

3. Also identify unexpected log / behaviour, and investigate it.

## Create stagenet & mainnet image

### stagenet

1. Raise a PR to merge all changes from `develop` branch -> `stagenet` branch, once the PR get approved & merged stagenet image should be created automatically by pipeline
   for example: https://gitlab.com/mayachain/mayanode/-/pipelines/433627075
2. Make sure "build-mayanode" pipeline is successful, you should be able to see the docker image has been build and tagged successfully

```logs
Successfully built a15350678d3e
Successfully tagged registry.gitlab.com/mayachain/mayanode:testnet
Successfully tagged registry.gitlab.com/mayachain/mayanode:testnet-0
Successfully tagged registry.gitlab.com/mayachain/mayanode:testnet-0.77
Successfully tagged registry.gitlab.com/mayachain/mayanode:testnet-0.77.2
Successfully tagged registry.gitlab.com/mayachain/mayanode:8be434a
```

### mainnet

1. Raise a PR to merge all changes from `develop` branch -> `mainnet` branch, once the PR get approved & merged, chaosnet image should be created automatically by pipeline
   for example: https://gitlab.com/mayachain/mayanode/-/pipelines/433627314
2. Make sure "build-mayanode" pipeline step is successful, when you click the step, inside you should be able to see

```logs
Successfully built fdfd001f96ba
Successfully tagged registry.gitlab.com/mayachain/mayanode:chaosnet-multichain
Successfully tagged registry.gitlab.com/mayachain/mayanode:chaosnet-multichain-0
Successfully tagged registry.gitlab.com/mayachain/mayanode:chaosnet-multichain-0.77
Successfully tagged registry.gitlab.com/mayachain/mayanode:chaosnet-multichain-0.77.2
Successfully tagged registry.gitlab.com/mayachain/mayanode:d24c9bd
```

## Release to stagenet

node-launcher repository: https://gitlab.com/mayachain/devops/node-launcher/

### Raise PR in node-launcher

1. Raise PR to release version to stagenet, usually just change mayanode-stack/stagenet.yaml file, to use the new image tag
   for example: https://gitlab.com/mayachain/devops/node-launcher/-/merge_requests/390
2. Post the PR to #community-devs channel, and tag @maya-ah-kin team to approve, it will need at least 2 approvals

### Sync a node from genesis to tip

1. Start a new validator on stagenet, using the new image tag, let it sync from genesis to the tip, make sure mayanode pod doesn't get consensus failure. This will ensure a new node can always join by syncing from genesis.

## Release to mainnet

How to release to mainnet.

### Raise PR in node-launcher

1. Raise PR to release version to stagenet, usually just change mayanode-stack/stagenet.yaml file, to use the new image tag
   for example: https://gitlab.com/mayachain/devops/node-launcher/-/merge_requests/390
2. Post the PR to #devops channel, and tag @maya-ah-kin team to approve, it will need at least 4 approvals

### Sync a full node from genesis

1. Start a new validator on mainnet, using the new image tag, let it sync from genesis to the tip, make sure mayanode pod doesn't get consensus failure.
2. Sync takes a few days right now, If for some reason, sync does get into consensus failure, then the image can't be released, need to investigate and find out what cause the issue, and fix it

## Pre-release check

1. Quickly go through all the PRs in the release.
2. Apply the latest changes to a standby node and monitor the following:
   1. MAYANode pod didn't get into `CrashloopBackoff`
   2. Version has been set correctly
   3. Bifrost started correctly
   4. Identify which daemons will be restarted during this update, include those in the announcement

## Release

1. Post release announcement in #mayanode-mainnet
2. Create a tag from `release-X.Y.Z` branch. for example: https://gitlab.com/mayachain/mayanode/-/tags/v0.76.0
3. Create a gitlab release from that tag: https://gitlab.com/mayachain/mayanode/-/releases
