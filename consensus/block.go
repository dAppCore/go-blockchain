// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package consensus

import (
	"fmt"
	"sort"

	"forge.lthn.ai/core/go-blockchain/config"
)

// IsPoS returns true if the block flags indicate a Proof-of-Stake block.
// Bit 0 of the flags byte is the PoS indicator.
func IsPoS(flags uint8) bool {
	return flags&1 != 0
}

// CheckTimestamp validates a block's timestamp against future limits and
// the median of recent timestamps.
func CheckTimestamp(blockTimestamp uint64, flags uint8, adjustedTime uint64, recentTimestamps []uint64) error {
	// Future time limit.
	limit := config.BlockFutureTimeLimit
	if IsPoS(flags) {
		limit = config.PosBlockFutureTimeLimit
	}
	if blockTimestamp > adjustedTime+limit {
		return fmt.Errorf("%w: %d > %d + %d", ErrTimestampFuture,
			blockTimestamp, adjustedTime, limit)
	}

	// Median check — only when we have enough history.
	if uint64(len(recentTimestamps)) < config.TimestampCheckWindow {
		return nil
	}

	median := medianTimestamp(recentTimestamps)
	if blockTimestamp < median {
		return fmt.Errorf("%w: %d < median %d", ErrTimestampOld,
			blockTimestamp, median)
	}

	return nil
}

// medianTimestamp returns the median of a slice of timestamps.
func medianTimestamp(timestamps []uint64) uint64 {
	sorted := make([]uint64, len(timestamps))
	copy(sorted, timestamps)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	n := len(sorted)
	if n == 0 {
		return 0
	}
	return sorted[n/2]
}
