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
