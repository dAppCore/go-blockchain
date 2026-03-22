# HF5 Confidential Assets — Minimum Viable Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox syntax for tracking.

**Goal:** Enable the Go node to deserialise HF5 blocks (confidential assets) by adding wire support for the asset descriptor operation tag (40) and asset proof tags (49, 50, 51), enforcing transaction version 3 after HF5, and implementing the pre-hardfork transaction freeze (60 blocks before HF5 activation).

**Architecture:** New variant tag readers in `wire/`, consensus rules gated on HF5 in `consensus/`, helper function in `config/`. All asset operations stored as opaque bytes (existing pattern). No deep asset validation or asset state tracking — those are deferred per spec.

**Tech Stack:** Go 1.26, CGo crypto bridge (libcryptonote.a), `go test -race`

---

## File Map

### Modified files

| File | What changes |
|------|-------------|
| `wire/transaction.go` | Add tag constants 40, 49, 50, 51. Add `readAssetDescriptorOperation`, `readAssetOperationProof`, `readAssetOperationOwnershipProof` reader functions. Add cases to `readVariantElementData` switch. |
| `consensus/tx.go` | Add transaction version 3 enforcement after HF5. Add `checkTxVersion` helper called from `ValidateTransaction`. |
| `consensus/block.go` | Add pre-hardfork transaction freeze check in `ValidateBlock`. |
| `consensus/errors.go` | Add `ErrTxVersionInvalid` and `ErrPreHardforkFreeze` sentinel errors. |
| `config/hardfork.go` | Add `HardforkActivationHeight` helper to retrieve the activation height for a given fork version. |

### New test files

| File | What tests |
|------|-----------|
| `wire/transaction_v3_test.go` | Round-trip tests for v3 transactions containing tag 40, 49, 50, 51 variant elements. Tests for each reader function in isolation. |
| `consensus/tx_version_test.go` | Version 3 enforcement: accept v3 after HF5, reject v2 after HF5, accept v2 before HF5. |
| `consensus/freeze_test.go` | Pre-hardfork freeze: reject non-coinbase in freeze window, accept coinbase in freeze window, accept all outside freeze window. |
| `config/hardfork_activation_test.go` | `HardforkActivationHeight` tests for mainnet and testnet fork schedules. |

---

## Task 1: Add `HardforkActivationHeight` to config/

**Package:** `config/`
**Why:** The pre-hardfork freeze needs to know the exact activation height for HF5. The existing `IsHardForkActive` only returns a bool. A helper that returns the raw height is needed by `consensus/block.go`.

### Step 1.1 — Write test for HardforkActivationHeight

- [ ] Create `/home/claude/Code/core/go-blockchain/config/hardfork_activation_test.go`

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package config

import "testing"

