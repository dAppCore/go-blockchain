# Phase 4 Design: RPC Client

## Summary

Phase 4 adds a JSON-RPC 2.0 client for querying the Lethean daemon. The `rpc/`
package provides typed Go methods for 10 core daemon endpoints covering chain
state, block headers, transaction details, and block submission.

This is a client-only phase. Server implementation is deferred to Phase 5+ when
chain storage is available. The client talks to the existing C++ daemon via
HTTP POST to `/json_rpc`.

## Decisions

| Question | Decision |
|----------|----------|
| Client or server? | Client only — server needs chain storage (Phase 5) |
| Endpoint scope | Tier 1: 10 core chain query endpoints |
| Transport | JSON-RPC 2.0 only (no legacy JSON paths, no binary) |
| Dependencies | stdlib `net/http` + `encoding/json` only |
| Testing | Integration tests against C++ testnet daemon |
| Architecture | Single `rpc/` package, thin typed client |

## Package Structure

```
rpc/
  client.go            -- Client struct, JSON-RPC 2.0 transport
  client_test.go       -- Transport tests (mock HTTP server)
  types.go             -- Shared response types (BlockHeader, TxInfo, etc.)
  info.go              -- GetInfo, GetHeight, GetBlockCount
  blocks.go            -- GetLastBlockHeader, GetBlockHeaderByHeight,
                          GetBlockHeaderByHash, GetBlocksDetails
  transactions.go      -- GetTxDetails, GetTransactions
  mining.go            -- SubmitBlock
  integration_test.go  -- Build-tagged C++ testnet tests
```

## Client Transport

```go
type Client struct {
    url        string       // "http://localhost:46941/json_rpc"
    httpClient *http.Client
}

func NewClient(daemonURL string) *Client
func NewClientWithHTTP(daemonURL string, httpClient *http.Client) *Client
```

Every RPC call goes through a single internal method:

```go
func (c *Client) call(method string, params any, result any) error
```

This builds the JSON-RPC 2.0 request:

```json
{
  "jsonrpc": "2.0",
  "id": "0",
  "method": "getblockcount",
  "params": {}
}
```

POSTs to the daemon URL, and unmarshals the `result` field from the response.
If the daemon returns a JSON-RPC error object, it is converted to an `*RPCError`.

### Error Handling

```go
type RPCError struct {
    Code    int
    Message string
}

func (e *RPCError) Error() string
```

Daemon error codes from `core_rpc_server_error_codes.h`:

```
WRONG_PARAM        = -1
TOO_BIG_HEIGHT     = -2
WRONG_BLOCKBLOB    = -6
BLOCK_NOT_ACCEPTED = -7
CORE_BUSY          = -9
NOT_FOUND          = -14
```

HTTP-level errors (connection refused, timeouts) are returned as standard Go
errors, not `*RPCError`. Callers can use `errors.As` to distinguish.

### URL Construction

`NewClient("http://localhost:46941")` automatically appends `/json_rpc` if the
path is empty. `NewClient("http://localhost:46941/json_rpc")` uses the URL
as-is.

## Response Types

### BlockHeader

