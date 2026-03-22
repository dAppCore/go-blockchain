# HF6 Block Time Halving Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox syntax for tracking.

**Goal:** Correct the difficulty target gate from HF2 to HF6 so the PoW target stays at 120s until HF6 activates, then switches to 240s. Add the matching PoS difficulty function that follows the same HF6 gate.

**Architecture:** `chain/difficulty.go` already computes PoW difficulty via the LWMA algorithm in `difficulty/`. The HF2 gate is a Zano-ism -- Lethean mainnet uses 120s blocks between HF2 (height 10,080) and HF6 (height 999,999,999). The fix changes the gate constant and adds a parallel `NextPoSDifficulty` method with identical logic but using the PoS target constants.

**Tech Stack:** Go 1.26, go-store (SQLite), go test -race

---

## File Map

### Modified files

| File | What changes |
|------|-------------|
| `chain/difficulty.go` | Change HF2 gate to HF6. Add `NextPoSDifficulty` method. Add comment explaining the HF2-to-HF6 correction. |
| `chain/difficulty_test.go` | Rename `preHF2Forks` to `preHF6Forks`. Add HF6 boundary tests for both PoW and PoS (Good/Bad/Ugly). |

### Unchanged files (reference only)

| File | Role |
|------|------|
| `config/hardfork.go` | Defines `HF6` constant and `MainnetForks`/`TestnetForks` schedules. No changes needed. |
| `config/config.go` | Defines `DifficultyPowTargetHF6`, `DifficultyPosTargetHF6` constants. No changes needed. |
| `difficulty/difficulty.go` | Pure LWMA algorithm -- takes target as parameter. No changes needed. |

---

## Task 1: Fix the HF2-to-HF6 gate and add PoS difficulty

**Package:** `chain/`
**Why:** The current code gates the 240s PoW target on HF2 (block 10,080), but Lethean mainnet uses 120s blocks until HF6 (999,999,999). This means the Go node would compute incorrect difficulty for every block between 10,081 and the future HF6 activation. Additionally, there is no PoS difficulty function -- PoS blocks also need the 120s-to-240s switch at HF6.

### Step 1.1 -- Write tests for the HF6 difficulty boundary

- [ ] Edit `/home/claude/Code/core/go-blockchain/chain/difficulty_test.go`

Replace the entire file with:

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"testing"

	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/types"
	store "dappco.re/go/core/store"
	"github.com/stretchr/testify/require"
)

// preHF6Forks is a fork schedule where HF6 never activates,
// so both PoW and PoS targets stay at 120s.
var preHF6Forks = []config.HardFork{
	{Version: config.HF0Initial, Height: 0},
}

// hf6ActiveForks is a fork schedule where HF6 activates at height 100,
// switching both PoW and PoS targets to 240s from block 101 onwards.
var hf6ActiveForks = []config.HardFork{
	{Version: config.HF0Initial, Height: 0},
	{Version: config.HF1, Height: 0},
	{Version: config.HF2, Height: 0},
	{Version: config.HF3, Height: 0},
	{Version: config.HF4Zarcanum, Height: 0},
	{Version: config.HF5, Height: 0},
	{Version: config.HF6, Height: 100},
}

// storeBlocks inserts genesis + n blocks with constant intervals and difficulty.
func storeBlocks(t *testing.T, c *Chain, count int, interval uint64, baseDiff uint64) {
	t.Helper()
	for i := uint64(0); i < uint64(count); i++ {
		err := c.PutBlock(&types.Block{}, &BlockMeta{
			Hash:           types.Hash{byte(i + 1)},
			Height:         i,
			Timestamp:      i * interval,
			Difficulty:     baseDiff,
			CumulativeDiff: baseDiff * (i + 1),
		})
		require.NoError(t, err)
	}
}

func TestNextDifficulty_Genesis(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	diff, err := c.NextDifficulty(0, preHF6Forks)
	require.NoError(t, err)
	require.Equal(t, uint64(1), diff)
}

func TestNextDifficulty_FewBlocks(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)

	// Store genesis + 4 blocks with constant 120s intervals and difficulty 1000.
	// Genesis at height 0 is excluded from the LWMA window.
	storeBlocks(t, c, 5, 120, 1000)

	// Next difficulty for height 5 uses blocks 1-4 (n=3 intervals).
	// LWMA formula with constant D and T gives D/n = 1000/3 = 333.
	diff, err := c.NextDifficulty(5, preHF6Forks)
	require.NoError(t, err)
	require.Greater(t, diff, uint64(0))

	expected := uint64(333)
	require.Equal(t, expected, diff)
}

