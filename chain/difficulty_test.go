// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"testing"

	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/types"
	store "dappco.re/go/core/store"
	"github.com/stretchr/testify/require"
)

// preHF6Forks is a fork schedule where HF6 never activates,
// so both PoW and PoS targets stay at 120s.
var preHF6Forks = []config.HardFork{
	{Version: config.HF0Initial, Height: 0},
}

// hf6ActiveForks is a fork schedule where HF6 activates at height 100,
// switching both PoW and PoS targets to 240s from block 101 onwards.
var hf6ActiveForks = []config.HardFork{
	{Version: config.HF0Initial, Height: 0},
	{Version: config.HF1, Height: 0},
	{Version: config.HF2, Height: 0},
	{Version: config.HF3, Height: 0},
	{Version: config.HF4Zarcanum, Height: 0},
	{Version: config.HF5, Height: 0},
	{Version: config.HF6, Height: 100},
}

// storeBlocks inserts count blocks with constant intervals and difficulty.
func storeBlocks(t *testing.T, c *Chain, count int, interval uint64, baseDiff uint64) {
	t.Helper()
	for i := uint64(0); i < uint64(count); i++ {
		err := c.PutBlock(&types.Block{}, &BlockMeta{
			Hash:           types.Hash{byte(i + 1)},
			Height:         i,
			Timestamp:      i * interval,
			Difficulty:     baseDiff,
			CumulativeDiff: baseDiff * (i + 1),
		})
		require.NoError(t, err)
	}
}

func TestDifficulty_NextDifficulty_Genesis_Ugly(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	diff, err := c.NextDifficulty(0, preHF6Forks)
	require.NoError(t, err)
	require.Equal(t, uint64(1), diff)
}

func TestDifficulty_NextDifficulty_FewBlocks_Ugly(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)

	// Store genesis + 4 blocks with constant 120s intervals and difficulty 1000.
	// Genesis at height 0 is excluded from the LWMA window.
	storeBlocks(t, c, 5, 120, 1000)

	// Next difficulty for height 5 uses blocks 1-4 (n=3 intervals).
	// LWMA formula with constant D and T gives D/n = 1000/3 = 333.
	diff, err := c.NextDifficulty(5, preHF6Forks)
	require.NoError(t, err)
	require.Greater(t, diff, uint64(0))

	expected := uint64(333)
	require.Equal(t, expected, diff)
}

func TestDifficulty_NextDifficulty_EmptyChain_Ugly(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)

	// Height 1 with no blocks stored -- should return starter difficulty.
	diff, err := c.NextDifficulty(1, preHF6Forks)
	require.NoError(t, err)
	require.Equal(t, uint64(1), diff)
}

// --- HF6 boundary tests ---

func TestDifficulty_NextDifficulty_HF6Boundary_Good(t *testing.T) {
	// Verify that blocks at height <= 100 use the 120s target and blocks
	// at height > 100 use the 240s target, given hf6ActiveForks.
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	storeBlocks(t, c, 105, 120, 1000)

	// Height 100 -- HF6 activates at heights > 100, so this is pre-HF6.
	diffPre, err := c.NextDifficulty(100, hf6ActiveForks)
	require.NoError(t, err)

	// Height 101 -- HF6 is active (height > 100), target becomes 240s.
	diffPost, err := c.NextDifficulty(101, hf6ActiveForks)
	require.NoError(t, err)

	// With 120s actual intervals and a 240s target, LWMA should produce
	// lower difficulty than with a 120s target. The post-HF6 difficulty
	// should differ from the pre-HF6 difficulty because the target doubled.
	require.NotEqual(t, diffPre, diffPost,
		"difficulty should change across HF6 boundary (120s vs 240s target)")
}

func TestDifficulty_NextDifficulty_HF6Boundary_Bad(t *testing.T) {
	// HF6 at height 999,999,999 (mainnet default) -- should never activate
	// for realistic heights, so the target stays at 120s.
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	storeBlocks(t, c, 105, 120, 1000)

	forks := config.MainnetForks
	diff100, err := c.NextDifficulty(100, forks)
	require.NoError(t, err)

	diff101, err := c.NextDifficulty(101, forks)
	require.NoError(t, err)

	// Both should use the same 120s target -- no HF6 in sight.
	require.Equal(t, diff100, diff101,
		"difficulty should be identical when HF6 is far in the future")
}

func TestDifficulty_NextDifficulty_HF6Boundary_Ugly(t *testing.T) {
	// HF6 at height 0 (active from genesis) -- the 240s target should
	// apply from the very first difficulty calculation.
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	storeBlocks(t, c, 5, 240, 1000)

	genesisHF6 := []config.HardFork{
		{Version: config.HF0Initial, Height: 0},
		{Version: config.HF6, Height: 0},
	}

	diff, err := c.NextDifficulty(4, genesisHF6)
	require.NoError(t, err)
	require.Greater(t, diff, uint64(0))
}

// --- PoS difficulty tests ---

func TestDifficulty_NextPoSDifficulty_Good(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	storeBlocks(t, c, 5, 120, 1000)

	// Pre-HF6: PoS target should be 120s (same as PoW).
	diff, err := c.NextPoSDifficulty(5, preHF6Forks)
	require.NoError(t, err)
	require.Equal(t, uint64(333), diff)
}

func TestDifficulty_NextPoSDifficulty_HF6Boundary_Good(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	storeBlocks(t, c, 105, 120, 1000)

	// Height 100 -- pre-HF6.
	diffPre, err := c.NextPoSDifficulty(100, hf6ActiveForks)
	require.NoError(t, err)

	// Height 101 -- post-HF6, target becomes 240s.
	diffPost, err := c.NextPoSDifficulty(101, hf6ActiveForks)
	require.NoError(t, err)

	require.NotEqual(t, diffPre, diffPost,
		"PoS difficulty should change across HF6 boundary")
}

func TestDifficulty_NextPoSDifficulty_Genesis_Ugly(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	diff, err := c.NextPoSDifficulty(0, preHF6Forks)
	require.NoError(t, err)
	require.Equal(t, uint64(1), diff)
}
