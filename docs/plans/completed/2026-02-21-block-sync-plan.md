# Block Sync Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Wire up NLSAG ring signature verification, sync the full chain via RPC, then replace RPC transport with native P2P block sync.

**Architecture:** Three layers built bottom-up. Signature verification wires the existing CGo bridge (`crypto.CheckRingSignature`) into the consensus path via a `RingOutputsFn` callback that fetches public keys from chain storage. RPC sync uses the existing `chain.Sync()` with bug fixes. P2P sync adds Levin command types (2003/2004) and a state machine that talks directly to peers.

**Tech Stack:** Go stdlib, `forge.lthn.ai/core/go-p2p/node/levin`, `forge.lthn.ai/core/go-blockchain/crypto` (CGo), `forge.lthn.ai/core/go-store` (SQLite).

---

## Task 1: NLSAG Ring Signature Verification

Wire up `crypto.CheckRingSignature()` in `consensus/verify.go` so pre-HF4
transactions have their ring signatures cryptographically verified.

**Files:**
- Modify: `consensus/verify.go:43-65`
- Test: `consensus/verify_test.go`

**Step 1: Write the failing test**

Add to `consensus/verify_test.go`:

```go
func TestVerifyV1Signatures_Good_MockRing(t *testing.T) {
	// Generate a key pair and sign a fake transaction.
	pub, sec, err := crypto.GenerateKeys()
	require.NoError(t, err)

	ki, err := crypto.GenerateKeyImage(pub, sec)
	require.NoError(t, err)

	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{
				Amount: 100,
				KeyOffsets: []types.TxOutRef{
					{Tag: types.RefTypeGlobalIndex, GlobalIndex: 0},
				},
				KeyImage: ki,
			},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 90, Target: types.TxOutToKey{Key: pub}},
		},
	}

	prefixHash := wire.TransactionPrefixHash(tx)

	// Generate a ring signature with ring size 1 (just our key).
	sigs, err := crypto.GenerateRingSignature(
		[32]byte(prefixHash), [32]byte(ki), [][32]byte{pub}, sec, 0)
	require.NoError(t, err)

	// Convert to types.Signature.
	tx.Signatures = [][]types.Signature{make([]types.Signature, 1)}
	tx.Signatures[0][0] = types.Signature(sigs[0])

	// Provide a mock getRingOutputs that returns our public key.
	getRing := func(amount uint64, offsets []uint64) ([]types.PublicKey, error) {
		return []types.PublicKey{types.PublicKey(pub)}, nil
	}

	err = VerifyTransactionSignatures(tx, config.MainnetForks, 100, getRing)
	require.NoError(t, err)
}

func TestVerifyV1Signatures_Bad_WrongSig(t *testing.T) {
	pub, sec, err := crypto.GenerateKeys()
	require.NoError(t, err)

	ki, err := crypto.GenerateKeyImage(pub, sec)
	require.NoError(t, err)

	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{
				Amount:   100,
				KeyOffsets: []types.TxOutRef{
					{Tag: types.RefTypeGlobalIndex, GlobalIndex: 0},
				},
				KeyImage: ki,
			},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 90, Target: types.TxOutToKey{Key: pub}},
		},
		Signatures: [][]types.Signature{
			{types.Signature{}}, // zeroed signature — should fail
		},
	}

	getRing := func(amount uint64, offsets []uint64) ([]types.PublicKey, error) {
		return []types.PublicKey{types.PublicKey(pub)}, nil
	}

	err = VerifyTransactionSignatures(tx, config.MainnetForks, 100, getRing)
	assert.Error(t, err)
}
```

Note: the `!integration` build tag is already on `verify_test.go`. These tests
use the CGo crypto bridge, so remove `!integration` or add a separate file.
Since the unit test suite uses `!integration`, put these in a new file:
`consensus/verify_crypto_test.go` (no build tag — always runs when CGo is
available, which it always is in this project).

**Step 2: Run tests to verify they fail**

Run: `go test -v -run TestVerifyV1Signatures ./consensus/`
Expected: FAIL — `verifyV1Signatures` returns nil without calling crypto.

**Step 3: Implement verifyV1Signatures**

Replace the TODO in `consensus/verify.go`:

```go
func verifyV1Signatures(tx *types.Transaction, getRingOutputs RingOutputsFn) error {
	// Count key inputs.
	var keyInputCount int
	for _, vin := range tx.Vin {
		if _, ok := vin.(types.TxInputToKey); ok {
			keyInputCount++
		}
	}

	if len(tx.Signatures) != keyInputCount {
		return fmt.Errorf("consensus: signature count %d != input count %d",
			len(tx.Signatures), keyInputCount)
	}

	if getRingOutputs == nil {
		return nil
	}

	prefixHash := wire.TransactionPrefixHash(tx)

	sigIdx := 0
	for _, vin := range tx.Vin {
		toKey, ok := vin.(types.TxInputToKey)
		if !ok {
			continue
		}

		// Extract absolute global indexes from key offsets.
		offsets := make([]uint64, len(toKey.KeyOffsets))
		for i, ref := range toKey.KeyOffsets {
			if ref.Tag != types.RefTypeGlobalIndex {
				return fmt.Errorf("consensus: input %d: unsupported ref tag 0x%02x", sigIdx, ref.Tag)
			}
			offsets[i] = ref.GlobalIndex
		}

		// Fetch ring member public keys.
		ringPubs, err := getRingOutputs(toKey.Amount, offsets)
		if err != nil {
			return fmt.Errorf("consensus: input %d: fetch ring outputs: %w", sigIdx, err)
		}

		if len(ringPubs) != len(toKey.KeyOffsets) {
			return fmt.Errorf("consensus: input %d: ring size %d != offset count %d",
				sigIdx, len(ringPubs), len(toKey.KeyOffsets))
		}

		// Convert to crypto types.
		pubs := make([][32]byte, len(ringPubs))
		for i, p := range ringPubs {
			pubs[i] = [32]byte(p)
		}

		sigs := make([][64]byte, len(tx.Signatures[sigIdx]))
		for i, s := range tx.Signatures[sigIdx] {
			sigs[i] = [64]byte(s)
		}

		if !crypto.CheckRingSignature([32]byte(prefixHash), [32]byte(toKey.KeyImage), pubs, sigs) {
			return fmt.Errorf("consensus: input %d: ring signature verification failed", sigIdx)
		}

		sigIdx++
	}

	return nil
}
```

