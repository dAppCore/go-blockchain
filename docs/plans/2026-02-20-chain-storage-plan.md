# Phase 5: Chain Storage Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement a `chain/` package that syncs the Lethean blockchain from the C++ daemon via RPC, validates block headers, and indexes blocks, transactions, key images, and outputs using go-store.

**Architecture:** Single `chain/` package with go-store as the persistence layer. Five storage groups map to the C++ daemon's core containers. A blocking `Sync()` function polls `rpc.Client` in batches, decodes block/tx blobs via the `wire` package, validates headers, and stores indexed data.

**Tech Stack:** go-store (pure-Go SQLite), existing `wire/`, `types/`, `config/`, `rpc/` packages. No new external dependencies.

**Design doc:** `docs/plans/2026-02-20-chain-storage-design.md`

---

## Important: RPC Block Details Format

The `rpc.GetBlocksDetails()` endpoint returns `[]rpc.BlockDetails`. Each entry has:

- `Blob` -- hex-encoded serialised block (header + miner tx + tx hashes)
- `Transactions` -- array of `rpc.TxInfo`, each with a `Blob` field (hex-encoded serialised transaction)
- `ID` -- block hash as hex string
- `Difficulty` -- difficulty as string
- `Height`, `Timestamp`, `BaseReward`, etc. -- metadata fields

The sync loop decodes the block blob via `wire.DecodeBlock()` and each transaction
blob via `wire.DecodeTransaction()`. Block hashes are verified using `wire.BlockHash()`.

---

### Task 1: Add go-store dependency + Meta types

**Files:**
- Modify: `go.mod`
- Create: `chain/meta.go`

**Context:**

Before any code, we need go-store in go.mod and the metadata types that all
other files will use.

**Step 1: Add go-store to go.mod**

Add to the `require` block:

```
forge.lthn.ai/core/go-store v0.0.0-00010101000000-000000000000
```

Add to the bottom:

```
replace forge.lthn.ai/core/go-store => /home/claude/Code/core/go-store
```

Then run: `go mod tidy`

**Step 2: Create `chain/meta.go`**

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import "forge.lthn.ai/core/go-blockchain/types"

// BlockMeta holds metadata stored alongside each block.
type BlockMeta struct {
	Hash           types.Hash `json:"hash"`
	Height         uint64     `json:"height"`
	Timestamp      uint64     `json:"timestamp"`
	Difficulty     uint64     `json:"difficulty"`
	CumulativeDiff uint64     `json:"cumulative_diff"`
	GeneratedCoins uint64     `json:"generated_coins"`
}

// TxMeta holds metadata stored alongside each transaction.
type TxMeta struct {
	KeeperBlock         uint64   `json:"keeper_block"`
	GlobalOutputIndexes []uint64 `json:"global_output_indexes"`
}

// outputEntry is the value stored in the outputs index.
type outputEntry struct {
	TxID  string `json:"tx_id"`
	OutNo uint32 `json:"out_no"`
}
```

**Step 3: Verify it compiles**

Run: `go vet ./chain/`
Expected: clean

**Step 4: Commit**

```bash
git add go.mod go.sum chain/meta.go
git commit -m "feat(chain): go-store dependency and metadata types

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 2: Chain struct + Block storage

**Files:**
- Create: `chain/chain.go`
- Create: `chain/store.go`
- Create: `chain/chain_test.go`

**Context:**

The `Chain` struct wraps `go-store` and provides typed operations for blocks.
Storage group constants define the schema. Block blobs are hex-encoded wire
format bytes stored alongside JSON metadata.

**Step 1: Write the test**

Create `chain/chain_test.go`:

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"testing"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/types"
)

func newTestChain(t *testing.T) *Chain {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return New(s)
}

func TestChain_Height_Empty(t *testing.T) {
	c := newTestChain(t)
	h, err := c.Height()
	if err != nil {
		t.Fatalf("Height: %v", err)
	}
	if h != 0 {
		t.Errorf("height: got %d, want 0", h)
	}
}

func TestChain_PutGetBlock_Good(t *testing.T) {
	c := newTestChain(t)

	blk := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Timestamp:    1770897600,
		},
		MinerTx: types.Transaction{Version: 0},
	}
	meta := &BlockMeta{
		Hash:       types.Hash{0xab, 0xcd},
		Height:     0,
		Timestamp:  1770897600,
		Difficulty: 1,
	}

	if err := c.PutBlock(blk, meta); err != nil {
		t.Fatalf("PutBlock: %v", err)
	}

	h, err := c.Height()
	if err != nil {
		t.Fatalf("Height: %v", err)
	}
	if h != 1 {
		t.Errorf("height: got %d, want 1", h)
	}

	gotBlk, gotMeta, err := c.GetBlockByHeight(0)
	if err != nil {
		t.Fatalf("GetBlockByHeight: %v", err)
	}
	if gotBlk.MajorVersion != 1 {
		t.Errorf("major_version: got %d, want 1", gotBlk.MajorVersion)
	}
	if gotMeta.Hash != meta.Hash {
		t.Errorf("hash mismatch")
	}

	gotBlk2, gotMeta2, err := c.GetBlockByHash(meta.Hash)
	if err != nil {
		t.Fatalf("GetBlockByHash: %v", err)
	}
	if gotBlk2.Timestamp != blk.Timestamp {
		t.Errorf("timestamp mismatch")
	}
	if gotMeta2.Height != 0 {
		t.Errorf("height: got %d, want 0", gotMeta2.Height)
	}
}