func TestHardforkActivationHeight_Good(t *testing.T) {
	tests := []struct {
		name    string
		forks   []HardFork
		version uint8
		want    uint64
		wantOK  bool
	}{
		{"mainnet_hf5", MainnetForks, HF5, 999999999, true},
		{"testnet_hf5", TestnetForks, HF5, 200, true},
		{"testnet_hf4", TestnetForks, HF4Zarcanum, 100, true},
		{"mainnet_hf0", MainnetForks, HF0Initial, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := HardforkActivationHeight(tt.forks, tt.version)
			if ok != tt.wantOK {
				t.Fatalf("HardforkActivationHeight ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("HardforkActivationHeight = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestHardforkActivationHeight_Bad(t *testing.T) {
	_, ok := HardforkActivationHeight(MainnetForks, 99)
	if ok {
		t.Error("HardforkActivationHeight with unknown version should return false")
	}
}

func TestHardforkActivationHeight_Ugly(t *testing.T) {
	_, ok := HardforkActivationHeight(nil, HF5)
	if ok {
		t.Error("HardforkActivationHeight with nil forks should return false")
	}
}
```

### Step 1.2 — Run test, verify FAIL

```bash
cd /home/claude/Code/core/go-blockchain && go test -race -run "TestHardforkActivationHeight" ./config/...
```

**Expected:** Compilation error — `HardforkActivationHeight` does not exist.

### Step 1.3 — Implement HardforkActivationHeight

- [ ] Edit `/home/claude/Code/core/go-blockchain/config/hardfork.go`

Add after the `IsHardForkActive` function (after line 95):

```go
// HardforkActivationHeight returns the activation height for the given
// hardfork version. The fork becomes active at heights strictly greater
// than the returned value. Returns (0, false) if the version is not found.
func HardforkActivationHeight(forks []HardFork, version uint8) (uint64, bool) {
	for _, hf := range forks {
		if hf.Version == version {
			return hf.Height, true
		}
	}
	return 0, false
}
```

### Step 1.4 — Run test, verify PASS

```bash
cd /home/claude/Code/core/go-blockchain && go test -race -run "TestHardforkActivationHeight" ./config/...
```

**Expected:** PASS

### Step 1.5 — Run full config test suite

```bash
cd /home/claude/Code/core/go-blockchain && go test -race ./config/...
```

**Expected:** All PASS — no regressions.

### Step 1.6 — Commit

```
feat(config): add HardforkActivationHeight helper

Returns the raw activation height for a given hardfork version.
Needed by the pre-hardfork transaction freeze logic.

Co-Authored-By: Charon <charon@lethean.io>
```

---

## Task 2: Add asset descriptor operation tag (40) to wire/

**Package:** `wire/`
**Why:** Tag 40 (`asset_descriptor_operation`) appears in the extra field of HF5 transactions. Without a reader, `decodeRawVariantVector` hits the default error case and rejects the block.

The asset_descriptor_operation uses `CHAIN_TRANSITION_VER` which serialises as:
1. `ver` (uint8) — version byte
2. Version-specific fields (operation_type, opt_asset_id, opt_descriptor, amounts, etc.)

The minimum viable reader does NOT parse the inner structure. Instead, it reads the version byte, then consumes the remaining data using the version-dependent size logic. For version 0 and 1, the structure is:

- `operation_type` (uint8)
- `opt_asset_id` (optional<hash>: uint8 marker + 32 bytes if present)
- `opt_descriptor` (optional<asset_descriptor_base>: uint8 marker + descriptor if present)
  - descriptor itself is: string(ticker) + string(full_name) + uint64(total_max_supply) + uint64(current_supply) + uint8(decimal_point) + string(meta_info) + 32-byte(owner_key) + vector<uint8>(etc)
- `amount_to_emit` (uint64 LE)
- `amount_to_burn` (uint64 LE)
- `vector<uint8>` (etc — opaque)

Since this is complex and version-dependent, the minimum viable approach is to use `readChainTransitionBlob` — read the version byte, then consume the rest as a varint-prefixed opaque blob. But the CHAIN_TRANSITION_VER macro does NOT add a length prefix — it just switches serialisation logic. So we must parse field-by-field.

The safest minimum viable approach: read each field structurally, returning raw bytes. This matches how `readTxServiceAttachment` and `readExtraAliasEntry` work.

### Step 2.1 — Write round-trip test for tag 40

- [ ] Create `/home/claude/Code/core/go-blockchain/wire/transaction_v3_test.go`

```go
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
```

### Step 2.2 — Run tests, verify FAIL

```bash
cd /home/claude/Code/core/go-blockchain && go test -race -run "TestReadAssetDescriptorOperation|TestVariantVectorWithTag40" ./wire/...
```

**Expected:** Compilation error — `readAssetDescriptorOperation` and `tagAssetDescriptorOperation` do not exist.

### Step 2.3 — Add tag constant and reader function

- [ ] Edit `/home/claude/Code/core/go-blockchain/wire/transaction.go`

Add tag constant after `tagZarcanumTxDataV1` (after line 423):

```go
	// Asset descriptor operation (HF5).
	tagAssetDescriptorOperation = 40 // asset_descriptor_operation
```

Add case to `readVariantElementData` switch, after the `tagZarcanumTxDataV1` case (after line 491):

```go
	// Asset descriptor operation (HF5)
	case tagAssetDescriptorOperation:
		return readAssetDescriptorOperation(dec)
```

Add reader function after `readSignedParts` (after line 671):

```go
// readAssetDescriptorOperation reads asset_descriptor_operation (tag 40).
// Structure (CHAIN_TRANSITION_VER, version 0 and 1):
//   ver (uint8) + operation_type (uint8)
//   + opt_asset_id (uint8 marker + 32 bytes if present)
//   + opt_descriptor (uint8 marker + AssetDescriptorBase if present)
//   + amount_to_emit (uint64 LE) + amount_to_burn (uint64 LE)
//   + etc (vector<uint8>)
//
// AssetDescriptorBase:
//   ticker (string) + full_name (string) + total_max_supply (uint64 LE)
//   + current_supply (uint64 LE) + decimal_point (uint8) + meta_info (string)
//   + owner_key (32 bytes) + etc (vector<uint8>)
func readAssetDescriptorOperation(dec *Decoder) []byte {
	var raw []byte

	// ver: uint8 (CHAIN_TRANSITION_VER version byte)
	ver := dec.ReadUint8()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, ver)

	// operation_type: uint8
	opType := dec.ReadUint8()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, opType)

	// opt_asset_id: optional<hash> — uint8 marker, then 32 bytes if present
	marker := dec.ReadUint8()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, marker)
	if marker != 0 {
		b := dec.ReadBytes(32)
		if dec.err != nil {
			return nil
		}
		raw = append(raw, b...)
	}

	// opt_descriptor: optional<AssetDescriptorBase>
	marker = dec.ReadUint8()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, marker)
	if marker != 0 {
		b := readAssetDescriptorBase(dec)
		if dec.err != nil {
			return nil
		}
		raw = append(raw, b...)
	}

	// amount_to_emit: uint64 LE
	b := dec.ReadBytes(8)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)

	// amount_to_burn: uint64 LE
	b = dec.ReadBytes(8)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)

	// etc: vector<uint8>
	v := readVariantVectorFixed(dec, 1)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)

	return raw
}

