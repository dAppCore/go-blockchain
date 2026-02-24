# Phase 5 Design: Chain Storage and Sync Client

## Summary

Phase 5 adds a `chain/` package that stores the Lethean blockchain by syncing
from the C++ daemon via RPC. It validates block headers (PoW, timestamps,
checkpoints, linkage) and builds five core indexes: blocks by height, block
hash-to-height, transactions, spent key images, and global output index.

This is a sync client, not a full independent validator. Transaction signatures,
ring proofs, and balance checks are deferred to a later phase. The sync client
trusts the C++ daemon for transaction validity while independently verifying
chain structure.

## Decisions

| Question | Decision |
|----------|----------|
| Scope | Sync client — fetch, validate headers, store and index |
| Storage backend | go-store (pure-Go SQLite, WAL mode, `:memory:` for tests) |
| Architecture | Single `chain/` package, split by file |
| Indexes | 5 core: blocks, block_index, transactions, spent_keys, outputs |
| Sync method | RPC polling via existing `rpc.Client` |
| Validation depth | Headers only (PoW, timestamps, checkpoints, linkage, size) |
| Transaction validation | Deferred — signatures, proofs, balance checks are Phase 7 |

## Storage Schema

The `chain/` package maps five go-store groups to the C++ daemon's core
containers:

| go-store Group | Key | Value | C++ Equivalent |
|----------------|-----|-------|----------------|
| `blocks` | height (zero-padded 10-digit decimal) | JSON: `BlockMeta` + hex blob | `m_db_blocks` |
| `block_index` | block hash (hex) | height (decimal string) | `m_db_blocks_index` |
| `transactions` | tx hash (hex) | JSON: `TxMeta` + hex blob | `m_db_transactions` |
| `spent_keys` | key image (hex) | height (decimal string) | `m_db_spent_keys` |
| `outputs:{amount}` | global index (decimal) | JSON: `{tx_id, out_no}` | `m_db_outputs` |

### Key encoding

- Heights as keys are zero-padded to 10 digits (`0000000000`, `0000000001`)
  so lexicographic ordering matches numeric ordering within go-store.
- Hashes and key images are hex-encoded (64 lowercase characters).
- Block and transaction blobs are hex-encoded wire format bytes from
  `wire.EncodeBlock` / `wire.EncodeTransaction`. Storing the raw binary
  enables re-decoding without data loss.

### Output index

The `outputs` group uses a dynamic suffix per amount: `outputs:0` for
Zarcanum hidden-amount outputs, `outputs:1000000000000` for 1 LTHN outputs.
This mirrors the C++ `m_db_outputs[amount]` array-per-amount structure.

## Chain API

```go
type Chain struct {
    store *store.Store
}

func New(store *store.Store) *Chain
```

### Block operations

```go
func (c *Chain) PutBlock(b *types.Block, meta *BlockMeta) error
func (c *Chain) GetBlockByHeight(height uint64) (*types.Block, *BlockMeta, error)
func (c *Chain) GetBlockByHash(hash types.Hash) (*types.Block, *BlockMeta, error)
func (c *Chain) TopBlock() (*types.Block, *BlockMeta, error)
func (c *Chain) Height() (uint64, error)
```

### BlockMeta

```go
type BlockMeta struct {
    Hash           types.Hash
    Height         uint64
    Timestamp      uint64
    Difficulty     uint64
    CumulativeDiff uint64
    GeneratedCoins uint64
}
```

### Transaction operations

```go
type TxMeta struct {
    KeeperBlock         uint64
    GlobalOutputIndexes []uint64
}

func (c *Chain) PutTransaction(tx *types.Transaction, meta *TxMeta) error
func (c *Chain) GetTransaction(hash types.Hash) (*types.Transaction, *TxMeta, error)
func (c *Chain) HasTransaction(hash types.Hash) bool
```

### Key image operations

```go
func (c *Chain) MarkSpent(ki types.KeyImage, height uint64) error
func (c *Chain) IsSpent(ki types.KeyImage) (bool, error)
```

### Output index operations

```go
func (c *Chain) PutOutput(amount uint64, txID types.Hash, outNo uint32) (uint64, error)
func (c *Chain) GetOutput(amount uint64, gindex uint64) (types.Hash, uint32, error)
func (c *Chain) OutputCount(amount uint64) (uint64, error)
```

## Header Validation

When a block arrives from the sync loop, the header is validated before storage.
Checks run in this order:

1. **Previous block linkage** -- `block.PrevID` must equal stored top block hash.
   Genesis block has zero prev_id.