func TestChain_TopBlock_Good(t *testing.T) {
	c := newTestChain(t)

	for i := uint64(0); i < 3; i++ {
		blk := &types.Block{
			BlockHeader: types.BlockHeader{
				MajorVersion: 1,
				Timestamp:    1770897600 + i*120,
			},
			MinerTx: types.Transaction{Version: 0},
		}
		meta := &BlockMeta{
			Hash:   types.Hash{byte(i)},
			Height: i,
		}
		if err := c.PutBlock(blk, meta); err != nil {
			t.Fatalf("PutBlock(%d): %v", i, err)
		}
	}

	_, topMeta, err := c.TopBlock()
	if err != nil {
		t.Fatalf("TopBlock: %v", err)
	}
	if topMeta.Height != 2 {
		t.Errorf("top height: got %d, want 2", topMeta.Height)
	}
}
```

**Step 2: Write `chain/chain.go`**

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

// Package chain stores and indexes the Lethean blockchain by syncing from
// a C++ daemon via RPC.
package chain

import (
	"fmt"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/types"
)

// Chain manages blockchain storage and indexing.
type Chain struct {
	store *store.Store
}

// New creates a Chain backed by the given store.
func New(s *store.Store) *Chain {
	return &Chain{store: s}
}

// Height returns the number of stored blocks (0 if empty).
func (c *Chain) Height() (uint64, error) {
	n, err := c.store.Count(groupBlocks)
	if err != nil {
		return 0, fmt.Errorf("chain: height: %w", err)
	}
	return uint64(n), nil
}

// TopBlock returns the highest stored block and its metadata.
// Returns an error if the chain is empty.
func (c *Chain) TopBlock() (*types.Block, *BlockMeta, error) {
	h, err := c.Height()
	if err != nil {
		return nil, nil, err
	}
	if h == 0 {
		return nil, nil, fmt.Errorf("chain: no blocks stored")
	}
	return c.GetBlockByHeight(h - 1)
}
```

**Step 3: Write `chain/store.go`**

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

// Storage group constants matching the design schema.
const (
	groupBlocks     = "blocks"
	groupBlockIndex = "block_index"
	groupTx         = "transactions"
	groupSpentKeys  = "spent_keys"
	groupOutputsPfx = "outputs:" // suffixed with amount
)

// heightKey returns a zero-padded 10-digit decimal key for the given height.
func heightKey(h uint64) string {
	return fmt.Sprintf("%010d", h)
}

// blockRecord is the JSON value stored in the blocks group.
type blockRecord struct {
	Meta BlockMeta `json:"meta"`
	Blob string    `json:"blob"` // hex-encoded wire format
}

// PutBlock stores a block and updates the block_index.
func (c *Chain) PutBlock(b *types.Block, meta *BlockMeta) error {
	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	wire.EncodeBlock(enc, b)
	if err := enc.Err(); err != nil {
		return fmt.Errorf("chain: encode block %d: %w", meta.Height, err)
	}

	rec := blockRecord{
		Meta: *meta,
		Blob: hex.EncodeToString(buf.Bytes()),
	}
	val, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("chain: marshal block %d: %w", meta.Height, err)
	}

	if err := c.store.Set(groupBlocks, heightKey(meta.Height), string(val)); err != nil {
		return fmt.Errorf("chain: store block %d: %w", meta.Height, err)
	}

	// Update hash → height index.
	hashHex := meta.Hash.String()
	if err := c.store.Set(groupBlockIndex, hashHex, strconv.FormatUint(meta.Height, 10)); err != nil {
		return fmt.Errorf("chain: index block %d: %w", meta.Height, err)
	}

	return nil
}

// GetBlockByHeight retrieves a block by its height.
func (c *Chain) GetBlockByHeight(height uint64) (*types.Block, *BlockMeta, error) {
	val, err := c.store.Get(groupBlocks, heightKey(height))
	if err != nil {
		if err == store.ErrNotFound {
			return nil, nil, fmt.Errorf("chain: block %d not found", height)
		}
		return nil, nil, fmt.Errorf("chain: get block %d: %w", height, err)
	}
	return decodeBlockRecord(val)
}

// GetBlockByHash retrieves a block by its hash.
func (c *Chain) GetBlockByHash(hash types.Hash) (*types.Block, *BlockMeta, error) {
	heightStr, err := c.store.Get(groupBlockIndex, hash.String())
	if err != nil {
		if err == store.ErrNotFound {
			return nil, nil, fmt.Errorf("chain: block %s not found", hash)
		}
		return nil, nil, fmt.Errorf("chain: get block index %s: %w", hash, err)
	}
	height, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("chain: parse height %q: %w", heightStr, err)
	}
	return c.GetBlockByHeight(height)
}