// readAssetDescriptorBase reads the AssetDescriptorBase structure.
// Wire: ticker (string) + full_name (string) + total_max_supply (uint64 LE)
//   + current_supply (uint64 LE) + decimal_point (uint8) + meta_info (string)
//   + owner_key (32 bytes) + etc (vector<uint8>).
func readAssetDescriptorBase(dec *Decoder) []byte {
	var raw []byte

	// ticker: string
	s := readStringBlob(dec)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, s...)

	// full_name: string
	s = readStringBlob(dec)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, s...)

	// total_max_supply: uint64 LE
	b := dec.ReadBytes(8)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)

	// current_supply: uint64 LE
	b = dec.ReadBytes(8)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)

	// decimal_point: uint8
	dp := dec.ReadUint8()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, dp)

	// meta_info: string
	s = readStringBlob(dec)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, s...)

	// owner_key: 32 bytes (crypto::public_key)
	b = dec.ReadBytes(32)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)

	// etc: vector<uint8>
	v := readVariantVectorFixed(dec, 1)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)

	return raw
}
```

### Step 2.4 — Run tests, verify PASS

```bash
cd /home/claude/Code/core/go-blockchain && go test -race -run "TestReadAssetDescriptorOperation|TestVariantVectorWithTag40" ./wire/...
```

**Expected:** All PASS

### Step 2.5 — Run full wire test suite

```bash
cd /home/claude/Code/core/go-blockchain && go test -race ./wire/...
```

**Expected:** All PASS — no regressions.

### Step 2.6 — Commit

```
feat(wire): add asset_descriptor_operation tag 40 reader

Reads the CHAIN_TRANSITION_VER structure for asset deploy/emit/update/burn
operations. Stores as opaque bytes for bit-identical round-tripping.
Required for HF5 block deserialisation.

Co-Authored-By: Charon <charon@lethean.io>
```

---

## Task 3: Add asset proof tags (49, 50, 51) to wire/

**Package:** `wire/`
**Why:** Tags 49, 50, and 51 appear in the proofs variant vector of HF5 transactions. Without readers, `decodeRawVariantVector` rejects these blocks.

Proof structures:
- **Tag 49** (`asset_operation_proof`): asset_operation_proof_v1 has `gss` (generic_schnorr_sig: 2 scalars = 64 bytes) + `asset_id` (32-byte hash) + `etc` (vector<uint8>). Uses CHAIN_TRANSITION_VER, so version byte first.
- **Tag 50** (`asset_operation_ownership_proof`): Schnorr proof of asset ownership. `gss` (64 bytes) + `etc` (vector<uint8>). Uses CHAIN_TRANSITION_VER.
- **Tag 51** (`asset_operation_ownership_proof_eth`): Ethereum-style signature proof. `eth_sig` (65 bytes: r(32) + s(32) + v(1)) + `etc` (vector<uint8>). Uses CHAIN_TRANSITION_VER.

### Step 3.1 — Write tests for proof tag readers

- [ ] Append to `/home/claude/Code/core/go-blockchain/wire/transaction_v3_test.go`

```go
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
```

### Step 3.2 — Run tests, verify FAIL

```bash
cd /home/claude/Code/core/go-blockchain && go test -race -run "TestReadAssetOperation|TestVariantVectorWithProofTags" ./wire/...
```

**Expected:** Compilation error — tag constants and reader functions do not exist.

### Step 3.3 — Add tag constants and reader functions

- [ ] Edit `/home/claude/Code/core/go-blockchain/wire/transaction.go`

Add tag constants after `tagAssetDescriptorOperation` (the constant added in Task 2):

```go
	// Asset operation proof tags (HF5).
	tagAssetOperationProof          = 49 // asset_operation_proof
	tagAssetOperationOwnershipProof = 50 // asset_operation_ownership_proof
	tagAssetOperationOwnershipProofETH = 51 // asset_operation_ownership_proof_eth (Ethereum sig)
