---
title: RPC Reference
description: Daemon and wallet JSON-RPC 2.0 API documentation.
---

# RPC Reference

The Lethean node exposes two RPC interfaces: a **daemon** API for blockchain queries and node control, and a **wallet** API for account management and transactions. Both use JSON-RPC 2.0 over HTTP.

- **Daemon RPC:** Port 36941 (mainnet), 46941 (testnet)
- **Wallet RPC:** User-configured port alongside the wallet process

## Go Client

The `rpc/` package provides a typed Go client:

```go
import "forge.lthn.ai/core/go-blockchain/rpc"

// Create a client (appends /json_rpc automatically)
client := rpc.NewClient("http://localhost:36941")

// Or with a custom HTTP client
client := rpc.NewClientWithHTTP("http://localhost:36941", &http.Client{
    Timeout: 60 * time.Second,
})
```

The client supports two transport modes:
- `call()` -- JSON-RPC 2.0 via `/json_rpc` (most endpoints)
- `legacyCall()` -- Plain JSON POST to dedicated URI paths (some endpoints registered via `MAP_URI_AUTO_JON2` in the C++ daemon)

All daemon responses include a `"status": "OK"` field. The client checks this and returns an error for non-OK values.

## Implemented Go Methods

### Node Information

```go
// Comprehensive daemon status
info, err := client.GetInfo()
// Returns: Height, TxCount, PowDifficulty, OutgoingConnectionsCount,
//          IncomingConnectionsCount, BlockReward, TotalCoins, ...

// Current blockchain height (legacy endpoint)
height, err := client.GetHeight()

// Block count
count, err := client.GetBlockCount()
```

### Block Queries

```go
// Most recent block header
header, err := client.GetLastBlockHeader()
// Returns: Height, Hash, Timestamp, MajorVersion, Difficulty, Reward, ...

// Block header by height
header, err := client.GetBlockHeaderByHeight(1000)

// Block header by hash
header, err := client.GetBlockHeaderByHash("ab3f...")

// Full block details (range)
blocks, err := client.GetBlocksDetails(0, 10)
// Returns: []BlockDetails with transactions, cumulative difficulty, etc.
```

### Transaction Queries

```go
// Detailed transaction info
txInfo, err := client.GetTxDetails("543b...")

// Fetch raw transactions by hash (legacy endpoint)
txsHex, missed, err := client.GetTransactions([]string{"ab3f...", "cd5e..."})
```

### Mining

```go
// Submit a mined block
err := client.SubmitBlock("hexblob...")
```

### Wallet Support

```go
// Fetch random decoy outputs for ring construction
outs, err := client.GetRandomOutputs(amount, 15)  // 15 decoys for HF4+

// Broadcast a signed transaction
err := client.SendRawTransaction(txBlob)
```

## Go Response Types

```go
type BlockHeader struct {
    MajorVersion uint8  `json:"major_version"`
    MinorVersion uint8  `json:"minor_version"`
    Timestamp    uint64 `json:"timestamp"`
    PrevHash     string `json:"prev_hash"`
    Nonce        uint64 `json:"nonce"`
    OrphanStatus bool   `json:"orphan_status"`
    Height       uint64 `json:"height"`
    Depth        uint64 `json:"depth"`
    Hash         string `json:"hash"`
    Difficulty   string `json:"difficulty"`
    Reward       uint64 `json:"reward"`
}

type DaemonInfo struct {
    Height                   uint64 `json:"height"`
    TxCount                  uint64 `json:"tx_count"`
    TxPoolSize               uint64 `json:"tx_pool_size"`
    AltBlocksCount           uint64 `json:"alt_blocks_count"`
    OutgoingConnectionsCount uint64 `json:"outgoing_connections_count"`
    IncomingConnectionsCount uint64 `json:"incoming_connections_count"`
    PowDifficulty            uint64 `json:"pow_difficulty"`
    PosDifficulty            string `json:"pos_difficulty"`
    BlockReward              uint64 `json:"block_reward"`
    DefaultFee               uint64 `json:"default_fee"`
    MinimumFee               uint64 `json:"minimum_fee"`
    TotalCoins               string `json:"total_coins"`
    PosAllowed               bool   `json:"pos_allowed"`
    // ...
}
```