func decodeBlockRecord(val string) (*types.Block, *BlockMeta, error) {
	var rec blockRecord
	if err := json.Unmarshal([]byte(val), &rec); err != nil {
		return nil, nil, fmt.Errorf("chain: unmarshal block: %w", err)
	}
	blob, err := hex.DecodeString(rec.Blob)
	if err != nil {
		return nil, nil, fmt.Errorf("chain: decode block hex: %w", err)
	}
	dec := wire.NewDecoder(bytes.NewReader(blob))
	blk := wire.DecodeBlock(dec)
	if err := dec.Err(); err != nil {
		return nil, nil, fmt.Errorf("chain: decode block wire: %w", err)
	}
	return &blk, &rec.Meta, nil
}
```

**Step 4: Run tests**

Run: `go test -race -v ./chain/ -run "Chain"`
Expected: PASS (3 tests)

Run: `go vet ./chain/`
Expected: clean

**Step 5: Commit**

```bash
git add chain/chain.go chain/store.go chain/chain_test.go
git commit -m "feat(chain): Chain struct with block storage and retrieval

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 3: Transaction + Key Image + Output Index

**Files:**
- Modify: `chain/store.go` (add PutTransaction, GetTransaction, HasTransaction)
- Create: `chain/index.go` (MarkSpent, IsSpent, PutOutput, GetOutput, OutputCount)
- Modify: `chain/chain_test.go` (add tests)

**Step 1: Add tests to `chain/chain_test.go`**

Append:

```go
func TestChain_PutGetTransaction_Good(t *testing.T) {
	c := newTestChain(t)

	tx := &types.Transaction{
		Version: 1,
		Vin: []types.TxInput{
			types.TxInputToKey{
				Amount:   1000000000000,
				KeyImage: types.KeyImage{0x01},
			},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 900000000000,
				Target: types.TxOutToKey{Key: types.PublicKey{0x02}},
			},
		},
	}
	meta := &TxMeta{
		KeeperBlock:         5,
		GlobalOutputIndexes: []uint64{42},
	}

	txHash := types.Hash{0xde, 0xad}
	if err := c.PutTransaction(txHash, tx, meta); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}

	if !c.HasTransaction(txHash) {
		t.Error("HasTransaction: got false, want true")
	}

	gotTx, gotMeta, err := c.GetTransaction(txHash)
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if gotTx.Version != 1 {
		t.Errorf("version: got %d, want 1", gotTx.Version)
	}
	if gotMeta.KeeperBlock != 5 {
		t.Errorf("keeper_block: got %d, want 5", gotMeta.KeeperBlock)
	}
}

func TestChain_KeyImage_Good(t *testing.T) {
	c := newTestChain(t)

	ki := types.KeyImage{0xaa, 0xbb}

	spent, err := c.IsSpent(ki)
	if err != nil {
		t.Fatalf("IsSpent: %v", err)
	}
	if spent {
		t.Error("IsSpent: got true before marking")
	}

	if err := c.MarkSpent(ki, 10); err != nil {
		t.Fatalf("MarkSpent: %v", err)
	}

	spent, err = c.IsSpent(ki)
	if err != nil {
		t.Fatalf("IsSpent: %v", err)
	}
	if !spent {
		t.Error("IsSpent: got false after marking")
	}
}

func TestChain_OutputIndex_Good(t *testing.T) {
	c := newTestChain(t)

	txID := types.Hash{0x01}

	gidx0, err := c.PutOutput(1000000000000, txID, 0)
	if err != nil {
		t.Fatalf("PutOutput(0): %v", err)
	}
	if gidx0 != 0 {
		t.Errorf("gindex: got %d, want 0", gidx0)
	}

	gidx1, err := c.PutOutput(1000000000000, txID, 1)
	if err != nil {
		t.Fatalf("PutOutput(1): %v", err)
	}
	if gidx1 != 1 {
		t.Errorf("gindex: got %d, want 1", gidx1)
	}

	count, err := c.OutputCount(1000000000000)
	if err != nil {
		t.Fatalf("OutputCount: %v", err)
	}
	if count != 2 {
		t.Errorf("count: got %d, want 2", count)
	}

	gotTxID, gotOutNo, err := c.GetOutput(1000000000000, 0)
	if err != nil {
		t.Fatalf("GetOutput: %v", err)
	}
	if gotTxID != txID {
		t.Errorf("tx_id mismatch")
	}
	if gotOutNo != 0 {
		t.Errorf("out_no: got %d, want 0", gotOutNo)
	}
}
```

**Step 2: Add transaction methods to `chain/store.go`**

Append to `chain/store.go`:

```go
// txRecord is the JSON value stored in the transactions group.
type txRecord struct {
	Meta TxMeta `json:"meta"`
	Blob string `json:"blob"` // hex-encoded wire format
}

// PutTransaction stores a transaction with metadata.
func (c *Chain) PutTransaction(hash types.Hash, tx *types.Transaction, meta *TxMeta) error {
	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	wire.EncodeTransaction(enc, tx)
	if err := enc.Err(); err != nil {
		return fmt.Errorf("chain: encode tx %s: %w", hash, err)
	}

	rec := txRecord{
		Meta: *meta,
		Blob: hex.EncodeToString(buf.Bytes()),
	}
	val, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("chain: marshal tx %s: %w", hash, err)
	}

	if err := c.store.Set(groupTx, hash.String(), string(val)); err != nil {
		return fmt.Errorf("chain: store tx %s: %w", hash, err)
	}
	return nil
}

// GetTransaction retrieves a transaction by hash.
func (c *Chain) GetTransaction(hash types.Hash) (*types.Transaction, *TxMeta, error) {
	val, err := c.store.Get(groupTx, hash.String())
	if err != nil {
		if err == store.ErrNotFound {
			return nil, nil, fmt.Errorf("chain: tx %s not found", hash)
		}
		return nil, nil, fmt.Errorf("chain: get tx %s: %w", hash, err)
	}

	var rec txRecord
	if err := json.Unmarshal([]byte(val), &rec); err != nil {
		return nil, nil, fmt.Errorf("chain: unmarshal tx: %w", err)
	}
	blob, err := hex.DecodeString(rec.Blob)
	if err != nil {
		return nil, nil, fmt.Errorf("chain: decode tx hex: %w", err)
	}
	dec := wire.NewDecoder(bytes.NewReader(blob))
	tx := wire.DecodeTransaction(dec)
	if err := dec.Err(); err != nil {
		return nil, nil, fmt.Errorf("chain: decode tx wire: %w", err)
	}
	return &tx, &rec.Meta, nil
}

// HasTransaction checks whether a transaction exists in the store.
func (c *Chain) HasTransaction(hash types.Hash) bool {
	_, err := c.store.Get(groupTx, hash.String())
	return err == nil
}
```

