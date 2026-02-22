// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"testing"
	"time"

	store "forge.lthn.ai/core/go-store"

	"forge.lthn.ai/core/go-blockchain/chain"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

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

// seedChain creates an in-memory chain with n blocks.
func seedChain(t *testing.T, n int) *chain.Chain {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	c := chain.New(s)
	for i := 0; i < n; i++ {
		blk := &types.Block{
			BlockHeader: types.BlockHeader{
				MajorVersion: 1,
				Timestamp:    1770897600 + uint64(i)*120,
			},
			MinerTx: testCoinbaseTx(uint64(i)),
		}
		meta := &chain.BlockMeta{
			Hash:       types.Hash{byte(i)},
			Height:     uint64(i),
			Timestamp:  1770897600 + uint64(i)*120,
			Difficulty: 1000 + uint64(i)*100,
		}
		if err := c.PutBlock(blk, meta); err != nil {
			t.Fatalf("PutBlock(%d): %v", i, err)
		}
	}
	return c
}

func TestNewNode_Good(t *testing.T) {
	c := seedChain(t, 1)
	n := NewNode(c)
	if n == nil {
		t.Fatal("NewNode returned nil")
	}
	if n.Chain() != c {
		t.Error("Chain() does not return the same chain passed to NewNode")
	}
}

func TestNode_Status_Good(t *testing.T) {
	c := seedChain(t, 10)
	n := NewNode(c)

	status, err := n.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Height != 10 {
		t.Errorf("height: got %d, want 10", status.Height)
	}
	// Top block is at height 9, difficulty = 1000 + 9*100 = 1900.
	if status.Difficulty != 1900 {
		t.Errorf("difficulty: got %d, want 1900", status.Difficulty)
	}
	wantHash := types.Hash{9}
	if status.TopHash != wantHash {
		t.Errorf("top hash: got %x, want %x", status.TopHash, wantHash)
	}
	wantTime := time.Unix(int64(1770897600+9*120), 0)
	if !status.TipTime.Equal(wantTime) {
		t.Errorf("tip time: got %v, want %v", status.TipTime, wantTime)
	}
}

func TestNode_Status_Empty(t *testing.T) {
	c := seedChain(t, 0)
	n := NewNode(c)

	status, err := n.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Height != 0 {
		t.Errorf("height: got %d, want 0", status.Height)
	}
}

func TestNode_WaitForStatus_Good(t *testing.T) {
	c := seedChain(t, 3)
	n := NewNode(c)

	cmd := n.WaitForStatus()
	if cmd == nil {
		t.Fatal("WaitForStatus returned nil cmd")
	}

	msg := cmd()
	status, ok := msg.(NodeStatusMsg)
	if !ok {
		t.Fatalf("WaitForStatus cmd returned %T, want NodeStatusMsg", msg)
	}
	if status.Height != 3 {
		t.Errorf("height: got %d, want 3", status.Height)
	}
}

func TestNode_Tick_Good(t *testing.T) {
	c := seedChain(t, 5)
	n := NewNode(c)
	n.interval = 50 * time.Millisecond

	start := time.Now()
	msg := n.Tick()()
	elapsed := time.Since(start)

	status, ok := msg.(NodeStatusMsg)
	if !ok {
		t.Fatalf("Tick cmd returned %T, want NodeStatusMsg", msg)
	}
	if status.Height != 5 {
		t.Errorf("height: got %d, want 5", status.Height)
	}
	if elapsed < 40*time.Millisecond {
		t.Errorf("Tick returned too quickly (%v), expected at least ~50ms", elapsed)
	}
}
