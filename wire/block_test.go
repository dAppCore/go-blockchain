// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wire

import (
	"bytes"
	"testing"

	"dappco.re/go/core/blockchain/types"
)

// testnetGenesisHeader returns the genesis block header for the Lethean testnet.
func testnetGenesisHeader() types.BlockHeader {
	return types.BlockHeader{
		MajorVersion: 1,
		Nonce:        101011010221, // CURRENCY_FORMATION_VERSION(100) + 101011010121
		PrevID:       types.Hash{}, // all zeros
		MinorVersion: 0,
		Timestamp:    1770897600, // 2026-02-12 12:00:00 UTC
		Flags:        0,
	}
}

func TestEncodeBlockHeader_Good(t *testing.T) {
	h := testnetGenesisHeader()

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeBlockHeader(enc, &h)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	data := buf.Bytes()

	// Verify structure:
	// byte 0:    major_version = 0x01
	// bytes 1-8: nonce LE
	// bytes 9-40: prev_id (32 zeros)
	// byte 41:   minor_version varint = 0x00
	// bytes 42+: timestamp varint
	// last byte: flags = 0x00

	if data[0] != 0x01 {
		t.Errorf("major_version: got 0x%02x, want 0x01", data[0])
	}
	if data[len(data)-1] != 0x00 {
		t.Errorf("flags: got 0x%02x, want 0x00", data[len(data)-1])
	}
}

func TestBlockHeaderRoundTrip_Good(t *testing.T) {
	h := testnetGenesisHeader()

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeBlockHeader(enc, &h)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	got := DecodeBlockHeader(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	if got.MajorVersion != h.MajorVersion {
		t.Errorf("MajorVersion: got %d, want %d", got.MajorVersion, h.MajorVersion)
	}
	if got.Nonce != h.Nonce {
		t.Errorf("Nonce: got %d, want %d", got.Nonce, h.Nonce)
	}
	if got.PrevID != h.PrevID {
		t.Errorf("PrevID: got %x, want %x", got.PrevID, h.PrevID)
	}
	if got.MinorVersion != h.MinorVersion {
		t.Errorf("MinorVersion: got %d, want %d", got.MinorVersion, h.MinorVersion)
	}
	if got.Timestamp != h.Timestamp {
		t.Errorf("Timestamp: got %d, want %d", got.Timestamp, h.Timestamp)
	}
	if got.Flags != h.Flags {
		t.Errorf("Flags: got %d, want %d", got.Flags, h.Flags)
	}
}

func TestBlockRoundTrip_Good(t *testing.T) {
	// Build the genesis block and round-trip it through EncodeBlock/DecodeBlock.
	rawTx := testnetGenesisRawTx()
	dec := NewDecoder(bytes.NewReader(rawTx))
	minerTx := DecodeTransaction(dec)
	if dec.Err() != nil {
		t.Fatalf("decode miner tx: %v", dec.Err())
	}

	block := types.Block{
		BlockHeader: testnetGenesisHeader(),
		MinerTx:     minerTx,
	}

	// Encode.
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeBlock(enc, &block)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	// Decode.
	dec2 := NewDecoder(bytes.NewReader(buf.Bytes()))
	got := DecodeBlock(dec2)
	if dec2.Err() != nil {
		t.Fatalf("decode error: %v", dec2.Err())
	}

	// Re-encode and compare bytes.
	var rtBuf bytes.Buffer
	enc2 := NewEncoder(&rtBuf)
	EncodeBlock(enc2, &got)
	if enc2.Err() != nil {
		t.Fatalf("re-encode error: %v", enc2.Err())
	}
	if !bytes.Equal(rtBuf.Bytes(), buf.Bytes()) {
		t.Errorf("block round-trip mismatch")
	}

	// Verify block hash is unchanged after round-trip.
	if BlockHash(&got) != BlockHash(&block) {
		t.Errorf("block hash changed after round-trip")
	}
}

func TestBlockWithTxHashesRoundTrip_Good(t *testing.T) {
	block := types.Block{
		BlockHeader: testnetGenesisHeader(),
		MinerTx: types.Transaction{
			Version: 1,
			Vin:     []types.TxInput{types.TxInputGenesis{Height: 0}},
			Vout: []types.TxOutput{types.TxOutputBare{
				Amount: 1000,
				Target: types.TxOutToKey{Key: types.PublicKey{0xAA}},
			}},
			Extra:      EncodeVarint(0),
			Attachment: EncodeVarint(0),
		},
		TxHashes: []types.Hash{
			{0x01, 0x02, 0x03},
			{0xDE, 0xAD, 0xBE, 0xEF},
		},
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeBlock(enc, &block)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	got := DecodeBlock(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	if len(got.TxHashes) != 2 {
		t.Fatalf("tx_hashes count: got %d, want 2", len(got.TxHashes))
	}
	if got.TxHashes[0] != block.TxHashes[0] {
		t.Errorf("tx_hashes[0]: got %x, want %x", got.TxHashes[0], block.TxHashes[0])
	}
	if got.TxHashes[1] != block.TxHashes[1] {
		t.Errorf("tx_hashes[1]: got %x, want %x", got.TxHashes[1], block.TxHashes[1])
	}
}