New imports needed in `consensus/verify.go`:
```go
"forge.lthn.ai/core/go-blockchain/crypto"
"forge.lthn.ai/core/go-blockchain/wire"
```

**Step 4: Run tests to verify they pass**

Run: `go test -v -run TestVerifyV1Signatures ./consensus/`
Expected: PASS

**Step 5: Run full test suite**

Run: `go test -race ./...`
Expected: All pass.

**Step 6: Commit**

```bash
git add consensus/verify.go consensus/verify_crypto_test.go
git commit -m "feat(consensus): wire up NLSAG ring signature verification

Connect crypto.CheckRingSignature() to verifyV1Signatures() so
pre-HF4 transactions have their ring signatures cryptographically
verified when a RingOutputsFn callback is provided.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## Task 2: Chain RingOutputsFn Callback

Provide a `getRingOutputs` implementation in `chain/` that looks up public keys
from the output index and stored transactions.

**Files:**
- Modify: `chain/sync.go:170-173` (pass real callback instead of nil)
- Create: `chain/ring.go`
- Test: `chain/ring_test.go`

**Step 1: Write the failing test**

Create `chain/ring_test.go`:

```go
package chain

import (
	"testing"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
	"github.com/stretchr/testify/require"
)

func TestGetRingOutputs_Good(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)

	// Create a transaction with a known output.
	pubKey := types.PublicKey{1, 2, 3}
	tx := types.Transaction{
		Version: types.VersionPreHF4,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 0}},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 1000,
				Target: types.TxOutToKey{Key: pubKey, MixAttr: 0},
			},
		},
	}
	txHash := wire.TransactionHash(&tx)

	// Store the transaction.
	err = c.PutTransaction(txHash, &tx, &TxMeta{KeeperBlock: 0, GlobalOutputIndexes: []uint64{0}})
	require.NoError(t, err)

	// Index the output at global index 0 for amount 1000.
	gidx, err := c.PutOutput(1000, txHash, 0)
	require.NoError(t, err)
	require.Equal(t, uint64(0), gidx)

	// Fetch ring outputs.
	pubs, err := c.GetRingOutputs(1000, []uint64{0})
	require.NoError(t, err)
	require.Len(t, pubs, 1)
	require.Equal(t, pubKey, pubs[0])
}

func TestGetRingOutputs_Bad_NotFound(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	_, err = c.GetRingOutputs(1000, []uint64{99})
	require.Error(t, err)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -v -run TestGetRingOutputs ./chain/`
Expected: FAIL — `GetRingOutputs` does not exist.

**Step 3: Implement GetRingOutputs**

Create `chain/ring.go`:

```go
package chain

import (
	"fmt"

	"forge.lthn.ai/core/go-blockchain/types"
)

// GetRingOutputs fetches the public keys for the given global output indexes
// at the specified amount. This implements the consensus.RingOutputsFn
// signature for use during signature verification.
func (c *Chain) GetRingOutputs(amount uint64, offsets []uint64) ([]types.PublicKey, error) {
	pubs := make([]types.PublicKey, len(offsets))
	for i, gidx := range offsets {
		txHash, outNo, err := c.GetOutput(amount, gidx)
		if err != nil {
			return nil, fmt.Errorf("ring output %d (amount=%d, gidx=%d): %w", i, amount, gidx, err)
		}

		tx, _, err := c.GetTransaction(txHash)
		if err != nil {
			return nil, fmt.Errorf("ring output %d: tx %s: %w", i, txHash, err)
		}

		if int(outNo) >= len(tx.Vout) {
			return nil, fmt.Errorf("ring output %d: tx %s has %d outputs, want index %d",
				i, txHash, len(tx.Vout), outNo)
		}

		switch out := tx.Vout[outNo].(type) {
		case types.TxOutputBare:
			pubs[i] = out.Target.Key
		default:
			return nil, fmt.Errorf("ring output %d: unsupported output type %T", i, out)
		}
	}
	return pubs, nil
}
```

**Step 4: Wire into sync.go**

In `chain/sync.go`, change the `VerifySignatures` block (line ~170-173) from
passing `nil` to passing `c.GetRingOutputs`:

```go
		if opts.VerifySignatures {
			if err := consensus.VerifyTransactionSignatures(&tx, opts.Forks, bd.Height, c.GetRingOutputs); err != nil {
				return fmt.Errorf("verify tx signatures %s: %w", txInfo.ID, err)
			}
		}
```

**Step 5: Run tests to verify they pass**

Run: `go test -v -run TestGetRingOutputs ./chain/`
Expected: PASS

**Step 6: Run full test suite**

Run: `go test -race ./...`
Expected: All pass.

**Step 7: Commit**

```bash
git add chain/ring.go chain/ring_test.go chain/sync.go
git commit -m "feat(chain): add GetRingOutputs callback for signature verification

Implements consensus.RingOutputsFn by looking up output public keys from
the chain's global output index and transaction store. Wired into the
sync loop so VerifySignatures=true uses real crypto verification.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## Task 3: Context Cancellation and Progress Logging

Add `context.Context` to `chain.Sync()` for graceful shutdown and progress
logging during long syncs.

**Files:**
- Modify: `chain/sync.go`
- Modify: `chain/sync_test.go` (update callers)
- Modify: `chain/integration_test.go` (update callers)

**Step 1: Update Sync signature**

In `chain/sync.go`, change:

```go
func (c *Chain) Sync(client *rpc.Client, opts SyncOptions) error {
```

to:

```go
func (c *Chain) Sync(ctx context.Context, client *rpc.Client, opts SyncOptions) error {
```

Add context check in the main loop:

```go
	for localHeight < remoteHeight {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// ... rest of loop
	}
```

Add progress logging every 100 blocks:

```go
func (c *Chain) processBlock(bd rpc.BlockDetails, opts SyncOptions) error {
	if bd.Height > 0 && bd.Height%100 == 0 {
		log.Printf("sync: processing block %d", bd.Height)
	}
	// ... rest unchanged
```

Add `"context"` and `"log"` imports.

**Step 2: Update all callers**

In `chain/sync_test.go`, update mock sync calls to pass `context.Background()`.
In `chain/integration_test.go`, same.

**Step 3: Run tests**

Run: `go test -race ./chain/`
Expected: All pass.

**Step 4: Commit**

```bash
git add chain/sync.go chain/sync_test.go chain/integration_test.go
git commit -m "feat(chain): add context cancellation and progress logging to Sync

Sync() now accepts context.Context for graceful shutdown. Logs progress
every 100 blocks.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## Task 4: Handle TxInputZC Key Images

The `processBlock` function in `chain/sync.go` only marks key images for
`TxInputToKey` inputs. V2+ blocks contain `TxInputZC` inputs that also have
key images. Fix this.

**Files:**
- Modify: `chain/sync.go:188-195`
- Modify: `chain/sync_test.go`

**Step 1: Write the failing test**

Add to `chain/sync_test.go` a test that processes a block with a `TxInputZC`
and verifies its key image is marked as spent.

```go
func TestProcessBlock_ZCInputKeyImage(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)

	// ... set up genesis block first (reuse existing helper)
	// ... then create block 1 with a TxInputZC input
	// ... verify MarkSpent was called for the ZC key image
}
```

(The exact test shape depends on existing test helpers — adapt to the mock
RPC pattern already in `sync_test.go`.)

**Step 2: Fix the key image loop**

In `chain/sync.go`, extend the key image marking loop:

```go
		for _, vin := range tx.Vin {
			switch inp := vin.(type) {
			case types.TxInputToKey:
				if err := c.MarkSpent(inp.KeyImage, bd.Height); err != nil {
					return fmt.Errorf("mark spent %s: %w", inp.KeyImage, err)
				}
			case types.TxInputZC:
				if err := c.MarkSpent(inp.KeyImage, bd.Height); err != nil {
					return fmt.Errorf("mark spent %s: %w", inp.KeyImage, err)
				}
			}
		}