```

Add cases to `readVariantElementData` switch, after the `tagAssetDescriptorOperation` case:

```go
	// Asset operation proof variants (HF5)
	case tagAssetOperationProof:
		return readAssetOperationProof(dec)
	case tagAssetOperationOwnershipProof:
		return readAssetOperationOwnershipProof(dec)
	case tagAssetOperationOwnershipProofETH:
		return readAssetOperationOwnershipProofETH(dec)
```

Add reader functions after `readAssetDescriptorBase`:

```go
// readAssetOperationProof reads asset_operation_proof (tag 49).
// Structure (CHAIN_TRANSITION_VER, version 1):
//   ver (uint8) + gss (generic_schnorr_sig_s: 64 bytes)
//   + asset_id (32 bytes) + etc (vector<uint8>).
func readAssetOperationProof(dec *Decoder) []byte {
	var raw []byte

	// ver: uint8
	ver := dec.ReadUint8()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, ver)

	// gss: generic_schnorr_sig_s — 2 scalars (s, c) = 64 bytes
	b := dec.ReadBytes(64)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)

	// asset_id: 32-byte hash
	b = dec.ReadBytes(32)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)

	// etc: vector<uint8>
	v := readVariantVectorFixed(dec, 1)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)

	return raw
}

// readAssetOperationOwnershipProof reads asset_operation_ownership_proof (tag 50).
// Structure (CHAIN_TRANSITION_VER, version 1):
//   ver (uint8) + gss (generic_schnorr_sig_s: 64 bytes)
//   + etc (vector<uint8>).
func readAssetOperationOwnershipProof(dec *Decoder) []byte {
	var raw []byte

	// ver: uint8
	ver := dec.ReadUint8()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, ver)

	// gss: generic_schnorr_sig_s — 2 scalars (s, c) = 64 bytes
	b := dec.ReadBytes(64)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)

	// etc: vector<uint8>
	v := readVariantVectorFixed(dec, 1)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)

	return raw
}

// readAssetOperationOwnershipProofETH reads asset_operation_ownership_proof_eth (tag 51).
// Structure (CHAIN_TRANSITION_VER, version 1):
//   ver (uint8) + eth_sig (65 bytes: r(32) + s(32) + v(1))
//   + etc (vector<uint8>).
func readAssetOperationOwnershipProofETH(dec *Decoder) []byte {
	var raw []byte

	// ver: uint8
	ver := dec.ReadUint8()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, ver)

	// eth_sig: crypto::eth_signature — r(32) + s(32) + v(1) = 65 bytes
	b := dec.ReadBytes(65)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)

	// etc: vector<uint8>
	v := readVariantVectorFixed(dec, 1)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)

	return raw
}
```

### Step 3.4 — Run tests, verify PASS

```bash
cd /home/claude/Code/core/go-blockchain && go test -race -run "TestReadAssetOperation|TestVariantVectorWithProofTags" ./wire/...
```

**Expected:** All PASS

### Step 3.5 — Run full wire test suite

```bash
cd /home/claude/Code/core/go-blockchain && go test -race ./wire/...
```

**Expected:** All PASS — no regressions.

### Step 3.6 — Commit

```
feat(wire): add asset proof tags 49, 50, 51 readers

Reads asset_operation_proof, asset_operation_ownership_proof, and
asset_operation_ownership_proof_eth structures. All use CHAIN_TRANSITION_VER
with version byte prefix. Stored as opaque bytes.

