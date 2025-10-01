# MAYANode Docker

## Fullnode

The default image will start a fullnode:

```bash
docker run \
  -e CHAIN_ID=mayachain-mainnet-v1 \
  -e NET=mainnet \
  registry.gitlab.com/mayachain/mayanode:chaosnet-multichain
```

Since this image tag contains the latest version of MAYANode, the node can auto update by simply placing this in a loop to re-pull the image on exit:

```bash
while true; do
  docker pull registry.gitlab.com/mayachain/mayanode:chaosnet-multichain
  docker run -e NET=mainnet registry.gitlab.com/mayachain/mayanode:chaosnet-multichain
do
```

The above commands also apply to `testnet` and `stagenet` by simply using the respective image (in these cases `-e NET=...` is not required):

```code
testnet  => registry.gitlab.com/mayachain/mayanode:testnet
stagenet => registry.gitlab.com/mayachain/mayanode:stagenet
```

## Validator

Officially supported deployments of MAYANode validators require a working understanding of Kubernetes and related infrastructure. See the [Cluster Launcher](https://gitlab.com/mayachain/devops/cluster-launcher) repo for cluster Terraform resources, and the [Node Launcher](https://gitlab.com/mayachain/devops/node-launcher) repo for deployment utilities which internally leveraging Helm.

## Mocknet

The development environment leverages Docker Compose V2 to create a mock network - this is included in the latest version of Docker Desktop for Mac and Windows, and can be added as a plugin on Linux by following the instructions [here](https://docs.docker.com/compose/cli-command/#installing-compose-v2).

The mocknet configuration is vanilla, leveraging Docker Compose profiles which can be combined at user discretion. The following profiles exist:

```code
mayanode => mayanode only
bifrost  => bifrost and mayanode dependency
midgard  => midgard and mayanode dependency
mocknet  => all mocknet dependencies
```

### Keys

We leverage the following keys for testing and local mocknet setup, created with a simplified mnemonic for ease of reference. We refer to these keys by the name of the animal used:

```text
cat => cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat cat crawl
dog => dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog fossil
fox => fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox fox filter
pig => pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig pig quick
```

### Examples

Example commands are provided below for those less familiar with Docker Compose features:

```bash
# start a mocknet with all dependencies
docker compose --profile mocknet up -d

# multiple profiles are supported, start a mocknet and midgard
docker compose --profile mocknet --profile midgard up -d

# check running services
docker compose ps

# tail the logs of all services
docker compose logs -f

# tail the logs of only mayanode and bifrost
docker compose logs -f mayanode bifrost

# enter a shell in the mayanode container
docker compose exec mayanode sh

# copy a file from the mayanode container
docker compose cp mayanode:/root/.mayanode/config/genesis.json .

# rebuild all buildable services (mayanode and bifrost)
docker compose build

# export mayanode genesis
docker compose stop mayanode
docker compose run mayanode -- mayanode export
docker compose start mayanode

# hard fork mayanode
docker compose stop mayanode
docker compose run /docker/scripts/hard-fork.sh

# stop mocknet services
docker compose --profile mocknet down

# clear mocknet docker volumes
docker compose --profile mocknet down -v
```

## Multi-Node Mocknet

The Docker Compose configuration has been extended to support a multi-node local network. Starting the multinode network requires the `mocknet-cluster` profile:

```bash
docker compose --profile mocknet-cluster up -d
```

Once the mocknet is running, you can open open a shell in the `cli` service to access CLIs for interacting with the mocknet:

```bash
docker compose run cli

# increase default 60 block churn (keyring password is "password")
mayanode tx mayachain mimir CHURNINTERVAL 1000 --from dog $TX_FLAGS

# set limit to 1 new node per churn (keyring password is "password")
mayanode tx mayachain mimir NUMBEROFNEWNODESPERCHURN 1 --from dog $TX_FLAGS
```

## Local Mainnet Fork of EVM Chain

Using hardhat, you can run a mainnet fork locally of any EVM chain and use it the mocknet stack. This allows you to interact with all of the DEXes, smart contracts
and Liquidity Pools deployed mainnet in your local mocknet environment. This simplifies testing EVM chain clients, routers, and aggregators.

This guide will go over how to fork AVAX C-Chain locally, and use it in the mocknet stack.

1. Spin up the local mocknet fork from your hardhat repo: (e.g. https://gitlab.com/mayachain/chains/avalanche)
2.

```bash
npx hardhat node --fork https://api.avax.network/ext/bc/C/rpc
```

2. Deploy any Router/Aggregator Contracts to your local mocknet fork using hardhat
3.
4. Point Bifröst at your local EVM node, and be sure to pass in a starting block height close to the tip, otherwise Bifröst will scan every block from 0:

```bash
AVAX_HOST=http://host.docker.internal:8545/ext/bc/C/rpc AVAX_START_BLOCK_HEIGHT=16467608 make reset-mocknet
```

## Bootstrap Mocknet Data

You can leverage the smoke tests to bootstrap local vaults with a subset of test data. Run:

```bash
make bootstrap-mocknet
```