```

**Step 3: Run tests**

Run: `go test -race ./chain/`
Expected: All pass.

**Step 4: Commit**

```bash
git add chain/sync.go chain/sync_test.go
git commit -m "fix(chain): mark ZC input key images as spent during sync

TxInputZC (v2+) inputs have key images that must be tracked for
double-spend detection, same as TxInputToKey.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## Task 5: Full RPC Sync Integration Test

Run the RPC sync against the testnet daemon all the way to tip. This is the
end-to-end validation that wire decoding, consensus, and storage work
together for the full chain.

**Files:**
- Modify: `chain/integration_test.go`

**Step 1: Add sync-to-tip test**

```go
func TestIntegration_SyncToTip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long sync test in short mode")
	}

	client := rpc.NewClientWithHTTP(testnetRPCAddr, &http.Client{Timeout: 60 * time.Second})

	remoteHeight, err := client.GetHeight()
	if err != nil {
		t.Skipf("testnet daemon not reachable at %s: %v", testnetRPCAddr, err)
	}
	t.Logf("testnet height: %d", remoteHeight)

	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)

	opts := SyncOptions{
		VerifySignatures: false, // first pass: no sigs
		Forks:            config.TestnetForks,
	}

	err = c.Sync(context.Background(), client, opts)
	require.NoError(t, err)

	finalHeight, _ := c.Height()
	t.Logf("synced %d blocks", finalHeight)
	require.Equal(t, remoteHeight, finalHeight)

	// Verify genesis.
	_, genMeta, err := c.GetBlockByHeight(0)
	require.NoError(t, err)
	expectedHash, _ := types.HashFromHex(GenesisHash)
	require.Equal(t, expectedHash, genMeta.Hash)
}
```

**Step 2: Run it**

Run: `go test -tags integration -v -run TestIntegration_SyncToTip ./chain/ -timeout 300s`

Fix any wire decode or validation errors that surface. Common issues:
- Unknown extra variant tags (should be handled by raw byte skipping)
- Output amount mismatches for Zarcanum transactions
- Block size validation too strict

**Step 3: If errors surface, fix and re-run**

Each fix gets its own test + commit. Iterate until the full chain syncs clean.

**Step 4: Commit**

```bash
git add chain/integration_test.go
git commit -m "test(chain): add full sync-to-tip integration test

Syncs the entire testnet chain via RPC and verifies every block
decodes, validates, and stores correctly.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## Task 6: RPC Sync with Signature Verification

After Task 5 passes, re-run with `VerifySignatures: true` to prove the
NLSAG verification works on real chain data.

**Files:**
- Modify: `chain/integration_test.go`

**Step 1: Add verified sync test**

```go
func TestIntegration_SyncWithSignatures(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long sync test in short mode")
	}

	client := rpc.NewClientWithHTTP(testnetRPCAddr, &http.Client{Timeout: 60 * time.Second})

	remoteHeight, err := client.GetHeight()
	if err != nil {
		t.Skipf("testnet daemon not reachable: %v", err)
	}

	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)

	opts := SyncOptions{
		VerifySignatures: true,
		Forks:            config.TestnetForks,
	}

	err = c.Sync(context.Background(), client, opts)
	require.NoError(t, err)

	finalHeight, _ := c.Height()
	t.Logf("synced %d blocks with signature verification", finalHeight)
	require.Equal(t, remoteHeight, finalHeight)
}
```

Note: this test may be slow (crypto verification for every spending tx). The
testnet has few spending transactions, so it should complete in reasonable time.
If it fails, it means the ring output lookup or signature verification has a bug.

**Step 2: Run and fix**

Run: `go test -tags integration -v -run TestIntegration_SyncWithSignatures ./chain/ -timeout 600s`

**Step 3: Commit**

```bash
git add chain/integration_test.go
git commit -m "test(chain): verify ring signatures during full chain sync

All pre-HF4 spending transactions on testnet pass NLSAG ring
signature verification end-to-end.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## Task 7: P2P RequestGetObjects / ResponseGetObjects Types

Add the missing P2P command types for block fetching (2003/2004).

**Files:**
- Modify: `p2p/relay.go` (add new types alongside existing ones)
- Test: `p2p/relay_test.go` (add round-trip tests)

**Step 1: Write the failing test**

Add to `p2p/relay_test.go`:

