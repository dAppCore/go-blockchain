# HF3 Block Version 2 Support

**Date:** 2026-03-16
**Author:** Charon
**Package:** `dappco.re/go/core/blockchain`
**Status:** Approved

## Context

HF3 increments the block major version from 1 to 2 (`HF3_BLOCK_MAJOR_VERSION`). This is a preparatory hardfork for Zarcanum (HF4). No new transaction types, no new validation rules — purely a version gate.

On mainnet, HF3 is at height 999,999,999 (future). On testnet, HF3 activates at height 0 (genesis).

## Scope

- Add block major version validation to `consensus/block.go` `ValidateBlock`
- Validate version progression: HF0→0, HF1/HF2→1, HF3→2, HF4+→3

## Design

### consensus/block.go — ValidateBlock

Add a `checkBlockVersion` function called from `ValidateBlock`:

```go
func checkBlockVersion(majorVersion uint8, height uint64, forks []config.HardFork) error {
    expected := expectedBlockMajorVersion(height, forks)
    if majorVersion != expected {
        return fmt.Errorf("%w: got %d, expected %d at height %d",
            ErrBlockVersion, majorVersion, expected, height)
    }
    return nil
}

func expectedBlockMajorVersion(height uint64, forks []config.HardFork) uint8 {
    switch {
    case config.IsHardForkActive(forks, config.HF4Zarcanum, height):
        return config.CurrentBlockMajorVersion // 3
    case config.IsHardForkActive(forks, config.HF3, height):
        return config.HF3BlockMajorVersion // 2
    case config.IsHardForkActive(forks, config.HF1, height):
        return config.HF1BlockMajorVersion // 1
    default:
        return config.BlockMajorVersionInitial // 0
    }
}
```

This covers all hardforks in one function. `ValidateBlock` signature needs `forks []config.HardFork` added (it currently receives forks via the caller).

### errors.go

Add `ErrBlockVersion` sentinel error.

### Testing

- Test version 0 valid pre-HF1, rejected post-HF1
- Test version 1 valid post-HF1, rejected pre-HF1 and post-HF3
- Test version 2 valid post-HF3, rejected pre-HF3 and post-HF4
- Test version 3 valid post-HF4
- Test with both mainnet and testnet fork schedules

## Note

This function also satisfies HF1's block version requirement (issue #8 from the HF1 spec review). Implementing this as part of HF3 means the HF1 plan doesn't need a separate version check — this single function handles all hardforks.

## Out of scope

- Block minor version validation (not consensus-critical in current chain)