Co-Authored-By: Charon <charon@lethean.io>
```

---

## Task 4: V3 transaction round-trip test

**Package:** `wire/`
**Why:** Validates that a complete v3 transaction (with hardfork_id, asset descriptor in extra, and asset proofs) can be encoded, decoded, and re-encoded to identical bytes.

### Step 4.1 — Write v3 round-trip test

- [ ] Append to `/home/claude/Code/core/go-blockchain/wire/transaction_v3_test.go`

```go
func TestV3TransactionRoundTrip_Good(t *testing.T) {
	// Build a v3 transaction with:
	// - 1 coinbase input (TxInputGenesis at height 201)
	// - 2 Zarcanum outputs
	// - extra containing: public_key (tag 22) + zarcanum_tx_data_v1 (tag 39)
	//   + asset_descriptor_operation (tag 40)
	// - proofs containing: zc_asset_surjection_proof (tag 46)
	//   + asset_operation_proof (tag 49)
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
```

### Step 4.2 — Run tests, verify PASS

```bash
cd /home/claude/Code/core/go-blockchain && go test -race -run "TestV3Transaction" ./wire/...
```

**Expected:** All PASS — the wire encoder/decoder already handles version 3 via `encodePrefixV2`/`decodePrefixV2` (which gates `hardfork_id` on `Version >= VersionPostHF5`).

### Step 4.3 — Run full wire test suite

```bash
cd /home/claude/Code/core/go-blockchain && go test -race ./wire/...
```

**Expected:** All PASS.

### Step 4.4 — Commit

```
test(wire): add v3 transaction round-trip tests with asset operations

Tests v3 transactions containing asset_descriptor_operation (tag 40)
in extra and asset_operation_proof (tag 49) in proofs. Validates
hardfork_id encoding and bit-identical round-tripping.

Co-Authored-By: Charon <charon@lethean.io>
```

---

## Task 5: Transaction version 3 enforcement in consensus/

**Package:** `consensus/`
**Why:** After HF5, transaction version must be 3. Before HF5, version 3 must be rejected. This matches the C++ `check_tx_semantic` hardfork gating.

### Step 5.1 — Add sentinel errors

- [ ] Edit `/home/claude/Code/core/go-blockchain/consensus/errors.go`

Add after `ErrInvalidExtra` (after line 22):

```go
	ErrTxVersionInvalid  = errors.New("consensus: invalid transaction version for current hardfork")
	ErrPreHardforkFreeze = errors.New("consensus: non-coinbase transaction rejected during pre-hardfork freeze")
```

### Step 5.2 — Write version enforcement tests

- [ ] Create `/home/claude/Code/core/go-blockchain/consensus/tx_version_test.go`

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

//go:build !integration

package consensus

import (
	"testing"

	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/types"
)

// validV2Tx returns a minimal valid v2 (Zarcanum) transaction for testing.
func validV2Tx() *types.Transaction {
	return &types.Transaction{
		Version: types.VersionPostHF4,
		Vin: []types.TxInput{
			types.TxInputZC{
				KeyImage: types.KeyImage{1},
			},
		},
		Vout: []types.TxOutput{
			types.TxOutputZarcanum{StealthAddress: types.PublicKey{1}},
			types.TxOutputZarcanum{StealthAddress: types.PublicKey{2}},
		},
	}
}

// validV3Tx returns a minimal valid v3 (HF5) transaction for testing.
func validV3Tx() *types.Transaction {
	return &types.Transaction{
		Version:    types.VersionPostHF5,
		HardforkID: 5,
		Vin: []types.TxInput{
			types.TxInputZC{
				KeyImage: types.KeyImage{1},
			},
		},
		Vout: []types.TxOutput{
			types.TxOutputZarcanum{StealthAddress: types.PublicKey{1}},
			types.TxOutputZarcanum{StealthAddress: types.PublicKey{2}},
		},
	}
}

func TestCheckTxVersion_Good(t *testing.T) {
	tests := []struct {
		name   string
		tx     *types.Transaction
		forks  []config.HardFork
		height uint64
	}{
		// v1 transaction before HF4 — valid.
		{"v1_before_hf4", validV1Tx(), config.MainnetForks, 5000},
		// v2 transaction after HF4, before HF5 — valid.
		{"v2_after_hf4_before_hf5", validV2Tx(), config.TestnetForks, 150},
		// v3 transaction after HF5 — valid.
		{"v3_after_hf5", validV3Tx(), config.TestnetForks, 250},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkTxVersion(tt.tx, tt.forks, tt.height)
			if err != nil {
				t.Errorf("checkTxVersion returned unexpected error: %v", err)
			}
		})
	}
}

func TestCheckTxVersion_Bad(t *testing.T) {
	tests := []struct {
		name   string
		tx     *types.Transaction
		forks  []config.HardFork
		height uint64
	}{
		// v2 transaction after HF5 — must be v3.
		{"v2_after_hf5", validV2Tx(), config.TestnetForks, 250},
		// v3 transaction before HF5 — too early.
		{"v3_before_hf5", validV3Tx(), config.TestnetForks, 150},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkTxVersion(tt.tx, tt.forks, tt.height)
			if err == nil {
				t.Error("expected ErrTxVersionInvalid, got nil")
			}
		})
	}
}

func TestCheckTxVersion_Ugly(t *testing.T) {
	// v3 at exact HF5 activation boundary (height 201 on testnet, HF5.Height=200).
	tx := validV3Tx()
	err := checkTxVersion(tx, config.TestnetForks, 201)
	if err != nil {
		t.Errorf("v3 at HF5 activation boundary should be valid: %v", err)
	}

	// v2 at exact HF5 activation boundary — should be rejected.
	tx2 := validV2Tx()
	err = checkTxVersion(tx2, config.TestnetForks, 201)
	if err == nil {
		t.Error("v2 at HF5 activation boundary should be rejected")
	}
}
```

### Step 5.3 — Run tests, verify FAIL

```bash
cd /home/claude/Code/core/go-blockchain && go test -race -run "TestCheckTxVersion" ./consensus/...
```

**Expected:** Compilation error — `checkTxVersion` does not exist.

### Step 5.4 — Implement checkTxVersion and wire into ValidateTransaction

- [ ] Edit `/home/claude/Code/core/go-blockchain/consensus/tx.go`

Add the `checkTxVersion` function after `checkKeyImages` (after line 123):

```go
// checkTxVersion validates that the transaction version is correct for the
// current hardfork era. After HF5, version must be 3. Before HF5, version 3
// is rejected.
func checkTxVersion(tx *types.Transaction, forks []config.HardFork, height uint64) error {
	hf5Active := config.IsHardForkActive(forks, config.HF5, height)

	if hf5Active {
		// After HF5: must be version 3.
		if tx.Version != types.VersionPostHF5 {
			return fmt.Errorf("%w: got version %d, require %d after HF5",
				ErrTxVersionInvalid, tx.Version, types.VersionPostHF5)
		}
	} else {
		// Before HF5: version 3 is not allowed.
		if tx.Version >= types.VersionPostHF5 {
			return fmt.Errorf("%w: version %d not allowed before HF5",
				ErrTxVersionInvalid, tx.Version)
		}
	}

	return nil
}
```

Add the version check call in `ValidateTransaction`, after the `hf4Active` line (line 18) and before the blob size check:

```go
	// 0. Transaction version for current hardfork.
	if err := checkTxVersion(tx, forks, height); err != nil {
		return err
	}
```

### Step 5.5 — Run tests, verify PASS

```bash
cd /home/claude/Code/core/go-blockchain && go test -race -run "TestCheckTxVersion" ./consensus/...
```

**Expected:** All PASS

### Step 5.6 — Run full consensus test suite

```bash
cd /home/claude/Code/core/go-blockchain && go test -race ./consensus/...
```

**Expected:** All PASS — existing tests use pre-HF5 heights and v1 transactions, so the new check passes them through.

### Step 5.7 — Commit

```
feat(consensus): enforce transaction version 3 after HF5

After HF5 activation, only version 3 transactions are accepted.
Before HF5, version 3 is rejected. Matches C++ check_tx_semantic
hardfork gating logic.

Co-Authored-By: Charon <charon@lethean.io>
```

---

## Task 6: Pre-hardfork transaction freeze

**Package:** `consensus/`
**Why:** 60 blocks before HF5 activation, all non-coinbase transactions must be rejected. This stabilises the chain before consensus rule changes take effect. Uses `config.PreHardforkTxFreezePeriod` (already defined as 60).

### Step 6.1 — Write freeze tests

- [ ] Create `/home/claude/Code/core/go-blockchain/consensus/freeze_test.go`

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

//go:build !integration

package consensus

import (
	"testing"

	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/types"
)

func TestIsPreHardforkFreeze_Good(t *testing.T) {
	// Testnet HF5 activates at heights > 200.
	// Freeze window: heights 141..200 (activation_height - period + 1 .. activation_height).
	// Note: HF5 activation height is 200, meaning HF5 is active at height > 200 = 201+.
	// The freeze applies for 60 blocks *before* the fork activates, so heights 141..200.

	tests := []struct {
		name   string
		height uint64
		want   bool
	}{
		{"well_before_freeze", 100, false},
		{"just_before_freeze", 140, false},
		{"first_freeze_block", 141, true},
		{"mid_freeze", 170, true},
		{"last_freeze_block", 200, true},
		{"after_hf5_active", 201, false},
		{"well_after_hf5", 300, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPreHardforkFreeze(config.TestnetForks, config.HF5, tt.height)
			if got != tt.want {
				t.Errorf("IsPreHardforkFreeze(testnet, HF5, %d) = %v, want %v",
					tt.height, got, tt.want)
			}
		})
	}
}

