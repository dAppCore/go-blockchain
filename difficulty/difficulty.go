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
// arrive at the desired interval on average. Each solve-time interval is
// weighted linearly by its recency — more recent intervals have greater
// influence on the result.
package difficulty

import (
	"math/big"
)

// Algorithm constants matching the C++ source.
const (
	// Window is the number of blocks in the legacy difficulty window.
	Window uint64 = 720

	// Lag is the additional lookback beyond the window (legacy).
	Lag uint64 = 15

	// Cut is the number of extreme timestamps trimmed (legacy).
	Cut uint64 = 60

	// BlocksCount is the total number of blocks considered (Window + Lag).
	// Used by legacy algorithms; the LWMA uses LWMAWindow instead.
	BlocksCount uint64 = Window + Lag

	// LWMAWindow is the number of solve-time intervals used by the LWMA
	// algorithm (N=60). This means we need N+1 = 61 block entries.
	LWMAWindow uint64 = 60
)

// StarterDifficulty is the minimum difficulty returned when there is
// insufficient data to calculate a proper value.
var StarterDifficulty = big.NewInt(1)

// NextDifficulty calculates the next block difficulty using the LWMA algorithm.
//
// Parameters:
//   - timestamps: block timestamps ordered from oldest to newest.
//   - cumulativeDiffs: cumulative difficulties corresponding to each block.
//   - target: the desired block interval in seconds (e.g. 120 for PoW/PoS).
//
// Returns the calculated difficulty for the next block.
//
// The algorithm matches the C++ next_difficulty_lwma() in difficulty.cpp:
//
//	next_D = total_work * T * (n+1) / (2 * weighted_solvetimes * n)
//
// where each solve-time interval i is weighted by its position (1..n),
// giving more influence to recent blocks.
func NextDifficulty(timestamps []uint64, cumulativeDiffs []*big.Int, target uint64) *big.Int {
	// Need at least 2 entries to compute one solve-time interval.
	if len(timestamps) < 2 || len(cumulativeDiffs) < 2 {
		return new(big.Int).Set(StarterDifficulty)
	}

	length := len(timestamps)

	// Trim to at most N+1 entries (N solve-time intervals).
	maxEntries := int(LWMAWindow) + 1
	if length > maxEntries {
		// Keep the most recent entries.
		offset := length - maxEntries
		timestamps = timestamps[offset:]
		cumulativeDiffs = cumulativeDiffs[offset:]
		length = maxEntries
	}

	// n = number of solve-time intervals.
	n := int64(length - 1)
	T := int64(target)

	// Compute linearly weighted solve-times.
	// Weight i (1..n) gives more recent intervals higher influence.
	var weightedSolveTimes int64
	for i := int64(1); i <= n; i++ {
		st := int64(timestamps[i]) - int64(timestamps[i-1])

		// Clamp to [-6T, 6T] to limit timestamp manipulation impact.
		if st < -(6 * T) {
			st = -(6 * T)
		}
		if st > 6*T {
			st = 6 * T
		}

		weightedSolveTimes += st * i
	}

	// Guard against zero or negative (pathological timestamps).
	if weightedSolveTimes <= 0 {
		weightedSolveTimes = 1
	}

	// Total work across the window.
	totalWork := new(big.Int).Sub(cumulativeDiffs[n], cumulativeDiffs[0])
	if totalWork.Sign() <= 0 {
		return new(big.Int).Set(StarterDifficulty)
	}

	// LWMA formula: next_D = total_work * T * (n+1) / (2 * weighted_solvetimes * n)
	numerator := new(big.Int).Mul(totalWork, big.NewInt(T*(n+1)))
	denominator := big.NewInt(2 * weightedSolveTimes * n)

	nextDiff := new(big.Int).Div(numerator, denominator)

	// Ensure we never return zero difficulty.
	if nextDiff.Sign() <= 0 {
		return new(big.Int).Set(StarterDifficulty)
	}

	return nextDiff
}
