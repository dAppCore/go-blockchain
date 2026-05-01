// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package difficulty

import (
	"math/big"
	"testing"

	"dappco.re/go/core/blockchain/config"
)

func TestDifficulty_NextDifficulty_Good(t *testing.T) {
	// Synthetic test: constant block times at exactly the target interval.
	// With the LWMA-1 formula, constant D gives next_D = D/n for full window.
	target := config.BlockTarget
	const numBlocks = 100

	timestamps := make([]uint64, numBlocks)
	cumulativeDiffs := make([]*big.Int, numBlocks)

	baseDifficulty := big.NewInt(1000)
	for i := 0; i < numBlocks; i++ {
		timestamps[i] = uint64(i) * target
		cumulativeDiffs[i] = new(big.Int).Mul(baseDifficulty, big.NewInt(int64(i)))
	}

	result := NextDifficulty(timestamps, cumulativeDiffs, target)
	if result.Sign() <= 0 {
		t.Fatalf("NextDifficulty returned non-positive value: %s", result)
	}

	// LWMA trims to last 61 entries (N+1=61), giving n=60 intervals.
	// Formula: D/n = 1000/60 = 16.
	expected := big.NewInt(16)
	if result.Cmp(expected) != 0 {
		t.Errorf("NextDifficulty with constant intervals: got %s, expected %s", result, expected)
	}
}

func TestDifficulty_NextDifficultyEmpty_Good(t *testing.T) {
	// Empty input should return starter difficulty.
	result := NextDifficulty(nil, nil, config.BlockTarget)
	if result.Cmp(StarterDifficulty) != 0 {
		t.Errorf("NextDifficulty(nil, nil, %d) = %s, want %s", config.BlockTarget, result, StarterDifficulty)
	}
}

func TestDifficulty_NextDifficultySingleEntry_Good(t *testing.T) {
	// A single entry is insufficient for calculation.
	timestamps := []uint64{1000}
	diffs := []*big.Int{big.NewInt(100)}
	result := NextDifficulty(timestamps, diffs, config.BlockTarget)
	if result.Cmp(StarterDifficulty) != 0 {
		t.Errorf("NextDifficulty with single entry = %s, want %s", result, StarterDifficulty)
	}
}

func TestDifficulty_NextDifficultyFastBlocks_Good(t *testing.T) {
	// When blocks come faster than the target, difficulty should increase
	// relative to the constant-rate result.
	target := config.BlockTarget
	const numBlocks = 50
	const actualInterval uint64 = 60 // half the target — blocks are too fast

	timestamps := make([]uint64, numBlocks)
	cumulativeDiffs := make([]*big.Int, numBlocks)

	baseDifficulty := big.NewInt(1000)
	for i := 0; i < numBlocks; i++ {
		timestamps[i] = uint64(i) * actualInterval
		cumulativeDiffs[i] = new(big.Int).Mul(baseDifficulty, big.NewInt(int64(i)))
	}

	resultFast := NextDifficulty(timestamps, cumulativeDiffs, target)

	// Now compute with on-target intervals for comparison.
	timestampsTarget := make([]uint64, numBlocks)
	for i := 0; i < numBlocks; i++ {
		timestampsTarget[i] = uint64(i) * target
	}
	resultTarget := NextDifficulty(timestampsTarget, cumulativeDiffs, target)

	if resultFast.Cmp(resultTarget) <= 0 {
		t.Errorf("fast blocks (%s) should produce higher difficulty than target-rate blocks (%s)",
			resultFast, resultTarget)
	}
}

func TestDifficulty_NextDifficultySlowBlocks_Good(t *testing.T) {
	// When blocks come slower than the target, difficulty should decrease
	// relative to the constant-rate result.
	target := config.BlockTarget
	const numBlocks = 50
	const actualInterval uint64 = 240 // double the target — blocks are too slow

	timestamps := make([]uint64, numBlocks)
	cumulativeDiffs := make([]*big.Int, numBlocks)

	baseDifficulty := big.NewInt(1000)
	for i := 0; i < numBlocks; i++ {
		timestamps[i] = uint64(i) * actualInterval
		cumulativeDiffs[i] = new(big.Int).Mul(baseDifficulty, big.NewInt(int64(i)))
	}

	resultSlow := NextDifficulty(timestamps, cumulativeDiffs, target)

	// Compute with on-target intervals for comparison.
	timestampsTarget := make([]uint64, numBlocks)
	for i := 0; i < numBlocks; i++ {
		timestampsTarget[i] = uint64(i) * target
	}
	resultTarget := NextDifficulty(timestampsTarget, cumulativeDiffs, target)

	if resultSlow.Cmp(resultTarget) >= 0 {
		t.Errorf("slow blocks (%s) should produce lower difficulty than target-rate blocks (%s)",
			resultSlow, resultTarget)
	}
}

func TestDifficulty_NextDifficulty_Ugly(t *testing.T) {
	// Two entries with zero time span — should handle gracefully.
	timestamps := []uint64{1000, 1000}
	diffs := []*big.Int{big.NewInt(0), big.NewInt(100)}
	result := NextDifficulty(timestamps, diffs, config.BlockTarget)
	if result.Sign() <= 0 {
		t.Errorf("NextDifficulty with zero time span should still return positive, got %s", result)
	}
}

func TestDifficulty_Constants_Good(t *testing.T) {
	if Window != 720 {
		t.Errorf("Window: got %d, want 720", Window)
	}
	if Lag != 15 {
		t.Errorf("Lag: got %d, want 15", Lag)
	}
	if Cut != 60 {
		t.Errorf("Cut: got %d, want 60", Cut)
	}
	if BlocksCount != 735 {
		t.Errorf("BlocksCount: got %d, want 735", BlocksCount)
	}
	if LWMAWindow != 60 {
		t.Errorf("LWMAWindow: got %d, want 60", LWMAWindow)
	}
}
