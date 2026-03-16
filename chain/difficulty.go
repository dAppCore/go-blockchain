// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"math/big"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-blockchain/difficulty"
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