2. **Height sequence** -- block height must equal stored height + 1.

3. **Timestamp** -- block timestamp must be within acceptable range of the
   median of the last N timestamps (from `config.TimestampCheckWindow`). Must
   not be too far in the future.

4. **Checkpoint match** -- if a hardcoded checkpoint exists for this height,
   block hash must match exactly. Blocks in the checkpoint zone skip expensive
   validation (matching C++ behaviour).

5. **PoW verification** -- for PoW blocks (flags bit 0 == 0): compute the block
   hash via `crypto.FastHash`, check it meets the difficulty target. Difficulty
   is calculated from the previous blocks using `difficulty.NextDifficulty()`.

6. **Block size** -- total serialised size within `config.MaxBlockSize`.

### What is deferred

- Transaction signature verification
- Ring proof validation
- Balance checks
- Miner tx reward validation
- PoS kernel validation (PoS blocks accepted on header validity + checkpoint
  trust during sync)

## Sync Loop

```go
func (c *Chain) Sync(client *rpc.Client) error
```

The sync loop is a blocking function. No background goroutine, no event loop.
The caller controls when and how often to call it.

### Algorithm

1. Get local height from `c.Height()`.
2. Get remote height from `client.GetHeight()`.
3. If local >= remote, return nil (already synced).
4. Fetch blocks in batches using `client.GetBlocksDetails(start, batchSize)`.
   Batch size of 10 (daemon caps response size).
5. For each block in the batch:
   - Decode the block blob via `wire.DecodeBlock()`.
   - Validate header (checks 1-6 above).
   - Extract and store each transaction with global output indexes.
   - Mark input key images as spent.
   - Store the block with metadata.
6. Repeat from step 2 until caught up.
7. Return nil.

### Error handling

- RPC errors return to the caller (who decides retry policy).
- Validation failures return an error with height and details.
- go-store errors are wrapped and returned.

### Genesis check

On first run (height == 0), the sync fetches block 0 and validates its hash
matches `cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963`.
This is the chain identity check.

## Package Structure

```
chain/
  chain.go              -- Chain struct, New(), Height(), TopBlock()
  store.go              -- Storage schema constants, PutBlock, GetBlock, PutTransaction
  index.go              -- Output index, key image, block index operations
  validate.go           -- Header validation (linkage, timestamp, PoW, checkpoint, size)
  sync.go               -- RPC sync loop
  meta.go               -- BlockMeta, TxMeta types
  chain_test.go         -- Storage round-trip tests
  validate_test.go      -- Header validation tests
  sync_test.go          -- Sync loop tests with mock RPC
  integration_test.go   -- Build-tagged C++ testnet sync test
```

### Dependencies

New `go.mod` entry: `forge.lthn.ai/core/go-store` with local replace directive
(same pattern as `go-p2p`).

The `chain/` package imports `types/`, `wire/`, `config/`, `difficulty/`,
`crypto/`, and `rpc/`. This is the first package that ties the full stack
together.

## Testing Strategy

### Unit tests (go-store `:memory:`)

- Storage operations: PutBlock/GetBlock round-trip, PutTransaction/GetTransaction,
  key image marking, output index append/lookup.
- Header validation: good block accepted, bad prev_id rejected, bad timestamp
  rejected, checkpoint mismatch rejected, block too large rejected.
- Index consistency: store 10 blocks, verify all lookups by height and hash,
  verify output counts match.

### Integration-style tests (mock RPC server)

- Sync loop with `httptest.NewServer` returning canned block data for heights 0-5.
- Verify sync stores all blocks, transactions, key images, outputs correctly.
- Verify sync stops at remote height.
- Verify sync resumes from local height on second call.
- Verify genesis hash validation catches wrong chain.

### Build-tagged integration test (C++ testnet)

- `//go:build integration`
- Sync first 10 blocks from `localhost:46941`.
- Verify block 0 hash matches genesis.
- Verify transaction count matches for those blocks.
- Verify key images indexed correctly.

### Coverage target

Greater than 80% across `chain/` files.

## C++ Reference Files

- `src/currency_core/blockchain_storage.h` -- container definitions, validation flow
- `src/currency_core/blockchain_storage.cpp` -- `handle_block_to_main_chain()`, output index
- `src/currency_core/tx_pool.h` -- mempool structure (deferred)
- `src/currency_core/checkpoints.h` -- checkpoint system
- `src/rpc/core_rpc_server_commands_defs.h` -- RPC response structures
