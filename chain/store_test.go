// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"testing"

	"dappco.re/go/core/blockchain/types"
	"dappco.re/go/core/blockchain/wire"
	store "dappco.re/go/core/store"
)

// newTempChain creates a Chain backed by a file-based store in t.TempDir().
func newTempChain(t *testing.T) *Chain {
	t.Helper()
	directory := t.TempDir()
	s, err := store.New(directory + "/test.db")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return New(s)
}

// testBlock returns a minimal block and metadata at the given height.
func testBlock(height uint64) (*types.Block, *BlockMeta) {
	blk := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Timestamp:    1770897600 + height*120,
		},
		MinerTx: testCoinbaseTx(height),
	}
	meta := &BlockMeta{
		Hash:       types.Hash{byte(height & 0xFF), byte((height >> 8) & 0xFF)},
		Height:     height,
		Timestamp:  1770897600 + height*120,
		Difficulty: 100 + height,
	}
	return blk, meta
}

// --- Good (happy path) ---

func TestStore_PutBlock_Good(t *testing.T) {
	c := newTempChain(t)

	blk, meta := testBlock(0)
	if err := c.PutBlock(blk, meta); err != nil {
		t.Fatalf("PutBlock: %v", err)
	}

	height, err := c.Height()
	if err != nil {
		t.Fatalf("Height: %v", err)
	}
	if height != 1 {
		t.Errorf("height: got %d, want 1", height)
	}
}

func TestStore_PutBlock_MultipleBlocks_Good(t *testing.T) {
	c := newTempChain(t)

	for i := uint64(0); i < 5; i++ {
		blk, meta := testBlock(i)
		if err := c.PutBlock(blk, meta); err != nil {
			t.Fatalf("PutBlock(%d): %v", i, err)
		}
	}

	height, err := c.Height()
	if err != nil {
		t.Fatalf("Height: %v", err)
	}
	if height != 5 {
		t.Errorf("height: got %d, want 5", height)
	}
}

func TestStore_GetBlockByHeight_Good(t *testing.T) {
	c := newTempChain(t)

	blk, meta := testBlock(0)
	if err := c.PutBlock(blk, meta); err != nil {
		t.Fatalf("PutBlock: %v", err)
	}

	gotBlock, gotMeta, err := c.GetBlockByHeight(0)
	if err != nil {
		t.Fatalf("GetBlockByHeight: %v", err)
	}
	if gotBlock.MajorVersion != 1 {
		t.Errorf("major_version: got %d, want 1", gotBlock.MajorVersion)
	}
	if gotMeta.Hash != meta.Hash {
		t.Errorf("hash mismatch: got %s, want %s", gotMeta.Hash, meta.Hash)
	}
	if gotMeta.Timestamp != meta.Timestamp {
		t.Errorf("timestamp: got %d, want %d", gotMeta.Timestamp, meta.Timestamp)
	}
}

func TestStore_GetBlockByHeight_MultipleHeights_Good(t *testing.T) {
	c := newTempChain(t)

	for i := uint64(0); i < 3; i++ {
		blk, meta := testBlock(i)
		if err := c.PutBlock(blk, meta); err != nil {
			t.Fatalf("PutBlock(%d): %v", i, err)
		}
	}

	for i := uint64(0); i < 3; i++ {
		_, gotMeta, err := c.GetBlockByHeight(i)
		if err != nil {
			t.Fatalf("GetBlockByHeight(%d): %v", i, err)
		}
		if gotMeta.Height != i {
			t.Errorf("height %d: gotMeta.Height = %d", i, gotMeta.Height)
		}
	}
}

func TestStore_GetBlockByHash_Good(t *testing.T) {
	c := newTempChain(t)

	blk, meta := testBlock(0)
	if err := c.PutBlock(blk, meta); err != nil {
		t.Fatalf("PutBlock: %v", err)
	}

	gotBlock, gotMeta, err := c.GetBlockByHash(meta.Hash)
	if err != nil {
		t.Fatalf("GetBlockByHash: %v", err)
	}
	if gotBlock.Timestamp != blk.Timestamp {
		t.Errorf("timestamp: got %d, want %d", gotBlock.Timestamp, blk.Timestamp)
	}
	if gotMeta.Height != 0 {
		t.Errorf("height: got %d, want 0", gotMeta.Height)
	}
}

func TestStore_GetBlockByHash_MultipleBlocks_Good(t *testing.T) {
	c := newTempChain(t)

	hashes := make([]types.Hash, 3)
	for i := uint64(0); i < 3; i++ {
		blk, meta := testBlock(i)
		hashes[i] = meta.Hash
		if err := c.PutBlock(blk, meta); err != nil {
			t.Fatalf("PutBlock(%d): %v", i, err)
		}
	}

	for i := uint64(0); i < 3; i++ {
		_, gotMeta, err := c.GetBlockByHash(hashes[i])
		if err != nil {
			t.Fatalf("GetBlockByHash(%d): %v", i, err)
		}
		if gotMeta.Height != i {
			t.Errorf("hash lookup %d: height = %d, want %d", i, gotMeta.Height, i)
		}
	}
}