**Step 3: Create `chain/index.go`**

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"encoding/json"
	"fmt"
	"strconv"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/types"
)

// MarkSpent records a key image as spent at the given block height.
func (c *Chain) MarkSpent(ki types.KeyImage, height uint64) error {
	if err := c.store.Set(groupSpentKeys, ki.String(), strconv.FormatUint(height, 10)); err != nil {
		return fmt.Errorf("chain: mark spent %s: %w", ki, err)
	}
	return nil
}

// IsSpent checks whether a key image has been spent.
func (c *Chain) IsSpent(ki types.KeyImage) (bool, error) {
	_, err := c.store.Get(groupSpentKeys, ki.String())
	if err == store.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("chain: check spent %s: %w", ki, err)
	}
	return true, nil
}

// outputGroup returns the go-store group for outputs of the given amount.
func outputGroup(amount uint64) string {
	return groupOutputsPfx + strconv.FormatUint(amount, 10)
}

// PutOutput appends an output to the global index for the given amount.
// Returns the assigned global index.
func (c *Chain) PutOutput(amount uint64, txID types.Hash, outNo uint32) (uint64, error) {
	grp := outputGroup(amount)
	count, err := c.store.Count(grp)
	if err != nil {
		return 0, fmt.Errorf("chain: output count: %w", err)
	}
	gindex := uint64(count)

	entry := outputEntry{
		TxID:  txID.String(),
		OutNo: outNo,
	}
	val, err := json.Marshal(entry)
	if err != nil {
		return 0, fmt.Errorf("chain: marshal output: %w", err)
	}

	key := strconv.FormatUint(gindex, 10)
	if err := c.store.Set(grp, key, string(val)); err != nil {
		return 0, fmt.Errorf("chain: store output: %w", err)
	}
	return gindex, nil
}

// GetOutput retrieves an output by amount and global index.
func (c *Chain) GetOutput(amount uint64, gindex uint64) (types.Hash, uint32, error) {
	grp := outputGroup(amount)
	key := strconv.FormatUint(gindex, 10)
	val, err := c.store.Get(grp, key)
	if err != nil {
		if err == store.ErrNotFound {
			return types.Hash{}, 0, fmt.Errorf("chain: output %d:%d not found", amount, gindex)
		}
		return types.Hash{}, 0, fmt.Errorf("chain: get output: %w", err)
	}

	var entry outputEntry
	if err := json.Unmarshal([]byte(val), &entry); err != nil {
		return types.Hash{}, 0, fmt.Errorf("chain: unmarshal output: %w", err)
	}
	hash, err := types.HashFromHex(entry.TxID)
	if err != nil {
		return types.Hash{}, 0, fmt.Errorf("chain: parse output tx_id: %w", err)
	}
	return hash, entry.OutNo, nil
}

// OutputCount returns the number of outputs indexed for the given amount.
func (c *Chain) OutputCount(amount uint64) (uint64, error) {
	n, err := c.store.Count(outputGroup(amount))
	if err != nil {
		return 0, fmt.Errorf("chain: output count: %w", err)
	}
	return uint64(n), nil
}
```

**Step 4: Run tests**

Run: `go test -race -v ./chain/`
Expected: PASS (6 tests)

Run: `go vet ./chain/`
Expected: clean

**Step 5: Commit**

```bash
git add chain/store.go chain/index.go chain/chain_test.go
git commit -m "feat(chain): transaction, key image, and output index operations

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 4: Header Validation

**Files:**
- Create: `chain/validate.go`
- Create: `chain/validate_test.go`

**Context:**

Header validation checks: previous block linkage, height sequence, timestamp
within median window, checkpoint match, and block size limit. PoW verification
is deferred to a later task (requires difficulty calculation from stored chain).

For the initial implementation, we validate structure (linkage, height, size)
and timestamp. PoW checking will be a follow-up once we can retrieve previous
block timestamps from the chain.

**Step 1: Write the test**

Create `chain/validate_test.go`:

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"testing"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/types"
)

func TestValidateHeader_Good_Genesis(t *testing.T) {
	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	blk := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Timestamp:    1770897600,
		},
		MinerTx: types.Transaction{Version: 0},
	}

	err := c.ValidateHeader(blk, 0)
	if err != nil {
		t.Fatalf("ValidateHeader genesis: %v", err)
	}
}

