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

// NextDifficulty computes the expected difficulty for the block at the given
// height, using the LWMA algorithm over stored block history.
//
// The genesis block (height 0) is excluded from the difficulty window,
// matching the C++ daemon's load_targetdata_cache which skips index 0.
//
// The target block time depends on the hardfork schedule: 120s pre-HF2,
// 240s post-HF2 (matching DIFFICULTY_POW_TARGET_HF6 in the C++ source).
func (c *Chain) NextDifficulty(height uint64, forks []config.HardFork) (uint64, error) {
	if height == 0 {
		return 1, nil
	}

	// LWMA needs N+1 entries (N solve-time intervals).
	// Start from height 1 — genesis is excluded from the difficulty window.
	maxLookback := difficulty.LWMAWindow + 1
	lookback := height // height excludes genesis since we start from 1
	if lookback > maxLookback {
		lookback = maxLookback
	}

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

	for i := 0; i < count; i++ {
		meta, err := c.getBlockMeta(startHeight + uint64(i))
		if err != nil {
			// Fewer blocks than expected — use what we have.
			timestamps = timestamps[:i]
			cumulDiffs = cumulDiffs[:i]
			break
		}
		timestamps[i] = meta.Timestamp
		cumulDiffs[i] = new(big.Int).SetUint64(meta.CumulativeDiff)
	}

	// Determine the target block time based on hardfork status.
	// HF2 doubles the target from 120s to 240s.
	target := config.DifficultyPowTarget
	if config.IsHardForkActive(forks, config.HF2, height) {
		target = config.DifficultyPowTargetHF6
	}

	result := difficulty.NextDifficulty(timestamps, cumulDiffs, target)
	return result.Uint64(), nil
}
