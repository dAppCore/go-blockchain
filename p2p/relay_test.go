// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package p2p

import (
	"bytes"
	"testing"

	"dappco.re/go/core/p2p/node/levin"
)

func TestNewBlockNotification_Good_Roundtrip(t *testing.T) {
	original := NewBlockNotification{
		BlockBlob: []byte{0x01, 0x02, 0x03},
		TxBlobs:   [][]byte{{0xAA}, {0xBB, 0xCC}},
		Height:    6300,
	}
	data, err := original.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var got NewBlockNotification
	if err := got.Decode(data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Height != 6300 {
		t.Errorf("height: got %d, want 6300", got.Height)
	}
	if !bytes.Equal(got.BlockBlob, original.BlockBlob) {
		t.Errorf("block: got %x, want %x", got.BlockBlob, original.BlockBlob)
	}
	if len(got.TxBlobs) != 2 {
		t.Fatalf("txs: got %d, want 2", len(got.TxBlobs))
	}
	if !bytes.Equal(got.TxBlobs[0], []byte{0xAA}) {
		t.Errorf("tx[0]: got %x, want AA", got.TxBlobs[0])
	}
}

func TestNewTransactionsNotification_Good_Roundtrip(t *testing.T) {
	original := NewTransactionsNotification{
		TxBlobs: [][]byte{{0x01}, {0x02}, {0x03}},
	}
	data, err := original.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var got NewTransactionsNotification
	if err := got.Decode(data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.TxBlobs) != 3 {
		t.Errorf("txs: got %d, want 3", len(got.TxBlobs))
	}
}

func TestRequestChain_Good_Roundtrip(t *testing.T) {
	hash := make([]byte, 32)
	hash[0] = 0xFF
	original := RequestChain{BlockIDs: [][]byte{hash}}
	data, err := original.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	// Decode back via storage — block_ids is a single concatenated blob
	// (KV_SERIALIZE_CONTAINER_POD_AS_BLOB in C++).
	s, err := levin.DecodeStorage(data)
	if err != nil {
		t.Fatalf("decode storage: %v", err)
	}
	if v, ok := s["block_ids"]; ok {
		blob, err := v.AsString()
		if err != nil {
			t.Fatalf("AsString: %v", err)
		}
		if len(blob) != 32 || blob[0] != 0xFF {
			t.Errorf("block_ids roundtrip failed: len=%d, first=%x", len(blob), blob[0])
		}
	} else {
		t.Error("block_ids not found in decoded storage")
	}
}

func TestResponseChainEntry_Good_Decode(t *testing.T) {
	hash := make([]byte, 32)
	hash[31] = 0xAB
	// m_block_ids is an object array of block_context_info,
	// each with "h" (hash blob) and "cumul_size" (uint64).
	entry := levin.Section{
		"h":          levin.StringVal(hash),
		"cumul_size": levin.Uint64Val(1234),
	}
	s := levin.Section{
		"start_height": levin.Uint64Val(100),
		"total_height": levin.Uint64Val(6300),
		"m_block_ids":  levin.ObjectArrayVal([]levin.Section{entry}),
	}
	data, err := levin.EncodeStorage(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var resp ResponseChainEntry
	if err := resp.Decode(data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.StartHeight != 100 {
		t.Errorf("start_height: got %d, want 100", resp.StartHeight)
	}
	if resp.TotalHeight != 6300 {
		t.Errorf("total_height: got %d, want 6300", resp.TotalHeight)
	}
	if len(resp.BlockIDs) != 1 {
		t.Fatalf("block_ids: got %d, want 1", len(resp.BlockIDs))
	}
	if resp.BlockIDs[0][31] != 0xAB {
		t.Errorf("block_ids[0][31]: got %x, want AB", resp.BlockIDs[0][31])
	}
	if len(resp.Blocks) != 1 {
		t.Fatalf("blocks: got %d, want 1", len(resp.Blocks))
	}
	if resp.Blocks[0].CumulSize != 1234 {
		t.Errorf("cumul_size: got %d, want 1234", resp.Blocks[0].CumulSize)
	}
}

func TestRequestGetObjects_RoundTrip(t *testing.T) {
	req := RequestGetObjects{
		Blocks: [][]byte{
			make([]byte, 32), // zero hash
			{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
				17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
		},
	}
	data, err := req.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded RequestGetObjects
	if err := decoded.Decode(data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded.Blocks) != 2 {
		t.Fatalf("blocks: got %d, want 2", len(decoded.Blocks))
	}
	if !bytes.Equal(decoded.Blocks[0], req.Blocks[0]) {
		t.Errorf("blocks[0]: got %x, want %x", decoded.Blocks[0], req.Blocks[0])
	}
	if !bytes.Equal(decoded.Blocks[1], req.Blocks[1]) {
		t.Errorf("blocks[1]: got %x, want %x", decoded.Blocks[1], req.Blocks[1])
	}
}

func TestRequestGetObjects_WithTxs(t *testing.T) {
	txHash := make([]byte, 32)
	txHash[0] = 0xAA
	txHash[1] = 0xBB
	txHash[2] = 0xCC
	req := RequestGetObjects{
		Blocks: [][]byte{make([]byte, 32)},
		Txs:    [][]byte{txHash},
	}
	data, err := req.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded RequestGetObjects
	if err := decoded.Decode(data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded.Txs) != 1 {
		t.Fatalf("txs: got %d, want 1", len(decoded.Txs))
	}
	if !bytes.Equal(decoded.Txs[0], req.Txs[0]) {
		t.Errorf("txs[0]: got %x, want %x", decoded.Txs[0], req.Txs[0])
	}
}

func TestRequestGetObjects_Empty(t *testing.T) {
	req := RequestGetObjects{}
	data, err := req.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded RequestGetObjects
	if err := decoded.Decode(data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded.Blocks) != 0 {
		t.Errorf("blocks: got %d, want 0", len(decoded.Blocks))
	}
	if len(decoded.Txs) != 0 {
		t.Errorf("txs: got %d, want 0", len(decoded.Txs))
	}
}

func TestResponseGetObjects_Decode(t *testing.T) {
	// Build a ResponseGetObjects via portable storage sections, simulating
	// what a peer would send.
	blockEntry1 := levin.Section{
		"block": levin.StringVal([]byte{0x01, 0x02, 0x03}),
		"txs":   levin.StringArrayVal([][]byte{{0xAA}, {0xBB}}),
	}
	blockEntry2 := levin.Section{
		"block": levin.StringVal([]byte{0x04, 0x05}),
		"txs":   levin.StringArrayVal([][]byte{}),
	}
	missedHash := make([]byte, 32)
	missedHash[0] = 0xFF
	// missed_ids uses KV_SERIALIZE_CONTAINER_POD_AS_BLOB in C++.
	s := levin.Section{
		"blocks":                   levin.ObjectArrayVal([]levin.Section{blockEntry1, blockEntry2}),
		"missed_ids":               levin.StringVal(missedHash),
		"current_blockchain_height": levin.Uint64Val(6300),
	}
	data, err := levin.EncodeStorage(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var resp ResponseGetObjects
	if err := resp.Decode(data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.CurrentHeight != 6300 {
		t.Errorf("height: got %d, want 6300", resp.CurrentHeight)
	}
	if len(resp.Blocks) != 2 {
		t.Fatalf("blocks: got %d, want 2", len(resp.Blocks))
	}
	if !bytes.Equal(resp.Blocks[0].Block, []byte{0x01, 0x02, 0x03}) {
		t.Errorf("blocks[0].block: got %x, want 010203", resp.Blocks[0].Block)
	}
	if len(resp.Blocks[0].Txs) != 2 {
		t.Fatalf("blocks[0].txs: got %d, want 2", len(resp.Blocks[0].Txs))
	}
	if !bytes.Equal(resp.Blocks[0].Txs[0], []byte{0xAA}) {
		t.Errorf("blocks[0].txs[0]: got %x, want AA", resp.Blocks[0].Txs[0])
	}
	if !bytes.Equal(resp.Blocks[1].Block, []byte{0x04, 0x05}) {
		t.Errorf("blocks[1].block: got %x, want 0405", resp.Blocks[1].Block)
	}
	if len(resp.Blocks[1].Txs) != 0 {
		t.Errorf("blocks[1].txs: got %d, want 0", len(resp.Blocks[1].Txs))
	}
	if len(resp.MissedIDs) != 1 {
		t.Fatalf("missed_ids: got %d, want 1", len(resp.MissedIDs))
	}
	if resp.MissedIDs[0][0] != 0xFF {
		t.Errorf("missed_ids[0][0]: got %x, want FF", resp.MissedIDs[0][0])
	}
}

func TestResponseGetObjects_Empty(t *testing.T) {
	s := levin.Section{
		"blocks":                   levin.ObjectArrayVal([]levin.Section{}),
		"current_blockchain_height": levin.Uint64Val(0),
	}
	data, err := levin.EncodeStorage(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var resp ResponseGetObjects
	if err := resp.Decode(data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Blocks) != 0 {
		t.Errorf("blocks: got %d, want 0", len(resp.Blocks))
	}
	if resp.CurrentHeight != 0 {
		t.Errorf("height: got %d, want 0", resp.CurrentHeight)
	}
}

func TestResponseGetObjects_Encode_RoundTrip(t *testing.T) {
	resp := ResponseGetObjects{
		Blocks: []BlockCompleteEntry{
			{
				Block: []byte{0x01, 0x02},
				Txs:   [][]byte{{0xAA}, {0xBB}},
			},
			{
				Block: []byte{0x03},
				Txs:   [][]byte{},
			},
		},
		MissedIDs:     [][]byte{make([]byte, 32)},
		CurrentHeight: 42,
	}
	data, err := resp.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded ResponseGetObjects
	if err := decoded.Decode(data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.CurrentHeight != 42 {
		t.Errorf("height: got %d, want 42", decoded.CurrentHeight)
	}
	if len(decoded.Blocks) != 2 {
		t.Fatalf("blocks: got %d, want 2", len(decoded.Blocks))
	}
	if !bytes.Equal(decoded.Blocks[0].Block, []byte{0x01, 0x02}) {
		t.Errorf("blocks[0].block: got %x, want 0102", decoded.Blocks[0].Block)
	}
	if len(decoded.Blocks[0].Txs) != 2 {
		t.Errorf("blocks[0].txs: got %d, want 2", len(decoded.Blocks[0].Txs))
	}
	if !bytes.Equal(decoded.Blocks[1].Block, []byte{0x03}) {
		t.Errorf("blocks[1].block: got %x, want 03", decoded.Blocks[1].Block)
	}
	if len(decoded.MissedIDs) != 1 {
		t.Fatalf("missed_ids: got %d, want 1", len(decoded.MissedIDs))
	}
}

func TestTimedSyncRequest_Good_Encode(t *testing.T) {
	req := TimedSyncRequest{
		PayloadData: CoreSyncData{CurrentHeight: 42},
	}
	data, err := req.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty encoding")
	}
	// Verify it can be decoded
	s, err := levin.DecodeStorage(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if v, ok := s["payload_data"]; ok {
		obj, err := v.AsSection()
		if err != nil {
			t.Fatalf("payload_data: %v", err)
		}
		if h, ok := obj["current_height"]; ok {
			height, _ := h.AsUint64()
			if height != 42 {
				t.Errorf("height: got %d, want 42", height)
			}
		}
	}
}

func TestTimedSyncResponse_Good_Decode(t *testing.T) {
	sync := CoreSyncData{CurrentHeight: 500}
	s := levin.Section{
		"local_time":   levin.Int64Val(1708444800),
		"payload_data": levin.ObjectVal(sync.MarshalSection()),
	}
	data, err := levin.EncodeStorage(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var resp TimedSyncResponse
	if err := resp.Decode(data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.LocalTime != 1708444800 {
		t.Errorf("local_time: got %d, want 1708444800", resp.LocalTime)
	}
	if resp.PayloadData.CurrentHeight != 500 {
		t.Errorf("height: got %d, want 500", resp.PayloadData.CurrentHeight)
	}
}
