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

	result := difficulty.NextDifficulty(timestamps, cumulDiffs, config.BlockTarget)
	return result.Uint64(), nil
}
