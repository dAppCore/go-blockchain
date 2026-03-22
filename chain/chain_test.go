// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"testing"

	store "dappco.re/go/core/store"
	"dappco.re/go/core/blockchain/types"
	"dappco.re/go/core/blockchain/wire"
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

func TestChain_PutGetTransaction_Good(t *testing.T) {
	c := newTestChain(t)

	tx := &types.Transaction{
		Version: 1,
		Vin: []types.TxInput{
			types.TxInputToKey{
				Amount:     1000000000000,
				KeyImage:   types.KeyImage{0x01},
				EtcDetails: wire.EncodeVarint(0),
			},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 900000000000,
				Target: types.TxOutToKey{Key: types.PublicKey{0x02}},
			},
		},
		Extra:      wire.EncodeVarint(0),
		Attachment: wire.EncodeVarint(0),
	}
	meta := &TxMeta{
		KeeperBlock:         5,
		GlobalOutputIndexes: []uint64{42},
	}

	txHash := types.Hash{0xde, 0xad}
	if err := c.PutTransaction(txHash, tx, meta); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}

	if !c.HasTransaction(txHash) {
		t.Error("HasTransaction: got false, want true")
	}

	gotTx, gotMeta, err := c.GetTransaction(txHash)
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if gotTx.Version != 1 {
		t.Errorf("version: got %d, want 1", gotTx.Version)
	}
	if gotMeta.KeeperBlock != 5 {
		t.Errorf("keeper_block: got %d, want 5", gotMeta.KeeperBlock)
	}
}

func TestChain_KeyImage_Good(t *testing.T) {
	c := newTestChain(t)

	ki := types.KeyImage{0xaa, 0xbb}

	spent, err := c.IsSpent(ki)
	if err != nil {
		t.Fatalf("IsSpent: %v", err)
	}
	if spent {
		t.Error("IsSpent: got true before marking")
	}

	if err := c.MarkSpent(ki, 10); err != nil {
		t.Fatalf("MarkSpent: %v", err)
	}

	spent, err = c.IsSpent(ki)
	if err != nil {
		t.Fatalf("IsSpent: %v", err)
	}
	if !spent {
		t.Error("IsSpent: got false after marking")
	}
}

func TestChain_TopBlock_Empty(t *testing.T) {
	c := newTestChain(t)

	_, _, err := c.TopBlock()
	if err == nil {
		t.Fatal("TopBlock on empty chain: expected error, got nil")
	}
}

func TestChain_GetBlockByHeight_NotFound(t *testing.T) {
	c := newTestChain(t)

	_, _, err := c.GetBlockByHeight(99)
	if err == nil {
		t.Fatal("GetBlockByHeight(99): expected error, got nil")
	}
	want := "Chain.GetBlockByHeight: chain: block 99 not found"
	if got := err.Error(); got != want {
		t.Errorf("error message: got %q, want %q", got, want)
	}
}

func TestChain_GetBlockByHash_NotFound(t *testing.T) {
	c := newTestChain(t)

	bogus := types.Hash{0xff, 0xfe, 0xfd}
	_, _, err := c.GetBlockByHash(bogus)
	if err == nil {
		t.Fatal("GetBlockByHash(bogus): expected error, got nil")
	}
}

func TestChain_GetTransaction_NotFound(t *testing.T) {
	c := newTestChain(t)

	bogus := types.Hash{0xde, 0xad, 0xbe, 0xef}

	if c.HasTransaction(bogus) {
		t.Error("HasTransaction(bogus): got true, want false")
	}

	_, _, err := c.GetTransaction(bogus)
	if err == nil {
		t.Fatal("GetTransaction(bogus): expected error, got nil")
	}
}

func TestChain_GetOutput_NotFound(t *testing.T) {
	c := newTestChain(t)

	_, _, err := c.GetOutput(1000000, 42)
	if err == nil {
		t.Fatal("GetOutput(nonexistent): expected error, got nil")
	}
}

func TestChain_OutputCount_Empty(t *testing.T) {
	c := newTestChain(t)

	count, err := c.OutputCount(999)
	if err != nil {
		t.Fatalf("OutputCount: %v", err)
	}
	if count != 0 {
		t.Errorf("output count for unindexed amount: got %d, want 0", count)
	}
}

func TestChain_IndexOutputs_Zarcanum(t *testing.T) {
	c := newTestChain(t)

	// Transaction with a Zarcanum output (hidden amount, indexed at amount 0).
	tx := &types.Transaction{
		Version: 1,
		Vout: []types.TxOutput{
			types.TxOutputZarcanum{
				StealthAddress:   types.PublicKey{0x01},
				ConcealingPoint:  types.PublicKey{0x02},
				AmountCommitment: types.PublicKey{0x03},
				BlindedAssetID:   types.PublicKey{0x04},
				EncryptedAmount:  42,
				MixAttr:          0,
			},
		},
	}
	txHash := types.Hash{0xaa}

	gindexes, err := c.indexOutputs(txHash, tx)
	if err != nil {
		t.Fatalf("indexOutputs: %v", err)
	}
	if len(gindexes) != 1 {
		t.Fatalf("gindexes length: got %d, want 1", len(gindexes))
	}
	if gindexes[0] != 0 {
		t.Errorf("gindex: got %d, want 0", gindexes[0])
	}

	// Zarcanum outputs are indexed with amount=0.
	count, err := c.OutputCount(0)
	if err != nil {
		t.Fatalf("OutputCount(0): %v", err)
	}
	if count != 1 {
		t.Errorf("output count for amount 0: got %d, want 1", count)
	}
}

func TestChain_OutputIndex_Good(t *testing.T) {
	c := newTestChain(t)

	txID := types.Hash{0x01}

	gidx0, err := c.PutOutput(1000000000000, txID, 0)
	if err != nil {
		t.Fatalf("PutOutput(0): %v", err)
	}
	if gidx0 != 0 {
		t.Errorf("gindex: got %d, want 0", gidx0)
	}

	gidx1, err := c.PutOutput(1000000000000, txID, 1)
	if err != nil {
		t.Fatalf("PutOutput(1): %v", err)
	}
	if gidx1 != 1 {
		t.Errorf("gindex: got %d, want 1", gidx1)
	}

	count, err := c.OutputCount(1000000000000)
	if err != nil {
		t.Fatalf("OutputCount: %v", err)
	}
	if count != 2 {
		t.Errorf("count: got %d, want 2", count)
	}

	gotTxID, gotOutNo, err := c.GetOutput(1000000000000, 0)
	if err != nil {
		t.Fatalf("GetOutput: %v", err)
	}
	if gotTxID != txID {
		t.Errorf("tx_id mismatch")
	}
	if gotOutNo != 0 {
		t.Errorf("out_no: got %d, want 0", gotOutNo)
	}
}
