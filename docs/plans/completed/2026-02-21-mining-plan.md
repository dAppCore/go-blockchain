# Phase 8: Mining Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Solo PoW miner that fetches block templates from a C++ daemon, grinds nonces with RandomX, and submits solutions.

**Architecture:** Single-threaded mining loop in a new `mining/` package. Talks to the daemon via `rpc.Client` (existing). Uses `crypto.RandomXHash` (existing CGo bridge) for PoW hashing and `consensus.CheckDifficulty` for solution validation. Template provided by daemon — miner only iterates the nonce.

**Tech Stack:** Go stdlib, existing `rpc/`, `crypto/`, `wire/`, `types/`, `consensus/` packages. No new dependencies.

---

### Task 1: Add GetBlockTemplate to RPC client

**Files:**
- Modify: `rpc/mining.go`
- Modify: `rpc/mining_test.go`

**Step 1: Write the failing test**

Add to `rpc/mining_test.go`:

```go
func TestGetBlockTemplate_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req jsonRPCRequest
		json.Unmarshal(body, &req)
		if req.Method != "getblocktemplate" {
			t.Errorf("method: got %q, want %q", req.Method, "getblocktemplate")
		}
		// Verify wallet_address is in params.
		raw, _ := json.Marshal(req.Params)
		if !bytes.Contains(raw, []byte("iTHN")) {
			t.Errorf("params should contain wallet address, got: %s", raw)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Result: json.RawMessage(`{
				"difficulty": "42",
				"height": 100,
				"blocktemplate_blob": "0100000000000000000000000000",
				"prev_hash": "cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963",
				"block_reward_without_fee": 1000000000000,
				"block_reward": 1000000000000,
				"txs_fee": 0,
				"status": "OK"
			}`),
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	resp, err := c.GetBlockTemplate("iTHNtestaddr")
	if err != nil {
		t.Fatalf("GetBlockTemplate: %v", err)
	}
	if resp.Difficulty != "42" {
		t.Errorf("difficulty: got %q, want %q", resp.Difficulty, "42")
	}
	if resp.Height != 100 {
		t.Errorf("height: got %d, want 100", resp.Height)
	}
	if resp.BlockReward != 1000000000000 {
		t.Errorf("block_reward: got %d, want 1000000000000", resp.BlockReward)
	}
}

func TestGetBlockTemplate_Bad_Status(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Result:  json.RawMessage(`{"status":"BUSY"}`),
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.GetBlockTemplate("iTHNtestaddr")
	if err == nil {
		t.Fatal("expected error for BUSY status")
	}
}
```

Add `"bytes"` to the import block in `rpc/mining_test.go`.

**Step 2: Run test to verify it fails**

Run: `go test -run TestGetBlockTemplate ./rpc/ -v`
Expected: FAIL — `GetBlockTemplate` not defined.

**Step 3: Write minimal implementation**

Add to `rpc/mining.go`:

```go
// BlockTemplateResponse is the daemon's response to getblocktemplate.
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

// GetBlockTemplate requests a block template from the daemon for mining.
func (c *Client) GetBlockTemplate(walletAddr string) (*BlockTemplateResponse, error) {
	params := struct {
		WalletAddress string `json:"wallet_address"`
	}{WalletAddress: walletAddr}
	var resp BlockTemplateResponse
	if err := c.call("getblocktemplate", params, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "OK" {
		return nil, fmt.Errorf("getblocktemplate: status %q", resp.Status)
	}
	return &resp, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -run TestGetBlockTemplate ./rpc/ -v`
Expected: PASS (both `_Good` and `_Bad_Status`).

**Step 5: Commit**

