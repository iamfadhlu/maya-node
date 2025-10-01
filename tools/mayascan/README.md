# mayascan

mayascan provides:

1. Python `mayascan` library and CLI for data wrangling of blocks, events, transactions, and messages.
2. Golang package with a `Scan` function to easily scan blocks from a Golang channel.

## Python

### Installation

Simply run the following from this directory:

```bash
pip3 install .
```

### Docker

You can alternatively leverage Docker for running this utility in a pre-built container:

```bash
alias mayascan="docker run -it --rm registry.gitlab.com/mayachain/mayanode:mayascan"
```

### Examples

You can use one liners in the CLI:

```bash
# all swap events
mayascan 'events(lambda b,tx,e: e, types={"swap"}), start=-1'

# gas used
mayascan 'transactions(lambda b,tx: (tx["hash"],tx["result"]["gas_used"])), start=-1'

# failed transactions
mayascan 'transactions(lambda b,tx: (tx["hash"],tx["result"]["code"]), failed=True), start=-1'

# slash and leave events
mayascan 'events(lambda b,tx,e: e, types={"slash", "validator_request_leave"}), start=-1'

# bond slash events
mayascan 'events(lambda b,tx,e: e if e["bond_type"] == "\u0003" else None, types={"bond"}), start=-1'

# observed outbounds
mayascan 'messages(lambda b,tx,m: tx, types={"MsgObservedTxOut"}), start=-1'
```

Alternatively import the library to create more complex listener functions:

```python
# count outbound observations by chain
import collections, json, mayascan

counts = collections.defaultdict(lambda: 0)

def listen(height, tx, msg):
    global counts
    for tx in msg["txs"]:
        counts[tx["tx"]["chain"]] += 1

mayascan.scan(mayascan.messages(listen, types={"MsgObservedTxOut"}), start=-100, stop=-1)

print(json.dumps(counts, indent=2))
```

## Golang

```golang
package main

import (
	"gitlab.com/mayachain/mayanode/tools/mayascan"
)

func main() {
	for block := range mayascan.Scan(-200, -100) {
		println(block.Header.Height, "has", len(block.Txs), "txs")
	}
}
```

## Advanced

Override the following default config values with the Golang or Python packages via the corresponding environment variables:

```text
API_ENDPOINT = https://mayanode.mayachain.info
RPC_ENDPOINT = https://tendermint.mayachain.info
PARALLELISM  = 4
```