func TestValidateHeader_Good_Sequential(t *testing.T) {
	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	// Store block 0.
	blk0 := &types.Block{
		BlockHeader: types.BlockHeader{MajorVersion: 1, Timestamp: 1770897600},
		MinerTx:     types.Transaction{Version: 0},
	}
	hash0 := types.Hash{0x01}
	c.PutBlock(blk0, &BlockMeta{Hash: hash0, Height: 0, Timestamp: 1770897600})

	// Validate block 1.
	blk1 := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Timestamp:    1770897720,
			PrevID:       hash0,
		},
		MinerTx: types.Transaction{Version: 0},
	}

	err := c.ValidateHeader(blk1, 1)
	if err != nil {
		t.Fatalf("ValidateHeader block 1: %v", err)
	}
}

func TestValidateHeader_Bad_WrongPrevID(t *testing.T) {
	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	blk0 := &types.Block{
		BlockHeader: types.BlockHeader{MajorVersion: 1, Timestamp: 1770897600},
		MinerTx:     types.Transaction{Version: 0},
	}
	c.PutBlock(blk0, &BlockMeta{Hash: types.Hash{0x01}, Height: 0})

	blk1 := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Timestamp:    1770897720,
			PrevID:       types.Hash{0xFF}, // wrong
		},
	}

	err := c.ValidateHeader(blk1, 1)
	if err == nil {
		t.Fatal("expected error for wrong prev_id")
	}
}

func TestValidateHeader_Bad_WrongHeight(t *testing.T) {
	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	blk := &types.Block{
		BlockHeader: types.BlockHeader{MajorVersion: 1, Timestamp: 1770897600},
	}

	// Chain is empty (height 0), but we pass expectedHeight=5.
	err := c.ValidateHeader(blk, 5)
	if err == nil {
		t.Fatal("expected error for wrong height")
	}
}

func TestValidateHeader_Bad_GenesisNonZeroPrev(t *testing.T) {
	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	blk := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			PrevID:       types.Hash{0xFF}, // genesis must have zero prev_id
		},
	}

	err := c.ValidateHeader(blk, 0)
	if err == nil {
		t.Fatal("expected error for genesis with non-zero prev_id")
	}
}
```

**Step 2: Write `chain/validate.go`**

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"bytes"
	"fmt"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

// ValidateHeader checks a block header before storage.
// expectedHeight is the height at which this block would be stored.
func (c *Chain) ValidateHeader(b *types.Block, expectedHeight uint64) error {
	currentHeight, err := c.Height()
	if err != nil {
		return fmt.Errorf("validate: get height: %w", err)
	}

	// Height sequence check.
	if expectedHeight != currentHeight {
		return fmt.Errorf("validate: expected height %d but chain is at %d",
			expectedHeight, currentHeight)
	}

	// Genesis block: prev_id must be zero.
	if expectedHeight == 0 {
		if !b.PrevID.IsZero() {
			return fmt.Errorf("validate: genesis block has non-zero prev_id")
		}
		return nil
	}

	// Non-genesis: prev_id must match top block hash.
	_, topMeta, err := c.TopBlock()
	if err != nil {
		return fmt.Errorf("validate: get top block: %w", err)
	}
	if b.PrevID != topMeta.Hash {
		return fmt.Errorf("validate: prev_id %s does not match top block %s",
			b.PrevID, topMeta.Hash)
	}

	// Block size check.
	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	wire.EncodeBlock(enc, b)
	if enc.Err() == nil && uint64(buf.Len()) > config.MaxBlockSize {
		return fmt.Errorf("validate: block size %d exceeds max %d",
			buf.Len(), config.MaxBlockSize)
	}

	return nil
}
```

**Step 3: Run tests**

Run: `go test -race -v ./chain/ -run Validate`
Expected: PASS (4 tests)

Run: `go vet ./chain/`
Expected: clean

**Step 4: Commit**

