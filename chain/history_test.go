// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"testing"

	"forge.lthn.ai/core/go-blockchain/types"
	"github.com/stretchr/testify/require"
)

func TestSparseChainHistory_Empty(t *testing.T) {
	c := newTestChain(t)

	history, err := c.SparseChainHistory()
	require.NoError(t, err)
	require.Len(t, history, 1) // just the zero hash (genesis placeholder)
	require.True(t, history[0].IsZero())
}

func TestSparseChainHistory_FewBlocks(t *testing.T) {
	c := newTestChain(t)

	// Store 5 blocks with known hashes.
	for i := uint64(0); i < 5; i++ {
		hash := types.Hash{byte(i + 1)}
		blk := &types.Block{
			BlockHeader: types.BlockHeader{MajorVersion: 1},
			MinerTx:     testCoinbaseTx(i),
		}
		if i > 0 {
			blk.PrevID = types.Hash{byte(i)}
		}
		err := c.PutBlock(blk, &BlockMeta{Hash: hash, Height: i})
		require.NoError(t, err)
	}

	history, err := c.SparseChainHistory()
	require.NoError(t, err)

	// With 5 blocks (heights 0-4), all within the first 10 so step=1
	// throughout. Should return hashes for heights 4, 3, 2, 1, 0.
	require.Greater(t, len(history), 0)

	// First entry should be top block hash (height 4).
	require.Equal(t, types.Hash{5}, history[0])

	// Last entry should be genesis hash (height 0).
	require.Equal(t, types.Hash{1}, history[len(history)-1])

	// All 5 blocks should be present since count < 10.
	require.Len(t, history, 5)
}

func TestSparseChainHistory_SingleBlock(t *testing.T) {
	c := newTestChain(t)

	blk := &types.Block{
		BlockHeader: types.BlockHeader{MajorVersion: 1},
		MinerTx:     testCoinbaseTx(0),
	}
	err := c.PutBlock(blk, &BlockMeta{Hash: types.Hash{0xaa}, Height: 0})
	require.NoError(t, err)

	history, err := c.SparseChainHistory()
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, types.Hash{0xaa}, history[0])
}

func TestSparseChainHistory_ExponentialSpacing(t *testing.T) {
	c := newTestChain(t)

	// Store 20 blocks (heights 0-19) to exercise exponential stepping.
	for i := uint64(0); i < 20; i++ {
		hash := types.Hash{byte(i + 1)}
		blk := &types.Block{
			BlockHeader: types.BlockHeader{MajorVersion: 1},
			MinerTx:     testCoinbaseTx(i),
		}
		if i > 0 {
			blk.PrevID = types.Hash{byte(i)}
		}
		err := c.PutBlock(blk, &BlockMeta{Hash: hash, Height: i})
		require.NoError(t, err)
	}

	history, err := c.SparseChainHistory()
	require.NoError(t, err)

	// First entry should be top block (height 19).
	require.Equal(t, types.Hash{20}, history[0])

	// Last entry should be genesis (height 0).
	require.Equal(t, types.Hash{1}, history[len(history)-1])

	// Should have fewer entries than total blocks due to exponential steps.
	require.Less(t, len(history), 20)

	// First 10 entries should be consecutive (step=1): heights 19, 18, ..., 10.
	for i := 0; i < 10; i++ {
		expected := types.Hash{byte(20 - i)}
		require.Equal(t, expected, history[i],
			"entry %d: expected hash for height %d", i, 19-i)
	}

	// After the first 10, steps double each entry:
	// Entry 10: step becomes 2, current = 10 - 2 = 8 -> hash {9}
	// Entry 11: step becomes 4, current = 8 - 4 = 4  -> hash {5}
	// Entry 12: step becomes 8, current 4 < 8, jump to 0 -> hash {1} (genesis)
	require.Equal(t, types.Hash{byte(9)}, history[10])  // height 8
	require.Equal(t, types.Hash{byte(5)}, history[11])  // height 4
	require.Equal(t, types.Hash{byte(1)}, history[12])  // height 0 (genesis)
	require.Len(t, history, 13)
}
