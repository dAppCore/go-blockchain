# Local Difficulty Computation — Design

Date: 2026-02-21

## Problem

P2P sync hardcodes `difficulty=0` when calling `processBlockBlobs()`. This means:

1. Blocks stored via P2P have no difficulty metadata
2. `consensus.CheckDifficulty()` skips validation when difficulty is 0
3. Cumulative difficulty tracking is broken (always 0)

## Solution

Compute the next block's difficulty locally using the existing
`difficulty.NextDifficulty()` LWMA algorithm, fed by timestamps and
cumulative diffs from the stored block history.

### Changes

1. **`config/config.go`** — add `BlockTarget = 120` constant (seconds).
2. **`chain/difficulty.go`** (new) — `Chain.NextDifficulty(height)` reads up
   to 735 blocks of history from the store, calls `difficulty.NextDifficulty()`.
3. **`chain/p2psync.go`** — replace `difficulty=0` with
   `c.NextDifficulty(blockHeight)`.

### Algorithm

```
func (c *Chain) NextDifficulty(height uint64) (uint64, error):
    if height == 0:
        return 1, nil  // genesis has difficulty 1

    // Read up to BlocksCount (735) previous blocks.
    lookback = min(height, difficulty.BlocksCount)
    startHeight = height - lookback

    timestamps = []
    cumulDiffs = []
    for h = startHeight; h < height; h++:
        _, meta = c.GetBlockByHeight(h)
        timestamps.append(meta.Timestamp)
        cumulDiffs.append(meta.CumulativeDiff)

    return difficulty.NextDifficulty(timestamps, cumulDiffs, config.BlockTarget)
```

### Validation

Compare P2P-computed difficulties against RPC-provided difficulties from the
daemon. They should match for every block on testnet.