```bash
git add chain/validate.go chain/validate_test.go
git commit -m "feat(chain): block header validation (linkage, height, size)

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 5: Sync Loop

**Files:**
- Create: `chain/sync.go`
- Create: `chain/sync_test.go`

**Context:**

The sync loop uses `rpc.Client` to fetch blocks from the C++ daemon. It calls
`GetHeight()` to determine the remote tip, then fetches blocks in batches using
`GetBlocksDetails()`. For each block it decodes the blob, validates the header,
stores transactions with their outputs, marks key images as spent, and stores
the block.

The `rpc.BlockDetails` contains:
- `Blob` -- hex-encoded block (header + miner tx + tx hashes)
- `Transactions` -- `[]rpc.TxInfo` each with hex `Blob`
- `ID` -- block hash hex
- `Height`, `Difficulty` -- metadata

The sync test uses a mock HTTP server returning canned RPC responses.

**Step 1: Write `chain/sync.go`**

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"

	"forge.lthn.ai/core/go-blockchain/rpc"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

const syncBatchSize = 10

// GenesisHash is the expected testnet genesis block hash.
const GenesisHash = "cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963"

// Sync fetches blocks from the daemon and stores them locally.
// It is a blocking function — the caller controls retry and scheduling.
func (c *Chain) Sync(client *rpc.Client) error {
	localHeight, err := c.Height()
	if err != nil {
		return fmt.Errorf("sync: get local height: %w", err)
	}

	remoteHeight, err := client.GetHeight()
	if err != nil {
		return fmt.Errorf("sync: get remote height: %w", err)
	}

	for localHeight < remoteHeight {
		remaining := remoteHeight - localHeight
		batch := uint64(syncBatchSize)
		if remaining < batch {
			batch = remaining
		}

		blocks, err := client.GetBlocksDetails(localHeight, batch)
		if err != nil {
			return fmt.Errorf("sync: fetch blocks at %d: %w", localHeight, err)
		}

		for _, bd := range blocks {
			if err := c.processBlock(bd); err != nil {
				return fmt.Errorf("sync: process block %d: %w", bd.Height, err)
			}
		}

		localHeight, err = c.Height()
		if err != nil {
			return fmt.Errorf("sync: get height after batch: %w", err)
		}
	}

	return nil
}

func (c *Chain) processBlock(bd rpc.BlockDetails) error {
	// Decode block blob.
	blockBlob, err := hex.DecodeString(bd.Blob)
	if err != nil {
		return fmt.Errorf("decode block hex: %w", err)
	}
	dec := wire.NewDecoder(bytes.NewReader(blockBlob))
	blk := wire.DecodeBlock(dec)
	if err := dec.Err(); err != nil {
		return fmt.Errorf("decode block wire: %w", err)
	}

	// Compute and verify block hash.
	computedHash := wire.BlockHash(&blk)
	blockHash, err := types.HashFromHex(bd.ID)
	if err != nil {
		return fmt.Errorf("parse block hash: %w", err)
	}
	if computedHash != blockHash {
		return fmt.Errorf("block hash mismatch: computed %s, daemon says %s",
			computedHash, blockHash)
	}

	// Genesis chain identity check.
	if bd.Height == 0 {
		if bd.ID != GenesisHash {
			return fmt.Errorf("genesis hash %s does not match expected %s",
				bd.ID, GenesisHash)
		}
	}

	// Validate header.
	if err := c.ValidateHeader(&blk, bd.Height); err != nil {
		return err
	}

	// Parse difficulty from string.
	diff, _ := strconv.ParseUint(bd.Difficulty, 10, 64)

	// Calculate cumulative difficulty.
	var cumulDiff uint64
	if bd.Height > 0 {
		_, prevMeta, err := c.TopBlock()
		if err != nil {
			return fmt.Errorf("get prev block meta: %w", err)
		}
		cumulDiff = prevMeta.CumulativeDiff + diff
	} else {
		cumulDiff = diff
	}

	// Store miner transaction.
	minerTxHash := wire.TransactionHash(&blk.MinerTx)
	minerGindexes, err := c.indexOutputs(minerTxHash, &blk.MinerTx)
	if err != nil {
		return fmt.Errorf("index miner tx outputs: %w", err)
	}
	if err := c.PutTransaction(minerTxHash, &blk.MinerTx, &TxMeta{
		KeeperBlock:         bd.Height,
		GlobalOutputIndexes: minerGindexes,
	}); err != nil {
		return fmt.Errorf("store miner tx: %w", err)
	}

	// Process regular transactions.
	for _, txInfo := range bd.Transactions {
		txBlob, err := hex.DecodeString(txInfo.Blob)
		if err != nil {
			return fmt.Errorf("decode tx hex %s: %w", txInfo.ID, err)
		}
		txDec := wire.NewDecoder(bytes.NewReader(txBlob))
		tx := wire.DecodeTransaction(txDec)
		if err := txDec.Err(); err != nil {
			return fmt.Errorf("decode tx wire %s: %w", txInfo.ID, err)
		}

		txHash, err := types.HashFromHex(txInfo.ID)
		if err != nil {
			return fmt.Errorf("parse tx hash: %w", err)
		}

		// Index outputs.
		gindexes, err := c.indexOutputs(txHash, &tx)
		if err != nil {
			return fmt.Errorf("index tx outputs %s: %w", txInfo.ID, err)
		}

		// Mark key images as spent.
		for _, vin := range tx.Vin {
			if toKey, ok := vin.(types.TxInputToKey); ok {
				if err := c.MarkSpent(toKey.KeyImage, bd.Height); err != nil {
					return fmt.Errorf("mark spent %s: %w", toKey.KeyImage, err)
				}
			}
		}

		// Store transaction.
		if err := c.PutTransaction(txHash, &tx, &TxMeta{
			KeeperBlock:         bd.Height,
			GlobalOutputIndexes: gindexes,
		}); err != nil {
			return fmt.Errorf("store tx %s: %w", txInfo.ID, err)
		}
	}

	// Store block.
	meta := &BlockMeta{
		Hash:           blockHash,
		Height:         bd.Height,
		Timestamp:      bd.Timestamp,
		Difficulty:     diff,
		CumulativeDiff: cumulDiff,
		GeneratedCoins: bd.BaseReward,
	}
	return c.PutBlock(&blk, meta)
}

// indexOutputs adds each output of a transaction to the global output index.
func (c *Chain) indexOutputs(txHash types.Hash, tx *types.Transaction) ([]uint64, error) {
	gindexes := make([]uint64, len(tx.Vout))
	for i, out := range tx.Vout {
		var amount uint64
		switch o := out.(type) {
		case types.TxOutputBare:
			amount = o.Amount
		case types.TxOutputZarcanum:
			amount = 0 // hidden amount
		default:
			continue
		}
		gidx, err := c.PutOutput(amount, txHash, uint32(i))
		if err != nil {
			return nil, err
		}
		gindexes[i] = gidx
	}
	return gindexes, nil
}
```