## Full Daemon RPC Reference

The following tables document all methods available on the C++ daemon. Methods marked with **(Go)** have typed Go client implementations.

### Node Information

| Method | Description | Go |
|--------|-------------|-----|
| `getinfo` | Comprehensive node status | **(Go)** `GetInfo()` |
| `getheight` | Current blockchain height | **(Go)** `GetHeight()` |
| `getblockcount` | Total block count | **(Go)** `GetBlockCount()` |

### Block Queries

| Method | Description | Go |
|--------|-------------|-----|
| `getblockhash` | Hash of block at height | -- |
| `getblocktemplate` | Template for mining | -- |
| `submitblock` | Submit mined block | **(Go)** `SubmitBlock()` |
| `submitblock2` | Extended block submission | -- |
| `get_last_block_header` | Most recent block header | **(Go)** `GetLastBlockHeader()` |
| `get_block_header_by_hash` | Block header by hash | **(Go)** `GetBlockHeaderByHash()` |
| `get_block_header_by_height` | Block header by height | **(Go)** `GetBlockHeaderByHeight()` |
| `get_blocks_details` | Detailed block info (range) | **(Go)** `GetBlocksDetails()` |
| `get_block_details` | Single block details | -- |
| `get_alt_blocks_details` | Alternative chain blocks | -- |

### Transaction Queries

| Method | Description | Go |
|--------|-------------|-----|
| `gettransactions` | Fetch txs by hash | **(Go)** `GetTransactions()` |
| `get_tx_details` | Detailed tx information | **(Go)** `GetTxDetails()` |
| `decrypt_tx_details` | Decrypt private tx data | -- |
| `get_tx_global_outputs_indexes` | Global output indices | -- |
| `search_by_id` | Search blocks/txs by hash | -- |
| `get_est_height_from_date` | Estimate height from timestamp | -- |

### Transaction Pool

| Method | Description |
|--------|-------------|
| `get_pool_info` | Pool status and pending aliases |
| `get_tx_pool` | All pool transactions |
| `get_pool_txs_details` | Detailed pool tx info |
| `get_pool_txs_brief_details` | Brief pool tx summaries |
| `get_all_pool_tx_list` | List all pool tx hashes |
| `get_current_core_tx_expiration_median` | Expiration median timestamp |
| `reset_tx_pool` | Clear transaction pool |
| `remove_tx_from_pool` | Remove specific tx |

### Key Images

| Method | Description |
|--------|-------------|
| `check_keyimages` | Check spent status (1=unspent, 0=spent) |

### Output Selection

| Method | Description | Go |
|--------|-------------|-----|
| `get_random_outs` | Random outputs for mixing | **(Go)** `GetRandomOutputs()` |
| `get_random_outs3` | Version 3 random outputs | -- |
| `get_multisig_info` | Multisig output details | -- |
| `get_global_index_info` | Global output index stats | -- |

### Alias Operations

| Method | Description |
|--------|-------------|
| `get_alias_details` | Resolve alias to address |
| `get_alias_reward` | Cost to register alias |
| `get_all_aliases` | List all registered aliases |
| `get_aliases` | Paginated alias list |
| `get_aliases_by_address` | Aliases for an address |
| `get_integrated_address` | Create integrated address |

### Asset Operations

| Method | Description |
|--------|-------------|
| `get_asset_info` | Asset details by ID |
| `get_assets_list` | List registered assets |

### Mining Control

| Method | Description |
|--------|-------------|
| `start_mining` | Start PoW mining |
| `stop_mining` | Stop PoW mining |
| `get_pos_mining_details` | PoS staking details |

### Raw Transaction

| Method | Description | Go |
|--------|-------------|-----|
| `sendrawtransaction` | Broadcast raw tx | **(Go)** `SendRawTransaction()` |
| `force_relay` | Force relay tx blobs | -- |

### Binary Endpoints

Some methods use binary (epee portable storage) serialisation for performance:

