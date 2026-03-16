# HF1 Transaction Types Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox syntax for tracking.

**Goal:** Add HTLC and multisig transaction type support for hardfork 1 (block 10,080) so the Go node can sync past HF1.

**Architecture:** New types in types/, wire encoding/decoding in wire/, consensus validation gated on HF1. TxOutputBare.Target refactored from concrete TxOutToKey to TxOutTarget interface. All existing tests must continue to pass.

**Tech Stack:** Go 1.26, CGo crypto bridge (libcryptonote.a), go test -race

---

## File Map

### Modified files

| File | What changes |
|------|-------------|
| `types/transaction.go` | Add `TxOutTarget` interface, `TargetType()` on `TxOutToKey`, new types `TxInputHTLC`, `TxInputMultisig`, `TxOutMultisig`, `TxOutHTLC`. Change `TxOutputBare.Target` from `TxOutToKey` to `TxOutTarget`. |
| `wire/transaction.go` | Add encode/decode cases for `TxInputHTLC`, `TxInputMultisig` in `encodeInputs`/`decodeInputs`. Add target switch for `TxOutMultisig`/`TxOutHTLC` in all four output functions (`encodeOutputsV1`, `decodeOutputsV1`, `encodeOutputsV2`, `decodeOutputsV2`). |
| `consensus/tx.go` | Change `checkInputTypes`/`checkOutputs` signatures to accept `hf1Active`. Gate HTLC/multisig on HF1. Update `checkKeyImages` for `TxInputHTLC`. |
| `consensus/fee.go` | Update `sumInputs` to include `TxInputHTLC.Amount` and `TxInputMultisig.Amount`. |
| `consensus/block.go` | Add block major version check in `ValidateBlock` for HF1. |
| `consensus/verify.go` | Update `verifyV1Signatures` to count `TxInputHTLC` alongside `TxInputToKey`. |
| `consensus/errors.go` | Add `ErrBlockMajorVersion` sentinel. |
| `chain/ring.go` | Type-assert `TxOutputBare.Target` to `TxOutToKey` when extracting key. |
| `chain/sync.go` | No change needed — `indexOutputs` accesses `TxOutputBare.Amount` (outer struct), not target fields. |
| `wallet/scanner.go` | Type-assert `bare.Target` to `TxOutToKey` before accessing `.Key`. |
| `wallet/builder.go` | No change — constructs `TxOutToKey{}` which satisfies `TxOutTarget` interface. |
| `tui/explorer_model.go` | Type-assert `v.Target` to `TxOutToKey` before accessing `.Key`. |

### Modified test files

| File | What changes |
|------|-------------|
| `types/transaction_test.go` | New file — tests for `TxOutTarget` interface, `TargetType()`, `InputType()` on new types. |
| `wire/transaction_test.go` | Add round-trip tests for HTLC/multisig inputs and output targets. |
| `consensus/tx_test.go` | Add tests for HF1-gated input/output acceptance and rejection. |
| `consensus/fee_test.go` | Add tests for `sumInputs` with HTLC/multisig amounts. |
| `consensus/block_test.go` | Add tests for block major version validation. |
| `consensus/verify_test.go` | Add structural test for mixed TxInputToKey + TxInputHTLC signature count. |

---

## Task 1: TxOutTarget interface + TxOutToKey.TargetType() method

**Package:** `types/`
**Why:** Establishes the interface that all output target types will implement. Must exist before changing `TxOutputBare.Target`.

### Step 1.1 — Write test for TxOutTarget interface compliance

- [ ] Create `/home/claude/Code/core/go-blockchain/types/transaction_test.go`

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package types

import "testing"

func TestTxOutToKey_TargetType_Good(t *testing.T) {
	var target TxOutTarget = TxOutToKey{Key: PublicKey{1}, MixAttr: 0}
	if target.TargetType() != TargetTypeToKey {
		t.Errorf("TargetType: got %d, want %d", target.TargetType(), TargetTypeToKey)
	}
}
```

### Step 1.2 — Run test, verify FAIL

```bash
go test -race -run TestTxOutToKey_TargetType_Good ./types/...
```

**Expected:** Compilation error — `TxOutTarget` and `TargetType()` do not exist yet.

### Step 1.3 — Implement TxOutTarget interface and TargetType() on TxOutToKey

- [ ] Edit `/home/claude/Code/core/go-blockchain/types/transaction.go`

Add after the `TxOutToKey` struct (after line 123):

```go
// TxOutTarget is the interface implemented by all output target types
// within a TxOutputBare. Each target variant has a unique wire tag.
type TxOutTarget interface {
	TargetType() uint8
}

// TargetType returns the wire variant tag for to_key targets.
func (t TxOutToKey) TargetType() uint8 { return TargetTypeToKey }
```

### Step 1.4 — Run test, verify PASS

```bash
go test -race -run TestTxOutToKey_TargetType_Good ./types/...
```

**Expected:** PASS — `TxOutToKey` satisfies `TxOutTarget`.

### Step 1.5 — Commit

```
feat(types): add TxOutTarget interface with TargetType method

Co-Authored-By: Charon <charon@lethean.io>
```

---

## Task 2: Update TxOutputBare.Target to interface + fix all call sites

**Package:** `types/`, `wire/`, `consensus/`, `chain/`, `wallet/`, `tui/`
**Why:** This is the breaking change. All direct field access (`out.Target.Key`) must become type assertions. Must be done as one atomic change so the build compiles.

### Step 2.1 — Run existing tests, verify all PASS (baseline)

```bash
go test -race ./types/... ./wire/... ./consensus/...
```

**Expected:** All PASS — establishes baseline before the breaking change.

### Step 2.2 — Change TxOutputBare.Target type and fix all call sites

- [ ] Edit `/home/claude/Code/core/go-blockchain/types/transaction.go`

Change `TxOutputBare.Target` from `TxOutToKey` to `TxOutTarget`:

```go
// TxOutputBare is a transparent (pre-Zarcanum) transaction output.
type TxOutputBare struct {
	// Amount in atomic units.
	Amount uint64

	// Target is the output destination. Before HF1 this is always TxOutToKey;
	// after HF1 it may also be TxOutMultisig or TxOutHTLC.
	Target TxOutTarget
}
```

- [ ] Edit `/home/claude/Code/core/go-blockchain/wire/transaction.go` — `encodeOutputsV1` (line ~253-255)

Replace:
```go
		case types.TxOutputBare:
			enc.WriteVarint(v.Amount)
			// Target is a variant (txout_target_v)
			enc.WriteVariantTag(types.TargetTypeToKey)
			enc.WriteBlob32((*[32]byte)(&v.Target.Key))
			enc.WriteUint8(v.Target.MixAttr)
```

With:
```go
		case types.TxOutputBare:
			enc.WriteVarint(v.Amount)
			// Target is a variant (txout_target_v)
			switch tgt := v.Target.(type) {
			case types.TxOutToKey:
				enc.WriteVariantTag(types.TargetTypeToKey)
				enc.WriteBlob32((*[32]byte)(&tgt.Key))
				enc.WriteUint8(tgt.MixAttr)
			}
```

- [ ] Edit `/home/claude/Code/core/go-blockchain/wire/transaction.go` — `decodeOutputsV1` (line ~275-276)

Replace:
```go
		case types.TargetTypeToKey:
			dec.ReadBlob32((*[32]byte)(&out.Target.Key))
			out.Target.MixAttr = dec.ReadUint8()
```

With:
```go
		case types.TargetTypeToKey:
			var tgt types.TxOutToKey
			dec.ReadBlob32((*[32]byte)(&tgt.Key))
			tgt.MixAttr = dec.ReadUint8()
			out.Target = tgt
```

- [ ] Edit `/home/claude/Code/core/go-blockchain/wire/transaction.go` — `encodeOutputsV2` (line ~293-296)

Replace:
```go
		case types.TxOutputBare:
			enc.WriteVarint(v.Amount)
			enc.WriteVariantTag(types.TargetTypeToKey)
			enc.WriteBlob32((*[32]byte)(&v.Target.Key))
			enc.WriteUint8(v.Target.MixAttr)