func TestStore_PutBlock_RoundTrip_Good(t *testing.T) {
	c := newTempChain(t)

	blk := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Nonce:        42,
			Timestamp:    1770897600,
			Flags:        0,
		},
		MinerTx: testCoinbaseTx(0),
	}
	meta := &BlockMeta{
		Hash:           types.Hash{0xde, 0xad, 0xbe, 0xef},
		Height:         0,
		Timestamp:      1770897600,
		Difficulty:     500,
		CumulativeDiff: 500,
		GeneratedCoins: 1000000,
	}

	if err := c.PutBlock(blk, meta); err != nil {
		t.Fatalf("PutBlock: %v", err)
	}

	gotBlock, gotMeta, err := c.GetBlockByHeight(0)
	if err != nil {
		t.Fatalf("GetBlockByHeight: %v", err)
	}

	if gotBlock.Nonce != 42 {
		t.Errorf("nonce: got %d, want 42", gotBlock.Nonce)
	}
	if gotMeta.Difficulty != 500 {
		t.Errorf("difficulty: got %d, want 500", gotMeta.Difficulty)
	}
	if gotMeta.CumulativeDiff != 500 {
		t.Errorf("cumulative_diff: got %d, want 500", gotMeta.CumulativeDiff)
	}
	if gotMeta.GeneratedCoins != 1000000 {
		t.Errorf("generated_coins: got %d, want 1000000", gotMeta.GeneratedCoins)
	}
}

// --- Bad (expected errors) ---

func TestStore_GetBlockByHeight_NotFound_Bad(t *testing.T) {
	c := newTempChain(t)

	_, _, err := c.GetBlockByHeight(0)
	if err == nil {
		t.Fatal("GetBlockByHeight on empty chain: expected error, got nil")
	}
}

func TestStore_GetBlockByHeight_WrongHeight_Bad(t *testing.T) {
	c := newTempChain(t)

	blk, meta := testBlock(0)
	if err := c.PutBlock(blk, meta); err != nil {
		t.Fatalf("PutBlock: %v", err)
	}

	_, _, err := c.GetBlockByHeight(999)
	if err == nil {
		t.Fatal("GetBlockByHeight(999): expected error, got nil")
	}
}

func TestStore_GetBlockByHash_NotFound_Bad(t *testing.T) {
	c := newTempChain(t)

	bogusHash := types.Hash{0xff, 0xfe, 0xfd}
	_, _, err := c.GetBlockByHash(bogusHash)
	if err == nil {
		t.Fatal("GetBlockByHash(bogus): expected error, got nil")
	}
}

func TestStore_GetBlockByHash_WrongHash_Bad(t *testing.T) {
	c := newTempChain(t)

	blk, meta := testBlock(0)
	if err := c.PutBlock(blk, meta); err != nil {
		t.Fatalf("PutBlock: %v", err)
	}

	differentHash := types.Hash{0xff, 0xff, 0xff}
	_, _, err := c.GetBlockByHash(differentHash)
	if err == nil {
		t.Fatal("GetBlockByHash(wrong hash): expected error, got nil")
	}
}

func TestStore_PutBlock_Overwrite_Bad(t *testing.T) {
	c := newTempChain(t)

	blk1, meta1 := testBlock(0)
	if err := c.PutBlock(blk1, meta1); err != nil {
		t.Fatalf("PutBlock(first): %v", err)
	}

	// Overwrite the same height with different metadata.
	blk2 := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Timestamp:    9999999,
		},
		MinerTx: testCoinbaseTx(0),
	}
	meta2 := &BlockMeta{
		Hash:      types.Hash{0xaa, 0xbb},
		Height:    0,
		Timestamp: 9999999,
	}

	if err := c.PutBlock(blk2, meta2); err != nil {
		t.Fatalf("PutBlock(overwrite): %v", err)
	}

	// The overwritten block should be the one we read back.
	_, gotMeta, err := c.GetBlockByHeight(0)
	if err != nil {
		t.Fatalf("GetBlockByHeight: %v", err)
	}
	if gotMeta.Timestamp != 9999999 {
		t.Errorf("timestamp: got %d, want 9999999 (overwritten)", gotMeta.Timestamp)
	}
}

// --- Ugly (edge cases) ---

func TestStore_PutGetBlock_EmptyChain_Ugly(t *testing.T) {
	c := newTempChain(t)

	height, err := c.Height()
	if err != nil {
		t.Fatalf("Height: %v", err)
	}
	if height != 0 {
		t.Errorf("empty chain height: got %d, want 0", height)
	}
}

