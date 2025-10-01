---
title: Maya Protocol / Zcash
weight: 500
hidden: true
---

This document serves both as an introduction to the integration
project of Zcash/ZEC into Maya Protocol (MP) and a technical feasibility
study. In the first, we describe the architecture of MP and the workflows
for deposit/withdraw and swap.

Next, we'll discuss how to add ZEC as a new chain which will allow
the creation of the liquidity pool ZEC.

We aim to have transparent & shielded ZEC available both as a
source and a destination of a swap.

We'll use Bitcoin as a starting point since ZEC data model and
implementation are closely related (
at least for the transparent addresses).

We'll also discuss how shielded ZEC can be added.

The Zcash implementation is now part of the main repository.

## Overview of Maya Protocol

### DEX

MP is a Decentralized Exchange. The main use case is to
exchange one crypto currency for another. There are multiple
cryptos supported, among which we have Bitcoin, Ethereum, etc.

The goal of the project is to add Zcash, i.e. ZEC, so that
any user can exchange (swap) ZEC for any currency
that MP supports (or vice-versa).

Traditionally, one would have to use the services of an
exchange like Binance. But these exchanges require KYC
and hold your funds. They are
called centralized exchanges (CEX).

### CEX

Users of a CEX place buy or sell orders on the platform.
The platform continuously attempts to match buy and sell
orders on their price. When it happens, a trade is made
and currency is exchanged between the participants.

### Liquidity Pools

A DEX such as MP does not work with orders. Instead
they use a mechanism based on liquidity pools.

Every trading pool, for example ZEC, is associated
with two assets. Let's say ZEC and BTC.
This allows a user to swap ZEC for BTC.

{{% notice note %}}
The assets are not merged between pairs. BTC in
the ZEC pool is not the same as BTC in the ETH pool.
{{% /notice %}}

Liquidity providers can contribute funds into any of the pools
by sending to the vault. To indicate their intent, a memo
must be attached to the transaction.

In the smoke tests, this action is shown as

```bash
PROVIDER-1 => VAULT ADD:ZEC.ZEC:PROVIDER-1 2.50000000 ZEC.ZEC
```

This adds 2.5 ZEC from the user "PROVIDER-1" into the pool ZEC.

```bash
PROVIDER-1 => VAULT ADD:ZEC.ZEC:PROVIDER-1 1,000.00000000 MAYA.CACAO
```

This add 1000 CACAO to the ZEC pool.

{{% notice note %}}
The pool is named as `<chain>.<asset>`. For Zcash that
has a single asset, we use `ZEC.ZEC` for simplicity.
It could be `ZCASH.ZEC` too.
{{% /notice %}}

### Swaps

To make a swap, a user sends one type of currency
to the vault and receives the other type of currency
in exchange. The system determines dynamically the exchange
rate based on the amount (= depth) in each of currency
of the swap for the given pool.

We denote a swap as

```bash
USER-1 => VAULT SWAP:MAYA.CACAO:USER-1 1.00000000 ZEC.ZEC
```

User 1 sent 1 ZEC to the MP vault and wants to receive
CACAO in return.

### Memos

As you can see, in addition to the sender/receiver/amount
of the transaction, the memo plays an important role
in specifying the purpose.

{{% notice note %}}
In a real memo, the aliases such as "USER-1" is replaced
by an address.
{{% /notice %}}

## Components

Now let's move to the components of MP that makes this
possible in a _decentralized_ and _permission-less_ way.

Like every decentralized crypto project, there are
many nodes, i.e. computers that run the same set of software.
The nodes maintain a common state that varies between project
but is always eventually persisted in a blockchain.

For Bitcoin, the state is the amount of BTC associated with
addresses (or output scripts). Users make transactions
that (if correct) modify the state by moving funds from one
address to another.

In MP, the common state is the amount in the pools
which should track the funds deposited and swapped.
(And also information to track the liquidity providers)

A transaction would add/remove liquidity, or swap assets.
Consensus is reached when enough (67%) of the validating
nodes agree on the transaction.

### Maya blockchain

The MP blockchain is the persistent ledger of every state
change in the MP pools. For example, if someone swaps
by doing

```bash
USER-1 => VAULT SWAP:MAYA.CACAO:USER-1 1.00000000 ZEC.ZEC
```

It will be written in the MP blockchain. Therefore swaps
are not _encrypted_.

{{% notice info %}}
Currently, only transparent Zcash addresses are fully supported. While the codebase includes some initial support for shielded addresses (sapling and orchard), additional work is required to fully enable shielded transactions in production.
{{% /notice %}}