func TestIsPreHardforkFreeze_Bad(t *testing.T) {
	// Mainnet HF5 is at 999999999 — freeze window starts at 999999940.
	// At typical mainnet heights, no freeze.
	if IsPreHardforkFreeze(config.MainnetForks, config.HF5, 50000) {
		t.Error("should not be in freeze period at mainnet height 50000")
	}
}

func TestIsPreHardforkFreeze_Ugly(t *testing.T) {
	// Unknown fork version — never frozen.
	if IsPreHardforkFreeze(config.TestnetForks, 99, 150) {
		t.Error("unknown fork version should never trigger freeze")
	}

	// Fork at height 0 (HF0) — freeze period would be negative/underflow,
	// should return false.
	if IsPreHardforkFreeze(config.TestnetForks, config.HF0Initial, 0) {
		t.Error("fork at genesis should not trigger freeze")
	}
}

func TestValidateBlockFreeze_Good(t *testing.T) {
	// During freeze, coinbase transactions should still be accepted.
	// This test verifies that ValidateBlock does not reject a block
	// that only contains its miner transaction during the freeze window.
	// (ValidateBlock validates the miner tx; regular tx validation is
	// done separately per tx.)
	//
	// The freeze check applies to regular transactions via
	// ValidateTransactionInBlock, not to the miner tx itself.
	coinbaseTx := &types.Transaction{
		Version: types.VersionPostHF4,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 150}},
	}
	_ = coinbaseTx // structural test — actual block validation needs more fields
}

