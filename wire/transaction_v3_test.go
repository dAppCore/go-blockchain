// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wire

import (
	"bytes"
	"testing"
)

// buildAssetDescriptorOpBlob constructs a minimal asset_descriptor_operation
// binary blob (version 1, operation_type=register, with descriptor, no asset_id).
func buildAssetDescriptorOpBlob() []byte {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	// ver: uint8 = 1
	enc.WriteUint8(1)
	// operation_type: uint8 = 0 (register)
	enc.WriteUint8(0)
	// opt_asset_id: absent (marker = 0)
	enc.WriteUint8(0)
	// opt_descriptor: present (marker = 1)
	enc.WriteUint8(1)
	// -- AssetDescriptorBase --
	// ticker: string "LTHN" (varint len + bytes)
	enc.WriteVarint(4)
	enc.WriteBytes([]byte("LTHN"))
	// full_name: string "Lethean" (varint len + bytes)
	enc.WriteVarint(7)
	enc.WriteBytes([]byte("Lethean"))
	// total_max_supply: uint64 LE
	enc.WriteUint64LE(1000000)
	// current_supply: uint64 LE
	enc.WriteUint64LE(0)
	// decimal_point: uint8
	enc.WriteUint8(12)
	// meta_info: string "" (empty)
	enc.WriteVarint(0)
	// owner_key: 32 bytes
	enc.WriteBytes(make([]byte, 32))
	// etc: vector<uint8> (empty)
	enc.WriteVarint(0)
	// -- end AssetDescriptorBase --
	// amount_to_emit: uint64 LE
	enc.WriteUint64LE(0)
	// amount_to_burn: uint64 LE
	enc.WriteUint64LE(0)
	// etc: vector<uint8> (empty)
	enc.WriteVarint(0)

	return buf.Bytes()
}

func TestReadAssetDescriptorOperation_Good(t *testing.T) {
	blob := buildAssetDescriptorOpBlob()
	dec := NewDecoder(bytes.NewReader(blob))
	got := readAssetDescriptorOperation(dec)
	if dec.Err() != nil {
		t.Fatalf("readAssetDescriptorOperation failed: %v", dec.Err())
	}
	if !bytes.Equal(got, blob) {
		t.Fatalf("round-trip mismatch: got %d bytes, want %d bytes", len(got), len(blob))
	}
}

func TestReadAssetDescriptorOperation_Bad(t *testing.T) {
	// Truncated blob — should error.
	dec := NewDecoder(bytes.NewReader([]byte{1, 0}))
	_ = readAssetDescriptorOperation(dec)
	if dec.Err() == nil {
		t.Fatal("expected error for truncated asset descriptor operation")
	}
}

// buildAssetDescriptorOpEmitBlob constructs an emit operation (version 1,
// operation_type=1, with asset_id, no descriptor).
func buildAssetDescriptorOpEmitBlob() []byte {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	// ver: uint8 = 1
	enc.WriteUint8(1)
	// operation_type: uint8 = 1 (emit)
	enc.WriteUint8(1)
	// opt_asset_id: present (marker = 1) + 32-byte hash
	enc.WriteUint8(1)
	enc.WriteBytes(bytes.Repeat([]byte{0xAB}, 32))
	// opt_descriptor: absent (marker = 0)
	enc.WriteUint8(0)
	// amount_to_emit: uint64 LE = 500000
	enc.WriteUint64LE(500000)
	// amount_to_burn: uint64 LE = 0
	enc.WriteUint64LE(0)
	// etc: vector<uint8> (empty)
	enc.WriteVarint(0)

	return buf.Bytes()
}

func TestReadAssetDescriptorOperationEmit_Good(t *testing.T) {
	blob := buildAssetDescriptorOpEmitBlob()
	dec := NewDecoder(bytes.NewReader(blob))
	got := readAssetDescriptorOperation(dec)
	if dec.Err() != nil {
		t.Fatalf("readAssetDescriptorOperation (emit) failed: %v", dec.Err())
	}
	if !bytes.Equal(got, blob) {
		t.Fatalf("round-trip mismatch: got %d bytes, want %d bytes", len(got), len(blob))
	}
}