The MP blockchain uses the Tendermint blockchain engine.

{{% notice info %}}
Tendermint employs a Byzantine Fault Tolerance algorithm, which allows
the network to reach consensus even if some of the nodes (less than 33%)
act maliciously or are faulty.
{{% /notice %}}

### Asguard Vault

The vault is where the pools keep the funds. Vaults
use threshold signatures for spending and therefore
their public address is a combination of the signing
entities. They rotate frequently. The system will
automatically migrate funds from one vault to the next.

It is important that users sent funds to the current
vault or their funds will be lost.

A vault is an address. It is not a process or a node.
There is a single public key that is then encoded
in various chains.

In other words, the vault address for Bitcoin and
Zcash have the same public key and public key hash.

{{% notice note %}}
Currently, the currencies use the same elliptic curve
(secp256k1) and the same signing algorithm (ECDSA).

This is a requirement that Zcash fulfills for transparent addresses.
{{% /notice %}}

### MayaNode

Mayanode is the client to the MP blockchain. It is where
the logic for protocol is implemented. Each node runs
the mayanode software. It exposes a REST interface
and is a client to the MP blockchain.

Mayanode is chain agnostic and operates on the grounds
that all currencies work essentially the same way, i.e.
when you exchange 1 ZEC for BTC, it does not matter
what ZEC and BTC are. The only important factors
are the pool depth (and other pool state).

Therefore in the Mayanode code, there is no chain
specific code. There is just the list of chain codes
that the MP supports

#### Adding ZEC

{{% notice note %}}
ZEC must be added to the list of supported chains
in `support_chains_xxxnet_vxxx.go`)
{{% /notice %}}

### Bifrost

Bifrost is a program that runs along with the mayanode
and interfaces with the crypto currency blockchains
like BTC, ETH, etc.

It has chain client code that communicates with the
chain nodes.

For example, in the case of Zcash, we would have

{{< mermaid >}}
sequenceDiagram
zcashd ->> zcash_client: observe zcash network
zcash_client ->> bifrost: wallet, tx parsing
bifrost ->> mayanode: pool event
{{< /mermaid >}}

{{% notice info %}}
The `zcash_client` is a program that bridges
the fullnode zcash client `zcashd` with bifrost.
{{% /notice %}}

There are several functionalities that were
not included in `zcashd` that `bitcoind` has.
They are going to be emulated by the `zcash_client`.

Moreover, the RPC that the bifrost chain client
uses are bitcoin specific and don't work with
shielded addresses.

{{% notice note %}}
There is a single instance of the bifrost
serving all the chains. It is written
in Go.
{{% /notice %}}

## Adding ZEC II

The chainclient code is in `pkg/chainclients`

- `loadchains`
- `client`
- `key_sign_wrapper`
- `signer`
- `tss_signable`

```go
type ChainClient interface {
	SignTx(tx stypes.TxOutItem, height int64) ([]byte, []byte, error)
	BroadcastTx(_ stypes.TxOutItem, _ []byte) (string, error)
	GetHeight() (int64, error)
	GetAddress(poolPubKey common.PubKey) string
	GetAccount(poolPubKey common.PubKey, height *big.Int) (common.Account, error)
	GetAccountByAddress(address string, height *big.Int) (common.Account, error)
	GetChain() common.Chain
	OnObservedTxIn(txIn types.TxInItem, blockHeight int64)
	Start(globalTxsQueue chan stypes.TxIn, globalErrataQueue chan stypes.ErrataBlock, globalSolvencyQueue chan stypes.Solvency)
	GetConfig() config.BifrostChainConfiguration
	GetConfirmationCount(txIn stypes.TxIn) int64
	ConfirmationCountReady(txIn stypes.TxIn) bool
	IsBlockScannerHealthy() bool
	Stop()
}
```

and the BlockScanner interface

```go
// BlockScannerFetcher define the methods a block scanner need to implement
type BlockScannerFetcher interface {
	// FetchMemPool scan the mempool
	FetchMemPool(height int64) (types.TxIn, error)
	// FetchTxs scan block with the given height
	FetchTxs(height int64) (types.TxIn, error)
	// GetHeight return current block height
	GetHeight() (int64, error)
}
```

### Zcash Client Implementation

The Zcash client implementation follows the Bitcoin pattern but with key differences to accommodate Zcash's unique features. The implementation is structured as:

- **ZcashClient**: Implements the UTXOClient interface for interacting with zcashd
- **Rust Library**: Core Zcash functions are implemented in Rust and exposed to Go via UniFFI bindings