func TestStore_PutBlock_HeightZero_Ugly(t *testing.T) {
	c := newTempChain(t)

	blk, meta := testBlock(0)
	if err := c.PutBlock(blk, meta); err != nil {
		t.Fatalf("PutBlock at height 0: %v", err)
	}

	gotBlock, gotMeta, err := c.GetBlockByHeight(0)
	if err != nil {
		t.Fatalf("GetBlockByHeight(0): %v", err)
	}
	if gotBlock == nil {
		t.Fatal("block at height 0 should not be nil")
	}
	if gotMeta.Height != 0 {
		t.Errorf("height: got %d, want 0", gotMeta.Height)
	}
}

func TestStore_GetBlockByHash_ZeroHash_Ugly(t *testing.T) {
	c := newTempChain(t)

	zeroHash := types.Hash{}
	_, _, err := c.GetBlockByHash(zeroHash)
	if err == nil {
		t.Fatal("GetBlockByHash(zero hash): expected error on empty chain, got nil")
	}
}

func TestStore_PutBlock_ZeroHashMeta_Ugly(t *testing.T) {
	c := newTempChain(t)

	blk := &types.Block{
		BlockHeader: types.BlockHeader{MajorVersion: 1},
		MinerTx:     testCoinbaseTx(0),
	}
	meta := &BlockMeta{
		Hash:   types.Hash{}, // zero hash
		Height: 0,
	}

	if err := c.PutBlock(blk, meta); err != nil {
		t.Fatalf("PutBlock with zero hash: %v", err)
	}

	// Should be retrievable by height.
	_, gotMeta, err := c.GetBlockByHeight(0)
	if err != nil {
		t.Fatalf("GetBlockByHeight: %v", err)
	}
	if !gotMeta.Hash.IsZero() {
		t.Errorf("expected zero hash in metadata, got %s", gotMeta.Hash)
	}

	// Should also be retrievable by zero hash via the index.
	_, _, err = c.GetBlockByHash(types.Hash{})
	if err != nil {
		t.Fatalf("GetBlockByHash(zero): %v", err)
	}
}

func TestStore_PutBlock_LargeHeight_Ugly(t *testing.T) {
	c := newTempChain(t)

	// Store a block at a high height to test the height key padding.
	blk := &types.Block{
		BlockHeader: types.BlockHeader{MajorVersion: 1},
		MinerTx:     testCoinbaseTx(9999999),
	}
	meta := &BlockMeta{
		Hash:   types.Hash{0xab},
		Height: 9999999,
	}

	if err := c.PutBlock(blk, meta); err != nil {
		t.Fatalf("PutBlock at large height: %v", err)
	}

	gotBlock, gotMeta, err := c.GetBlockByHeight(9999999)
	if err != nil {
		t.Fatalf("GetBlockByHeight(9999999): %v", err)
	}
	if gotBlock == nil {
		t.Fatal("block at large height should not be nil")
	}
	if gotMeta.Height != 9999999 {
		t.Errorf("height: got %d, want 9999999", gotMeta.Height)
	}
}

func TestStore_PutBlock_TransactionRoundTrip_Ugly(t *testing.T) {
	// Verify that a block with a more complex miner tx round-trips correctly.
	c := newTempChain(t)

	minerTx := types.Transaction{
		Version: 1,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 42}},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 5000000,
				Target: types.TxOutToKey{Key: types.PublicKey{0xaa, 0xbb}},
			},
			types.TxOutputBare{
				Amount: 3000000,
				Target: types.TxOutToKey{Key: types.PublicKey{0xcc, 0xdd}},
			},
		},
		Extra:      wire.EncodeVarint(0),
		Attachment: wire.EncodeVarint(0),
	}

	blk := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Timestamp:    1770897600,
		},
		MinerTx: minerTx,
	}
	meta := &BlockMeta{
		Hash:   types.Hash{0x01, 0x02},
		Height: 42,
	}

	if err := c.PutBlock(blk, meta); err != nil {
		t.Fatalf("PutBlock: %v", err)
	}

	gotBlock, _, err := c.GetBlockByHeight(42)
	if err != nil {
		t.Fatalf("GetBlockByHeight: %v", err)
	}
	if len(gotBlock.MinerTx.Vout) != 2 {
		t.Fatalf("miner tx outputs: got %d, want 2", len(gotBlock.MinerTx.Vout))
	}
	bare, ok := gotBlock.MinerTx.Vout[0].(types.TxOutputBare)
	if !ok {
		t.Fatal("first output should be TxOutputBare")
	}
	if bare.Amount != 5000000 {
		t.Errorf("first output amount: got %d, want 5000000", bare.Amount)
	}
}