Returned by `getlastblockheader`, `getblockheaderbyheight`, and
`getblockheaderbyhash`:

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
```

Difficulty is a string because it can exceed uint64 range on high-difficulty
chains. Hash and PrevHash are hex-encoded 32-byte values.

### DaemonInfo

Returned by `getinfo`. Only the most useful fields are mapped:

```go
type DaemonInfo struct {
    Height                      uint64 `json:"height"`
    TxCount                     uint64 `json:"tx_count"`
    TxPoolSize                  uint64 `json:"tx_pool_size"`
    AltBlocksCount              uint64 `json:"alt_blocks_count"`
    OutgoingConnectionsCount    uint64 `json:"outgoing_connections_count"`
    IncomingConnectionsCount    uint64 `json:"incoming_connections_count"`
    SynchronizedConnectionsCount uint64 `json:"synchronized_connections_count"`
    DaemonNetworkState          uint64 `json:"daemon_network_state"`
    SynchronizationStartHeight  uint64 `json:"synchronization_start_height"`
    MaxNetSeenHeight            uint64 `json:"max_net_seen_height"`
    PowDifficulty               uint64 `json:"pow_difficulty"`
    PosDifficulty               string `json:"pos_difficulty"`
    BlockReward                 uint64 `json:"block_reward"`
    DefaultFee                  uint64 `json:"default_fee"`
    MinimumFee                  uint64 `json:"minimum_fee"`
    LastBlockTimestamp          uint64 `json:"last_block_timestamp"`
    LastBlockHash               string `json:"last_block_hash"`
    AliasCount                  uint64 `json:"alias_count"`
    TotalCoins                  string `json:"total_coins"`
    PosAllowed                  bool   `json:"pos_allowed"`
    CurrentMaxAllowedBlockSize  uint64 `json:"current_max_allowed_block_size"`
}
```

The `getinfo` request sends `flags: 0` by default (cheapest query). A
`GetInfoFull()` variant could be added later with `flags: 1048575`.

### BlockDetails

Returned by `get_blocks_details`:

```go
type BlockDetails struct {
    Height          uint64   `json:"height"`
    Timestamp       uint64   `json:"timestamp"`
    ActualTimestamp  uint64   `json:"actual_timestamp"`
    BaseReward      uint64   `json:"base_reward"`
    SummaryReward   uint64   `json:"summary_reward"`
    TotalFee        uint64   `json:"total_fee"`
    ID              string   `json:"id"`
    PrevID          string   `json:"prev_id"`
    Difficulty      string   `json:"difficulty"`
    Type            uint64   `json:"type"`
    IsOrphan        bool     `json:"is_orphan"`
    CumulativeSize  uint64   `json:"block_cumulative_size"`
    Blob            string   `json:"blob"`
    ObjectInJSON    string   `json:"object_in_json"`
    Transactions    []TxInfo `json:"transactions_details"`
}
```

### TxInfo

Returned by `get_tx_details` and embedded in `BlockDetails`:

```go
type TxInfo struct {
    ID           string `json:"id"`
    BlobSize     uint64 `json:"blob_size"`
    Fee          uint64 `json:"fee"`
    Timestamp    uint64 `json:"timestamp"`
    KeeperBlock  int64  `json:"keeper_block"`
    Blob         string `json:"blob"`
    ObjectInJSON string `json:"object_in_json"`
}
```

`KeeperBlock` is -1 for unconfirmed transactions (in mempool). `Blob` is the
hex-encoded serialised transaction. Detailed input/output parsing (outs, ins,
extra) is deferred — the raw blob can be decoded using the existing `wire/`
package.

## Endpoints

| Go method | JSON-RPC method | Request params | Returns |
|-----------|----------------|----------------|---------|
| `GetInfo()` | `getinfo` | `{flags: 0}` | `*DaemonInfo, error` |
| `GetHeight()` | `getheight` | `{}` | `uint64, error` |
| `GetBlockCount()` | `getblockcount` | `{}` | `uint64, error` |
| `GetLastBlockHeader()` | `getlastblockheader` | `{}` | `*BlockHeader, error` |
| `GetBlockHeaderByHeight(h uint64)` | `getblockheaderbyheight` | `{height: h}` | `*BlockHeader, error` |
| `GetBlockHeaderByHash(hash string)` | `getblockheaderbyhash` | `{hash: hash}` | `*BlockHeader, error` |
| `GetBlocksDetails(start, count uint64)` | `get_blocks_details` | `{height_start, count, ignore_transactions: false}` | `[]BlockDetails, error` |
| `GetTxDetails(hash string)` | `get_tx_details` | `{tx_hash: hash}` | `*TxInfo, error` |
| `GetTransactions(hashes []string)` | `gettransactions` | `{txs_hashes: [...]}` | `txHex []string, missed []string, error` |
| `SubmitBlock(hexBlob string)` | `submitblock` | `[hexBlob]` | `error` |

### Special Cases

- `getlastblockheader` takes no params (empty object `{}`).
- `submitblock` takes an array of strings (not an object) as params.
- `getinfo` sends `{flags: 0}` to avoid expensive calculations.
- `gettransactions` returns two lists: found transaction hex blobs and missed
  hashes.

## Testing Strategy

### Unit Tests (mock HTTP server)

Each endpoint gets a test using `httptest.NewServer` that:

1. Verifies the incoming JSON-RPC request (method name, params)
2. Returns a known JSON response
3. Asserts the Go function returns the correct typed values

Additional transport tests:
- Connection refused → error (not panic)
- Invalid JSON response → error
- JSON-RPC error response → `*RPCError`
- HTTP 500 → error
- Timeout → error

### Integration Tests (C++ testnet)

Build-tagged `//go:build integration` test that:

1. Creates a client pointing at `localhost:46941`
2. Calls `GetHeight()` — verifies > 0
3. Calls `GetLastBlockHeader()` — verifies fields populated
4. Calls `GetBlockHeaderByHeight(0)` — genesis block, verifies hash matches
   `cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963`
5. Calls `GetInfo()` — verifies height matches, connections > 0
6. Calls `GetBlocksDetails(0, 1)` — verifies genesis block details

The genesis block hash check is the Phase 4 equivalent of the Phase 1 hash
verification — if it matches, the entire RPC client is correctly parsing the
daemon's responses.

## C++ Reference Files

- `src/rpc/core_rpc_server_commands_defs.h` — all command structs
- `src/rpc/core_rpc_server.h` — method registration
- `src/rpc/core_rpc_server_error_codes.h` — error codes
- `src/rpc/core_rpc_server.cpp` — handler implementations
