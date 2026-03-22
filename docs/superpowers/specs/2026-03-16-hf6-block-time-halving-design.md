# HF6 Block Time Halving

**Date:** 2026-03-16
**Author:** Charon
**Package:** `dappco.re/go/core/blockchain`
**Status:** Draft
**Depends on:** HF5 (confidential assets)

## Context

HF6 doubles the PoW and PoS block targets from 120s to 240s, effectively halving the emission rate without changing the per-block reward. Blocks per day drop from ~1440 to ~720.

On both mainnet and testnet, HF6 is at height 999,999,999 (future/reserved).

**What's already implemented:**
- `DifficultyPowTargetHF6 = 240` and `DifficultyPosTargetHF6 = 240` constants in config
- `DifficultyTotalTargetHF6` computed constant
- `chain/difficulty.go` already switches target based on HF2 — same pattern extends to HF6

**What's NOT implemented:**
- The difficulty switch in `chain/difficulty.go` gates on HF2 but uses the HF6 constants. This is technically correct for the Zano chain where HF2 and the difficulty change are the same thing, but for Lethean the naming is misleading.
- Minimum build version enforcement for HF6

## Scope

This is a ~10 line change.

### chain/difficulty.go

Currently:
```go
target := config.DifficultyPowTarget
if config.IsHardForkActive(forks, config.HF2, height) {
    target = config.DifficultyPowTargetHF6
}
```

After HF6 support:
```go
target := config.DifficultyPowTarget // 120s
if config.IsHardForkActive(forks, config.HF6, height) {
    target = config.DifficultyPowTargetHF6 // 240s
}
```

Wait — looking at this again, the current code gates the 240s target on HF2 (block 10,080), not HF6 (999,999,999). This means blocks after HF2 are already using the 240s target. Need to check whether this is intentional for Lethean or a bug from the Zano port.

**TODO:** Verify with the C++ daemon what target time blocks after height 10,080 actually use. If Lethean mainnet uses 120s until HF6, then the current code is wrong (should gate on HF6 not HF2). If Lethean follows Zano's schedule where HF2 = difficulty change, then it's correct and HF6 is a no-op.

### Consensus timestamp validation

The `BlockFutureTimeLimit` and `PosBlockFutureTimeLimit` may need adjustment for HF6 if the block time changes. Currently 2 hours for PoW and 20 minutes for PoS — these are reasonable for both 120s and 240s targets.

### Testing

- Difficulty calculation with 240s target
- Verify existing difficulty tests still pass
- Integration test: compute difficulty across HF6 boundary on testnet

## Out of scope

- PoS target adjustments (same 240s, already in config)
- Emission schedule calculations (per-block reward stays the same)