func TestNextDifficulty_EmptyChain(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)

	// Height 1 with no blocks stored -- should return starter difficulty.
	diff, err := c.NextDifficulty(1, preHF6Forks)
	require.NoError(t, err)
	require.Equal(t, uint64(1), diff)
}

// --- HF6 boundary tests ---

func TestNextDifficulty_HF6Boundary_Good(t *testing.T) {
	// Verify that blocks at height <= 100 use the 120s target and blocks
	// at height > 100 use the 240s target, given hf6ActiveForks.
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	storeBlocks(t, c, 105, 120, 1000)

	// Height 100 -- HF6 activates at heights > 100, so this is pre-HF6.
	diffPre, err := c.NextDifficulty(100, hf6ActiveForks)
	require.NoError(t, err)

	// Height 101 -- HF6 is active (height > 100), target becomes 240s.
	diffPost, err := c.NextDifficulty(101, hf6ActiveForks)
	require.NoError(t, err)

	// With 120s actual intervals and a 240s target, LWMA should produce
	// lower difficulty than with a 120s target. The post-HF6 difficulty
	// should differ from the pre-HF6 difficulty because the target doubled.
	require.NotEqual(t, diffPre, diffPost,
		"difficulty should change across HF6 boundary (120s vs 240s target)")
}

func TestNextDifficulty_HF6Boundary_Bad(t *testing.T) {
	// HF6 at height 999,999,999 (mainnet default) -- should never activate
	// for realistic heights, so the target stays at 120s.
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	storeBlocks(t, c, 105, 120, 1000)

	forks := config.MainnetForks
	diff100, err := c.NextDifficulty(100, forks)
	require.NoError(t, err)

	diff101, err := c.NextDifficulty(101, forks)
	require.NoError(t, err)

	// Both should use the same 120s target -- no HF6 in sight.
	require.Equal(t, diff100, diff101,
		"difficulty should be identical when HF6 is far in the future")
}

func TestNextDifficulty_HF6Boundary_Ugly(t *testing.T) {
	// HF6 at height 0 (active from genesis) -- the 240s target should
	// apply from the very first difficulty calculation.
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	storeBlocks(t, c, 5, 240, 1000)

	genesisHF6 := []config.HardFork{
		{Version: config.HF0Initial, Height: 0},
		{Version: config.HF6, Height: 0},
	}

	diff, err := c.NextDifficulty(4, genesisHF6)
	require.NoError(t, err)
	require.Greater(t, diff, uint64(0))
}

// --- PoS difficulty tests ---

func TestNextPoSDifficulty_Good(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	storeBlocks(t, c, 5, 120, 1000)

	// Pre-HF6: PoS target should be 120s (same as PoW).
	diff, err := c.NextPoSDifficulty(5, preHF6Forks)
	require.NoError(t, err)
	require.Equal(t, uint64(333), diff)
}

func TestNextPoSDifficulty_HF6Boundary_Good(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	storeBlocks(t, c, 105, 120, 1000)

	// Height 100 -- pre-HF6.
	diffPre, err := c.NextPoSDifficulty(100, hf6ActiveForks)
	require.NoError(t, err)

	// Height 101 -- post-HF6, target becomes 240s.
	diffPost, err := c.NextPoSDifficulty(101, hf6ActiveForks)
	require.NoError(t, err)

	require.NotEqual(t, diffPre, diffPost,
		"PoS difficulty should change across HF6 boundary")
}