```go
func TestRequestGetObjects_RoundTrip(t *testing.T) {
	req := RequestGetObjects{
		Blocks: [][]byte{
			make([]byte, 32), // zero hash
			{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
				17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
		},
	}
	data, err := req.Encode()
	require.NoError(t, err)

	var decoded RequestGetObjects
	err = decoded.Decode(data)
	require.NoError(t, err)
	require.Len(t, decoded.Blocks, 2)
	require.Equal(t, req.Blocks[1], decoded.Blocks[1])
}

func TestResponseGetObjects_Decode(t *testing.T) {
	// Build a minimal response with one block entry.
	blockEntry := levin.Section{
		"block": levin.StringVal([]byte{0x01, 0x02}),
		"txs":   levin.StringArrayVal([][]byte{{0x03, 0x04}}),
	}
	s := levin.Section{
		"blocks":                       levin.ObjectArrayVal([]levin.Section{blockEntry}),
		"missed_ids":                   levin.StringArrayVal(nil),
		"current_blockchain_height":    levin.Uint64Val(100),
	}
	data, err := levin.EncodeStorage(s)
	require.NoError(t, err)

	var resp ResponseGetObjects
	err = resp.Decode(data)
	require.NoError(t, err)
	require.Len(t, resp.Blocks, 1)
	require.Equal(t, []byte{0x01, 0x02}, resp.Blocks[0].Block)
	require.Len(t, resp.Blocks[0].Txs, 1)
	require.Equal(t, uint64(100), resp.CurrentHeight)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -v -run TestRequestGetObjects ./p2p/`
Expected: FAIL — types don't exist.

**Step 3: Implement the types**

Add to `p2p/relay.go`:

```go
// BlockCompleteEntry holds a block blob and its transaction blobs.
type BlockCompleteEntry struct {
	Block []byte
	Txs   [][]byte
}

// RequestGetObjects is NOTIFY_REQUEST_GET_OBJECTS (2003).
type RequestGetObjects struct {
	Blocks [][]byte // 32-byte block hashes
	Txs    [][]byte // 32-byte tx hashes (usually empty for sync)
}

// Encode serialises the request.
func (r *RequestGetObjects) Encode() ([]byte, error) {
	s := levin.Section{
		"blocks": levin.StringArrayVal(r.Blocks),
	}
	if len(r.Txs) > 0 {
		s["txs"] = levin.StringArrayVal(r.Txs)
	}
	return levin.EncodeStorage(s)
}

// Decode parses a request from storage bytes.
func (r *RequestGetObjects) Decode(data []byte) error {
	s, err := levin.DecodeStorage(data)
	if err != nil {
		return err
	}
	if v, ok := s["blocks"]; ok {
		r.Blocks, _ = v.AsStringArray()
	}
	if v, ok := s["txs"]; ok {
		r.Txs, _ = v.AsStringArray()
	}
	return nil
}

// ResponseGetObjects is NOTIFY_RESPONSE_GET_OBJECTS (2004).
type ResponseGetObjects struct {
	Blocks        []BlockCompleteEntry
	MissedIDs     [][]byte
	CurrentHeight uint64
}

// Decode parses a response from storage bytes.
func (r *ResponseGetObjects) Decode(data []byte) error {
	s, err := levin.DecodeStorage(data)
	if err != nil {
		return err
	}
	if v, ok := s["current_blockchain_height"]; ok {
		r.CurrentHeight, _ = v.AsUint64()
	}
	if v, ok := s["missed_ids"]; ok {
		r.MissedIDs, _ = v.AsStringArray()
	}
	if v, ok := s["blocks"]; ok {
		entries, _ := v.AsObjectArray()
		r.Blocks = make([]BlockCompleteEntry, len(entries))
		for i, entry := range entries {
			if blk, ok := entry["block"]; ok {
				r.Blocks[i].Block, _ = blk.AsString()
			}
			if txs, ok := entry["txs"]; ok {
				r.Blocks[i].Txs, _ = txs.AsStringArray()
			}
		}
	}
	return nil
}
```

Note: Check if `levin.ObjectArrayVal` and `v.AsObjectArray()` exist. If not,
these need to be added to go-p2p. The C++ daemon sends `blocks` as a
`vector<block_complete_entry>` which serialises as an array of objects in
portable storage. If `AsObjectArray` doesn't exist, use the raw section
parsing approach instead.

**Step 4: Run tests to verify they pass**

Run: `go test -v -run "TestRequestGetObjects|TestResponseGetObjects" ./p2p/`
Expected: PASS

**Step 5: Run full test suite**

Run: `go test -race ./...`
Expected: All pass.

**Step 6: Commit**

```bash
git add p2p/relay.go p2p/relay_test.go
git commit -m "feat(p2p): add RequestGetObjects and ResponseGetObjects types

Encode/decode for NOTIFY_REQUEST_GET_OBJECTS (2003) and
NOTIFY_RESPONSE_GET_OBJECTS (2004), including BlockCompleteEntry
for block + transaction blob pairs.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## Task 8: P2P Integration Test — Request Chain + Get Objects

Verify the new P2P types work against the real testnet daemon by performing a
chain request and block fetch.

**Files:**
- Modify: `p2p/integration_test.go`

**Step 1: Add chain sync integration test**

```go
func TestIntegration_RequestChainAndGetObjects(t *testing.T) {
	conn, err := net.DialTimeout("tcp", testnetP2PAddr, 10*time.Second)
	if err != nil {
		t.Skipf("testnet daemon not reachable: %v", err)
	}
	defer conn.Close()

	lc := levin.NewConnection(conn)

	// --- Handshake first ---
	// (reuse handshake code from TestIntegration_Handshake)
	var peerIDBuf [8]byte
	rand.Read(peerIDBuf[:])
	peerID := binary.LittleEndian.Uint64(peerIDBuf[:])

	req := HandshakeRequest{
		NodeData: NodeData{
			NetworkID: config.NetworkIDTestnet,
			PeerID:    peerID,
			LocalTime: time.Now().Unix(),
			MyPort:    0,
		},
		PayloadData: CoreSyncData{
			CurrentHeight:  1,
			ClientVersion:  config.ClientVersion,
			NonPruningMode: true,
		},
	}
	payload, err := EncodeHandshakeRequest(&req)
	require.NoError(t, err)
	require.NoError(t, lc.WritePacket(CommandHandshake, payload, true))

	hdr, data, err := lc.ReadPacket()
	require.NoError(t, err)
	require.Equal(t, uint32(CommandHandshake), hdr.Command)

	// --- Request chain ---
	genesisHash, _ := hex.DecodeString("cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963")
	chainReq := RequestChain{
		BlockIDs: [][]byte{genesisHash},
	}
	chainPayload, err := chainReq.Encode()
	require.NoError(t, err)
	require.NoError(t, lc.WritePacket(CommandRequestChain, chainPayload, false))

	// Read response (notification, not request-response).
	hdr, data, err = lc.ReadPacket()
	require.NoError(t, err)
	t.Logf("got command %d", hdr.Command)

	// The daemon may send timed_sync or other messages first — read until
	// we get RESPONSE_CHAIN_ENTRY.
	for hdr.Command != CommandResponseChain {
		hdr, data, err = lc.ReadPacket()
		require.NoError(t, err)
		t.Logf("skipping command %d", hdr.Command)
	}

	var chainResp ResponseChainEntry
	require.NoError(t, chainResp.Decode(data))
	t.Logf("chain response: start=%d, total=%d, block_ids=%d",
		chainResp.StartHeight, chainResp.TotalHeight, len(chainResp.BlockIDs))
	require.Greater(t, len(chainResp.BlockIDs), 0)

	// --- Request first block ---
	// BlockIDs in the response are packed 32-byte hashes.
	firstHash := chainResp.BlockIDs[0]
	if len(firstHash) < 32 {
		t.Fatalf("block hash too short: %d bytes", len(firstHash))
	}

	getReq := RequestGetObjects{
		Blocks: [][]byte{firstHash[:32]},
	}
	getPayload, err := getReq.Encode()
	require.NoError(t, err)
	require.NoError(t, lc.WritePacket(CommandRequestObjects, getPayload, false))

	// Read until RESPONSE_GET_OBJECTS.
	for {
		hdr, data, err = lc.ReadPacket()
		require.NoError(t, err)
		if hdr.Command == CommandResponseObjects {
			break
		}
		t.Logf("skipping command %d", hdr.Command)
	}

	var getResp ResponseGetObjects
	require.NoError(t, getResp.Decode(data))
	t.Logf("get_objects response: %d blocks, %d missed, height=%d",
		len(getResp.Blocks), len(getResp.MissedIDs), getResp.CurrentHeight)
	require.Len(t, getResp.Blocks, 1)
	require.Greater(t, len(getResp.Blocks[0].Block), 0)
	t.Logf("block blob: %d bytes", len(getResp.Blocks[0].Block))
}
```

**Step 2: Run it**

Run: `go test -tags integration -v -run TestIntegration_RequestChainAndGetObjects ./p2p/ -timeout 30s`

Fix any decode issues.

**Step 3: Commit**

```bash
git add p2p/integration_test.go
git commit -m "test(p2p): integration test for chain request and block fetch