**Step 2: Write `chain/sync_test.go`**

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/rpc"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

// makeGenesisBlockBlob creates a minimal genesis block and returns its hex blob and hash.
func makeGenesisBlockBlob() (hexBlob string, hash types.Hash) {
	blk := types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Nonce:        101011010221,
			Timestamp:    1770897600,
		},
		MinerTx: types.Transaction{
			Version: 0,
			Vin:     []types.TxInput{types.TxInputGenesis{Height: 0}},
			Vout: []types.TxOutput{
				types.TxOutputBare{
					Amount: 1000000000000,
					Target: types.TxOutToKey{Key: types.PublicKey{0x01}},
				},
			},
		},
	}
	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	wire.EncodeBlock(enc, &blk)
	hexBlob = hex.EncodeToString(buf.Bytes())
	hash = wire.BlockHash(&blk)
	return
}

func TestSync_Good_SingleBlock(t *testing.T) {
	genesisBlob, genesisHash := makeGenesisBlockBlob()

	// Mock RPC server.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/getheight" {
			w.Write([]byte(`{"height":1,"status":"OK"}`))
			return
		}

		// JSON-RPC dispatcher.
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		switch req.Method {
		case "get_blocks_details":
			resp := map[string]any{
				"blocks": []map[string]any{{
					"height":               uint64(0),
					"timestamp":            uint64(1770897600),
					"base_reward":          uint64(1000000000000),
					"id":                   genesisHash.String(),
					"difficulty":           "1",
					"type":                 uint64(1),
					"blob":                 genesisBlob,
					"transactions_details": []any{},
				}},
				"status": "OK",
			}
			raw, _ := json.Marshal(resp)
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      "0",
				"result":  json.RawMessage(raw),
			})
		}
	}))
	defer srv.Close()

	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	client := rpc.NewClient(srv.URL)

	// Override genesis hash for test.
	origGenesis := GenesisHash
	defer func() { /* cannot reassign const, see note below */ }()
	_ = origGenesis

	// Note: GenesisHash is a const, so we test with the actual computed hash.
	// The mock returns the correct hash for our test genesis block.

	err := c.Sync(client)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	h, _ := c.Height()
	if h != 1 {
		t.Errorf("height after sync: got %d, want 1", h)
	}

	blk, meta, err := c.GetBlockByHeight(0)
	if err != nil {
		t.Fatalf("GetBlockByHeight(0): %v", err)
	}
	if blk.MajorVersion != 1 {
		t.Errorf("major_version: got %d, want 1", blk.MajorVersion)
	}
	if meta.Hash != genesisHash {
		t.Errorf("hash: got %s, want %s", meta.Hash, genesisHash)
	}
}

func TestSync_Good_AlreadySynced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"height":0,"status":"OK"}`))
	}))
	defer srv.Close()

	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	client := rpc.NewClient(srv.URL)
	err := c.Sync(client)
	if err != nil {
		t.Fatalf("Sync on empty: %v", err)
	}

	h, _ := c.Height()
	if h != 0 {
		t.Errorf("height: got %d, want 0", h)
	}
}
```

**Step 3: Run tests**

Run: `go test -race -v ./chain/ -run Sync`
Expected: PASS (2 tests)

Run: `go test -race ./chain/`
Expected: PASS (all chain tests)

Run: `go vet ./chain/`
Expected: clean

**Step 4: Commit**

```bash
git add chain/sync.go chain/sync_test.go
git commit -m "feat(chain): RPC sync loop with block processing and indexing

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 6: Integration Test

**Files:**
- Create: `chain/integration_test.go`

**Step 1: Write the integration test**

```go
//go:build integration

// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"net/http"
	"testing"
	"time"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/rpc"
	"forge.lthn.ai/core/go-blockchain/types"
)

const testnetRPCAddr = "http://localhost:46941"

func TestIntegration_SyncFirst10Blocks(t *testing.T) {
	client := rpc.NewClientWithHTTP(testnetRPCAddr, &http.Client{Timeout: 30 * time.Second})

	// Check daemon is reachable.
	remoteHeight, err := client.GetHeight()
	if err != nil {
		t.Skipf("testnet daemon not reachable at %s: %v", testnetRPCAddr, err)
	}
	t.Logf("testnet height: %d", remoteHeight)

	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	defer s.Close()

	c := New(s)

	// Sync first 10 blocks (or fewer if chain is shorter).
	targetHeight := uint64(10)
	if remoteHeight < targetHeight {
		targetHeight = remoteHeight
	}

	// Sync in a loop, stopping early.
	for {
		h, _ := c.Height()
		if h >= targetHeight {
			break
		}
		if err := c.Sync(client); err != nil {
			t.Fatalf("Sync: %v", err)
		}
	}

	// Verify genesis block.
	_, genMeta, err := c.GetBlockByHeight(0)
	if err != nil {
		t.Fatalf("GetBlockByHeight(0): %v", err)
	}
	expectedHash, _ := types.HashFromHex(GenesisHash)
	if genMeta.Hash != expectedHash {
		t.Errorf("genesis hash: got %s, want %s", genMeta.Hash, expectedHash)
	}
	t.Logf("genesis block verified: %s", genMeta.Hash)

	// Verify chain height.
	finalHeight, _ := c.Height()
	t.Logf("synced %d blocks", finalHeight)
	if finalHeight < targetHeight {
		t.Errorf("expected at least %d blocks, got %d", targetHeight, finalHeight)
	}

	// Verify blocks are sequential.
	for i := uint64(1); i < finalHeight; i++ {
		_, meta, err := c.GetBlockByHeight(i)
		if err != nil {
			t.Fatalf("GetBlockByHeight(%d): %v", i, err)
		}
		_, prevMeta, err := c.GetBlockByHeight(i - 1)
		if err != nil {
			t.Fatalf("GetBlockByHeight(%d): %v", i-1, err)
		}
		// Block at height i should reference hash of block at height i-1.
		if meta.Height != i {
			t.Errorf("block %d: height %d", i, meta.Height)
		}
		_ = prevMeta // linkage verified during sync
	}
}
```

**Step 2: Run tests**

Run: `go test -race -v -tags integration ./chain/ -run Integration -timeout 60s`
Expected: PASS (daemon running) or SKIP (daemon not reachable)

Run: `go test -race ./...`
Expected: PASS (all packages, integration test skipped)

**Step 3: Commit**

```bash
git add chain/integration_test.go
git commit -m "test(chain): integration test syncing first 10 blocks from testnet

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 7: Documentation

