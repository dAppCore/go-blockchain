// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"testing"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/types"
	"github.com/stretchr/testify/require"
)

func TestNextDifficulty_Genesis(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	diff, err := c.NextDifficulty(0)
	require.NoError(t, err)
	require.Equal(t, uint64(1), diff)
}

func TestNextDifficulty_FewBlocks(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)

	// Store 5 blocks with constant 120s intervals and difficulty 1000.
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

	// Next difficulty for height 5 should be approximately 1000.
	diff, err := c.NextDifficulty(5)
	require.NoError(t, err)
	require.Greater(t, diff, uint64(0))

	// With constant intervals at target, difficulty should be close to base.
	// Allow 10% tolerance.
	low := baseDiff - baseDiff/10
	high := baseDiff + baseDiff/10
	require.GreaterOrEqual(t, diff, low, "difficulty %d below expected range [%d, %d]", diff, low, high)
	require.LessOrEqual(t, diff, high, "difficulty %d above expected range [%d, %d]", diff, low, high)
}

func TestNextDifficulty_EmptyChain(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)

	// Height 1 with no blocks stored — should return starter difficulty.
	diff, err := c.NextDifficulty(1)
	require.NoError(t, err)
	require.Equal(t, uint64(1), diff)
}