| Endpoint | Description |
|----------|-------------|
| `/getblocks.bin` | Fast block sync |
| `/get_o_indexes.bin` | Output global indices |
| `/getrandom_outs.bin` | Random outputs for ring construction |

## Full Wallet RPC Reference

### Wallet State

| Method | Description |
|--------|-------------|
| `getbalance` | Account balance (total + unlocked) |
| `getaddress` | Wallet address |
| `get_wallet_info` | Wallet metadata |
| `get_wallet_restore_info` | Seed phrase for backup |
| `get_seed_phrase_info` | Validate a seed phrase |
| `store` | Save wallet to disk |

### Transfers

| Method | Description |
|--------|-------------|
| `transfer` | Send LTHN (destinations, fee, mixin, payment_id, comment) |
| `get_payments` | Payments by payment ID |
| `get_bulk_payments` | Payments for multiple IDs |
| `get_recent_txs_and_info` | Recent transaction history |
| `search_for_transactions` | Search wallet transactions |

### Address Utilities

| Method | Description |
|--------|-------------|
| `make_integrated_address` | Create integrated address |
| `split_integrated_address` | Decode integrated address |

### Alias Management

| Method | Description |
|--------|-------------|
| `register_alias` | Register new alias |
| `update_alias` | Update alias details |

### Offline Signing

| Method | Description |
|--------|-------------|
| `sign_transfer` | Sign an unsigned tx |
| `submit_transfer` | Broadcast signed tx |

### Output Management

| Method | Description |
|--------|-------------|
| `sweep_below` | Consolidate small outputs |
| `get_bare_outs_stats` | Statistics of bare outputs |
| `sweep_bare_outs` | Convert bare outputs to confidential |

### HTLC (Hash Time-Locked Contracts)

| Method | Description |
|--------|-------------|
| `create_htlc_proposal` | Create HTLC proposal |
| `get_list_of_active_htlc` | List active HTLCs |
| `redeem_htlc` | Redeem HTLC with preimage |
| `check_htlc_redeemed` | Check redemption status |

### Ionic Swaps

| Method | Description |
|--------|-------------|
| `ionic_swap_generate_proposal` | Create swap proposal |
| `ionic_swap_get_proposal_info` | Decode swap proposal |
| `ionic_swap_accept_proposal` | Accept and execute swap |

### Asset Operations

| Method | Description |
|--------|-------------|
| `assets_deploy` | Register new asset (ticker, name, supply) |
| `assets_emit` | Emit additional supply |
| `assets_update` | Update asset metadata |
| `assets_burn` | Burn asset supply |
| `assets_whitelist_get` | Get whitelisted assets |
| `assets_whitelist_add` | Whitelist an asset |
| `assets_whitelist_remove` | Remove from whitelist |

### Escrow Contracts

| Method | Description |
|--------|-------------|
| `contracts_send_proposal` | Send escrow proposal |
| `contracts_accept_proposal` | Accept escrow proposal |
| `contracts_get_all` | List all contracts |
| `contracts_release` | Release escrow funds |
| `contracts_request_cancel` | Request cancellation |
| `contracts_accept_cancel` | Accept cancellation |

### Marketplace

| Method | Description |
|--------|-------------|
| `marketplace_get_my_offers` | List own offers |
| `marketplace_push_offer` | Create offer |
| `marketplace_push_update_offer` | Update offer |
| `marketplace_cancel_offer` | Cancel offer |

### Cryptographic Utilities

| Method | Description |
|--------|-------------|
| `sign_message` | Sign arbitrary message |
| `encrypt_data` | Encrypt data |
| `decrypt_data` | Decrypt data |

## Wire Format Notes

- **Daemon JSON-RPC:** Standard JSON-RPC 2.0 at `/json_rpc`. Method calls use `{"method": "getinfo", "params": {...}}`.
- **Wallet JSON-RPC:** Same format at `/json_rpc` on the wallet RPC port.
- **Binary endpoints:** Use epee portable storage serialisation, accessed via direct HTTP POST to the endpoint path.
- **P2P protocol:** Uses the Levin binary protocol (see [Networking](networking.md)), not JSON-RPC.