```

With:
```go
		case types.TxOutputBare:
			enc.WriteVarint(v.Amount)
			switch tgt := v.Target.(type) {
			case types.TxOutToKey:
				enc.WriteVariantTag(types.TargetTypeToKey)
				enc.WriteBlob32((*[32]byte)(&tgt.Key))
				enc.WriteUint8(tgt.MixAttr)
			}
```

- [ ] Edit `/home/claude/Code/core/go-blockchain/wire/transaction.go` — `decodeOutputsV2` (line ~324-326)

Replace:
```go
			if targetTag == types.TargetTypeToKey {
				dec.ReadBlob32((*[32]byte)(&out.Target.Key))
				out.Target.MixAttr = dec.ReadUint8()
```

With:
```go
			if targetTag == types.TargetTypeToKey {
				var tgt types.TxOutToKey
				dec.ReadBlob32((*[32]byte)(&tgt.Key))
				tgt.MixAttr = dec.ReadUint8()
				out.Target = tgt
```

- [ ] Edit `/home/claude/Code/core/go-blockchain/chain/ring.go` — `GetRingOutputs` (line ~38)

Replace:
```go
		case types.TxOutputBare:
			pubs[i] = out.Target.Key
```

With:
```go
		case types.TxOutputBare:
			toKey, ok := out.Target.(types.TxOutToKey)
			if !ok {
				return nil, fmt.Errorf("ring output %d: unsupported target type %T", i, out.Target)
			}
			pubs[i] = toKey.Key
```

- [ ] Edit `/home/claude/Code/core/go-blockchain/wallet/scanner.go` — (line ~67)

Replace:
```go
		if types.PublicKey(expectedPub) != bare.Target.Key {
```

With:
```go
		toKey, ok := bare.Target.(types.TxOutToKey)
		if !ok {
			continue
		}
		if types.PublicKey(expectedPub) != toKey.Key {
```

- [ ] Edit `/home/claude/Code/core/go-blockchain/tui/explorer_model.go` — (line ~328)

Replace:
```go
			case types.TxOutputBare:
				b.WriteString(fmt.Sprintf("  [%d] bare amount=%d key=%x\n", i, v.Amount, v.Target.Key[:4]))
```

With:
```go
			case types.TxOutputBare:
				if toKey, ok := v.Target.(types.TxOutToKey); ok {
					b.WriteString(fmt.Sprintf("  [%d] bare amount=%d key=%x\n", i, v.Amount, toKey.Key[:4]))
				} else {
					b.WriteString(fmt.Sprintf("  [%d] bare amount=%d target=%T\n", i, v.Amount, v.Target))
				}
```

- [ ] Fix all test files that construct `TxOutputBare{..., Target: types.TxOutToKey{...}}` — these already use the concrete type in the struct literal, which satisfies the interface, so **no test changes needed for construction**. However, test assertions that access `.Target.Key` directly must be updated.

- [ ] Edit `/home/claude/Code/core/go-blockchain/wire/transaction_test.go` — any line accessing `bare.Target.Key`

Replace:
```go
	if bare.Target.Key[0] != 0xDE || bare.Target.Key[1] != 0xAD {
		t.Errorf("target key: got %x, want DE AD...", bare.Target.Key[:2])
	}
```

With:
```go
	toKey, ok := bare.Target.(types.TxOutToKey)
	if !ok {
		t.Fatalf("target type: got %T, want TxOutToKey", bare.Target)
	}
	if toKey.Key[0] != 0xDE || toKey.Key[1] != 0xAD {
		t.Errorf("target key: got %x, want DE AD...", toKey.Key[:2])
	}
```

### Step 2.3 — Run all tests, verify PASS

```bash
go vet ./...
go test -race ./types/... ./wire/... ./consensus/... ./chain/... ./wallet/... ./tui/...
```

**Expected:** All PASS — the interface refactor is transparent to existing behaviour.

### Step 2.4 — Commit

```
refactor(types): change TxOutputBare.Target to TxOutTarget interface

Prepares for HF1 output target types (TxOutMultisig, TxOutHTLC).
All call sites updated to type-assert TxOutToKey where needed.

Co-Authored-By: Charon <charon@lethean.io>
```

---

## Task 3: New output target types — TxOutMultisig, TxOutHTLC

**Package:** `types/`
**Why:** These types must exist before wire encoding can reference them.

### Step 3.1 — Write tests for new target types

- [ ] Append to `/home/claude/Code/core/go-blockchain/types/transaction_test.go`

```go
func TestTxOutMultisig_TargetType_Good(t *testing.T) {
	var target TxOutTarget = TxOutMultisig{MinimumSigs: 2, Keys: []PublicKey{{1}, {2}}}
	if target.TargetType() != TargetTypeMultisig {
		t.Errorf("TargetType: got %d, want %d", target.TargetType(), TargetTypeMultisig)
	}
}

func TestTxOutHTLC_TargetType_Good(t *testing.T) {
	var target TxOutTarget = TxOutHTLC{
		Flags:      0,
		Expiration: 10080,
	}
	if target.TargetType() != TargetTypeHTLC {
		t.Errorf("TargetType: got %d, want %d", target.TargetType(), TargetTypeHTLC)
	}
}
```

### Step 3.2 — Run tests, verify FAIL

```bash
go test -race -run "TestTxOutMultisig_TargetType_Good|TestTxOutHTLC_TargetType_Good" ./types/...
```

**Expected:** Compilation error — `TxOutMultisig` and `TxOutHTLC` do not exist.

### Step 3.3 — Implement new target types

- [ ] Edit `/home/claude/Code/core/go-blockchain/types/transaction.go`

Add after the `TxOutToKey.TargetType()` method:

```go
// TxOutMultisig is the txout_multisig target variant (HF1+).
// Spendable when minimum_sigs of the listed keys sign.
type TxOutMultisig struct {
	MinimumSigs uint64
	Keys        []PublicKey
}

// TargetType returns the wire variant tag for multisig targets.
func (t TxOutMultisig) TargetType() uint8 { return TargetTypeMultisig }

// TxOutHTLC is the txout_htlc target variant (HF1+).
// Hash Time-Locked Contract: redeemable with hash preimage before
// expiration, refundable after expiration.
type TxOutHTLC struct {
	HTLCHash   Hash      // 32-byte hash lock
	Flags      uint8     // bit 0: 0=SHA256, 1=RIPEMD160
	Expiration uint64    // block height deadline
	PKRedeem   PublicKey // recipient key (can redeem before expiration)
	PKRefund   PublicKey // sender key (can refund after expiration)
}

// TargetType returns the wire variant tag for HTLC targets.
func (t TxOutHTLC) TargetType() uint8 { return TargetTypeHTLC }
```

### Step 3.4 — Run tests, verify PASS

```bash
go test -race -run "TestTxOutMultisig_TargetType_Good|TestTxOutHTLC_TargetType_Good" ./types/...
```

**Expected:** PASS.

### Step 3.5 — Commit

```
feat(types): add TxOutMultisig and TxOutHTLC target types

Output target types for HF1 HTLC and multisig transactions.

Co-Authored-By: Charon <charon@lethean.io>
```

---

## Task 4: New input types — TxInputHTLC, TxInputMultisig

**Package:** `types/`
**Why:** Input types must exist before wire and consensus can reference them.

### Step 4.1 — Write tests for new input types

- [ ] Append to `/home/claude/Code/core/go-blockchain/types/transaction_test.go`

```go
func TestTxInputHTLC_InputType_Good(t *testing.T) {
	var input TxInput = TxInputHTLC{
		HTLCOrigin: "test",
		Amount:     1000,
		KeyImage:   KeyImage{1},
	}
	if input.InputType() != InputTypeHTLC {
		t.Errorf("InputType: got %d, want %d", input.InputType(), InputTypeHTLC)
	}
}

func TestTxInputMultisig_InputType_Good(t *testing.T) {
	var input TxInput = TxInputMultisig{
		Amount:    500,
		SigsCount: 2,
	}
	if input.InputType() != InputTypeMultisig {
		t.Errorf("InputType: got %d, want %d", input.InputType(), InputTypeMultisig)
	}
}
```

### Step 4.2 — Run tests, verify FAIL

```bash
go test -race -run "TestTxInputHTLC_InputType_Good|TestTxInputMultisig_InputType_Good" ./types/...
```

**Expected:** Compilation error — `TxInputHTLC` and `TxInputMultisig` do not exist.

### Step 4.3 — Implement new input types

- [ ] Edit `/home/claude/Code/core/go-blockchain/types/transaction.go`

Add after the `TxInputZC` type (after `InputType()` for ZC, around line 170):

```go
// TxInputHTLC extends TxInputToKey with an HTLC origin hash (HF1+).
// Wire order: HTLCOrigin (string) serialised BEFORE parent fields (C++ quirk).
// Carries Amount, KeyOffsets, KeyImage, EtcDetails — same as TxInputToKey.
type TxInputHTLC struct {
	HTLCOrigin string       // C++ field: hltc_origin (transposed in source)
	Amount     uint64
	KeyOffsets []TxOutRef
	KeyImage   KeyImage
	EtcDetails []byte       // opaque variant vector
}

// InputType returns the wire variant tag for HTLC inputs.
func (t TxInputHTLC) InputType() uint8 { return InputTypeHTLC }

// TxInputMultisig spends from a multisig output (HF1+).
type TxInputMultisig struct {
	Amount        uint64
	MultisigOutID Hash   // 32-byte hash identifying the multisig output
	SigsCount     uint64
	EtcDetails    []byte // opaque variant vector
}

// InputType returns the wire variant tag for multisig inputs.
func (t TxInputMultisig) InputType() uint8 { return InputTypeMultisig }
```

### Step 4.4 — Run tests, verify PASS

```bash
go test -race -run "TestTxInputHTLC_InputType_Good|TestTxInputMultisig_InputType_Good" ./types/...
```

**Expected:** PASS.

### Step 4.5 — Run full types test suite

```bash
go test -race ./types/...
```

**Expected:** All PASS.

### Step 4.6 — Commit

```
feat(types): add TxInputHTLC and TxInputMultisig input types

Input types for HF1 HTLC and multisig transactions.

Co-Authored-By: Charon <charon@lethean.io>
```

---

## Task 5: Wire encoding/decoding for new input types

**Package:** `wire/`
**Why:** The node must deserialise blocks containing HTLC/multisig inputs to sync past HF1.

### Step 5.1 — Write round-trip test for TxInputHTLC

- [ ] Append to `/home/claude/Code/core/go-blockchain/wire/transaction_test.go`

```go
func TestHTLCInputRoundTrip_Good(t *testing.T) {
	tx := types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputHTLC{
				HTLCOrigin: "test_origin",
				Amount:     42000,
				KeyOffsets: []types.TxOutRef{
					{Tag: types.RefTypeGlobalIndex, GlobalIndex: 100},
				},
				KeyImage:   types.KeyImage{0xAA},
				EtcDetails: EncodeVarint(0),
			},
		},
		Vout: []types.TxOutput{types.TxOutputBare{
			Amount: 41000,
			Target: types.TxOutToKey{Key: types.PublicKey{0x01}},
		}},
		Extra: EncodeVarint(0),
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransactionPrefix(enc, &tx)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	got := DecodeTransactionPrefix(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	if len(got.Vin) != 1 {
		t.Fatalf("vin count: got %d, want 1", len(got.Vin))
	}
	htlc, ok := got.Vin[0].(types.TxInputHTLC)
	if !ok {
		t.Fatalf("vin[0] type: got %T, want TxInputHTLC", got.Vin[0])
	}
	if htlc.HTLCOrigin != "test_origin" {
		t.Errorf("HTLCOrigin: got %q, want %q", htlc.HTLCOrigin, "test_origin")
	}
	if htlc.Amount != 42000 {
		t.Errorf("Amount: got %d, want 42000", htlc.Amount)
	}
	if htlc.KeyImage[0] != 0xAA {
		t.Errorf("KeyImage[0]: got 0x%02x, want 0xAA", htlc.KeyImage[0])
	}
}
```

### Step 5.2 — Write round-trip test for TxInputMultisig

- [ ] Append to `/home/claude/Code/core/go-blockchain/wire/transaction_test.go`

```go
func TestMultisigInputRoundTrip_Good(t *testing.T) {
	tx := types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputMultisig{
				Amount:        50000,
				MultisigOutID: types.Hash{0xBB},
				SigsCount:     3,
				EtcDetails:    EncodeVarint(0),
			},
		},
		Vout: []types.TxOutput{types.TxOutputBare{
			Amount: 49000,
			Target: types.TxOutToKey{Key: types.PublicKey{0x02}},
		}},
		Extra: EncodeVarint(0),
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransactionPrefix(enc, &tx)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	got := DecodeTransactionPrefix(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	if len(got.Vin) != 1 {
		t.Fatalf("vin count: got %d, want 1", len(got.Vin))
	}
	msig, ok := got.Vin[0].(types.TxInputMultisig)
	if !ok {
		t.Fatalf("vin[0] type: got %T, want TxInputMultisig", got.Vin[0])
	}
	if msig.Amount != 50000 {
		t.Errorf("Amount: got %d, want 50000", msig.Amount)
	}
	if msig.MultisigOutID[0] != 0xBB {
		t.Errorf("MultisigOutID[0]: got 0x%02x, want 0xBB", msig.MultisigOutID[0])
	}
	if msig.SigsCount != 3 {
		t.Errorf("SigsCount: got %d, want 3", msig.SigsCount)
	}
}
```

### Step 5.3 — Run tests, verify FAIL

```bash
go test -race -run "TestHTLCInputRoundTrip_Good|TestMultisigInputRoundTrip_Good" ./wire/...
```

**Expected:** Fails — `encodeInputs`/`decodeInputs` do not handle the new tags. Likely hits "unsupported input tag" error on decode, or silently skips on encode.

### Step 5.4 — Implement input encoding for TxInputHTLC and TxInputMultisig

- [ ] Edit `/home/claude/Code/core/go-blockchain/wire/transaction.go` — `encodeInputs` function

Add cases after the `TxInputZC` case (before the closing `}` of the switch):

```go
		case types.TxInputHTLC:
			// Wire order: hltc_origin (string) BEFORE parent fields (C++ quirk).
			enc.WriteVarint(uint64(len(v.HTLCOrigin)))
			enc.WriteRawBytes([]byte(v.HTLCOrigin))
			enc.WriteVarint(v.Amount)
			encodeKeyOffsets(enc, v.KeyOffsets)
			enc.WriteBlob32((*[32]byte)(&v.KeyImage))
			enc.WriteBytes(v.EtcDetails)
		case types.TxInputMultisig:
			enc.WriteVarint(v.Amount)
			enc.WriteBlob32((*[32]byte)(&v.MultisigOutID))
			enc.WriteVarint(v.SigsCount)
			enc.WriteBytes(v.EtcDetails)
```

### Step 5.5 — Implement input decoding for TxInputHTLC and TxInputMultisig

- [ ] Edit `/home/claude/Code/core/go-blockchain/wire/transaction.go` — `decodeInputs` function

Add cases before the `default:` case:

```go
		case types.InputTypeHTLC:
			var in types.TxInputHTLC
			// Wire order: hltc_origin (string) BEFORE parent fields.
			originLen := dec.ReadVarint()
			if originLen > 0 && dec.Err() == nil {
				in.HTLCOrigin = string(dec.ReadBytes(int(originLen)))
			}
			in.Amount = dec.ReadVarint()
			in.KeyOffsets = decodeKeyOffsets(dec)
			dec.ReadBlob32((*[32]byte)(&in.KeyImage))
			in.EtcDetails = decodeRawVariantVector(dec)
			vin = append(vin, in)
		case types.InputTypeMultisig:
			var in types.TxInputMultisig
			in.Amount = dec.ReadVarint()
			dec.ReadBlob32((*[32]byte)(&in.MultisigOutID))
			in.SigsCount = dec.ReadVarint()
			in.EtcDetails = decodeRawVariantVector(dec)
			vin = append(vin, in)
```

### Step 5.6 — Check that Encoder has WriteRawBytes or equivalent

The existing `Encoder` has `WriteBytes` (which writes raw bytes). Check whether there is a distinction between `WriteBytes` (includes length prefix) and raw writes. If `WriteBytes` writes raw (no prefix), use it for the HTLC origin string body. If it includes a prefix, we need `WriteRawBytes`.

Look at the encoder to confirm. If `enc.WriteBytes` writes raw bytes (no varint prefix), then the HTLC origin encode should be:
```go
enc.WriteVarint(uint64(len(v.HTLCOrigin)))
enc.WriteBytes([]byte(v.HTLCOrigin))
```

But `WriteBytes` already exists and is used for `tx.Extra` which includes its own varint prefix. So we likely need a different method. Check the encoder API and use whichever writes raw bytes without a length prefix. The encoder likely has a method like `WriteRawBytes` or we write via the underlying writer. If no such method exists, add one to the encoder.

**Alternative:** Since `ReadBytes(n)` exists on the decoder (returns n raw bytes), the encoder should have a matching raw write. If it uses `WriteBytes([]byte)` as "write these exact bytes", then we can use:
```go
enc.WriteBytes(append(EncodeVarint(uint64(len(v.HTLCOrigin))), []byte(v.HTLCOrigin)...))
```

Or more cleanly, build the string blob:
```go
strBlob := append(EncodeVarint(uint64(len(v.HTLCOrigin))), []byte(v.HTLCOrigin)...)
enc.WriteBytes(strBlob)
```

**Implementation note:** Check how `enc.WriteBytes` works. From the existing code, `enc.WriteBytes(tx.Extra)` writes the Extra field which includes its own varint count. This suggests `WriteBytes` writes raw. The HTLC origin is a string serialised as `varint(len) + bytes`, so:

```go
// In encodeInputs, TxInputHTLC case:
enc.WriteVarint(uint64(len(v.HTLCOrigin)))
if len(v.HTLCOrigin) > 0 {
	enc.WriteBytes([]byte(v.HTLCOrigin))
}
```

Adjust as needed based on the actual encoder API.

### Step 5.7 — Run tests, verify PASS

```bash
go test -race -run "TestHTLCInputRoundTrip_Good|TestMultisigInputRoundTrip_Good" ./wire/...
```

**Expected:** PASS — round-trip encode/decode works for both new input types.

### Step 5.8 — Run full wire test suite

```bash
go test -race ./wire/...
```

**Expected:** All PASS — existing tests unaffected.

### Step 5.9 — Commit

```
feat(wire): encode/decode TxInputHTLC and TxInputMultisig

Adds wire serialisation for HF1 HTLC (tag 0x22) and multisig
(tag 0x02) input types.

Co-Authored-By: Charon <charon@lethean.io>
```

---

## Task 6: Wire encoding/decoding for new output targets — both V1 and V2

**Package:** `wire/`
**Why:** Blocks after HF1 may contain outputs with multisig or HTLC targets.

### Step 6.1 — Write round-trip test for TxOutMultisig target (V1)

- [ ] Append to `/home/claude/Code/core/go-blockchain/wire/transaction_test.go`

```go
func TestMultisigTargetV1RoundTrip_Good(t *testing.T) {
	tx := types.Transaction{
		Version: types.VersionPreHF4,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 1}},
		Vout: []types.TxOutput{types.TxOutputBare{
			Amount: 5000,
			Target: types.TxOutMultisig{
				MinimumSigs: 2,
				Keys:        []types.PublicKey{{0x01}, {0x02}, {0x03}},
			},
		}},
		Extra: EncodeVarint(0),
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransactionPrefix(enc, &tx)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	got := DecodeTransactionPrefix(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	bare, ok := got.Vout[0].(types.TxOutputBare)
	if !ok {
		t.Fatalf("vout[0] type: got %T, want TxOutputBare", got.Vout[0])
	}
	msig, ok := bare.Target.(types.TxOutMultisig)
	if !ok {
		t.Fatalf("target type: got %T, want TxOutMultisig", bare.Target)
	}
	if msig.MinimumSigs != 2 {
		t.Errorf("MinimumSigs: got %d, want 2", msig.MinimumSigs)
	}
	if len(msig.Keys) != 3 {
		t.Errorf("Keys count: got %d, want 3", len(msig.Keys))
	}
}
```

### Step 6.2 — Write round-trip test for TxOutHTLC target (V1)

- [ ] Append to `/home/claude/Code/core/go-blockchain/wire/transaction_test.go`

```go
func TestHTLCTargetV1RoundTrip_Good(t *testing.T) {
	tx := types.Transaction{
		Version: types.VersionPreHF4,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 1}},
		Vout: []types.TxOutput{types.TxOutputBare{
			Amount: 7000,
			Target: types.TxOutHTLC{
				HTLCHash:   types.Hash{0xCC},
				Flags:      1, // RIPEMD160
				Expiration: 20000,
				PKRedeem:   types.PublicKey{0xDD},
				PKRefund:   types.PublicKey{0xEE},
			},
		}},
		Extra: EncodeVarint(0),
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransactionPrefix(enc, &tx)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	got := DecodeTransactionPrefix(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	bare, ok := got.Vout[0].(types.TxOutputBare)
	if !ok {
		t.Fatalf("vout[0] type: got %T, want TxOutputBare", got.Vout[0])
	}
	htlc, ok := bare.Target.(types.TxOutHTLC)
	if !ok {
		t.Fatalf("target type: got %T, want TxOutHTLC", bare.Target)
	}
	if htlc.HTLCHash[0] != 0xCC {
		t.Errorf("HTLCHash[0]: got 0x%02x, want 0xCC", htlc.HTLCHash[0])
	}
	if htlc.Flags != 1 {
		t.Errorf("Flags: got %d, want 1", htlc.Flags)
	}
	if htlc.Expiration != 20000 {
		t.Errorf("Expiration: got %d, want 20000", htlc.Expiration)
	}
	if htlc.PKRedeem[0] != 0xDD {
		t.Errorf("PKRedeem[0]: got 0x%02x, want 0xDD", htlc.PKRedeem[0])
	}
	if htlc.PKRefund[0] != 0xEE {
		t.Errorf("PKRefund[0]: got 0x%02x, want 0xEE", htlc.PKRefund[0])
	}
}
```

### Step 6.3 — Run tests, verify FAIL

```bash
go test -race -run "TestMultisigTargetV1RoundTrip_Good|TestHTLCTargetV1RoundTrip_Good" ./wire/...
```

**Expected:** Fails — "unsupported target tag" on decode, silent skip on encode.

### Step 6.4 — Implement output target encoding in encodeOutputsV1

- [ ] Edit `/home/claude/Code/core/go-blockchain/wire/transaction.go` — `encodeOutputsV1`

Extend the target switch inside the `TxOutputBare` case to handle all target types:

```go
		case types.TxOutputBare:
			enc.WriteVarint(v.Amount)
			// Target is a variant (txout_target_v)
			switch tgt := v.Target.(type) {
			case types.TxOutToKey:
				enc.WriteVariantTag(types.TargetTypeToKey)
				enc.WriteBlob32((*[32]byte)(&tgt.Key))
				enc.WriteUint8(tgt.MixAttr)
			case types.TxOutMultisig:
				enc.WriteVariantTag(types.TargetTypeMultisig)
				enc.WriteVarint(tgt.MinimumSigs)
				enc.WriteVarint(uint64(len(tgt.Keys)))
				for i := range tgt.Keys {
					enc.WriteBlob32((*[32]byte)(&tgt.Keys[i]))
				}
			case types.TxOutHTLC:
				enc.WriteVariantTag(types.TargetTypeHTLC)
				enc.WriteBlob32((*[32]byte)(&tgt.HTLCHash))
				enc.WriteUint8(tgt.Flags)
				enc.WriteVarint(tgt.Expiration)
				enc.WriteBlob32((*[32]byte)(&tgt.PKRedeem))
				enc.WriteBlob32((*[32]byte)(&tgt.PKRefund))
			}
```

### Step 6.5 — Implement output target decoding in decodeOutputsV1

- [ ] Edit `/home/claude/Code/core/go-blockchain/wire/transaction.go` — `decodeOutputsV1`

Add cases to the target tag switch:

```go
		case types.TargetTypeToKey:
			var tgt types.TxOutToKey
			dec.ReadBlob32((*[32]byte)(&tgt.Key))
			tgt.MixAttr = dec.ReadUint8()
			out.Target = tgt
		case types.TargetTypeMultisig:
			var tgt types.TxOutMultisig
			tgt.MinimumSigs = dec.ReadVarint()
			keyCount := dec.ReadVarint()
			if keyCount > 0 && dec.Err() == nil {
				tgt.Keys = make([]types.PublicKey, keyCount)
				for j := uint64(0); j < keyCount; j++ {
					dec.ReadBlob32((*[32]byte)(&tgt.Keys[j]))
				}
			}
			out.Target = tgt
		case types.TargetTypeHTLC:
			var tgt types.TxOutHTLC
			dec.ReadBlob32((*[32]byte)(&tgt.HTLCHash))
			tgt.Flags = dec.ReadUint8()
			tgt.Expiration = dec.ReadVarint()
			dec.ReadBlob32((*[32]byte)(&tgt.PKRedeem))
			dec.ReadBlob32((*[32]byte)(&tgt.PKRefund))
			out.Target = tgt
```

### Step 6.6 — Implement output target encoding in encodeOutputsV2

- [ ] Edit `/home/claude/Code/core/go-blockchain/wire/transaction.go` — `encodeOutputsV2`

Same target switch as V1 inside the `TxOutputBare` case:

```go
		case types.TxOutputBare:
			enc.WriteVarint(v.Amount)
			switch tgt := v.Target.(type) {
			case types.TxOutToKey:
				enc.WriteVariantTag(types.TargetTypeToKey)
				enc.WriteBlob32((*[32]byte)(&tgt.Key))
				enc.WriteUint8(tgt.MixAttr)
			case types.TxOutMultisig:
				enc.WriteVariantTag(types.TargetTypeMultisig)
				enc.WriteVarint(tgt.MinimumSigs)
				enc.WriteVarint(uint64(len(tgt.Keys)))
				for i := range tgt.Keys {
					enc.WriteBlob32((*[32]byte)(&tgt.Keys[i]))
				}
			case types.TxOutHTLC:
				enc.WriteVariantTag(types.TargetTypeHTLC)
				enc.WriteBlob32((*[32]byte)(&tgt.HTLCHash))
				enc.WriteUint8(tgt.Flags)
				enc.WriteVarint(tgt.Expiration)
				enc.WriteBlob32((*[32]byte)(&tgt.PKRedeem))
				enc.WriteBlob32((*[32]byte)(&tgt.PKRefund))
			}
```

### Step 6.7 — Implement output target decoding in decodeOutputsV2

- [ ] Edit `/home/claude/Code/core/go-blockchain/wire/transaction.go` — `decodeOutputsV2`

Replace the `if/else` target tag check with a switch:

```go
		case types.OutputTypeBare:
			var out types.TxOutputBare
			out.Amount = dec.ReadVarint()
			targetTag := dec.ReadVariantTag()
			if dec.Err() != nil {
				return vout
			}
			switch targetTag {
			case types.TargetTypeToKey:
				var tgt types.TxOutToKey
				dec.ReadBlob32((*[32]byte)(&tgt.Key))
				tgt.MixAttr = dec.ReadUint8()
				out.Target = tgt
			case types.TargetTypeMultisig:
				var tgt types.TxOutMultisig
				tgt.MinimumSigs = dec.ReadVarint()
				keyCount := dec.ReadVarint()
				if keyCount > 0 && dec.Err() == nil {
					tgt.Keys = make([]types.PublicKey, keyCount)
					for j := uint64(0); j < keyCount; j++ {
						dec.ReadBlob32((*[32]byte)(&tgt.Keys[j]))
					}
				}
				out.Target = tgt
			case types.TargetTypeHTLC:
				var tgt types.TxOutHTLC
				dec.ReadBlob32((*[32]byte)(&tgt.HTLCHash))
				tgt.Flags = dec.ReadUint8()
				tgt.Expiration = dec.ReadVarint()
				dec.ReadBlob32((*[32]byte)(&tgt.PKRedeem))
				dec.ReadBlob32((*[32]byte)(&tgt.PKRefund))
				out.Target = tgt
			default:
				dec.err = fmt.Errorf("wire: unsupported target tag 0x%02x", targetTag)
				return vout
			}
			vout = append(vout, out)
```

### Step 6.8 — Run tests, verify PASS

```bash
go test -race -run "TestMultisigTargetV1RoundTrip_Good|TestHTLCTargetV1RoundTrip_Good" ./wire/...
```

**Expected:** PASS.

### Step 6.9 — Run full wire test suite

```bash
go test -race ./wire/...
```

**Expected:** All PASS — existing tests unaffected.

### Step 6.10 — Commit

```
feat(wire): encode/decode TxOutMultisig and TxOutHTLC targets

Adds target variant serialisation in both V1 and V2 output
encoders/decoders. Supports multisig (tag 0x04) and HTLC
(tag 0x23) targets within TxOutputBare.

Co-Authored-By: Charon <charon@lethean.io>
```

---

## Task 7: Consensus — checkInputTypes, checkOutputs + HF1 gating

**Package:** `consensus/`
**Why:** HTLC and multisig must be rejected before HF1, accepted after.

### Step 7.1 — Write test: HTLC input rejected pre-HF1

- [ ] Append to `/home/claude/Code/core/go-blockchain/consensus/tx_test.go`

```go
func TestCheckInputTypes_HTLCPreHF1_Bad(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputHTLC{Amount: 100, KeyImage: types.KeyImage{1}},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 90, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000) // pre-HF1 (10080)
	assert.ErrorIs(t, err, ErrInvalidInputType)
}
```

### Step 7.2 — Write test: HTLC input accepted post-HF1

- [ ] Append to `/home/claude/Code/core/go-blockchain/consensus/tx_test.go`

```go
func TestCheckInputTypes_HTLCPostHF1_Good(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputHTLC{
				Amount:   100,
				KeyImage: types.KeyImage{1},
			},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 90, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 20000) // post-HF1
	require.NoError(t, err)
}
```

### Step 7.3 — Write test: multisig input rejected pre-HF1

- [ ] Append to `/home/claude/Code/core/go-blockchain/consensus/tx_test.go`

```go
func TestCheckInputTypes_MultisigPreHF1_Bad(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputMultisig{Amount: 100},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 90, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrInvalidInputType)
}
```

### Step 7.4 — Write test: HTLC output target rejected pre-HF1

- [ ] Append to `/home/claude/Code/core/go-blockchain/consensus/tx_test.go`

```go
func TestCheckOutputs_HTLCTargetPreHF1_Bad(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: 100, KeyImage: types.KeyImage{1}},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 90,
				Target: types.TxOutHTLC{Expiration: 20000},
			},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrInvalidOutput)
}
```

### Step 7.5 — Write test: multisig output target accepted post-HF1

- [ ] Append to `/home/claude/Code/core/go-blockchain/consensus/tx_test.go`

```go
func TestCheckOutputs_MultisigTargetPostHF1_Good(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: 100, KeyImage: types.KeyImage{1}},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 90,
				Target: types.TxOutMultisig{MinimumSigs: 2, Keys: []types.PublicKey{{1}, {2}}},
			},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 20000) // post-HF1
	require.NoError(t, err)
}
```

### Step 7.6 — Run tests, verify FAIL

```bash
go test -race -run "TestCheckInputTypes_HTLC|TestCheckInputTypes_Multisig|TestCheckOutputs_HTLC|TestCheckOutputs_Multisig" ./consensus/...
```

**Expected:** Mixed failures — pre-HF1 rejection may already work via the `default` case, but post-HF1 acceptance will fail because the current `default` only checks `hf4Active`, not `hf1Active`.

### Step 7.7 — Implement HF1 gating in checkInputTypes

- [ ] Edit `/home/claude/Code/core/go-blockchain/consensus/tx.go`

Update `ValidateTransaction` to compute `hf1Active`:

```go
func ValidateTransaction(tx *types.Transaction, txBlob []byte, forks []config.HardFork, height uint64) error {
	hf1Active := config.IsHardForkActive(forks, config.HF1, height)
	hf4Active := config.IsHardForkActive(forks, config.HF4Zarcanum, height)
	// ... rest unchanged, but pass hf1Active to checkInputTypes and checkOutputs
```

Update `checkInputTypes` signature and body:

```go
func checkInputTypes(tx *types.Transaction, hf1Active, hf4Active bool) error {
	for _, vin := range tx.Vin {
		switch vin.(type) {
		case types.TxInputToKey:
			// Always valid.
		case types.TxInputGenesis:
			return fmt.Errorf("%w: txin_gen in regular transaction", ErrInvalidInputType)
		case types.TxInputHTLC, types.TxInputMultisig:
			if !hf1Active {
				return fmt.Errorf("%w: tag %d pre-HF1", ErrInvalidInputType, vin.InputType())
			}
		default:
			// Future types (ZC, etc.) — accept if HF4+.
			if !hf4Active {
				return fmt.Errorf("%w: tag %d pre-HF4", ErrInvalidInputType, vin.InputType())
			}
		}
	}
	return nil
}
```

Update the call in `ValidateTransaction`:

```go
	if err := checkInputTypes(tx, hf1Active, hf4Active); err != nil {
```

### Step 7.8 — Implement HF1 gating in checkOutputs

- [ ] Edit `/home/claude/Code/core/go-blockchain/consensus/tx.go`

Update `checkOutputs` signature and body:

```go
func checkOutputs(tx *types.Transaction, hf1Active, hf4Active bool) error {
	if len(tx.Vout) == 0 {
		return ErrNoOutputs
	}

	if hf4Active && uint64(len(tx.Vout)) < config.TxMinAllowedOutputs {
		return fmt.Errorf("%w: %d (min %d)", ErrTooFewOutputs, len(tx.Vout), config.TxMinAllowedOutputs)
	}

	if uint64(len(tx.Vout)) > config.TxMaxAllowedOutputs {
		return fmt.Errorf("%w: %d", ErrTooManyOutputs, len(tx.Vout))
	}

	for i, vout := range tx.Vout {
		switch o := vout.(type) {
		case types.TxOutputBare:
			if o.Amount == 0 {
				return fmt.Errorf("%w: output %d has zero amount", ErrInvalidOutput, i)
			}
			// Check target type gating.
			switch o.Target.(type) {
			case types.TxOutToKey:
				// Always valid.
			case types.TxOutMultisig, types.TxOutHTLC:
				if !hf1Active {
					return fmt.Errorf("%w: output %d has target type %d pre-HF1",
						ErrInvalidOutput, i, o.Target.TargetType())
				}
			}
		case types.TxOutputZarcanum:
			// Validated by proof verification.
		}
	}

	return nil
}
```

Update the call in `ValidateTransaction`:

```go
	if err := checkOutputs(tx, hf1Active, hf4Active); err != nil {
```

### Step 7.9 — Run tests, verify PASS

```bash
go test -race -run "TestCheckInputTypes_HTLC|TestCheckInputTypes_Multisig|TestCheckOutputs_HTLC|TestCheckOutputs_Multisig" ./consensus/...
```

**Expected:** All PASS.

### Step 7.10 — Run full consensus test suite

```bash
go test -race ./consensus/...
```

**Expected:** All PASS — existing tests unaffected.

### Step 7.11 — Commit

```
feat(consensus): gate HTLC and multisig types on HF1

checkInputTypes and checkOutputs now accept hf1Active flag.
HTLC and multisig inputs/outputs are rejected before HF1
(block 10,080) and accepted after.

Co-Authored-By: Charon <charon@lethean.io>
```

---

## Task 8: Consensus — sumInputs, checkKeyImages for HTLC/multisig

**Package:** `consensus/`
**Why:** Fee calculation and double-spend prevention must cover new input types.

### Step 8.1 — Write test: sumInputs includes HTLC amount

- [ ] Append to `/home/claude/Code/core/go-blockchain/consensus/fee_test.go`

```go
func TestTxFee_HTLCInput_Good(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputHTLC{Amount: 100, KeyImage: types.KeyImage{1}},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 90, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	fee, err := TxFee(tx)
	require.NoError(t, err)
	assert.Equal(t, uint64(10), fee)
}

func TestTxFee_MultisigInput_Good(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputMultisig{Amount: 200},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 150, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	fee, err := TxFee(tx)
	require.NoError(t, err)
	assert.Equal(t, uint64(50), fee)
}
```

### Step 8.2 — Write test: checkKeyImages catches duplicate HTLC key image

- [ ] Append to `/home/claude/Code/core/go-blockchain/consensus/tx_test.go`

```go
func TestCheckKeyImages_HTLCDuplicate_Bad(t *testing.T) {
	ki := types.KeyImage{0x42}
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputHTLC{Amount: 100, KeyImage: ki},
			types.TxInputHTLC{Amount: 50, KeyImage: ki}, // duplicate
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 140, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 20000) // post-HF1
	assert.ErrorIs(t, err, ErrDuplicateKeyImage)
}

func TestCheckKeyImages_HTLCAndToKeyDuplicate_Bad(t *testing.T) {
	ki := types.KeyImage{0x42}
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: 100, KeyImage: ki},
			types.TxInputHTLC{Amount: 50, KeyImage: ki}, // duplicate across types
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 140, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 20000)
	assert.ErrorIs(t, err, ErrDuplicateKeyImage)
}
```

### Step 8.3 — Run tests, verify FAIL

```bash
go test -race -run "TestTxFee_HTLCInput_Good|TestTxFee_MultisigInput_Good|TestCheckKeyImages_HTLC" ./consensus/...
```

**Expected:** Fee tests fail (HTLC/multisig amounts not summed — fee appears negative). Key image tests fail (HTLC key images not checked).

### Step 8.4 — Update sumInputs for HTLC and multisig

- [ ] Edit `/home/claude/Code/core/go-blockchain/consensus/fee.go` — `sumInputs`

Replace:
```go
func sumInputs(tx *types.Transaction) (uint64, error) {
	var total uint64
	for _, vin := range tx.Vin {
		toKey, ok := vin.(types.TxInputToKey)
		if !ok {
			continue
		}
		if total > math.MaxUint64-toKey.Amount {
			return 0, ErrInputOverflow
		}
		total += toKey.Amount
	}
	return total, nil
}
```

With:
```go
func sumInputs(tx *types.Transaction) (uint64, error) {
	var total uint64
	for _, vin := range tx.Vin {
		var amount uint64
		switch v := vin.(type) {
		case types.TxInputToKey:
			amount = v.Amount
		case types.TxInputHTLC:
			amount = v.Amount
		case types.TxInputMultisig:
			amount = v.Amount
		default:
			continue
		}
		if total > math.MaxUint64-amount {
			return 0, ErrInputOverflow
		}
		total += amount
	}
	return total, nil
}
```

### Step 8.5 — Update checkKeyImages for HTLC

- [ ] Edit `/home/claude/Code/core/go-blockchain/consensus/tx.go` — `checkKeyImages`

Replace:
```go
func checkKeyImages(tx *types.Transaction) error {
	seen := make(map[types.KeyImage]struct{})
	for _, vin := range tx.Vin {
		toKey, ok := vin.(types.TxInputToKey)
		if !ok {
			continue
		}
		if _, exists := seen[toKey.KeyImage]; exists {
			return fmt.Errorf("%w: %s", ErrDuplicateKeyImage, toKey.KeyImage)
		}
		seen[toKey.KeyImage] = struct{}{}
	}
	return nil
}
```

With:
```go
func checkKeyImages(tx *types.Transaction) error {
	seen := make(map[types.KeyImage]struct{})
	for _, vin := range tx.Vin {
		var ki types.KeyImage
		switch v := vin.(type) {
		case types.TxInputToKey:
			ki = v.KeyImage
		case types.TxInputHTLC:
			ki = v.KeyImage
		default:
			continue
		}
		if _, exists := seen[ki]; exists {
			return fmt.Errorf("%w: %s", ErrDuplicateKeyImage, ki)
		}
		seen[ki] = struct{}{}
	}
	return nil
}
```

### Step 8.6 — Run tests, verify PASS

```bash
go test -race -run "TestTxFee_HTLCInput_Good|TestTxFee_MultisigInput_Good|TestCheckKeyImages_HTLC" ./consensus/...
```

**Expected:** All PASS.

### Step 8.7 — Run full consensus test suite

```bash
go test -race ./consensus/...
```

**Expected:** All PASS.

### Step 8.8 — Commit

```
feat(consensus): include HTLC/multisig in fee calculation and key image checks