func TestVariantVectorWithTag40_Good(t *testing.T) {
	// Build a variant vector containing one element: tag 40 (asset_descriptor_operation).
	innerBlob := buildAssetDescriptorOpEmitBlob()

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	// count = 1
	enc.WriteVarint(1)
	// tag
	enc.WriteUint8(tagAssetDescriptorOperation)
	// data
	enc.WriteBytes(innerBlob)

	raw := buf.Bytes()

	// Decode as raw variant vector.
	dec := NewDecoder(bytes.NewReader(raw))
	got := decodeRawVariantVector(dec)
	if dec.Err() != nil {
		t.Fatalf("decodeRawVariantVector with tag 40 failed: %v", dec.Err())
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("round-trip mismatch: got %d bytes, want %d bytes", len(got), len(raw))
	}
}

func buildAssetOperationProofBlob() []byte {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	// ver: uint8 = 1
	enc.WriteUint8(1)
	// gss: generic_schnorr_sig_s — 2 scalars (s, c) = 64 bytes
	enc.WriteBytes(make([]byte, 64))
	// asset_id: 32-byte hash
	enc.WriteBytes(bytes.Repeat([]byte{0xCD}, 32))
	// etc: vector<uint8> (empty)
	enc.WriteVarint(0)

	return buf.Bytes()
}

func TestReadAssetOperationProof_Good(t *testing.T) {
	blob := buildAssetOperationProofBlob()
	dec := NewDecoder(bytes.NewReader(blob))
	got := readAssetOperationProof(dec)
	if dec.Err() != nil {
		t.Fatalf("readAssetOperationProof failed: %v", dec.Err())
	}
	if !bytes.Equal(got, blob) {
		t.Fatalf("round-trip mismatch: got %d bytes, want %d bytes", len(got), len(blob))
	}
}

func TestReadAssetOperationProof_Bad(t *testing.T) {
	dec := NewDecoder(bytes.NewReader([]byte{1}))
	_ = readAssetOperationProof(dec)
	if dec.Err() == nil {
		t.Fatal("expected error for truncated asset operation proof")
	}
}

func buildAssetOperationOwnershipProofBlob() []byte {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	// ver: uint8 = 1
	enc.WriteUint8(1)
	// gss: generic_schnorr_sig_s — 2 scalars = 64 bytes
	enc.WriteBytes(make([]byte, 64))
	// etc: vector<uint8> (empty)
	enc.WriteVarint(0)

	return buf.Bytes()
}

func TestReadAssetOperationOwnershipProof_Good(t *testing.T) {
	blob := buildAssetOperationOwnershipProofBlob()
	dec := NewDecoder(bytes.NewReader(blob))
	got := readAssetOperationOwnershipProof(dec)
	if dec.Err() != nil {
		t.Fatalf("readAssetOperationOwnershipProof failed: %v", dec.Err())
	}
	if !bytes.Equal(got, blob) {
		t.Fatalf("round-trip mismatch: got %d bytes, want %d bytes", len(got), len(blob))
	}
}

func buildAssetOperationOwnershipProofETHBlob() []byte {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	// ver: uint8 = 1
	enc.WriteUint8(1)
	// eth_sig: 65 bytes (r=32 + s=32 + v=1)
	enc.WriteBytes(make([]byte, 65))
	// etc: vector<uint8> (empty)
	enc.WriteVarint(0)

	return buf.Bytes()
}

func TestReadAssetOperationOwnershipProofETH_Good(t *testing.T) {
	blob := buildAssetOperationOwnershipProofETHBlob()
	dec := NewDecoder(bytes.NewReader(blob))
	got := readAssetOperationOwnershipProofETH(dec)
	if dec.Err() != nil {
		t.Fatalf("readAssetOperationOwnershipProofETH failed: %v", dec.Err())
	}
	if !bytes.Equal(got, blob) {
		t.Fatalf("round-trip mismatch: got %d bytes, want %d bytes", len(got), len(blob))
	}
}

