# go-blockchain Quickstart

## Build

```bash
cd ~/Code/core/go-blockchain
GOWORK=off go build -o core-chain ./cmd/core-chain/
```

## Sync the testnet (3 minutes)

```bash
./core-chain chain serve --testnet --seed 10.69.69.165:46942 --rpc-port 47941
```

## Query the chain

```bash
# JSON-RPC
curl -X POST http://127.0.0.1:47941/json_rpc \
  -d '{"jsonrpc":"2.0","id":"0","method":"getinfo"}'

# REST API  
curl http://127.0.0.1:47941/api/info
curl http://127.0.0.1:47941/api/aliases
curl http://127.0.0.1:47941/health

# Self-documentation
curl http://127.0.0.1:47941/openapi
```

## Create a wallet

```bash
./core-chain wallet create
# Address: iTHN...
# Seed: 25 words

./core-chain wallet restore --seed "word1 word2 ..."
```

## Use as Core service

```go
c := core.New()
blockchain.RegisterAllActions(c, chain, hsdURL, hsdKey)

// Now available as CLI + MCP + API:
result := c.Action("blockchain.chain.height").Run(ctx, opts)
result := c.Action("blockchain.wallet.create").Run(ctx, opts)
result := c.Action("blockchain.dns.resolve").Run(ctx, opts)
```

## 36 Core Actions

| Group | Actions |
|-------|---------|
| chain | height, info, block, synced, hardforks, stats, search |
| alias | list, get, capabilities |
| network | gateways, topology, vpn, dns |
| supply | total, hashrate |
| wallet | create, address, seed |
| crypto | hash, generate_keys, check_key, validate_address |
| asset | info, list, deploy |
| forge | release, issue, build, event |
| hsd | info, resolve, height |
| dns | resolve, names, discover |