sumInputs now sums TxInputHTLC.Amount and TxInputMultisig.Amount.
checkKeyImages now checks TxInputHTLC.KeyImage for double-spend.

Co-Authored-By: Charon <charon@lethean.io>
```

---

## Task 9: Consensus — verifyV1Signatures for HTLC inputs

**Package:** `consensus/`
**Why:** HTLC inputs use the same NLSAG ring signature scheme as TxInputToKey. The signature verification loop must count and verify both.

### Step 9.1 — Write structural test: mixed TxInputToKey + TxInputHTLC signature count

- [ ] Append to `/home/claude/Code/core/go-blockchain/consensus/verify_test.go`

```go
func TestVerifyV1Signatures_MixedHTLC_Good(t *testing.T) {
	// Structural check only (getRingOutputs = nil).
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: 100, KeyImage: types.KeyImage{1}},
			types.TxInputHTLC{Amount: 50, KeyImage: types.KeyImage{2}},
		},
		Signatures: [][]types.Signature{
			{{1}}, // sig for TxInputToKey
			{{2}}, // sig for TxInputHTLC
		},
	}
	err := VerifyTransactionSignatures(tx, config.MainnetForks, 20000, nil, nil)
	require.NoError(t, err)
}

func TestVerifyV1Signatures_MixedHTLC_Bad(t *testing.T) {
	// Wrong signature count.
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: 100, KeyImage: types.KeyImage{1}},
			types.TxInputHTLC{Amount: 50, KeyImage: types.KeyImage{2}},
		},
		Signatures: [][]types.Signature{
			{{1}}, // only 1 sig for 2 ring inputs
		},
	}
	err := VerifyTransactionSignatures(tx, config.MainnetForks, 20000, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature count")
}
```

### Step 9.2 — Run tests, verify FAIL

```bash
go test -race -run "TestVerifyV1Signatures_MixedHTLC" ./consensus/...
```

**Expected:** The `_Good` test fails because `verifyV1Signatures` only counts `TxInputToKey`, so signature count (2) != key input count (1).

### Step 9.3 — Update verifyV1Signatures to count HTLC inputs

- [ ] Edit `/home/claude/Code/core/go-blockchain/consensus/verify.go` — `verifyV1Signatures`

Replace the key input counting loop:
```go
	var keyInputCount int
	for _, vin := range tx.Vin {
		if _, ok := vin.(types.TxInputToKey); ok {
			keyInputCount++
		}
	}
