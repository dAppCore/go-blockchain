// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"testing"

	"dappco.re/go/core/blockchain/types"
	"dappco.re/go/core/blockchain/wire"
)

func TestGetRingOutputs_Good(t *testing.T) {
	c := newTestChain(t)

	pubKey := types.PublicKey{1, 2, 3}
	tx := types.Transaction{
		Version: types.VersionPreHF4,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 0}},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 1000,
				Target: types.TxOutToKey{Key: pubKey, MixAttr: 0},
			},
		},
		Extra:      wire.EncodeVarint(0),
		Attachment: wire.EncodeVarint(0),
	}
	txHash := wire.TransactionHash(&tx)

	err := c.PutTransaction(txHash, &tx, &TxMeta{KeeperBlock: 0, GlobalOutputIndexes: []uint64{0}})
	if err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}

	gidx, err := c.PutOutput(1000, txHash, 0)
	if err != nil {
		t.Fatalf("PutOutput: %v", err)
	}
	if gidx != 0 {
		t.Fatalf("gidx: got %d, want 0", gidx)
	}

	pubs, err := c.GetRingOutputs(1000, []uint64{0})
	if err != nil {
		t.Fatalf("GetRingOutputs: %v", err)
	}
	if len(pubs) != 1 {
		t.Fatalf("pubs length: got %d, want 1", len(pubs))
	}
	if pubs[0] != pubKey {
		t.Errorf("pubs[0]: got %x, want %x", pubs[0], pubKey)
	}
}

func TestGetRingOutputs_Good_MultipleOutputs(t *testing.T) {
	c := newTestChain(t)

	key1 := types.PublicKey{0x11}
	key2 := types.PublicKey{0x22}

	tx1 := types.Transaction{
		Version: types.VersionPreHF4,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 0}},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 500, Target: types.TxOutToKey{Key: key1}},
		},
		Extra:      wire.EncodeVarint(0),
		Attachment: wire.EncodeVarint(0),
	}
	tx1Hash := wire.TransactionHash(&tx1)

	tx2 := types.Transaction{
		Version: types.VersionPreHF4,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 1}},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 500, Target: types.TxOutToKey{Key: key2}},
		},
		Extra:      wire.EncodeVarint(0),
		Attachment: wire.EncodeVarint(0),
	}
	tx2Hash := wire.TransactionHash(&tx2)

	if err := c.PutTransaction(tx1Hash, &tx1, &TxMeta{KeeperBlock: 0, GlobalOutputIndexes: []uint64{0}}); err != nil {
		t.Fatalf("PutTransaction(tx1): %v", err)
	}
	if err := c.PutTransaction(tx2Hash, &tx2, &TxMeta{KeeperBlock: 1, GlobalOutputIndexes: []uint64{1}}); err != nil {
		t.Fatalf("PutTransaction(tx2): %v", err)
	}

	if _, err := c.PutOutput(500, tx1Hash, 0); err != nil {
		t.Fatalf("PutOutput(tx1): %v", err)
	}
	if _, err := c.PutOutput(500, tx2Hash, 0); err != nil {
		t.Fatalf("PutOutput(tx2): %v", err)
	}

	pubs, err := c.GetRingOutputs(500, []uint64{0, 1})
	if err != nil {
		t.Fatalf("GetRingOutputs: %v", err)
	}
	if len(pubs) != 2 {
		t.Fatalf("pubs length: got %d, want 2", len(pubs))
	}
	if pubs[0] != key1 {
		t.Errorf("pubs[0]: got %x, want %x", pubs[0], key1)
	}
	if pubs[1] != key2 {
		t.Errorf("pubs[1]: got %x, want %x", pubs[1], key2)
	}
}

func TestGetRingOutputs_Bad_OutputNotFound(t *testing.T) {
	c := newTestChain(t)

	_, err := c.GetRingOutputs(1000, []uint64{99})
	if err == nil {
		t.Fatal("GetRingOutputs: expected error for missing output, got nil")
	}
}

func TestGetRingOutputs_Bad_TxNotFound(t *testing.T) {
	c := newTestChain(t)

	// Index an output pointing to a transaction that does not exist in the store.
	fakeTxHash := types.Hash{0xde, 0xad}
	if _, err := c.PutOutput(1000, fakeTxHash, 0); err != nil {
		t.Fatalf("PutOutput: %v", err)
	}

	_, err := c.GetRingOutputs(1000, []uint64{0})
	if err == nil {
		t.Fatal("GetRingOutputs: expected error for missing tx, got nil")
	}
}

func TestGetRingOutputs_Bad_OutputIndexOutOfRange(t *testing.T) {
	c := newTestChain(t)

	tx := types.Transaction{
		Version: types.VersionPreHF4,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 0}},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 1000, Target: types.TxOutToKey{Key: types.PublicKey{0x01}}},
		},
		Extra:      wire.EncodeVarint(0),
		Attachment: wire.EncodeVarint(0),
	}
	txHash := wire.TransactionHash(&tx)

	if err := c.PutTransaction(txHash, &tx, &TxMeta{KeeperBlock: 0, GlobalOutputIndexes: []uint64{0}}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}

	// Index with outNo=5, which is beyond the transaction's single output.
	if _, err := c.PutOutput(1000, txHash, 5); err != nil {
		t.Fatalf("PutOutput: %v", err)
	}

	_, err := c.GetRingOutputs(1000, []uint64{0})
	if err == nil {
		t.Fatal("GetRingOutputs: expected error for out-of-range index, got nil")
	}
}

func TestGetRingOutputs_Good_EmptyOffsets(t *testing.T) {
	c := newTestChain(t)

	pubs, err := c.GetRingOutputs(1000, []uint64{})
	if err != nil {
		t.Fatalf("GetRingOutputs: %v", err)
	}
	if len(pubs) != 0 {
		t.Errorf("pubs length: got %d, want 0", len(pubs))
	}
}
