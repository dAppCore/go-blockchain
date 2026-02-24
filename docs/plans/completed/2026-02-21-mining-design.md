# Phase 8: Mining — Design Document

## Goal

Solo PoW miner that talks to a C++ daemon via JSON-RPC. Fetches block templates,
grinds nonces with RandomX, and submits solutions.

## Decisions

- **Solo miner** — not embedded in the Go node; talks to the C++ daemon via
  `getblocktemplate` / `submitblock` RPC.
- **Single-threaded** — one mining goroutine. The CGo RandomX bridge uses a
  static global cache/VM that is not thread-safe; matches that constraint.
- **Daemon-provided templates** — the daemon constructs the coinbase transaction,
  selects mempool transactions, and computes the reward. The miner only iterates
  the nonce.
- **No new dependencies** — builds on existing `rpc`, `crypto`, `wire`, `types`
  packages.

## Architecture

```
                ┌──────────────┐
                │  C++ Daemon  │
                │  (testnet)   │
                └──────┬───────┘
                       │ JSON-RPC
                ┌──────┴───────┐
                │  rpc.Client  │
                │  (existing)  │
                └──────┬───────┘
                       │
          ┌────────────┴────────────┐
          │      mining.Miner       │
          │                         │
          │  1. GetBlockTemplate()  │
          │  2. Compute header hash │
          │  3. Iterate nonces      │
          │  4. SubmitBlock()       │
          └────────────┬────────────┘
                       │ CGo
                ┌──────┴───────┐
                │ crypto.RandomX│
                │  (existing)  │
                └──────────────┘
```

## Mining Loop

### Template fetch

1. Call `getblocktemplate` with miner's wallet address.
2. Decode hex blob → `types.Block` via `wire.DecodeBlock`.
3. Compute the header mining hash (once per template):
   - Set nonce to 0 in the block.
   - Build `wire.BlockHashingBlob(block)`.
   - Keccak-256 the blob → 32-byte `headerMiningHash`.
4. Parse difficulty from the response (string → uint64).

### Nonce grinding

1. Start nonce at 0.
2. For each nonce: `crypto.RandomXHash("LetheanRandomXv1", headerMiningHash || nonce_LE)`.
3. Check result via `crypto.CheckDifficulty(powHash, difficulty)`.
4. On solution: set `block.Nonce = nonce`, serialise via `wire.EncodeBlock`,
   hex-encode, call `rpc.Client.SubmitBlock`.

### Template refresh triggers

- Context cancellation (shutdown).
- New block detected: poll `getinfo` every `PollInterval` (default 3s),
  compare height to current template height.
- Nonce exhaustion (re-fetch template, which gets a new timestamp).

### Optimisation

The C++ miner computes `BlockHashingBlob` with nonce=0 once, then Keccak-256's
it once. The nonce is appended separately as RandomX input (`headerHash || nonce_LE`).
The inner loop is therefore just RandomX + difficulty check — no re-serialisation.

## API

```go
type Config struct {
    DaemonURL    string
    WalletAddr   string
    PollInterval time.Duration        // default 3s
    OnBlockFound func(height uint64, hash types.Hash)
    OnNewTemplate func(height uint64, difficulty uint64)
}

type Miner struct { /* unexported fields */ }

func NewMiner(cfg Config) *Miner
func (m *Miner) Start(ctx context.Context) error  // blocks until ctx cancelled
func (m *Miner) Stats() Stats                     // safe from any goroutine

type Stats struct {
    Hashrate    float64
    BlocksFound uint64
    Height      uint64
    Difficulty  uint64
    Uptime      time.Duration
}
```

`Start(ctx)` is synchronous. The caller controls lifecycle via context. Stats
are updated atomically; a separate goroutine can call `Stats()` for display.

## RPC Addition

Add `GetBlockTemplate` to `rpc/mining.go`:

```go
type BlockTemplateRequest struct {
    WalletAddress string `json:"wallet_address"`
    ExtraText     string `json:"extra_text,omitempty"`
}

type BlockTemplateResponse struct {
    Difficulty            string `json:"difficulty"`
    Height                uint64 `json:"height"`
    BlockTemplateBlob     string `json:"blocktemplate_blob"`
    PrevHash              string `json:"prev_hash"`
    BlockRewardWithoutFee uint64 `json:"block_reward_without_fee"`
    BlockReward           uint64 `json:"block_reward"`
    TxsFee                uint64 `json:"txs_fee"`
    Status                string `json:"status"`
}

func (c *Client) GetBlockTemplate(walletAddr string) (*BlockTemplateResponse, error)
```

## Testing

### Unit tests (no daemon)

- `TestMinerStats` — atomic stats updates, hashrate calculation.
- `TestHeaderMiningHash` — known block → known header hash (genesis test vector).
- `TestNonceSolution` — known valid nonce → miner accepts it as solution.
- `TestTemplateRefresh` — mock RPC returns different heights, verify re-fetch.
- `TestGetBlockTemplate` — mock HTTP server, verify response parsing.

### Mock interface

```go
type TemplateProvider interface {
    GetBlockTemplate(walletAddr string) (*rpc.BlockTemplateResponse, error)
    SubmitBlock(hexBlob string) error
    GetInfo() (*rpc.DaemonInfo, error)
}
```

Real `rpc.Client` satisfies this. Tests inject a mock.

### Integration test

`//go:build integration` — connects to testnet daemon on `localhost:46941`,
fetches a real template, verifies pipeline (template parse → header hash →
nonce check), does not mine to solution.

### Coverage target

>85% on `mining/` package.

## File Summary

| File | Action | Purpose |
|------|--------|---------|
| `mining/miner.go` | new | Config, Miner, Stats, mining loop |
| `mining/hash.go` | new | HeaderMiningHash, nonce checking |
| `mining/miner_test.go` | new | Unit tests with mock RPC |
| `mining/hash_test.go` | new | Header hash and nonce verification |
| `mining/integration_test.go` | new | Integration test against testnet |
| `rpc/mining.go` | modify | Add GetBlockTemplate |
| `rpc/mining_test.go` | modify | Add GetBlockTemplate mock test |
| `docs/architecture.md` | modify | Add mining/ package |
| `docs/history.md` | modify | Record Phase 8 |