```bash
git add rpc/mining.go rpc/mining_test.go
git commit -m "feat(rpc): add GetBlockTemplate endpoint

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 2: Header mining hash computation

**Files:**
- Create: `mining/hash.go`
- Create: `mining/hash_test.go`

**Step 1: Write the failing test**

Create `mining/hash_test.go`:

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package mining

import (
	"bytes"
	"encoding/hex"
	"testing"

	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

func testnetGenesisHeader() types.BlockHeader {
	return types.BlockHeader{
		MajorVersion: 1,
		Nonce:        101011010221,
		PrevID:       types.Hash{},
		MinorVersion: 0,
		Timestamp:    1770897600,
		Flags:        0,
	}
}

func TestHeaderMiningHash_Good(t *testing.T) {
	// Build the genesis block from the known raw coinbase transaction.
	rawTx := testnetGenesisRawTx()
	dec := wire.NewDecoder(bytes.NewReader(rawTx))
	minerTx := wire.DecodeTransaction(dec)
	if dec.Err() != nil {
		t.Fatalf("decode genesis tx: %v", dec.Err())
	}

	block := types.Block{
		BlockHeader: testnetGenesisHeader(),
		MinerTx:     minerTx,
	}

	got := HeaderMiningHash(&block)

	// The header mining hash is computed with nonce=0, so manually compute
	// it to get the expected value.
	block.Nonce = 0
	blob := wire.BlockHashingBlob(&block)
	want := wire.Keccak256(blob)

	if got != want {
		t.Errorf("HeaderMiningHash:\n  got:  %s\n  want: %s",
			hex.EncodeToString(got[:]), hex.EncodeToString(want[:]))
	}
}

func TestHeaderMiningHash_Good_NonceIgnored(t *testing.T) {
	// HeaderMiningHash must produce the same result regardless of the
	// block's current nonce value.
	rawTx := testnetGenesisRawTx()
	dec := wire.NewDecoder(bytes.NewReader(rawTx))
	minerTx := wire.DecodeTransaction(dec)
	if dec.Err() != nil {
		t.Fatalf("decode genesis tx: %v", dec.Err())
	}

	block1 := types.Block{
		BlockHeader: testnetGenesisHeader(),
		MinerTx:     minerTx,
	}
	block2 := block1
	block2.Nonce = 999999

	h1 := HeaderMiningHash(&block1)
	h2 := HeaderMiningHash(&block2)

	if h1 != h2 {
		t.Errorf("HeaderMiningHash changed with different nonce:\n  nonce=%d: %x\n  nonce=%d: %x",
			block1.Nonce, h1, block2.Nonce, h2)
	}
}
```

Note: `testnetGenesisRawTx()` is needed in the test file. Copy the same helper from `wire/hash_test.go`:

```go
func testnetGenesisRawTx() []byte {
	u64s := [25]uint64{
		0xa080800100000101, 0x03018ae3c8e0c8cf, 0x7b0287d2a2218485, 0x720c5b385edbe3dd,
		0x178e7c64d18a598f, 0x98bb613ff63e6d03, 0x3814f971f9160500, 0x1c595f65f55d872e,
		0x835e5fd926b1f78d, 0xf597c7f5a33b6131, 0x2074496b139c8341, 0x64612073656b6174,
		0x20656761746e6176, 0x6e2065687420666f, 0x666f206572757461, 0x616d726f666e6920,
		0x696562206e6f6974, 0x207973616520676e, 0x6165727073206f74, 0x6168207475622064,
		0x7473206f74206472, 0x202d202e656c6966, 0x206968736f746153, 0x6f746f6d616b614e,
		0x0a0e0d66020b0015,
	}
	u8s := [2]uint8{0x00, 0x00}
	buf := make([]byte, 25*8+2)
	for i, v := range u64s {
		binary.LittleEndian.PutUint64(buf[i*8:], v)
	}
	buf[200] = u8s[0]
	buf[201] = u8s[1]
	return buf
}
```

Add `"encoding/binary"` to the import block.

**Step 2: Run test to verify it fails**

Run: `go test -run TestHeaderMiningHash ./mining/ -v`
Expected: FAIL — package `mining` doesn't exist yet.

**Step 3: Write minimal implementation**

Create `mining/hash.go`:

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

// Package mining provides a solo PoW miner that talks to a C++ daemon
// via JSON-RPC. It fetches block templates, grinds nonces with RandomX,
// and submits solutions.
package mining