Performs handshake, REQUEST_CHAIN, RESPONSE_CHAIN_ENTRY, then
REQUEST_GET_OBJECTS and RESPONSE_GET_OBJECTS against testnet daemon.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## Task 9: Sparse Chain History Builder

Implement the sparse chain history builder used by REQUEST_CHAIN. This builds
the exponentially-spaced block hash list that the C++ daemon expects.

**Files:**
- Create: `chain/history.go`
- Test: `chain/history_test.go`

**Step 1: Write the failing test**

Create `chain/history_test.go`:

```go
package chain

import (
	"testing"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/types"
	"github.com/stretchr/testify/require"
)

func TestSparseChainHistory_Empty(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	history, err := c.SparseChainHistory()
	require.NoError(t, err)
	require.Len(t, history, 1) // just the zero hash (genesis placeholder)
}

func TestSparseChainHistory_FewBlocks(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	// Store 5 blocks with known hashes.
	for i := uint64(0); i < 5; i++ {
		hash := types.Hash{byte(i + 1)}
		blk := &types.Block{}
		if i > 0 {
			blk.PrevID = types.Hash{byte(i)}
		}
		err := c.PutBlock(blk, &BlockMeta{Hash: hash, Height: i})
		require.NoError(t, err)
	}

	history, err := c.SparseChainHistory()
	require.NoError(t, err)

	// With 5 blocks (heights 0-4), should include recent blocks first,
	// then exponentially spaced, ending with genesis.
	require.Greater(t, len(history), 0)

	// Last entry should be genesis hash.
	require.Equal(t, types.Hash{1}, history[len(history)-1])

	// First entry should be top block hash.
	require.Equal(t, types.Hash{5}, history[0])
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -v -run TestSparseChainHistory ./chain/`
Expected: FAIL — function doesn't exist.

**Step 3: Implement SparseChainHistory**

Create `chain/history.go`:

```go
package chain

import "forge.lthn.ai/core/go-blockchain/types"

// SparseChainHistory builds the exponentially-spaced block hash list used by
// NOTIFY_REQUEST_CHAIN. Matches the C++ get_short_chain_history() algorithm:
// first 10 block hashes from the tip, then exponentially larger steps back
// to genesis.
func (c *Chain) SparseChainHistory() ([]types.Hash, error) {
	height, err := c.Height()
	if err != nil {
		return nil, err
	}

	if height == 0 {
		return []types.Hash{{}}, nil // zero hash placeholder
	}

	var hashes []types.Hash
	step := uint64(1)
	current := height - 1 // top block height

	for {
		_, meta, err := c.GetBlockByHeight(current)
		if err != nil {
			break
		}
		hashes = append(hashes, meta.Hash)

		if current == 0 {
			break
		}

		// First 10 entries: step=1, then double each time.
		if len(hashes) >= 10 {
			step *= 2
		}

		if current < step {
			if current > 0 {
				current = 0
				continue
			}
			break
		}
		current -= step
	}

	return hashes, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -v -run TestSparseChainHistory ./chain/`
Expected: PASS

**Step 5: Commit**

