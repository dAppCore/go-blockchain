# Local Difficulty Computation — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Compute block difficulty locally during P2P sync instead of hardcoding 0.

**Architecture:** `Chain.NextDifficulty(height)` reads stored block timestamps and cumulative diffs, calls `difficulty.NextDifficulty()` with `config.BlockTarget`. P2PSync calls this before `processBlockBlobs`.

**Tech Stack:** Go stdlib, `difficulty` package (LWMA), `config` package.

---

## Task 1: Add BlockTarget Config Constant

**Files:**
- Modify: `config/config.go`
- Modify: `difficulty/difficulty_test.go` (use the constant instead of magic 120)

**Step 1: Add BlockTarget to config**

In `config/config.go`, after the difficulty window constants section, add:

```go
// BlockTarget is the desired block interval in seconds.
// Both PoW and PoS blocks use the same 120-second target.
const BlockTarget uint64 = 120
```

Find the right location — near the other mining/difficulty constants.

**Step 2: Update difficulty tests to use it**

In `difficulty/difficulty_test.go`, replace all `const target uint64 = 120` with `config.BlockTarget`. Add the import for `forge.lthn.ai/core/go-blockchain/config`.

**Step 3: Run tests**

Run: `go test -race ./config/ ./difficulty/`
Expected: All pass.

**Step 4: Commit**

```bash
git add config/config.go difficulty/difficulty_test.go
git commit -m "feat(config): add BlockTarget constant (120s)

Replaces magic number 120 in difficulty tests with config.BlockTarget.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## Task 2: Implement Chain.NextDifficulty

**Files:**
- Create: `chain/difficulty.go`
- Create: `chain/difficulty_test.go`

**Step 1: Write the failing test**

Create `chain/difficulty_test.go`:

```go
package chain

import (
	"math/big"
	"testing"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/types"
	"github.com/stretchr/testify/require"
)

func TestNextDifficulty_Genesis(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	diff, err := c.NextDifficulty(0)
	require.NoError(t, err)
	require.Equal(t, uint64(1), diff)
}

func TestNextDifficulty_FewBlocks(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)

	// Store 5 blocks with constant 120s intervals and difficulty 1000.
	baseDiff := uint64(1000)
	for i := uint64(0); i < 5; i++ {
		err := c.PutBlock(&types.Block{}, &BlockMeta{
			Hash:           types.Hash{byte(i + 1)},
			Height:         i,
			Timestamp:      i * 120,
			Difficulty:     baseDiff,
			CumulativeDiff: baseDiff * (i + 1),
		})
		require.NoError(t, err)
	}

	// Next difficulty for height 5 should be approximately 1000.
	diff, err := c.NextDifficulty(5)
	require.NoError(t, err)
	require.Greater(t, diff, uint64(0))

	// With constant intervals at target, difficulty should be close to base.
	// Allow 10% tolerance.
	low := baseDiff - baseDiff/10
	high := baseDiff + baseDiff/10
	require.GreaterOrEqual(t, diff, low, "difficulty %d below expected range [%d, %d]", diff, low, high)
	require.LessOrEqual(t, diff, high, "difficulty %d above expected range [%d, %d]", diff, low, high)
}

func TestNextDifficulty_EmptyChain(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)

	// Height 1 with no blocks stored — should return starter difficulty.
	diff, err := c.NextDifficulty(1)
	require.NoError(t, err)
	require.Equal(t, uint64(1), diff)
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestNextDifficulty ./chain/`
Expected: FAIL — `NextDifficulty` does not exist.

**Step 3: Implement NextDifficulty**

Create `chain/difficulty.go`:

```go
package chain

import (
	"math/big"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-blockchain/difficulty"
)

// NextDifficulty computes the expected difficulty for the block at the given
// height, using the LWMA algorithm over stored block history.
func (c *Chain) NextDifficulty(height uint64) (uint64, error) {
	if height == 0 {
		return 1, nil
	}

	// Determine how far back to look.
	lookback := height
	if lookback > difficulty.BlocksCount {
		lookback = difficulty.BlocksCount
	}

	startHeight := height - lookback
	count := int(lookback)

	timestamps := make([]uint64, count)
	cumulDiffs := make([]*big.Int, count)

	for i := 0; i < count; i++ {
		_, meta, err := c.GetBlockByHeight(startHeight + uint64(i))
		if err != nil {
			// Fewer blocks than expected — use what we have.
			timestamps = timestamps[:i]
			cumulDiffs = cumulDiffs[:i]
			break
		}
		timestamps[i] = meta.Timestamp
		cumulDiffs[i] = new(big.Int).SetUint64(meta.CumulativeDiff)
	}

	result := difficulty.NextDifficulty(timestamps, cumulDiffs, config.BlockTarget)
	return result.Uint64(), nil
}
```

**Step 4: Run tests**

Run: `go test -v -run TestNextDifficulty ./chain/`
Expected: PASS

**Step 5: Run full suite**

Run: `go test -race ./...`
Expected: All pass.

**Step 6: Commit**

```bash
git add chain/difficulty.go chain/difficulty_test.go
git commit -m "feat(chain): add NextDifficulty for local LWMA computation

