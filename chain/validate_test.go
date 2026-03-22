// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"testing"

	store "dappco.re/go/core/store"
	"dappco.re/go/core/blockchain/types"
)

func TestValidateHeader_Good_Genesis(t *testing.T) {
	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	blk := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Timestamp:    1770897600,
		},
		MinerTx: testCoinbaseTx(0),
	}

	err := c.ValidateHeader(blk, 0)
	if err != nil {
		t.Fatalf("ValidateHeader genesis: %v", err)
	}
}

func TestValidateHeader_Good_Sequential(t *testing.T) {
	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	// Store block 0.
	blk0 := &types.Block{
		BlockHeader: types.BlockHeader{MajorVersion: 1, Timestamp: 1770897600},
		MinerTx:     testCoinbaseTx(0),
	}
	hash0 := types.Hash{0x01}
	c.PutBlock(blk0, &BlockMeta{Hash: hash0, Height: 0, Timestamp: 1770897600})

	// Validate block 1.
	blk1 := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Timestamp:    1770897720,
			PrevID:       hash0,
		},
		MinerTx: testCoinbaseTx(1),
	}

	err := c.ValidateHeader(blk1, 1)
	if err != nil {
		t.Fatalf("ValidateHeader block 1: %v", err)
	}
}

func TestValidateHeader_Bad_WrongPrevID(t *testing.T) {
	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	blk0 := &types.Block{
		BlockHeader: types.BlockHeader{MajorVersion: 1, Timestamp: 1770897600},
		MinerTx:     testCoinbaseTx(0),
	}
	c.PutBlock(blk0, &BlockMeta{Hash: types.Hash{0x01}, Height: 0})

	blk1 := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Timestamp:    1770897720,
			PrevID:       types.Hash{0xFF}, // wrong
		},
		MinerTx: testCoinbaseTx(1),
	}

	err := c.ValidateHeader(blk1, 1)
	if err == nil {
		t.Fatal("expected error for wrong prev_id")
	}
}

func TestValidateHeader_Bad_WrongHeight(t *testing.T) {
	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	blk := &types.Block{
		BlockHeader: types.BlockHeader{MajorVersion: 1, Timestamp: 1770897600},
		MinerTx:     testCoinbaseTx(0),
	}

	// Chain is empty (height 0), but we pass expectedHeight=5.
	err := c.ValidateHeader(blk, 5)
	if err == nil {
		t.Fatal("expected error for wrong height")
	}
}

func TestValidateHeader_Bad_GenesisNonZeroPrev(t *testing.T) {
	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	blk := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			PrevID:       types.Hash{0xFF}, // genesis must have zero prev_id
		},
		MinerTx: testCoinbaseTx(0),
	}

	err := c.ValidateHeader(blk, 0)
	if err == nil {
		t.Fatal("expected error for genesis with non-zero prev_id")
	}
}