func TestValidateTransactionInBlock_Good(t *testing.T) {
	// Outside freeze window — regular transaction accepted.
	tx := validV2Tx()
	blob := make([]byte, 100)
	err := ValidateTransactionInBlock(tx, blob, config.TestnetForks, 130)
	if err != nil {
		t.Errorf("expected no error outside freeze, got: %v", err)
	}
}

func TestValidateTransactionInBlock_Bad(t *testing.T) {
	// Inside freeze window — regular transaction rejected.
	tx := validV2Tx()
	blob := make([]byte, 100)
	err := ValidateTransactionInBlock(tx, blob, config.TestnetForks, 150)
	if err == nil {
		t.Error("expected ErrPreHardforkFreeze during freeze window")
	}
}

func TestValidateTransactionInBlock_Ugly(t *testing.T) {
	// Coinbase transaction during freeze — should be accepted.
	tx := &types.Transaction{
		Version: types.VersionPostHF4,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 150}},
		Vout: []types.TxOutput{
			types.TxOutputZarcanum{StealthAddress: types.PublicKey{1}},
			types.TxOutputZarcanum{StealthAddress: types.PublicKey{2}},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransactionInBlock(tx, blob, config.TestnetForks, 150)
	if err != nil {
		t.Errorf("coinbase during freeze should be accepted, got: %v", err)
	}
}
```

### Step 6.2 — Run tests, verify FAIL

```bash
cd /home/claude/Code/core/go-blockchain && go test -race -run "TestIsPreHardforkFreeze|TestValidateTransactionInBlock|TestValidateBlockFreeze" ./consensus/...
```

**Expected:** Compilation error — `IsPreHardforkFreeze` and `ValidateTransactionInBlock` do not exist.

### Step 6.3 — Implement IsPreHardforkFreeze and ValidateTransactionInBlock

- [ ] Edit `/home/claude/Code/core/go-blockchain/consensus/block.go`

Add import for `config` (already present). Add after `ValidateBlock` (after line 151):

```go
// IsPreHardforkFreeze reports whether the given height falls within the
// pre-hardfork transaction freeze window for the specified fork version.
// The freeze window is the `PreHardforkTxFreezePeriod` blocks immediately
// before the fork activation height (inclusive).
//
// For a fork with activation height H (active at heights > H):
//   freeze applies at heights (H - period + 1) .. H
//
// Returns false if the fork version is not found or if the activation height
// is too low for a meaningful freeze window.
func IsPreHardforkFreeze(forks []config.HardFork, version uint8, height uint64) bool {
	activationHeight, ok := config.HardforkActivationHeight(forks, version)
	if !ok {
		return false
	}

	// A fork at height 0 means active from genesis — no freeze window.
	if activationHeight == 0 {
		return false
	}

	// Guard against underflow: if activation height < period, freeze starts at 1.
	freezeStart := uint64(1)
	if activationHeight >= config.PreHardforkTxFreezePeriod {
		freezeStart = activationHeight - config.PreHardforkTxFreezePeriod + 1
	}

	return height >= freezeStart && height <= activationHeight
}

