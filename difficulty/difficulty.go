// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// You may obtain a copy of the licence at:
//
//     https://joinup.ec.europa.eu/software/page/eupl/licence-eupl
//
// SPDX-License-Identifier: EUPL-1.2

// Package difficulty implements the LWMA (Linear Weighted Moving Average)
// difficulty adjustment algorithm used by the Lethean blockchain for both
// PoW and PoS blocks.
//
// The algorithm examines a window of recent block timestamps and cumulative
// difficulties to calculate the next target difficulty, ensuring blocks
// arrive at the desired interval on average.
package difficulty

import (
	"math/big"
)

// Algorithm constants matching the C++ source.
const (
	// Window is the number of blocks in the difficulty calculation window.
	Window uint64 = 720

	// Lag is the additional lookback beyond the window.
	Lag uint64 = 15

	// Cut is the number of extreme timestamps trimmed from each end after
	// sorting. This dampens the effect of outlier timestamps.
	Cut uint64 = 60

	// BlocksCount is the total number of blocks considered (Window + Lag).
	BlocksCount uint64 = Window + Lag
)

// StarterDifficulty is the minimum difficulty returned when there is
// insufficient data to calculate a proper value.
var StarterDifficulty = big.NewInt(1)

// NextDifficulty calculates the next block difficulty using the LWMA algorithm.
//
// Parameters:
//   - timestamps: block timestamps for the last BlocksCount blocks, ordered
//     from oldest to newest.
//   - cumulativeDiffs: cumulative difficulties corresponding to each block.
//   - target: the desired block interval in seconds (e.g. 120 for PoW/PoS).
//
// Returns the calculated difficulty for the next block.
//
// If the input slices are too short to perform a meaningful calculation, the
// function returns StarterDifficulty.
func NextDifficulty(timestamps []uint64, cumulativeDiffs []*big.Int, target uint64) *big.Int {
	// Need at least 2 entries to compute a time span and difficulty delta.
	if len(timestamps) < 2 || len(cumulativeDiffs) < 2 {
		return new(big.Int).Set(StarterDifficulty)
	}

	length := uint64(len(timestamps))
	if length > BlocksCount {
		length = BlocksCount
	}

	// Use the available window, but ensure we have at least 2 points.
	windowSize := length
	if windowSize < 2 {
		return new(big.Int).Set(StarterDifficulty)
	}

	// Calculate the time span across the window.
	// Use only the last windowSize entries.
	startIdx := uint64(len(timestamps)) - windowSize
	endIdx := uint64(len(timestamps)) - 1

	timeSpan := timestamps[endIdx] - timestamps[startIdx]
	if timeSpan == 0 {
		timeSpan = 1 // prevent division by zero
	}

	// Calculate the difficulty delta across the same window.
	diffDelta := new(big.Int).Sub(cumulativeDiffs[endIdx], cumulativeDiffs[startIdx])
	if diffDelta.Sign() <= 0 {
		return new(big.Int).Set(StarterDifficulty)
	}

	// LWMA core: nextDiff = diffDelta * target / timeSpan
	// This keeps the difficulty proportional to the hash rate needed to
	// maintain the target block interval.
	nextDiff := new(big.Int).Mul(diffDelta, new(big.Int).SetUint64(target))
	nextDiff.Div(nextDiff, new(big.Int).SetUint64(timeSpan))

	// Ensure we never return zero difficulty.
	if nextDiff.Sign() <= 0 {
		return new(big.Int).Set(StarterDifficulty)
	}

	return nextDiff
}
