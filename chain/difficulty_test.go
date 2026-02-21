// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"testing"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-blockchain/types"
	store "forge.lthn.ai/core/go-store"
	"github.com/stretchr/testify/require"
)

// preHF2Forks is a fork schedule where HF2 never activates,
// so the target stays at 120s.
var preHF2Forks = []config.HardFork{
	{Version: config.HF0Initial, Height: 0},
}

func TestNextDifficulty_Genesis(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	diff, err := c.NextDifficulty(0, preHF2Forks)
	require.NoError(t, err)
	require.Equal(t, uint64(1), diff)
}

func TestNextDifficulty_FewBlocks(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)

	// Store genesis + 4 blocks with constant 120s intervals and difficulty 1000.
	// Genesis at height 0 is excluded from the LWMA window.
	baseDiff := uint64(1000)
	for i := uint64(0); i < 5; i++ {
		err := c.PutBlock(&types.Block{}, &BlockMeta{
			Hash:           types.Hash{byte(i + 1)},
			Height:         i,
			Timestamp:      i * 120,
			Difficulty:     baseDiff,
			CumulativeDiff: baseDiff * (i + 1),
		})
		require.NoError(t, err)
	}

	// Next difficulty for height 5 uses blocks 1-4 (n=3 intervals).
	// LWMA formula with constant D and T gives D/n = 1000/3 ≈ 333.
	diff, err := c.NextDifficulty(5, preHF2Forks)
	require.NoError(t, err)
	require.Greater(t, diff, uint64(0))

	// LWMA gives total_work * T * (n+1) / (2 * weighted_solvetimes * n).
	// For constant intervals: D/n = 1000/3 = 333.
	expected := uint64(333)
	require.Equal(t, expected, diff)
}

func TestNextDifficulty_EmptyChain(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)

	// Height 1 with no blocks stored — should return starter difficulty.
	diff, err := c.NextDifficulty(1, preHF2Forks)
	require.NoError(t, err)
	require.Equal(t, uint64(1), diff)
}