func TestVariantVectorWithProofTags_Good(t *testing.T) {
	// Build a variant vector with 3 elements: tags 49, 50, 51.
	proofBlob := buildAssetOperationProofBlob()
	ownershipBlob := buildAssetOperationOwnershipProofBlob()
	ethBlob := buildAssetOperationOwnershipProofETHBlob()

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	// count = 3
	enc.WriteVarint(3)
	// tag 49
	enc.WriteUint8(tagAssetOperationProof)
	enc.WriteBytes(proofBlob)
	// tag 50
	enc.WriteUint8(tagAssetOperationOwnershipProof)
	enc.WriteBytes(ownershipBlob)
	// tag 51
	enc.WriteUint8(tagAssetOperationOwnershipProofETH)
	enc.WriteBytes(ethBlob)

	raw := buf.Bytes()

	dec := NewDecoder(bytes.NewReader(raw))
	got := decodeRawVariantVector(dec)
	if dec.Err() != nil {
		t.Fatalf("decodeRawVariantVector with proof tags failed: %v", dec.Err())
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("round-trip mismatch: got %d bytes, want %d bytes", len(got), len(raw))
	}
}

func TestV3TransactionRoundTrip_Good(t *testing.T) {
	// Build a v3 transaction with:
	// - 1 coinbase input (TxInputGenesis at height 201)
	// - 2 Zarcanum outputs
	// - extra containing: public_key (tag 22) + zarcanum_tx_data_v1 (tag 39)
	// - proofs containing: zc_balance_proof (tag 48)
	// - hardfork_id = 5

	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	// --- prefix ---
	// version = 3
	enc.WriteVarint(3)
	// vin: 1 coinbase input
	enc.WriteVarint(1) // input count
	enc.WriteVariantTag(0) // txin_gen tag
	enc.WriteVarint(201) // height

	// extra: variant vector with 2 elements (public_key + zarcanum_tx_data_v1)
	enc.WriteVarint(2)
	// [0] public_key (tag 22): 32 bytes
	enc.WriteUint8(tagPublicKey)
	enc.WriteBytes(bytes.Repeat([]byte{0x11}, 32))
	// [1] zarcanum_tx_data_v1 (tag 39): 8-byte LE fee
	enc.WriteUint8(tagZarcanumTxDataV1)
	enc.WriteUint64LE(10000)

	// vout: 2 Zarcanum outputs
	enc.WriteVarint(2)
	for range 2 {
		enc.WriteVariantTag(38) // OutputTypeZarcanum
		enc.WriteBytes(make([]byte, 32)) // stealth_address
		enc.WriteBytes(make([]byte, 32)) // concealing_point
		enc.WriteBytes(make([]byte, 32)) // amount_commitment
		enc.WriteBytes(make([]byte, 32)) // blinded_asset_id
		enc.WriteUint64LE(0) // encrypted_amount
		enc.WriteUint8(0) // mix_attr
	}

	// hardfork_id = 5
	enc.WriteUint8(5)

	// --- suffix ---
	// attachment: empty
	enc.WriteVarint(0)
	// signatures: empty
	enc.WriteVarint(0)
	// proofs: 1 element — zc_balance_proof (tag 48, simplest: 96 bytes)
	enc.WriteVarint(1)
	enc.WriteUint8(tagZCBalanceProof)
	enc.WriteBytes(make([]byte, 96))

	blob := buf.Bytes()

	// Decode
	dec := NewDecoder(bytes.NewReader(blob))
	tx := DecodeTransaction(dec)
	if dec.Err() != nil {
		t.Fatalf("decode failed: %v", dec.Err())
	}

	// Verify structural fields
	if tx.Version != 3 {
		t.Errorf("version: got %d, want 3", tx.Version)
	}
	if tx.HardforkID != 5 {
		t.Errorf("hardfork_id: got %d, want 5", tx.HardforkID)
	}
	if len(tx.Vin) != 1 {
		t.Fatalf("input count: got %d, want 1", len(tx.Vin))
	}
	if len(tx.Vout) != 2 {
		t.Fatalf("output count: got %d, want 2", len(tx.Vout))
	}

	// Re-encode
	var reenc bytes.Buffer
	enc2 := NewEncoder(&reenc)
	EncodeTransaction(enc2, &tx)
	if enc2.Err() != nil {
		t.Fatalf("encode failed: %v", enc2.Err())
	}

	got := reenc.Bytes()
	if !bytes.Equal(got, blob) {
		t.Fatalf("round-trip mismatch: got %d bytes, want %d bytes\ngot:  %x\nwant: %x",
			len(got), len(blob), got[:min(len(got), 64)], blob[:min(len(blob), 64)])
	}
}