```bash
git add chain/history.go chain/history_test.go
git commit -m "feat(chain): add sparse chain history builder for P2P sync

Implements get_short_chain_history() algorithm: recent 10 block hashes
then exponentially-spaced hashes back to genesis.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## Task 10: Refactor processBlock to Accept Raw Blobs

Shared block processing that works for both RPC and P2P sync paths.

**Files:**
- Modify: `chain/sync.go`
- Test: existing tests must still pass

**Step 1: Extract blob processing**

Create a new method `processBlockBlobs` in `chain/sync.go` that takes raw
bytes instead of `rpc.BlockDetails`:

```go
// processBlockBlobs validates and stores a block from raw wire blobs.
// This is the shared processing path for both RPC and P2P sync.
func (c *Chain) processBlockBlobs(blockBlob []byte, txBlobs [][]byte,
	height uint64, difficulty uint64, opts SyncOptions) error {

	dec := wire.NewDecoder(bytes.NewReader(blockBlob))
	blk := wire.DecodeBlock(dec)
	if err := dec.Err(); err != nil {
		return fmt.Errorf("decode block wire: %w", err)
	}

	// Compute block hash.
	blockHash := wire.BlockHash(&blk)

	// Genesis chain identity check.
	if height == 0 {
		expected, _ := types.HashFromHex(GenesisHash)
		if blockHash != expected {
			return fmt.Errorf("genesis hash %s does not match expected %s",
				blockHash, expected)
		}
	}

	// Validate header.
	if err := c.ValidateHeader(&blk, height); err != nil {
		return err
	}

	// Validate miner transaction structure.
	if err := consensus.ValidateMinerTx(&blk.MinerTx, height, opts.Forks); err != nil {
		return fmt.Errorf("validate miner tx: %w", err)
	}

	// Calculate cumulative difficulty.
	var cumulDiff uint64
	if height > 0 {
		_, prevMeta, err := c.TopBlock()
		if err != nil {
			return fmt.Errorf("get prev block meta: %w", err)
		}
		cumulDiff = prevMeta.CumulativeDiff + difficulty
	} else {
		cumulDiff = difficulty
	}

	// Store miner transaction.
	minerTxHash := wire.TransactionHash(&blk.MinerTx)
	minerGindexes, err := c.indexOutputs(minerTxHash, &blk.MinerTx)
	if err != nil {
		return fmt.Errorf("index miner tx outputs: %w", err)
	}
	if err := c.PutTransaction(minerTxHash, &blk.MinerTx, &TxMeta{
		KeeperBlock:         height,
		GlobalOutputIndexes: minerGindexes,
	}); err != nil {
		return fmt.Errorf("store miner tx: %w", err)
	}

	// Process regular transactions from blobs.
	for i, txBlobBytes := range txBlobs {
		txDec := wire.NewDecoder(bytes.NewReader(txBlobBytes))
		tx := wire.DecodeTransaction(txDec)
		if err := txDec.Err(); err != nil {
			return fmt.Errorf("decode tx %d: %w", i, err)
		}

		if err := consensus.ValidateTransaction(&tx, txBlobBytes, opts.Forks, height); err != nil {
			return fmt.Errorf("validate tx %d: %w", i, err)
		}

		if opts.VerifySignatures {
			if err := consensus.VerifyTransactionSignatures(&tx, opts.Forks, height, c.GetRingOutputs); err != nil {
				return fmt.Errorf("verify tx %d signatures: %w", i, err)
			}
		}

		txHash := wire.TransactionHash(&tx)

		gindexes, err := c.indexOutputs(txHash, &tx)
		if err != nil {
			return fmt.Errorf("index tx %d outputs: %w", i, err)
		}

		for _, vin := range tx.Vin {
			switch inp := vin.(type) {
			case types.TxInputToKey:
				if err := c.MarkSpent(inp.KeyImage, height); err != nil {
					return fmt.Errorf("mark spent: %w", err)
				}
			case types.TxInputZC:
				if err := c.MarkSpent(inp.KeyImage, height); err != nil {
					return fmt.Errorf("mark spent: %w", err)
				}
			}
		}

		if err := c.PutTransaction(txHash, &tx, &TxMeta{
			KeeperBlock:         height,
			GlobalOutputIndexes: gindexes,
		}); err != nil {
			return fmt.Errorf("store tx %d: %w", i, err)
		}
	}

	// Store block.
	meta := &BlockMeta{
		Hash:           blockHash,
		Height:         height,
		Timestamp:      blk.Timestamp,
		Difficulty:     difficulty,
		CumulativeDiff: cumulDiff,
	}
	return c.PutBlock(&blk, meta)
}
```

Then refactor `processBlock` (the RPC path) to call `processBlockBlobs`:

```go
func (c *Chain) processBlock(bd rpc.BlockDetails, opts SyncOptions) error {
	blockBlob, err := hex.DecodeString(bd.Blob)
	if err != nil {
		return fmt.Errorf("decode block hex: %w", err)
	}

	var txBlobs [][]byte
	for _, txInfo := range bd.Transactions {
		txBlob, err := hex.DecodeString(txInfo.Blob)
		if err != nil {
			return fmt.Errorf("decode tx hex %s: %w", txInfo.ID, err)
		}
		txBlobs = append(txBlobs, txBlob)
	}

	diff, _ := strconv.ParseUint(bd.Difficulty, 10, 64)
	return c.processBlockBlobs(blockBlob, txBlobs, bd.Height, diff, opts)
}
```

Note: The RPC path previously verified block hash against `bd.ID` and used
`bd.BaseReward` for GeneratedCoins. The blob path computes the hash itself.
The P2P path doesn't provide difficulty or base reward — these must be
computed locally (difficulty from the LWMA algorithm, rewards from
`consensus.BlockReward`). For now, pass difficulty=0 from P2P and compute
it properly when needed. Mark `GeneratedCoins` as TODO for P2P path.

**Step 2: Run all tests**

Run: `go test -race ./chain/`
Expected: All existing tests still pass.

**Step 3: Commit**

```bash
git add chain/sync.go
git commit -m "refactor(chain): extract processBlockBlobs for shared RPC/P2P use

The block processing logic is now in processBlockBlobs() which takes
raw wire bytes. The RPC processBlock() becomes a thin hex-decode
wrapper. The P2P sync path will call processBlockBlobs() directly.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## Task 11: P2P Sync State Machine

Implement the P2P sync engine that runs the REQUEST_CHAIN / GET_OBJECTS
protocol loop.

**Files:**
- Create: `chain/p2psync.go`
- Test: `chain/p2psync_test.go`
- Integration test: `chain/integration_test.go`

**Step 1: Write a mock-based unit test**

Create `chain/p2psync_test.go` with a mock `P2PConnection` interface:

```go
package chain

import (
	"context"
	"testing"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/config"
	"github.com/stretchr/testify/require"
)

// mockP2PConn implements P2PConnection for testing.
type mockP2PConn struct {
	peerHeight uint64
	// blocks maps hash -> (blockBlob, txBlobs)
	blocks map[string]struct {
		blockBlob []byte
		txBlobs   [][]byte
	}
	chainHashes [][]byte
}

func (m *mockP2PConn) PeerHeight() uint64 { return m.peerHeight }
func (m *mockP2PConn) RequestChain(blockIDs [][]byte) (startHeight uint64, hashes [][]byte, err error) {
	return 0, m.chainHashes, nil
}
func (m *mockP2PConn) RequestObjects(blockHashes [][]byte) ([]BlockBlobEntry, error) {
	var entries []BlockBlobEntry
	for _, h := range blockHashes {
		key := string(h)
		if blk, ok := m.blocks[key]; ok {
			entries = append(entries, BlockBlobEntry{
				Block: blk.blockBlob,
				Txs:   blk.txBlobs,
			})
		}
	}
	return entries, nil
}

func TestP2PSync_EmptyChain(t *testing.T) {
	// Test that P2PSync with a mock that has no blocks is a no-op.
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	mock := &mockP2PConn{peerHeight: 0}

	opts := SyncOptions{Forks: config.TestnetForks}
	err = c.P2PSync(context.Background(), mock, opts)
	require.NoError(t, err)
}
```