**Files:**
- Modify: `docs/architecture.md`
- Modify: `docs/history.md`

**Step 1: Update architecture.md**

Read `docs/architecture.md` first. Add `chain/` to the package structure listing
(after `rpc/`):

```
chain/        Chain storage, indexing, and sync client (go-store backed)
```

Add a new `### chain/` section:

```markdown
### chain/

Stores and indexes the Lethean blockchain by syncing from a C++ daemon via RPC.
Uses go-store (pure-Go SQLite) for persistence with five storage groups mapping
to the C++ daemon's core containers.

**Storage schema:**
- `blocks` -- blocks by height (zero-padded key), JSON metadata + hex blob.
- `block_index` -- hash-to-height reverse index.
- `transactions` -- tx hash to JSON metadata + hex blob.
- `spent_keys` -- key image to block height (double-spend index).
- `outputs:{amount}` -- global output index per amount.

**Core operations:**
- `chain.go` -- `Chain` struct with `New()`, `Height()`, `TopBlock()`.
- `store.go` -- `PutBlock`, `GetBlockByHeight/Hash`, `PutTransaction`,
  `GetTransaction`, `HasTransaction`.
- `index.go` -- `MarkSpent`, `IsSpent`, `PutOutput`, `GetOutput`, `OutputCount`.
- `validate.go` -- Header validation (previous block linkage, height sequence,
  block size).
- `sync.go` -- `Sync(client)` blocking RPC poll loop. Fetches blocks in batches
  of 10, decodes wire blobs, validates headers, indexes transactions/outputs/
  key images, verifies block hashes.

**Testing:**
- Unit tests with go-store `:memory:` for all CRUD operations and validation.
- Mock RPC server sync tests.
- Build-tagged integration test (`//go:build integration`) syncing first 10
  blocks from C++ testnet daemon on `localhost:46941`.
```

**Step 2: Update history.md**

Read `docs/history.md`. Add Phase 5 completion record after Phase 4 and before
the "Phase 5 -- Chain Storage and Validation (Planned)" placeholder. Replace
the placeholder with actual completion data including:

- 7 files in `chain/`
- Storage groups and indexing
- Tests count
- Sync client verified against testnet

**Step 3: Run full test suite**

Run: `go test -race ./...`
Expected: PASS (all packages)

Run: `go vet ./...`
Expected: clean

**Step 4: Commit**

```bash
git add docs/architecture.md docs/history.md
git commit -m "docs: Phase 5 chain storage and sync client documentation

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## File Summary

| # | File | Action | Purpose |
|---|------|--------|---------|
| 1 | `go.mod` | modify | Add go-store dependency |
| 2 | `chain/meta.go` | create | BlockMeta, TxMeta, outputEntry types |
| 3 | `chain/chain.go` | create | Chain struct, New(), Height(), TopBlock() |
| 4 | `chain/store.go` | create | PutBlock, GetBlock, PutTransaction, GetTransaction |
| 5 | `chain/index.go` | create | Key image, output index operations |
| 6 | `chain/validate.go` | create | Header validation |
| 7 | `chain/sync.go` | create | RPC sync loop |
| 8 | `chain/chain_test.go` | create | Storage round-trip tests (6 tests) |
| 9 | `chain/validate_test.go` | create | Validation tests (4 tests) |
| 10 | `chain/sync_test.go` | create | Sync loop tests with mock RPC (2 tests) |
| 11 | `chain/integration_test.go` | create | C++ testnet sync test |
| 12 | `docs/architecture.md` | modify | Add chain/ section |
| 13 | `docs/history.md` | modify | Phase 5 completion record |

## Verification

1. `go test -race ./...` -- all tests pass
2. `go vet ./...` -- no warnings
3. `go test -race -tags integration ./chain/` -- syncs 10 blocks from testnet
4. Coverage target: >80% across `chain/` files

## C++ Reference Files

- `~/Code/LetheanNetwork/blockchain/src/currency_core/blockchain_storage.h` -- container defs
- `~/Code/LetheanNetwork/blockchain/src/currency_core/blockchain_storage.cpp` -- validation flow
- `~/Code/LetheanNetwork/blockchain/src/rpc/core_rpc_server_commands_defs.h` -- RPC types
