// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"testing"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

func newTestChain(t *testing.T) *Chain {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return New(s)
}

// testCoinbaseTx returns a minimal v1 coinbase transaction that round-trips
// cleanly through the wire encoder/decoder.
func testCoinbaseTx(height uint64) types.Transaction {
	return types.Transaction{
		Version: 1,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: height}},
		Vout: []types.TxOutput{types.TxOutputBare{
			Amount: 1000000,
			Target: types.TxOutToKey{Key: types.PublicKey{0x01}},
		}},
		Extra:      wire.EncodeVarint(0),
		Attachment: wire.EncodeVarint(0),
	}
}

func TestChain_Height_Empty(t *testing.T) {
	c := newTestChain(t)
	h, err := c.Height()
	if err != nil {
		t.Fatalf("Height: %v", err)
	}
	if h != 0 {
		t.Errorf("height: got %d, want 0", h)
	}
}

func TestChain_PutGetBlock_Good(t *testing.T) {
	c := newTestChain(t)

	blk := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Timestamp:    1770897600,
		},
		MinerTx: testCoinbaseTx(0),
	}
	meta := &BlockMeta{
		Hash:       types.Hash{0xab, 0xcd},
		Height:     0,
		Timestamp:  1770897600,
		Difficulty: 1,
	}

	if err := c.PutBlock(blk, meta); err != nil {
		t.Fatalf("PutBlock: %v", err)
	}

	h, err := c.Height()
	if err != nil {
		t.Fatalf("Height: %v", err)
	}
	if h != 1 {
		t.Errorf("height: got %d, want 1", h)
	}

	gotBlk, gotMeta, err := c.GetBlockByHeight(0)
	if err != nil {
		t.Fatalf("GetBlockByHeight: %v", err)
	}
	if gotBlk.MajorVersion != 1 {
		t.Errorf("major_version: got %d, want 1", gotBlk.MajorVersion)
	}
	if gotMeta.Hash != meta.Hash {
		t.Errorf("hash mismatch")
	}

	gotBlk2, gotMeta2, err := c.GetBlockByHash(meta.Hash)
	if err != nil {
		t.Fatalf("GetBlockByHash: %v", err)
	}
	if gotBlk2.Timestamp != blk.Timestamp {
		t.Errorf("timestamp mismatch")
	}
	if gotMeta2.Height != 0 {
		t.Errorf("height: got %d, want 0", gotMeta2.Height)
	}
}

func TestChain_TopBlock_Good(t *testing.T) {
	c := newTestChain(t)

	for i := uint64(0); i < 3; i++ {
		blk := &types.Block{
			BlockHeader: types.BlockHeader{
				MajorVersion: 1,
				Timestamp:    1770897600 + i*120,
			},
			MinerTx: testCoinbaseTx(i),
		}
		meta := &BlockMeta{
			Hash:   types.Hash{byte(i)},
			Height: i,
		}
		if err := c.PutBlock(blk, meta); err != nil {
			t.Fatalf("PutBlock(%d): %v", i, err)
		}
	}

	_, topMeta, err := c.TopBlock()
	if err != nil {
		t.Fatalf("TopBlock: %v", err)
	}
	if topMeta.Height != 2 {
		t.Errorf("top height: got %d, want 2", topMeta.Height)
	}
}