The client handles these primary responsibilities:

1. **Block and Transaction Observation**: Similar to Bitcoin (GetHeight, FetchTxs, etc.)
2. **Address Management**: Handling of Zcash transparent addresses
3. **Transaction Signing**: Working with Maya Protocol's TSS system

Signing transactions is more complex than normal because MP uses threshold signatures (TSS), as discussed later.

### Current Implementation: Transparent ZEC

The current implementation fully supports transparent Zcash transactions. Transparent ZEC operations function similarly to Bitcoin with the following key differences:

- Zcash uses legacy transparent addresses (Base58 format) rather than segwit addresses
- Transaction fee calculation follows ZIP-317 specifications rather than Bitcoin's fee rate model
- Transaction signature hashing uses Zcash-specific algorithms defined in ZIP-244

The Zcash client in Maya Protocol handles these differences transparently, allowing users to swap ZEC with other supported cryptocurrencies through transparent addresses without complications.

### Future Work: Shielded ZEC

While the codebase contains initial infrastructure for shielded transactions, full support for shielded addresses (Sapling and Orchard) requires additional development before being enabled in production. The main challenges with shielded transactions include:

- **Anonymous Senders**: Shielded transactions hide sender information, making error recovery more difficult. For instance, funds sent to the vault without a memo would be unrecoverable.

{{% notice note %}}
This cannot be prevented at the Zcash protocol level.
Anyone can send funds to any address as long as the
transaction is valid. It just happens that in this case
the funds would end up in the MP vault. However
wallets are expected to follow the MP protocol and include
a return address in the memo or always use transparent
funds.
{{% /notice %}}

- **Encrypted Outputs**: Shielded transactions encrypt output information, requiring observer nodes to decrypt outputs to verify transaction completion.

The current implementation includes the groundwork for these features, including:

- Output Viewing Key (OVK) generation from vault public keys
- Function shells for building and verifying shielded components
- Transaction structure handling for both transparent and shielded components

{{% notice note %}}
The OVK is derived using BLAKE-2b hash of the vault pubkey with the personalization string "Zcash_Maya_OVK\_\_".
{{% /notice %}}

### TSS

As stated before, signing transactions from the vault
require the participation of multiple signers. They
are clients to the MP blockchain and only sign transactions
that have verified.

They have a share of the secret key and if 2/3 of them
provide a valid signature, the signatures can be combined
to form a valid transaction. At no point can anyone
reconstruct the vault secret key.

The TSS algorithm is implemented in the `tss-lib`, so
we don't need to change that since Zcash also uses
ECDSA on the same EC.

However, we have to provide the SIGHASH, i.e.
the message to be signed.

Zcash transactions use a different algorithm
than Bitcoin for calculating the sighash.

The current MP code (in Bifrost), the transaction
is formed by selecting inputs from UTXOs and outputs
from the swap definition. Then the library BTCD
builds the unsigned transaction and derives the sighash.

That is to say, the logic that resides in `bitcoind`
to build transactions from the wallet cannot be used
because `bitcoind` does not support threshold signatures.
It only supports multisigs.

As a result, this is performed by the chain client
in `signer.go`

With Zcash, the algorithm for SIGHASH is completely
different than Bitcoin.

It is documented in
[ZIP-225](https://zips.z.cash/zip-0225)
and [ZIP-244](https://zips.z.cash/zip-0244)

Therefore, the transaction definition will be sent to
the `zcash-client` which will return the sighash
and the transaction parts.

It also responsible for the generation of the zero
knowledge proofs.

### BTCD

BTCSuite is the go library used by btcd,
btcd is an alternative full node bitcoin implementation written in Go.

{{% notice note %}}
There are no golang library for the low level
protocol components of zcash. The code is written
in Rust only.
{{% /notice %}}

This is another reason for the `zcash-client` bridge.

## Smoke Tests

The smoke tests use python versions of the chain client
and a simulator for the mayachain.

We need to add `zcash.py` to the chain clients,
define the account aliases, add Zcash to the
smoke tests script and to the test transactions.

## Docker & Deployment

`zcashd` has a fairly high deprecation rate. Every
6 months, node operators will have to deploy a new
version. The docker image can be found on a hub.
We need to include it in the docker compose file
and create a zcash service that also has the
`zcash-client`.

This is similar to the deployment of other chains
such as LTC, DOGE, etc.

The difference is the addition of the `zcash-client`
but that could be made part of the `zcash` docker image.