```

With:
```go
	var keyInputCount int
	for _, vin := range tx.Vin {
		switch vin.(type) {
		case types.TxInputToKey, types.TxInputHTLC:
			keyInputCount++
		}
	}
```

Update the signature verification loop to handle both types:
```go
	var sigIdx int
	for _, vin := range tx.Vin {
		var inp types.TxInputToKey
		switch v := vin.(type) {
		case types.TxInputToKey:
			inp = v
		case types.TxInputHTLC:
			inp = types.TxInputToKey{
				Amount:     v.Amount,
				KeyOffsets: v.KeyOffsets,
				KeyImage:   v.KeyImage,
				EtcDetails: v.EtcDetails,
			}
		default:
			continue
		}

		// Extract absolute global indices from key offsets.
		offsets := make([]uint64, len(inp.KeyOffsets))
		for i, ref := range inp.KeyOffsets {
			offsets[i] = ref.GlobalIndex
		}

		ringKeys, err := getRingOutputs(inp.Amount, offsets)
		// ... rest unchanged
```

### Step 9.4 — Run tests, verify PASS

```bash
go test -race -run "TestVerifyV1Signatures_MixedHTLC" ./consensus/...
```

**Expected:** All PASS.

### Step 9.5 — Run full consensus test suite

```bash
go test -race ./consensus/...
```

**Expected:** All PASS.

### Step 9.6 — Commit

```
feat(consensus): verify NLSAG signatures for HTLC inputs

verifyV1Signatures now counts and verifies TxInputHTLC alongside
TxInputToKey. HTLC inputs use the same ring signature scheme.

Co-Authored-By: Charon <charon@lethean.io>
```

---

## Task 10: Consensus — ValidateBlock major version check for HF1

**Package:** `consensus/`
**Why:** After HF1, blocks must have MajorVersion >= 1.

### Step 10.1 — Add error sentinel

- [ ] Edit `/home/claude/Code/core/go-blockchain/consensus/errors.go`

Add to the block validation errors group:

```go
	ErrBlockMajorVersion = errors.New("consensus: invalid block major version for height")
```

### Step 10.2 — Write test: block major version 0 rejected after HF1

- [ ] Append to `/home/claude/Code/core/go-blockchain/consensus/block_test.go`

```go
func TestValidateBlock_MajorVersionPreHF1_Good(t *testing.T) {
	blk := validBlock(5000) // pre-HF1
	blk.MajorVersion = 0
	err := ValidateBlock(&blk, 5000, 100, 100, 0, uint64(time.Now().Unix()), nil, config.MainnetForks)
	require.NoError(t, err)
}

func TestValidateBlock_MajorVersionPostHF1_Bad(t *testing.T) {
	blk := validBlock(20000) // post-HF1
	blk.MajorVersion = 0    // should be >= 1
	err := ValidateBlock(&blk, 20000, 100, 100, 0, uint64(time.Now().Unix()), nil, config.MainnetForks)
	assert.ErrorIs(t, err, ErrBlockMajorVersion)
}

func TestValidateBlock_MajorVersionPostHF1_Good(t *testing.T) {
	blk := validBlock(20000)
	blk.MajorVersion = config.HF1BlockMajorVersion
	err := ValidateBlock(&blk, 20000, 100, 100, 0, uint64(time.Now().Unix()), nil, config.MainnetForks)
	require.NoError(t, err)
}
```

**Note:** The `validBlock` helper constructs a minimal valid block. If it does not exist, create it. It needs a miner tx with TxInputGenesis at the correct height and at least one output. Adapt based on the existing test helpers in block_test.go.

### Step 10.3 — Run tests, verify FAIL

```bash
go test -race -run "TestValidateBlock_MajorVersion" ./consensus/...
```

**Expected:** The `_Bad` test passes (no version check yet, so no error). The test itself should assert an error is returned — so it will fail if no error is returned.

### Step 10.4 — Implement block major version check in ValidateBlock

- [ ] Edit `/home/claude/Code/core/go-blockchain/consensus/block.go` — `ValidateBlock`

Add at the top of `ValidateBlock`, before timestamp validation:

```go
	// Block major version check.
	hf1Active := config.IsHardForkActive(forks, config.HF1, height)
	if hf1Active {
		if blk.MajorVersion < config.HF1BlockMajorVersion {
			return fmt.Errorf("%w: got %d, need >= %d at height %d",
				ErrBlockMajorVersion, blk.MajorVersion, config.HF1BlockMajorVersion, height)
		}
	} else {
		if blk.MajorVersion >= config.HF1BlockMajorVersion {
			return fmt.Errorf("%w: got %d, must be < %d before HF1 at height %d",
				ErrBlockMajorVersion, blk.MajorVersion, config.HF1BlockMajorVersion, height)
		}
	}
```

### Step 10.5 — Run tests, verify PASS

```bash
go test -race -run "TestValidateBlock_MajorVersion" ./consensus/...
```

**Expected:** All PASS.

### Step 10.6 — Run full consensus test suite

```bash
go test -race ./consensus/...
```

**Expected:** All PASS. Check that existing ValidateBlock tests still pass — they may need their `MajorVersion` field set correctly for their height.

### Step 10.7 — Commit

```
feat(consensus): validate block major version for HF1

Blocks after HF1 (height 10,080) must have MajorVersion >= 1.
Blocks before HF1 must have MajorVersion < 1.

Co-Authored-By: Charon <charon@lethean.io>
```

---

## Task 11: Wire round-trip integration tests with constructed HF1 transactions

**Package:** `wire/`
**Why:** End-to-end verification that a complete HF1 transaction with mixed types survives encode/decode.

### Step 11.1 — Write integration round-trip test

- [ ] Append to `/home/claude/Code/core/go-blockchain/wire/transaction_test.go`

```go
func TestHF1MixedTxRoundTrip_Good(t *testing.T) {
	// Construct a v1 transaction with HTLC input, multisig output, and HTLC output.
	tx := types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{
				Amount: 100000,
				KeyOffsets: []types.TxOutRef{
					{Tag: types.RefTypeGlobalIndex, GlobalIndex: 42},
				},
				KeyImage:   types.KeyImage{0x01},
				EtcDetails: EncodeVarint(0),
			},
			types.TxInputHTLC{
				HTLCOrigin: "htlc_preimage_data",
				Amount:     50000,
				KeyOffsets: []types.TxOutRef{
					{Tag: types.RefTypeGlobalIndex, GlobalIndex: 99},
				},
				KeyImage:   types.KeyImage{0x02},
				EtcDetails: EncodeVarint(0),
			},
			types.TxInputMultisig{
				Amount:        30000,
				MultisigOutID: types.Hash{0xFF},
				SigsCount:     2,
				EtcDetails:    EncodeVarint(0),
			},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 70000,
				Target: types.TxOutToKey{Key: types.PublicKey{0xAA}},
			},
			types.TxOutputBare{
				Amount: 50000,
				Target: types.TxOutMultisig{
					MinimumSigs: 2,
					Keys:        []types.PublicKey{{0xBB}, {0xCC}},
				},
			},
			types.TxOutputBare{
				Amount: 40000,
				Target: types.TxOutHTLC{
					HTLCHash:   types.Hash{0xDD},
					Flags:      0,
					Expiration: 15000,
					PKRedeem:   types.PublicKey{0xEE},
					PKRefund:   types.PublicKey{0xFF},
				},
			},
		},
		Extra: EncodeVarint(0),
	}

	// Encode.
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransactionPrefix(enc, &tx)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}
	encoded := buf.Bytes()

	// Decode.
	dec := NewDecoder(bytes.NewReader(encoded))
	got := DecodeTransactionPrefix(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	// Verify inputs.
	if len(got.Vin) != 3 {
		t.Fatalf("vin count: got %d, want 3", len(got.Vin))
	}

	if _, ok := got.Vin[0].(types.TxInputToKey); !ok {
		t.Errorf("vin[0]: got %T, want TxInputToKey", got.Vin[0])
	}

	htlcIn, ok := got.Vin[1].(types.TxInputHTLC)
	if !ok {
		t.Fatalf("vin[1]: got %T, want TxInputHTLC", got.Vin[1])
	}
	if htlcIn.HTLCOrigin != "htlc_preimage_data" {
		t.Errorf("HTLCOrigin: got %q, want %q", htlcIn.HTLCOrigin, "htlc_preimage_data")
	}
	if htlcIn.Amount != 50000 {
		t.Errorf("HTLC Amount: got %d, want 50000", htlcIn.Amount)
	}

	msigIn, ok := got.Vin[2].(types.TxInputMultisig)
	if !ok {
		t.Fatalf("vin[2]: got %T, want TxInputMultisig", got.Vin[2])
	}
	if msigIn.Amount != 30000 {
		t.Errorf("Multisig Amount: got %d, want 30000", msigIn.Amount)
	}
	if msigIn.SigsCount != 2 {
		t.Errorf("SigsCount: got %d, want 2", msigIn.SigsCount)
	}

	// Verify outputs.
	if len(got.Vout) != 3 {
		t.Fatalf("vout count: got %d, want 3", len(got.Vout))
	}

	bare0, ok := got.Vout[0].(types.TxOutputBare)
	if !ok {
		t.Fatalf("vout[0]: got %T, want TxOutputBare", got.Vout[0])
	}
	if _, ok := bare0.Target.(types.TxOutToKey); !ok {
		t.Errorf("vout[0] target: got %T, want TxOutToKey", bare0.Target)
	}

	bare1, ok := got.Vout[1].(types.TxOutputBare)
	if !ok {
		t.Fatalf("vout[1]: got %T, want TxOutputBare", got.Vout[1])
	}
	msigTgt, ok := bare1.Target.(types.TxOutMultisig)
	if !ok {
		t.Fatalf("vout[1] target: got %T, want TxOutMultisig", bare1.Target)
	}
	if msigTgt.MinimumSigs != 2 {
		t.Errorf("MinimumSigs: got %d, want 2", msigTgt.MinimumSigs)
	}
	if len(msigTgt.Keys) != 2 {
		t.Errorf("Keys count: got %d, want 2", len(msigTgt.Keys))
	}

	bare2, ok := got.Vout[2].(types.TxOutputBare)
	if !ok {
		t.Fatalf("vout[2]: got %T, want TxOutputBare", got.Vout[2])
	}
	htlcTgt, ok := bare2.Target.(types.TxOutHTLC)
	if !ok {
		t.Fatalf("vout[2] target: got %T, want TxOutHTLC", bare2.Target)
	}
	if htlcTgt.Expiration != 15000 {
		t.Errorf("Expiration: got %d, want 15000", htlcTgt.Expiration)
	}

	// Re-encode and verify bit-identical.
	var buf2 bytes.Buffer
	enc2 := NewEncoder(&buf2)
	EncodeTransactionPrefix(enc2, &got)
	if enc2.Err() != nil {
		t.Fatalf("re-encode error: %v", enc2.Err())
	}

	if !bytes.Equal(encoded, buf2.Bytes()) {
		t.Errorf("round-trip not bit-identical: encoded %d bytes, re-encoded %d bytes",
			len(encoded), len(buf2.Bytes()))
	}
}
```

### Step 11.2 — Run test, verify PASS

```bash
go test -race -run TestHF1MixedTxRoundTrip_Good ./wire/...
```

**Expected:** PASS — if all previous tasks are complete. If it fails, the error indicates which component is broken.

### Step 11.3 — Run full test suite

```bash
go vet ./...
go test -race ./types/... ./wire/... ./consensus/...
go mod tidy
```

**Expected:** All PASS, no vet warnings, no module changes.

### Step 11.4 — Commit

```
test(wire): add HF1 mixed transaction round-trip integration test