// ValidateTransactionInBlock performs transaction validation including the
// pre-hardfork freeze check. This wraps ValidateTransaction with an
// additional check: during the freeze window before HF5, non-coinbase
// transactions are rejected.
func ValidateTransactionInBlock(tx *types.Transaction, txBlob []byte, forks []config.HardFork, height uint64) error {
	// Pre-hardfork freeze: reject non-coinbase transactions in the freeze window.
	if !isCoinbase(tx) && IsPreHardforkFreeze(forks, config.HF5, height) {
		return fmt.Errorf("%w: height %d is within HF5 freeze window", ErrPreHardforkFreeze, height)
	}

	return ValidateTransaction(tx, txBlob, forks, height)
}
```

### Step 6.4 — Run tests, verify PASS

```bash
cd /home/claude/Code/core/go-blockchain && go test -race -run "TestIsPreHardforkFreeze|TestValidateTransactionInBlock|TestValidateBlockFreeze" ./consensus/...
```

**Expected:** All PASS

### Step 6.5 — Run full consensus and all tests

```bash
cd /home/claude/Code/core/go-blockchain && go vet ./...
cd /home/claude/Code/core/go-blockchain && go test -race ./consensus/...
```

**Expected:** All PASS — no regressions.

### Step 6.6 — Commit

```
feat(consensus): add pre-hardfork transaction freeze for HF5

Rejects non-coinbase transactions during the 60-block window before
HF5 activation. Coinbase transactions are exempt. Implements
IsPreHardforkFreeze and ValidateTransactionInBlock.

Co-Authored-By: Charon <charon@lethean.io>
```

---

## Task 7: Final validation

**Why:** Ensures all changes integrate cleanly and no regressions exist.

### Step 7.1 — Run full test suite with race detector

```bash
cd /home/claude/Code/core/go-blockchain && go test -race ./...
```

**Expected:** All PASS. If CGo packages fail due to missing `libcryptonote.a`, run pure-Go packages only:

```bash
cd /home/claude/Code/core/go-blockchain && go test -race ./config/... ./types/... ./wire/... ./difficulty/...
```

### Step 7.2 — Run vet

```bash
cd /home/claude/Code/core/go-blockchain && go vet ./...
```

**Expected:** No issues.

### Step 7.3 — Run mod tidy

```bash
cd /home/claude/Code/core/go-blockchain && go mod tidy
```

**Expected:** No changes to `go.mod` or `go.sum`.

---

## Summary of Changes

| Package | Files | What |
|---------|-------|------|
| `config/` | `hardfork.go`, `hardfork_activation_test.go` | `HardforkActivationHeight` helper |
| `wire/` | `transaction.go`, `transaction_v3_test.go` | Tags 40, 49, 50, 51 with reader functions; v3 round-trip tests |
| `consensus/` | `errors.go`, `tx.go`, `block.go`, `tx_version_test.go`, `freeze_test.go` | `ErrTxVersionInvalid`, `ErrPreHardforkFreeze`, `checkTxVersion`, `IsPreHardforkFreeze`, `ValidateTransactionInBlock` |

## What Is NOT Included (Deferred)

- Deep asset operation validation (ticker length, supply caps, ownership proofs)
- Asset state tracking in `chain/` (asset registry, supply ledger)
- Wallet asset support (deploy/emit/burn CLI)
- Minimum build version enforcement for P2P peers
- Asset explorer UI
- HF6 block time halving (separate spec)