func TestV3TransactionWithAssetOps_Good(t *testing.T) {
	// Build a v3 transaction whose extra includes an asset_descriptor_operation (tag 40)
	// and whose proofs include an asset_operation_proof (tag 49).
	assetOpBlob := buildAssetDescriptorOpEmitBlob()
	proofBlob := buildAssetOperationProofBlob()

	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	// --- prefix ---
	enc.WriteVarint(3) // version
	// vin: 1 coinbase
	enc.WriteVarint(1)
	enc.WriteVariantTag(0) // txin_gen
	enc.WriteVarint(250)   // height

	// extra: 2 elements — public_key + asset_descriptor_operation
	enc.WriteVarint(2)
	enc.WriteUint8(tagPublicKey)
	enc.WriteBytes(bytes.Repeat([]byte{0x22}, 32))
	enc.WriteUint8(tagAssetDescriptorOperation)
	enc.WriteBytes(assetOpBlob)

	// vout: 2 Zarcanum outputs
	enc.WriteVarint(2)
	for range 2 {
		enc.WriteVariantTag(38)
		enc.WriteBytes(make([]byte, 32)) // stealth_address
		enc.WriteBytes(make([]byte, 32)) // concealing_point
		enc.WriteBytes(make([]byte, 32)) // amount_commitment
		enc.WriteBytes(make([]byte, 32)) // blinded_asset_id
		enc.WriteUint64LE(0)
		enc.WriteUint8(0)
	}

	// hardfork_id = 5
	enc.WriteUint8(5)

	// --- suffix ---
	enc.WriteVarint(0) // attachment
	enc.WriteVarint(0) // signatures

	// proofs: 2 elements — zc_balance_proof + asset_operation_proof
	enc.WriteVarint(2)
	enc.WriteUint8(tagZCBalanceProof)
	enc.WriteBytes(make([]byte, 96))
	enc.WriteUint8(tagAssetOperationProof)
	enc.WriteBytes(proofBlob)

	blob := buf.Bytes()

	// Decode
	dec := NewDecoder(bytes.NewReader(blob))
	tx := DecodeTransaction(dec)
	if dec.Err() != nil {
		t.Fatalf("decode failed: %v", dec.Err())
	}

	if tx.Version != 3 {
		t.Errorf("version: got %d, want 3", tx.Version)
	}
	if tx.HardforkID != 5 {
		t.Errorf("hardfork_id: got %d, want 5", tx.HardforkID)
	}

	// Re-encode and compare
	var reenc bytes.Buffer
	enc2 := NewEncoder(&reenc)
	EncodeTransaction(enc2, &tx)
	if enc2.Err() != nil {
		t.Fatalf("encode failed: %v", enc2.Err())
	}

	if !bytes.Equal(reenc.Bytes(), blob) {
		t.Fatalf("round-trip mismatch: got %d bytes, want %d bytes", len(reenc.Bytes()), len(blob))
	}
}

func TestV3TransactionDecode_Bad(t *testing.T) {
	// Truncated v3 transaction — version varint only.
	dec := NewDecoder(bytes.NewReader([]byte{0x03}))
	_ = DecodeTransaction(dec)
	if dec.Err() == nil {
		t.Fatal("expected error for truncated v3 transaction")
	}
}
