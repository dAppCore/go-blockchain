// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package p2p

import (
	"bytes"
	"testing"

	"forge.lthn.ai/core/go-p2p/node/levin"
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

	// Decode back via storage
	s, err := levin.DecodeStorage(data)
	if err != nil {
		t.Fatalf("decode storage: %v", err)
	}
	if v, ok := s["block_ids"]; ok {
		ids, err := v.AsStringArray()
		if err != nil {
			t.Fatalf("AsStringArray: %v", err)
		}
		if len(ids) != 1 || ids[0][0] != 0xFF {
			t.Errorf("block_ids roundtrip failed")
		}
	} else {
		t.Error("block_ids not found in decoded storage")
	}
}

func TestResponseChainEntry_Good_Decode(t *testing.T) {
	hash := make([]byte, 32)
	hash[31] = 0xAB
	s := levin.Section{
		"start_height": levin.Uint64Val(100),
		"total_height": levin.Uint64Val(6300),
		"m_block_ids":  levin.StringArrayVal([][]byte{hash}),
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
