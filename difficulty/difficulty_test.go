// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package difficulty

import (
	"math/big"
	"testing"

	"forge.lthn.ai/core/go-blockchain/config"
)

func TestNextDifficulty_Good(t *testing.T) {
	// Synthetic test: constant block times at exactly the target interval.
	// With perfectly timed blocks, the difficulty should remain stable.
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

	// With constant intervals, the result should be approximately equal to
	// the base difficulty. Allow some tolerance due to integer arithmetic.
	expected := baseDifficulty
	tolerance := new(big.Int).Div(expected, big.NewInt(10)) // 10% tolerance

	diff := new(big.Int).Sub(result, expected)
	diff.Abs(diff)
	if diff.Cmp(tolerance) > 0 {
		t.Errorf("NextDifficulty with constant intervals: got %s, expected ~%s (tolerance %s)",
			result, expected, tolerance)
	}
}

func TestNextDifficultyEmpty_Good(t *testing.T) {
	// Empty input should return starter difficulty.
	result := NextDifficulty(nil, nil, config.BlockTarget)
	if result.Cmp(StarterDifficulty) != 0 {
		t.Errorf("NextDifficulty(nil, nil, %d) = %s, want %s", config.BlockTarget, result, StarterDifficulty)
	}
}

func TestNextDifficultySingleEntry_Good(t *testing.T) {
	// A single entry is insufficient for calculation.
	timestamps := []uint64{1000}
	diffs := []*big.Int{big.NewInt(100)}
	result := NextDifficulty(timestamps, diffs, config.BlockTarget)
	if result.Cmp(StarterDifficulty) != 0 {
		t.Errorf("NextDifficulty with single entry = %s, want %s", result, StarterDifficulty)
	}
}

func TestNextDifficultyFastBlocks_Good(t *testing.T) {
	// When blocks come faster than the target, difficulty should increase.
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

	result := NextDifficulty(timestamps, cumulativeDiffs, target)
	if result.Cmp(baseDifficulty) <= 0 {
		t.Errorf("expected difficulty > %s for fast blocks, got %s", baseDifficulty, result)
	}
}

func TestNextDifficultySlowBlocks_Good(t *testing.T) {
	// When blocks come slower than the target, difficulty should decrease.
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

	result := NextDifficulty(timestamps, cumulativeDiffs, target)
	if result.Cmp(baseDifficulty) >= 0 {
		t.Errorf("expected difficulty < %s for slow blocks, got %s", baseDifficulty, result)
	}
}

func TestNextDifficulty_Ugly(t *testing.T) {
	// Two entries with zero time span — should handle gracefully.
	timestamps := []uint64{1000, 1000}
	diffs := []*big.Int{big.NewInt(0), big.NewInt(100)}
	result := NextDifficulty(timestamps, diffs, config.BlockTarget)
	if result.Sign() <= 0 {
		t.Errorf("NextDifficulty with zero time span should still return positive, got %s", result)
	}
}

func TestConstants_Good(t *testing.T) {
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
}