**Step 2: Implement P2PSync**

Create `chain/p2psync.go`:

```go
package chain

import (
	"context"
	"fmt"
	"log"
)

// P2PConnection abstracts the P2P communication needed for block sync.
type P2PConnection interface {
	// PeerHeight returns the peer's advertised chain height.
	PeerHeight() uint64

	// RequestChain sends NOTIFY_REQUEST_CHAIN and returns the response.
	RequestChain(blockIDs [][]byte) (startHeight uint64, hashes [][]byte, err error)

	// RequestObjects sends NOTIFY_REQUEST_GET_OBJECTS and returns block blobs.
	RequestObjects(blockHashes [][]byte) ([]BlockBlobEntry, error)
}

// BlockBlobEntry holds raw block and transaction blobs from a peer.
type BlockBlobEntry struct {
	Block []byte
	Txs   [][]byte
}

const p2pBatchSize = 200

// P2PSync synchronises the chain from a P2P peer. It runs the
// REQUEST_CHAIN / REQUEST_GET_OBJECTS protocol loop until the local
// chain reaches the peer's height.
func (c *Chain) P2PSync(ctx context.Context, conn P2PConnection, opts SyncOptions) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		localHeight, err := c.Height()
		if err != nil {
			return fmt.Errorf("p2p sync: get height: %w", err)
		}

		peerHeight := conn.PeerHeight()
		if localHeight >= peerHeight {
			return nil // synced
		}

		// Build sparse chain history.
		history, err := c.SparseChainHistory()
		if err != nil {
			return fmt.Errorf("p2p sync: build history: %w", err)
		}

		// Convert Hash to []byte for P2P.
		historyBytes := make([][]byte, len(history))
		for i, h := range history {
			b := make([]byte, 32)
			copy(b, h[:])
			historyBytes[i] = b
		}

		// Request chain entry.
		startHeight, blockIDs, err := conn.RequestChain(historyBytes)
		if err != nil {
			return fmt.Errorf("p2p sync: request chain: %w", err)
		}

		if len(blockIDs) == 0 {
			return nil // nothing to sync
		}

		log.Printf("p2p sync: chain entry from height %d, %d block IDs", startHeight, len(blockIDs))

		// Fetch blocks in batches.
		for i := 0; i < len(blockIDs); i += p2pBatchSize {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			end := i + p2pBatchSize
			if end > len(blockIDs) {
				end = len(blockIDs)
			}
			batch := blockIDs[i:end]

			entries, err := conn.RequestObjects(batch)
			if err != nil {
				return fmt.Errorf("p2p sync: request objects: %w", err)
			}

			currentHeight := startHeight + uint64(i)
			for j, entry := range entries {
				blockHeight := currentHeight + uint64(j)
				if blockHeight > 0 && blockHeight%100 == 0 {
					log.Printf("p2p sync: processing block %d", blockHeight)
				}

				// P2P path: difficulty=0 (TODO: compute from LWMA)
				if err := c.processBlockBlobs(entry.Block, entry.Txs,
					blockHeight, 0, opts); err != nil {
					return fmt.Errorf("p2p sync: process block %d: %w", blockHeight, err)
				}
			}
		}
	}
}
```

**Step 3: Run tests**

Run: `go test -v -run TestP2PSync ./chain/`
Expected: PASS

**Step 4: Commit**

```bash
git add chain/p2psync.go chain/p2psync_test.go
git commit -m "feat(chain): add P2P sync state machine

P2PSync() runs the REQUEST_CHAIN / REQUEST_GET_OBJECTS loop against
a P2PConnection interface. Reuses processBlockBlobs() for shared
validation logic.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## Task 12: Levin P2P Connection Adapter

Implement `P2PConnection` on top of the Levin connection for real peers.

**Files:**
- Create: `chain/levinconn.go`
- Test: integration test in `chain/integration_test.go`

**Step 1: Implement LevinP2PConn**

Create `chain/levinconn.go`:

```go
package chain

import (
	"fmt"

	"forge.lthn.ai/core/go-blockchain/p2p"
	levinpkg "forge.lthn.ai/core/go-p2p/node/levin"
)

// LevinP2PConn adapts a Levin connection to the P2PConnection interface.
type LevinP2PConn struct {
	conn       *levinpkg.Connection
	peerHeight uint64
}

// NewLevinP2PConn wraps a Levin connection for P2P sync.
// peerHeight is obtained from the handshake CoreSyncData.
func NewLevinP2PConn(conn *levinpkg.Connection, peerHeight uint64) *LevinP2PConn {
	return &LevinP2PConn{conn: conn, peerHeight: peerHeight}
}

func (c *LevinP2PConn) PeerHeight() uint64 { return c.peerHeight }

func (c *LevinP2PConn) RequestChain(blockIDs [][]byte) (uint64, [][]byte, error) {
	req := p2p.RequestChain{BlockIDs: blockIDs}
	payload, err := req.Encode()
	if err != nil {
		return 0, nil, fmt.Errorf("encode request_chain: %w", err)
	}

	// Send as notification (expectResponse=false) per CryptoNote protocol.
	if err := c.conn.WritePacket(p2p.CommandRequestChain, payload, false); err != nil {
		return 0, nil, fmt.Errorf("write request_chain: %w", err)
	}

	// Read until we get RESPONSE_CHAIN_ENTRY.
	for {
		hdr, data, err := c.conn.ReadPacket()
		if err != nil {
			return 0, nil, fmt.Errorf("read response_chain: %w", err)
		}
		if hdr.Command == p2p.CommandResponseChain {
			var resp p2p.ResponseChainEntry
			if err := resp.Decode(data); err != nil {
				return 0, nil, fmt.Errorf("decode response_chain: %w", err)
			}
			return resp.StartHeight, resp.BlockIDs, nil
		}
		// Skip other messages (timed_sync, etc.)
	}
}