Verifies that a transaction containing TxInputToKey, TxInputHTLC,
TxInputMultisig inputs with TxOutToKey, TxOutMultisig, TxOutHTLC
output targets survives bit-identical encode/decode round-trip.

Co-Authored-By: Charon <charon@lethean.io>
```

---

## Final Verification

After all tasks are complete:

```bash
# Full build and test
go vet ./...
go test -race ./...
go mod tidy

# Verify no untracked changes
git diff --stat
```

**Expected:** All packages build. All tests pass with race detector. `go mod tidy` produces no changes. The Go node can now deserialise blocks containing HF1 transaction types and validate them according to hardfork rules.

---

## Summary

| Task | Package | What | Commits |
|------|---------|------|---------|
| 1 | types/ | TxOutTarget interface + TargetType() | 1 |
| 2 | types/, wire/, chain/, wallet/, tui/ | TxOutputBare.Target → interface | 1 |
| 3 | types/ | TxOutMultisig, TxOutHTLC | 1 |
| 4 | types/ | TxInputHTLC, TxInputMultisig | 1 |
| 5 | wire/ | Input encode/decode | 1 |
| 6 | wire/ | Output target encode/decode (V1+V2) | 1 |
| 7 | consensus/ | checkInputTypes/checkOutputs HF1 gating | 1 |
| 8 | consensus/ | sumInputs/checkKeyImages for new types | 1 |
| 9 | consensus/ | verifyV1Signatures for HTLC | 1 |
| 10 | consensus/ | ValidateBlock major version | 1 |
| 11 | wire/ | Integration round-trip test | 1 |
| **Total** | | | **11 commits** |