func TestNextPoSDifficulty_Genesis(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	diff, err := c.NextPoSDifficulty(0, preHF6Forks)
	require.NoError(t, err)
	require.Equal(t, uint64(1), diff)
}
```

### Step 1.2 -- Run tests, verify FAIL

```bash
cd /home/claude/Code/core/go-blockchain && go test -race -run "TestNext.*Difficulty" ./chain/...
```

**Expected:** Compilation error -- `NextPoSDifficulty` does not exist. The renamed `preHF6Forks` replaces `preHF2Forks`. The `hf6ActiveForks` and `storeBlocks` helper are new.

### Step 1.3 -- Implement the fix

- [ ] Edit `/home/claude/Code/core/go-blockchain/chain/difficulty.go`

Replace the entire file with:

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"math/big"

	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/difficulty"
)

// nextDifficultyWith computes the expected difficulty for the block at the
// given height using the LWMA algorithm, parameterised by pre/post-HF6 targets.
//
// The genesis block (height 0) is excluded from the difficulty window,
// matching the C++ daemon's load_targetdata_cache which skips index 0.
//
// The target block time depends on the hardfork schedule:
//   - Pre-HF6: baseTarget (120s for both PoW and PoS on Lethean)
//   - Post-HF6: hf6Target (240s -- halves block rate, halves emission)
//
// NOTE: This was originally gated on HF2, matching the Zano upstream where
// HF2 coincides with the difficulty target change. Lethean mainnet keeps 120s
// blocks between HF2 (height 10,080) and HF6 (height 999,999,999), so the
// gate was corrected to HF6 in March 2026.
func (c *Chain) nextDifficultyWith(height uint64, forks []config.HardFork, baseTarget, hf6Target uint64) (uint64, error) {
	if height == 0 {
		return 1, nil
	}

	// LWMA needs N+1 entries (N solve-time intervals).
	// Start from height 1 -- genesis is excluded from the difficulty window.
	maxLookback := difficulty.LWMAWindow + 1
	lookback := min(height, maxLookback) // height excludes genesis since we start from 1

	// Start from max(1, height - lookback) to exclude genesis.
	startHeight := height - lookback
	if startHeight == 0 {
		startHeight = 1
		lookback = height - 1
	}

	if lookback == 0 {
		return 1, nil
	}

	count := int(lookback)
	timestamps := make([]uint64, count)
	cumulDiffs := make([]*big.Int, count)

	for i := range count {
		meta, err := c.getBlockMeta(startHeight + uint64(i))
		if err != nil {
			// Fewer blocks than expected -- use what we have.
			timestamps = timestamps[:i]
			cumulDiffs = cumulDiffs[:i]
			break
		}
		timestamps[i] = meta.Timestamp
		cumulDiffs[i] = new(big.Int).SetUint64(meta.CumulativeDiff)
	}

	// Determine the target block time based on hardfork status.
	// HF6 doubles the target from 120s to 240s (corrected from HF2 gate).
	target := baseTarget
	if config.IsHardForkActive(forks, config.HF6, height) {
		target = hf6Target
	}

	result := difficulty.NextDifficulty(timestamps, cumulDiffs, target)
	return result.Uint64(), nil
}

// NextDifficulty computes the expected PoW difficulty for the block at the
// given height. Pre-HF6 the target is 120s; post-HF6 it doubles to 240s.
func (c *Chain) NextDifficulty(height uint64, forks []config.HardFork) (uint64, error) {
	return c.nextDifficultyWith(height, forks, config.DifficultyPowTarget, config.DifficultyPowTargetHF6)
}

// NextPoSDifficulty computes the expected PoS difficulty for the block at the
// given height. Pre-HF6 the target is 120s; post-HF6 it doubles to 240s.
func (c *Chain) NextPoSDifficulty(height uint64, forks []config.HardFork) (uint64, error) {
	return c.nextDifficultyWith(height, forks, config.DifficultyPosTarget, config.DifficultyPosTargetHF6)
}
```

### Step 1.4 -- Run tests, verify PASS

```bash
cd /home/claude/Code/core/go-blockchain && go test -race -run "TestNext.*Difficulty" ./chain/...
```

**Expected:**

```
ok  	dappco.re/go/core/blockchain/chain	(cached)
```

All 10 tests pass: `TestNextDifficulty_Genesis`, `TestNextDifficulty_FewBlocks`, `TestNextDifficulty_EmptyChain`, `TestNextDifficulty_HF6Boundary_Good`, `TestNextDifficulty_HF6Boundary_Bad`, `TestNextDifficulty_HF6Boundary_Ugly`, `TestNextPoSDifficulty_Good`, `TestNextPoSDifficulty_HF6Boundary_Good`, `TestNextPoSDifficulty_Genesis`.

### Step 1.5 -- Run full test suite and vet

```bash
cd /home/claude/Code/core/go-blockchain && go test -race ./... && go vet ./... && go mod tidy
```

**Expected:** All tests pass, no vet warnings, no module changes.

### Step 1.6 -- Commit

```bash
cd /home/claude/Code/core/go-blockchain && git add chain/difficulty.go chain/difficulty_test.go && git commit -m "fix(chain): gate difficulty target switch on HF6, not HF2

The 240s PoW target was incorrectly gated on HF2 (block 10,080), matching
the Zano upstream where HF2 coincides with the difficulty target change.
Lethean mainnet uses 120s blocks between HF2 and HF6 (999,999,999), so
the gate is corrected to HF6.

Also adds NextPoSDifficulty with the same HF6 gate using the PoS target
constants (DifficultyPosTarget / DifficultyPosTargetHF6).

Both public methods delegate to a shared nextDifficultyWith helper to
avoid duplicating the LWMA window logic.

Co-Authored-By: Charon <charon@lethean.io>"
```