Reads stored block timestamps and cumulative difficulties, calls
difficulty.NextDifficulty() with config.BlockTarget. Returns uint64.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## Task 3: Wire into P2PSync

**Files:**
- Modify: `chain/p2psync.go:119-121`

**Step 1: Replace the hardcoded difficulty**

In `chain/p2psync.go`, change:

```go
			// P2P path: difficulty=0 (TODO: compute from LWMA)
			if err := c.processBlockBlobs(entry.Block, entry.Txs,
				blockHeight, 0, opts); err != nil {
```

To:

```go
			blockDiff, err := c.NextDifficulty(blockHeight)
			if err != nil {
				return fmt.Errorf("p2p sync: compute difficulty for block %d: %w", blockHeight, err)
			}

			if err := c.processBlockBlobs(entry.Block, entry.Txs,
				blockHeight, blockDiff, opts); err != nil {
```

**Step 2: Run unit tests**

Run: `go test -race ./chain/`
Expected: All pass (mock-based P2PSync tests still work because mock returns no blocks).

**Step 3: Commit**

```bash
git add chain/p2psync.go
git commit -m "feat(chain): compute difficulty locally during P2P sync

P2PSync now calls NextDifficulty() for each block instead of
hardcoding difficulty=0.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## Task 4: Integration Test — Verify Against RPC Difficulties

**Files:**
- Modify: `chain/integration_test.go`

**Step 1: Add comparison test**

Add a test that syncs via RPC (which gets daemon-provided difficulties), then
for each block, computes NextDifficulty locally and compares. This validates
our LWMA implementation matches the C++ daemon.

```go
func TestIntegration_DifficultyMatchesRPC(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping difficulty comparison test in short mode")
	}

	client := rpc.NewClientWithHTTP(testnetRPCAddr, &http.Client{Timeout: 60 * time.Second})

	_, err := client.GetHeight()
	if err != nil {
		t.Skipf("testnet daemon not reachable: %v", err)
	}

	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)

	// Sync a portion of the chain via RPC (which stores daemon-provided difficulty).
	opts := SyncOptions{
		VerifySignatures: false,
		Forks:            config.TestnetForks,
	}
	err = c.Sync(context.Background(), client, opts)
	require.NoError(t, err)

	finalHeight, _ := c.Height()
	t.Logf("synced %d blocks, checking difficulty computation", finalHeight)

	// For each block from height 1 onwards, verify our NextDifficulty matches
	// the daemon-provided difficulty stored in BlockMeta.
	mismatches := 0
	for h := uint64(1); h < finalHeight; h++ {
		_, meta, err := c.GetBlockByHeight(h)
		require.NoError(t, err)

		computed, err := c.NextDifficulty(h)
		require.NoError(t, err)

		if computed != meta.Difficulty {
			if mismatches < 10 {
				t.Logf("difficulty mismatch at height %d: computed=%d, daemon=%d",
					h, computed, meta.Difficulty)
			}
			mismatches++
		}
	}

	if mismatches > 0 {
		t.Errorf("%d/%d blocks have difficulty mismatches", mismatches, finalHeight-1)
	} else {
		t.Logf("all %d blocks have matching difficulty", finalHeight-1)
	}
}
```

**Step 2: Run the integration test**

Run: `go test -tags integration -v -run TestIntegration_DifficultyMatchesRPC ./chain/ -timeout 300s`

**CRITICAL**: If there are mismatches, the LWMA algorithm in `difficulty/difficulty.go`
may differ from the C++ implementation. Debug by comparing the exact timestamps and
cumulative diffs being fed to the algorithm. Common issues:
- Off-by-one in the lookback window (should we include the block at `height-1` or not?)
- The C++ code may use a different window size for early blocks
- Cumulative diff stored as uint64 may overflow (unlikely for testnet)

Fix any issues in `difficulty/difficulty.go` or `chain/difficulty.go`.

**Step 3: Run full suite**

Run: `go test -race ./...`
Run: `go vet ./...`

**Step 4: Commit**

```bash
git add chain/integration_test.go
git commit -m "test(chain): verify local difficulty matches daemon values

Compares NextDifficulty() output against daemon-provided difficulty
for every block synced via RPC.

Co-Authored-By: Charon <charon@lethean.io>"
```

If bug fixes were needed, commit those separately first.

---

## Summary

| Task | Component | Files |
|------|-----------|-------|
| 1 | BlockTarget constant | `config/config.go`, `difficulty/difficulty_test.go` |
| 2 | Chain.NextDifficulty | `chain/difficulty.go`, `chain/difficulty_test.go` |
| 3 | Wire into P2PSync | `chain/p2psync.go` |
| 4 | Integration test | `chain/integration_test.go` |

## Dependencies

```
Task 1 → Task 2 → Task 3
Task 2 → Task 4
```