import (
	"encoding/binary"

	"forge.lthn.ai/core/go-blockchain/consensus"
	"forge.lthn.ai/core/go-blockchain/crypto"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

// RandomXKey is the cache initialisation key for RandomX hashing.
var RandomXKey = []byte("LetheanRandomXv1")

// HeaderMiningHash computes the header hash used as input to RandomX.
// The nonce in the block is set to 0 before computing the hash, matching
// the C++ get_block_header_mining_hash() function.
//
// The result is deterministic for a given block template regardless of
// the block's current nonce value.
func HeaderMiningHash(b *types.Block) [32]byte {
	// Save and zero the nonce.
	savedNonce := b.Nonce
	b.Nonce = 0
	blob := wire.BlockHashingBlob(b)
	b.Nonce = savedNonce

	return wire.Keccak256(blob)
}

// CheckNonce tests whether a specific nonce produces a valid PoW solution
// for the given header mining hash and difficulty.
func CheckNonce(headerHash [32]byte, nonce, difficulty uint64) (bool, error) {
	var input [40]byte
	copy(input[:32], headerHash[:])
	binary.LittleEndian.PutUint64(input[32:], nonce)

	powHash, err := crypto.RandomXHash(RandomXKey, input[:])
	if err != nil {
		return false, err
	}

	return consensus.CheckDifficulty(types.Hash(powHash), difficulty), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -run TestHeaderMiningHash ./mining/ -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add mining/hash.go mining/hash_test.go
git commit -m "feat(mining): header mining hash and nonce checking

Port of C++ get_block_header_mining_hash(). Computes BlockHashingBlob
with nonce=0, Keccak-256's it. CheckNonce wraps RandomX + difficulty.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 3: Config, Stats, and Miner struct

**Files:**
- Create: `mining/miner.go`
- Create: `mining/miner_test.go`

**Step 1: Write the failing test**

Create `mining/miner_test.go`:

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package mining

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewMiner_Good(t *testing.T) {
	cfg := Config{
		DaemonURL:    "http://localhost:46941",
		WalletAddr:   "iTHNtestaddr",
		PollInterval: 5 * time.Second,
	}
	m := NewMiner(cfg)

	assert.NotNil(t, m)
	stats := m.Stats()
	assert.Equal(t, float64(0), stats.Hashrate)
	assert.Equal(t, uint64(0), stats.BlocksFound)
	assert.Equal(t, uint64(0), stats.Height)
	assert.Equal(t, uint64(0), stats.Difficulty)
	assert.Equal(t, time.Duration(0), stats.Uptime)
}

func TestNewMiner_Good_DefaultPollInterval(t *testing.T) {
	cfg := Config{
		DaemonURL:  "http://localhost:46941",
		WalletAddr: "iTHNtestaddr",
	}
	m := NewMiner(cfg)

	// PollInterval should default to 3s.
	assert.Equal(t, 3*time.Second, m.cfg.PollInterval)
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run TestNewMiner ./mining/ -v`
Expected: FAIL — `Config`, `NewMiner` not defined.

**Step 3: Write minimal implementation**

Create `mining/miner.go`:

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package mining

import (
	"sync/atomic"
	"time"

	"forge.lthn.ai/core/go-blockchain/rpc"
	"forge.lthn.ai/core/go-blockchain/types"
)

// TemplateProvider abstracts the RPC methods needed by the miner.
// The real rpc.Client satisfies this interface.
type TemplateProvider interface {
	GetBlockTemplate(walletAddr string) (*rpc.BlockTemplateResponse, error)
	SubmitBlock(hexBlob string) error
	GetInfo() (*rpc.DaemonInfo, error)
}

// Config holds the miner configuration.
type Config struct {
	// DaemonURL is the JSON-RPC endpoint of the C++ daemon.
	DaemonURL string

	// WalletAddr is the address that receives mining rewards.
	WalletAddr string

	// PollInterval is how often to check for new blocks. Default: 3s.
	PollInterval time.Duration

	// OnBlockFound is called after a solution is successfully submitted.
	// May be nil.
	OnBlockFound func(height uint64, hash types.Hash)

	// OnNewTemplate is called when a new block template is fetched.
	// May be nil.
	OnNewTemplate func(height uint64, difficulty uint64)

	// Provider is the RPC provider. If nil, a default rpc.Client is
	// created from DaemonURL.
	Provider TemplateProvider
}

// Stats holds read-only mining statistics.
type Stats struct {
	Hashrate    float64
	BlocksFound uint64
	Height      uint64
	Difficulty  uint64
	Uptime      time.Duration
}

// Miner is a solo PoW miner that talks to a C++ daemon via JSON-RPC.
type Miner struct {
	cfg       Config
	provider  TemplateProvider
	startTime time.Time

	// Atomic stats — accessed from Stats() on any goroutine.
	hashCount   atomic.Uint64
	blocksFound atomic.Uint64
	height      atomic.Uint64
	difficulty  atomic.Uint64
}

// NewMiner creates a new miner with the given configuration.
func NewMiner(cfg Config) *Miner {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 3 * time.Second
	}

	var provider TemplateProvider
	if cfg.Provider != nil {
		provider = cfg.Provider
	} else {
		provider = rpc.NewClient(cfg.DaemonURL)
	}

	return &Miner{
		cfg:      cfg,
		provider: provider,
	}
}

// Stats returns a snapshot of the current mining statistics.
// Safe to call from any goroutine.
func (m *Miner) Stats() Stats {
	var uptime time.Duration
	if !m.startTime.IsZero() {
		uptime = time.Since(m.startTime)
	}

	hashes := m.hashCount.Load()
	var hashrate float64
	if uptime > 0 {
		hashrate = float64(hashes) / uptime.Seconds()
	}

	return Stats{
		Hashrate:    hashrate,
		BlocksFound: m.blocksFound.Load(),
		Height:      m.height.Load(),
		Difficulty:  m.difficulty.Load(),
		Uptime:      uptime,
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test -run TestNewMiner ./mining/ -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add mining/miner.go mining/miner_test.go
git commit -m "feat(mining): Config, Stats, and Miner struct

TemplateProvider interface for testability. Atomic stats for
lock-free reads from any goroutine.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 4: Mining loop with mock provider

**Files:**
- Modify: `mining/miner.go`
- Modify: `mining/miner_test.go`

**Step 1: Write the failing test**

Add to `mining/miner_test.go`:

```go
func TestMiner_Start_Good_ShutdownOnCancel(t *testing.T) {
	mock := &mockProvider{
		templates: []*rpc.BlockTemplateResponse{
			{
				Difficulty:        "1",
				Height:            100,
				BlockTemplateBlob: hex.EncodeToString(minimalBlockBlob(t)),
				Status:            "OK",
			},
		},
		infos: []*rpc.DaemonInfo{{Height: 100}},
	}

	cfg := Config{
		DaemonURL:    "http://localhost:46941",
		WalletAddr:   "iTHNtestaddr",
		PollInterval: 100 * time.Millisecond,
		Provider:     mock,
	}
	m := NewMiner(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := m.Start(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	stats := m.Stats()
	assert.Equal(t, uint64(100), stats.Height)
	assert.Equal(t, uint64(1), stats.Difficulty)
}

func TestMiner_Start_Good_TemplateRefresh(t *testing.T) {
	// First call returns height 100, second returns 101 — triggers refresh.
	mock := &mockProvider{
		templates: []*rpc.BlockTemplateResponse{
			{Difficulty: "1", Height: 100, BlockTemplateBlob: hex.EncodeToString(minimalBlockBlob(t)), Status: "OK"},
			{Difficulty: "2", Height: 101, BlockTemplateBlob: hex.EncodeToString(minimalBlockBlob(t)), Status: "OK"},
		},
		infos: []*rpc.DaemonInfo{
			{Height: 100},
			{Height: 101}, // triggers refresh
			{Height: 101},
		},
	}

	cfg := Config{
		DaemonURL:    "http://localhost:46941",
		WalletAddr:   "iTHNtestaddr",
		PollInterval: 50 * time.Millisecond,
		Provider:     mock,
	}
	m := NewMiner(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	_ = m.Start(ctx)

	assert.GreaterOrEqual(t, mock.templateCalls.Load(), int64(2))
}
```

Add mock and helper to the test file:

```go
import (
	"bytes"
	"context"
	"encoding/hex"
	"sync/atomic"

	"forge.lthn.ai/core/go-blockchain/rpc"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

type mockProvider struct {
	templates     []*rpc.BlockTemplateResponse
	infos         []*rpc.DaemonInfo
	templateCalls atomic.Int64
	infoCalls     atomic.Int64
	submitCalls   atomic.Int64
	submitted     []string
}

func (m *mockProvider) GetBlockTemplate(walletAddr string) (*rpc.BlockTemplateResponse, error) {
	idx := int(m.templateCalls.Add(1) - 1)
	if idx >= len(m.templates) {
		idx = len(m.templates) - 1
	}
	return m.templates[idx], nil
}

func (m *mockProvider) SubmitBlock(hexBlob string) error {
	m.submitCalls.Add(1)
	m.submitted = append(m.submitted, hexBlob)
	return nil
}

func (m *mockProvider) GetInfo() (*rpc.DaemonInfo, error) {
	idx := int(m.infoCalls.Add(1) - 1)
	if idx >= len(m.infos) {
		idx = len(m.infos) - 1
	}
	return m.infos[idx], nil
}

// minimalBlockBlob returns a serialised block that can be decoded by wire.DecodeBlock.
func minimalBlockBlob(t *testing.T) []byte {
	t.Helper()
	block := types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Nonce:        0,
			Timestamp:    1770897600,
		},
		MinerTx: types.Transaction{
			Version: 1,
			Vin:     []types.TxInput{types.TxInputGenesis{Height: 100}},
			Vout: []types.TxOutput{types.TxOutputBare{
				Amount: 1000000000000,
				Target: types.TxOutToKey{},
			}},
			Extra: []byte{},
		},
	}
	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	wire.EncodeBlock(enc, &block)
	if enc.Err() != nil {
		t.Fatalf("encode block: %v", enc.Err())
	}
	return buf.Bytes()
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run "TestMiner_Start" ./mining/ -v`
Expected: FAIL — `Start` not defined.

**Step 3: Write minimal implementation**

Add to `mining/miner.go`:

```go
import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"

	"forge.lthn.ai/core/go-blockchain/crypto"
	"forge.lthn.ai/core/go-blockchain/wire"
)

// Start runs the mining loop. It blocks until ctx is cancelled.
// Returns the context error (typically context.Canceled or context.DeadlineExceeded).
func (m *Miner) Start(ctx context.Context) error {
	m.startTime = time.Now()

	for {
		// Fetch a block template.
		tmpl, err := m.provider.GetBlockTemplate(m.cfg.WalletAddr)
		if err != nil {
			// Transient RPC error — wait and retry.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(m.cfg.PollInterval):
				continue
			}
		}

		// Parse difficulty.
		diff, err := strconv.ParseUint(tmpl.Difficulty, 10, 64)
		if err != nil {
			return fmt.Errorf("mining: invalid difficulty %q: %w", tmpl.Difficulty, err)
		}

		// Decode the block template blob.
		blobBytes, err := hex.DecodeString(tmpl.BlockTemplateBlob)
		if err != nil {
			return fmt.Errorf("mining: invalid template blob hex: %w", err)
		}
		dec := wire.NewDecoder(bytes.NewReader(blobBytes))
		block := wire.DecodeBlock(dec)
		if dec.Err() != nil {
			return fmt.Errorf("mining: decode template: %w", dec.Err())
		}

		// Update stats.
		m.height.Store(tmpl.Height)
		m.difficulty.Store(diff)

		if m.cfg.OnNewTemplate != nil {
			m.cfg.OnNewTemplate(tmpl.Height, diff)
		}

		// Compute the header mining hash (once per template).
		headerHash := HeaderMiningHash(&block)

		// Mine until solution found or template becomes stale.
		if err := m.mine(ctx, &block, headerHash, diff); err != nil {
			return err
		}
	}
}

// mine grinds nonces against the given header hash and difficulty.
// Returns nil when a new template should be fetched (new block detected).
// Returns ctx.Err() when shutdown is requested.
func (m *Miner) mine(ctx context.Context, block *types.Block, headerHash [32]byte, difficulty uint64) error {
	pollTicker := time.NewTicker(m.cfg.PollInterval)
	defer pollTicker.Stop()

	currentHeight := m.height.Load()

	for nonce := uint64(0); ; nonce++ {
		// Check for shutdown or poll trigger.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pollTicker.C:
			// Check if chain has advanced.
			info, err := m.provider.GetInfo()
			if err == nil && info.Height > currentHeight {
				return nil // fetch new template
			}
			continue
		default:
		}

		// Compute RandomX hash.
		var input [40]byte
		copy(input[:32], headerHash[:])
		binary.LittleEndian.PutUint64(input[32:], nonce)

		powHash, err := crypto.RandomXHash(RandomXKey, input[:])
		if err != nil {
			return fmt.Errorf("mining: RandomX hash: %w", err)
		}

		m.hashCount.Add(1)

		if consensus.CheckDifficulty(types.Hash(powHash), difficulty) {
			// Solution found!
			block.Nonce = nonce

			var buf bytes.Buffer
			enc := wire.NewEncoder(&buf)
			wire.EncodeBlock(enc, block)
			if enc.Err() != nil {
				return fmt.Errorf("mining: encode solution: %w", enc.Err())
			}

			hexBlob := hex.EncodeToString(buf.Bytes())
			if err := m.provider.SubmitBlock(hexBlob); err != nil {
				return fmt.Errorf("mining: submit block: %w", err)
			}

			m.blocksFound.Add(1)

			if m.cfg.OnBlockFound != nil {
				m.cfg.OnBlockFound(currentHeight, wire.BlockHash(block))
			}

			return nil // fetch new template
		}
	}
}
```

Update the import block at the top of `mining/miner.go` to include all needed imports.

**Step 4: Run test to verify it passes**

Run: `go test -race -run "TestMiner_Start" ./mining/ -v -timeout 10s`
Expected: PASS.

**Step 5: Run all mining tests**

Run: `go test -race ./mining/ -v -timeout 30s`
Expected: All PASS.

**Step 6: Commit**

```bash
git add mining/miner.go mining/miner_test.go
git commit -m "feat(mining): mining loop with template fetch and nonce grinding

Start(ctx) blocks until cancelled. Polls daemon for new blocks,
decodes template, computes header hash once, grinds nonces with
RandomX, submits solutions.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 5: Block submission and callback tests

**Files:**
- Modify: `mining/miner_test.go`

**Step 1: Write the test**

Add to `mining/miner_test.go`:

```go
func TestMiner_Start_Good_BlockFound(t *testing.T) {
	// With difficulty=1, every hash is valid — should find a block immediately.
	var foundHeight uint64
	var foundHash types.Hash

	mock := &mockProvider{
		templates: []*rpc.BlockTemplateResponse{
			{Difficulty: "1", Height: 50, BlockTemplateBlob: hex.EncodeToString(minimalBlockBlob(t)), Status: "OK"},
		},
		infos: []*rpc.DaemonInfo{{Height: 50}},
	}

	cfg := Config{
		DaemonURL:    "http://localhost:46941",
		WalletAddr:   "iTHNtestaddr",
		PollInterval: 100 * time.Millisecond,
		Provider:     mock,
		OnBlockFound: func(height uint64, hash types.Hash) {
			foundHeight = height
			foundHash = hash
		},
	}
	m := NewMiner(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start will find a block (diff=1), submit it, then fetch another template.
	// The second template fetch is the same, so it finds another block.
	// Eventually the context times out.
	_ = m.Start(ctx)

	assert.Equal(t, uint64(50), foundHeight)
	assert.False(t, foundHash.IsZero())
	assert.GreaterOrEqual(t, mock.submitCalls.Load(), int64(1))
	assert.GreaterOrEqual(t, m.Stats().BlocksFound, uint64(1))
}

func TestMiner_Start_Good_StatsUpdate(t *testing.T) {
	mock := &mockProvider{
		templates: []*rpc.BlockTemplateResponse{
			{Difficulty: "1", Height: 200, BlockTemplateBlob: hex.EncodeToString(minimalBlockBlob(t)), Status: "OK"},
		},
		infos: []*rpc.DaemonInfo{{Height: 200}},
	}

	cfg := Config{
		DaemonURL:    "http://localhost:46941",
		WalletAddr:   "iTHNtestaddr",
		PollInterval: 100 * time.Millisecond,
		Provider:     mock,
	}
	m := NewMiner(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_ = m.Start(ctx)

	stats := m.Stats()
	assert.Greater(t, stats.Hashrate, float64(0))
	assert.Greater(t, stats.Uptime, time.Duration(0))
}
```

**Step 2: Run tests**

Run: `go test -race -run "TestMiner_Start_Good_Block|TestMiner_Start_Good_Stats" ./mining/ -v -timeout 30s`
Expected: PASS.

**Step 3: Commit**

```bash
git add mining/miner_test.go
git commit -m "test(mining): block submission and stats verification

Tests with difficulty=1 for guaranteed block finding. Verifies
OnBlockFound callback, submit count, hashrate > 0.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 6: Error handling tests

**Files:**
- Modify: `mining/miner_test.go`

**Step 1: Write the tests**

Add to `mining/miner_test.go`:

```go
func TestMiner_Start_Bad_InvalidDifficulty(t *testing.T) {
	mock := &mockProvider{
		templates: []*rpc.BlockTemplateResponse{
			{Difficulty: "not_a_number", Height: 100, BlockTemplateBlob: hex.EncodeToString(minimalBlockBlob(t)), Status: "OK"},
		},
		infos: []*rpc.DaemonInfo{{Height: 100}},
	}

	cfg := Config{
		DaemonURL:    "http://localhost:46941",
		WalletAddr:   "iTHNtestaddr",
		PollInterval: 100 * time.Millisecond,
		Provider:     mock,
	}
	m := NewMiner(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := m.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid difficulty")
}

func TestMiner_Start_Bad_InvalidBlob(t *testing.T) {
	mock := &mockProvider{
		templates: []*rpc.BlockTemplateResponse{
			{Difficulty: "1", Height: 100, BlockTemplateBlob: "not_valid_hex!", Status: "OK"},
		},
		infos: []*rpc.DaemonInfo{{Height: 100}},
	}

	cfg := Config{
		DaemonURL:    "http://localhost:46941",
		WalletAddr:   "iTHNtestaddr",
		PollInterval: 100 * time.Millisecond,
		Provider:     mock,
	}
	m := NewMiner(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := m.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid template blob hex")
}

type failingSubmitter struct {
	mockProvider
}

func (f *failingSubmitter) SubmitBlock(hexBlob string) error {
	return fmt.Errorf("connection refused")
}

func TestMiner_Start_Bad_SubmitFails(t *testing.T) {
	mock := &failingSubmitter{
		mockProvider: mockProvider{
			templates: []*rpc.BlockTemplateResponse{
				{Difficulty: "1", Height: 100, BlockTemplateBlob: hex.EncodeToString(minimalBlockBlob(t)), Status: "OK"},
			},
			infos: []*rpc.DaemonInfo{{Height: 100}},
		},
	}

	cfg := Config{
		DaemonURL:    "http://localhost:46941",
		WalletAddr:   "iTHNtestaddr",
		PollInterval: 100 * time.Millisecond,
		Provider:     mock,
	}
	m := NewMiner(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := m.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "submit block")
}
```

Add `"fmt"` to the import block if not already present.

**Step 2: Run tests**

Run: `go test -race -run "TestMiner_Start_Bad" ./mining/ -v -timeout 30s`
Expected: PASS.

**Step 3: Commit**

```bash
git add mining/miner_test.go
git commit -m "test(mining): error handling for invalid difficulty, blob, and submit

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 7: Full test suite verification

**Files:** None (verification only)

**Step 1: Run all mining tests with race detector**

Run: `go test -race ./mining/ -v -timeout 60s`
Expected: All PASS, no races.

**Step 2: Run go vet**

Run: `go vet ./mining/`
Expected: No warnings.

**Step 3: Run all project tests**

Run: `go test -race ./... -timeout 120s`
Expected: All 11 packages PASS.

**Step 4: Check test coverage**

Run: `go test -coverprofile=mining.cover ./mining/ && go tool cover -func=mining.cover`
Expected: >85% coverage on `mining/` package.

---

### Task 8: Integration test

**Files:**
- Create: `mining/integration_test.go`

**Step 1: Write the integration test**

Create `mining/integration_test.go`:

```go
//go:build integration

// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package mining

import (
	"bytes"
	"encoding/hex"
	"testing"

	"forge.lthn.ai/core/go-blockchain/rpc"
	"forge.lthn.ai/core/go-blockchain/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_GetBlockTemplate(t *testing.T) {
	client := rpc.NewClient("http://localhost:46941")

	// Get daemon info to check it's running.
	info, err := client.GetInfo()
	require.NoError(t, err, "daemon must be running on localhost:46941")
	t.Logf("daemon height: %d, pow_difficulty: %d", info.Height, info.PowDifficulty)

	// Get a block template.
	// Use the testnet genesis coinbase address (all zeros — won't receive real
	// coins but the daemon accepts it for template generation).
	tmpl, err := client.GetBlockTemplate("iTHNtestaddr")
	if err != nil {
		t.Skipf("getblocktemplate failed (may need valid address): %v", err)
	}

	assert.Greater(t, tmpl.Height, uint64(0))
	assert.NotEmpty(t, tmpl.Difficulty)
	assert.NotEmpty(t, tmpl.BlockTemplateBlob)

	// Decode the template blob.
	blobBytes, err := hex.DecodeString(tmpl.BlockTemplateBlob)
	require.NoError(t, err)

	dec := wire.NewDecoder(bytes.NewReader(blobBytes))
	block := wire.DecodeBlock(dec)
	require.NoError(t, dec.Err())

	t.Logf("template: height=%d, major=%d, timestamp=%d, txs=%d",
		tmpl.Height, block.MajorVersion, block.Timestamp, len(block.TxHashes))

	// Compute header mining hash.
	headerHash := HeaderMiningHash(&block)
	t.Logf("header mining hash: %s", hex.EncodeToString(headerHash[:]))

	// Verify the header hash is non-zero.
	assert.False(t, headerHash == [32]byte{})
}
```

**Step 2: Commit (test can't run without daemon)**

```bash
git add mining/integration_test.go
git commit -m "test(mining): integration test against C++ testnet daemon

Build-tagged //go:build integration. Fetches a real template,
decodes it, computes header mining hash.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 9: Update documentation

**Files:**
- Modify: `docs/architecture.md`
- Modify: `docs/history.md`

**Step 1: Update architecture.md**

Add `mining/` to the package structure listing and add a section after `wallet/`.

In the package structure tree, add: `mining/      Solo PoW miner (daemon RPC, RandomX nonce grinding)`.

Add a new section:

```markdown
### mining/

Solo PoW miner that talks to a C++ daemon via JSON-RPC. Single-threaded
mining loop: fetches block templates, computes a header mining hash once
per template, then grinds nonces with RandomX until a solution is found
or the chain advances.

**Core types:**
- `Config` -- daemon URL, wallet address, poll interval, callbacks.
- `Miner` -- mining loop with `Start(ctx)` (blocking) and `Stats()` (lock-free).
- `TemplateProvider` -- interface satisfied by `rpc.Client` for testability.

**Mining flow:**
1. `GetBlockTemplate(walletAddr)` -- fetches template from daemon.
2. `HeaderMiningHash(block)` -- Keccak-256 of `BlockHashingBlob` with nonce=0.
3. Nonce loop: `RandomXHash("LetheanRandomXv1", headerHash || nonce_LE)`.
4. `CheckDifficulty(powHash, difficulty)` -- solution found?
5. `SubmitBlock(hexBlob)` -- submits the solved block.

**Template refresh:** polls `GetInfo()` every `PollInterval` (default 3s) to
detect new blocks. Re-fetches the template when the chain height advances.

**Testing:** Mock `TemplateProvider` for unit tests. Build-tagged integration
test against C++ testnet daemon on `localhost:46941`.
```

**Step 2: Update history.md**

Replace the `## Phase 8 -- Mining (Planned)` section with the completed record (fill in
commit range and details after all tests pass).

**Step 3: Update rpc/ section in architecture.md**

In the rpc/ section, update the mining line from:
```
- `mining.go` -- `SubmitBlock`.
```
to:
```
- `mining.go` -- `GetBlockTemplate`, `SubmitBlock`.
```

And update the endpoint count from "10 core daemon endpoints" to "11 core daemon endpoints".

**Step 4: Commit**

```bash
git add docs/architecture.md docs/history.md
git commit -m "docs: Phase 8 mining complete

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 10: Final verification

**Step 1: Run all tests with race detector**

Run: `go test -race ./... -timeout 120s`
Expected: All packages PASS.

**Step 2: Run go vet**

Run: `go vet ./...`
Expected: No warnings.

**Step 3: Check mining coverage**

Run: `go test -coverprofile=mining.cover ./mining/ -timeout 60s && go tool cover -func=mining.cover`
Expected: >85%.

**Step 4: Verify commit log**

Run: `git log --oneline -10`
Expected: Clean sequence of mining commits.