func (c *LevinP2PConn) RequestObjects(blockHashes [][]byte) ([]BlockBlobEntry, error) {
	req := p2p.RequestGetObjects{Blocks: blockHashes}
	payload, err := req.Encode()
	if err != nil {
		return nil, fmt.Errorf("encode request_get_objects: %w", err)
	}

	if err := c.conn.WritePacket(p2p.CommandRequestObjects, payload, false); err != nil {
		return nil, fmt.Errorf("write request_get_objects: %w", err)
	}

	// Read until we get RESPONSE_GET_OBJECTS.
	for {
		hdr, data, err := c.conn.ReadPacket()
		if err != nil {
			return nil, fmt.Errorf("read response_get_objects: %w", err)
		}
		if hdr.Command == p2p.CommandResponseObjects {
			var resp p2p.ResponseGetObjects
			if err := resp.Decode(data); err != nil {
				return nil, fmt.Errorf("decode response_get_objects: %w", err)
			}
			entries := make([]BlockBlobEntry, len(resp.Blocks))
			for i, b := range resp.Blocks {
				entries[i] = BlockBlobEntry{Block: b.Block, Txs: b.Txs}
			}
			return entries, nil
		}
	}
}
```

**Step 2: Write integration test**

Add to `chain/integration_test.go`:

```go
func TestIntegration_P2PSync(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping P2P sync test in short mode")
	}

	// Dial testnet daemon P2P port.
	conn, err := net.DialTimeout("tcp", "localhost:46942", 10*time.Second)
	if err != nil {
		t.Skipf("testnet P2P not reachable: %v", err)
	}
	defer conn.Close()

	lc := levin.NewConnection(conn)

	// Handshake.
	var peerIDBuf [8]byte
	rand.Read(peerIDBuf[:])
	peerID := binary.LittleEndian.Uint64(peerIDBuf[:])

	req := p2p.HandshakeRequest{
		NodeData: p2p.NodeData{
			NetworkID: config.NetworkIDTestnet,
			PeerID:    peerID,
			LocalTime: time.Now().Unix(),
			MyPort:    0,
		},
		PayloadData: p2p.CoreSyncData{
			CurrentHeight:  1,
			ClientVersion:  config.ClientVersion,
			NonPruningMode: true,
		},
	}
	payload, err := p2p.EncodeHandshakeRequest(&req)
	require.NoError(t, err)
	require.NoError(t, lc.WritePacket(p2p.CommandHandshake, payload, true))

	hdr, data, err := lc.ReadPacket()
	require.NoError(t, err)
	require.Equal(t, uint32(p2p.CommandHandshake), hdr.Command)

	var resp p2p.HandshakeResponse
	require.NoError(t, resp.Decode(data))
	t.Logf("peer height: %d", resp.PayloadData.CurrentHeight)

	// Create P2P connection adapter.
	p2pConn := NewLevinP2PConn(lc, resp.PayloadData.CurrentHeight)

	// Create chain and sync.
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	opts := SyncOptions{
		VerifySignatures: false,
		Forks:            config.TestnetForks,
	}

	err = c.P2PSync(context.Background(), p2pConn, opts)
	require.NoError(t, err)

	finalHeight, _ := c.Height()
	t.Logf("P2P synced %d blocks", finalHeight)
	require.Equal(t, resp.PayloadData.CurrentHeight, finalHeight)
}
```

**Step 3: Run the integration test**

Run: `go test -tags integration -v -run TestIntegration_P2PSync ./chain/ -timeout 300s`

Iterate on any failures (decode issues, block hash mismatches, etc.).

**Step 4: Commit**

```bash
git add chain/levinconn.go chain/integration_test.go
git commit -m "feat(chain): add Levin P2P connection adapter and sync integration test

LevinP2PConn wraps a levin.Connection to implement the P2PConnection
interface. Integration test syncs the full testnet chain via P2P.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## Task 13: Update Documentation

Update docs with the new block sync capabilities.

**Files:**
- Modify: `docs/history.md`
- Modify: `docs/architecture.md`

**Step 1: Update history.md**

Add a new section for the block sync work:

```markdown
### Block Sync (Phase 3)

- NLSAG ring signature verification wired into consensus path
- Full testnet chain synced via RPC with all blocks validated
- P2P block sync via REQUEST_CHAIN / REQUEST_GET_OBJECTS protocol
- Sparse chain history builder for P2P sync requests
- Shared processBlockBlobs() for RPC and P2P paths
- Context cancellation and progress logging for long syncs
```

**Step 2: Update architecture.md**

Add `P2PConnection` interface documentation to the `chain/` section.
Add `RequestGetObjects` / `ResponseGetObjects` to the `p2p/` section.

**Step 3: Run final test suite**

Run: `go test -race ./...` and `go vet ./...`
Expected: All pass, no warnings.

**Step 4: Commit**

```bash
git add docs/history.md docs/architecture.md
git commit -m "docs: update architecture and history for block sync

Document NLSAG verification, P2P sync protocol, and shared block
processing.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## Summary

| Task | Component | Files |
|------|-----------|-------|
| 1 | NLSAG verification | `consensus/verify.go`, `consensus/verify_crypto_test.go` |
| 2 | RingOutputsFn callback | `chain/ring.go`, `chain/ring_test.go`, `chain/sync.go` |
| 3 | Context + logging | `chain/sync.go`, `chain/sync_test.go` |
| 4 | ZC key images | `chain/sync.go` |
| 5 | Full RPC sync test | `chain/integration_test.go` |
| 6 | Sig verification test | `chain/integration_test.go` |
| 7 | P2P command types | `p2p/relay.go`, `p2p/relay_test.go` |
| 8 | P2P integration test | `p2p/integration_test.go` |
| 9 | Sparse chain history | `chain/history.go`, `chain/history_test.go` |
| 10 | Refactor processBlock | `chain/sync.go` |
| 11 | P2P sync state machine | `chain/p2psync.go`, `chain/p2psync_test.go` |
| 12 | Levin adapter + test | `chain/levinconn.go`, `chain/integration_test.go` |
| 13 | Documentation | `docs/history.md`, `docs/architecture.md` |

## Dependencies

```
Task 1 → Task 2 → Task 6 (signature verification chain)
Task 3 → Task 5 (context needed for long sync)
Task 4 → Task 5 (ZC handling needed for testnet)
Task 7 → Task 8 (types before integration test)
Task 9 → Task 11 (history builder before sync engine)
Task 10 → Task 11 (shared processing before P2P sync)
Task 11 → Task 12 (sync engine before Levin adapter)
Task 12 → Task 13 (all code before docs)
```

Critical path: 1 → 2 → 3 → 4 → 5 → 10 → 7 → 9 → 11 → 12 → 13
