# V2+ Transaction Serialisation Design

## Context

Phase 1 implemented wire-format serialisation for v0/v1 (pre-HF4) transactions.
The v2+ code paths (`encodePrefixV2`, `decodePrefixV2`, `encodeSuffixV2`,
`decodeSuffixV2`, `encodeOutputsV2`, `decodeOutputsV2`) were written
speculatively but **never tested against real chain data**.

The testnet has HF4 active at height 100 (currently ~511). Every coinbase from
block 101 onwards is a version-2 transaction with Zarcanum outputs and proofs.

## Problem

Testing against real testnet data reveals three bugs and several missing handlers:

### Bugs

1. **V2 suffix order wrong**: Code writes `attachment + proofs`, but C++ wire
   format is `attachment + signatures + proofs`. The signatures variant vector
   is completely absent from `encodeSuffixV2`/`decodeSuffixV2`.

2. **`tagSignedParts` (17) reads 4 fixed bytes**: The C++ `signed_parts` struct
   uses `VARINT_FIELD` for both `n_outs` and `n_extras` — two varints, not a
   uint32 LE. This corrupts the stream when parsing real spending transactions.

3. **`tagZarcanumTxDataV1` (39) declared but unhandled**: The constant exists
   but `readVariantElementData` has no case for it. V2 transactions carry the
   fee in this extra variant. Wire format: single varint (fee).

### Missing Input Type

`InputTypeZC` (tag 0x25 / 37) — `txin_zc_input` — is the standard input for
Zarcanum spending transactions. Wire format:

```
key_offsets  — variant vector (txout_ref_v elements)
k_image      — 32 bytes
etc_details  — variant vector (txin_etc_details_v elements)
```

Note: unlike `txin_to_key`, there is **no amount field**.

### Missing Variant Tag Handlers

The `readVariantElementData` function needs handlers for proof and signature
tags so `decodeRawVariantVector` can determine element boundaries:

**Proof tags:**
- 46 (`zc_asset_surjection_proof`): vector of BGE_proof_s
- 47 (`zc_outs_range_proof`): bpp_serialized + aggregation_proof_serialized
- 48 (`zc_balance_proof`): generic_double_schnorr_sig_s (96 bytes fixed)

**Signature tags:**
- 42 (`NLSAG_sig`): vector of 64-byte signatures
- 43 (`ZC_sig`): 2 public_keys (64 bytes) + CLSAG_GGX_serialized
- 44 (`void_sig`): 0 bytes (empty struct)
- 45 (`zarcanum_sig`): 10 scalars (320 bytes) + bppe_serialized + public_key (32) + CLSAG_GGXXG_serialized

## Crypto Blob Wire Layouts

All crypto serialised structs use vectors of 32-byte scalars/points with varint
length prefixes, plus fixed-size blobs:

```
BGE_proof_s:             A(32) + B(32) + vec(Pk) + vec(f) + y(32) + z(32)
bpp_signature:           vec(L) + vec(R) + A0(32)+A(32)+B(32)+r(32)+s(32)+delta(32)
bppe_signature:          vec(L) + vec(R) + A0(32)+A(32)+B(32)+r(32)+s(32)+d1(32)+d2(32)
aggregation_proof:       vec(commitments) + vec(y0s) + vec(y1s) + c(32)
double_schnorr_sig:      c(32) + y0(32) + y1(32)
CLSAG_GGX:              c(32) + vec(r_g) + vec(r_x) + K1(32) + K2(32)
CLSAG_GGXXG:            c(32) + vec(r_g) + vec(r_x) + K1(32)+K2(32)+K3(32)+K4(32)
```

Where `vec(X)` = `varint(count) + 32*count bytes`.

## Approach

Full variant-level parsing — add handlers for every v2+ variant tag in
`readVariantElementData`, following the existing pattern. Each handler reads
field-by-field based on the C++ serialisation order and returns raw bytes.

The alternative (reading suffix as a raw blob) doesn't work because we need to
separate the three vectors (attachment, signatures, proofs) into distinct fields.

## Type Changes

Add `TxInputZC` struct to `types/transaction.go`:
```go
type TxInputZC struct {
    KeyOffsets []TxOutRef
    KeyImage   KeyImage
    EtcDetails []byte
}
```

Add `SignaturesRaw []byte` field to `Transaction` for v2+ raw signatures.
V0/v1 uses the structured `Signatures [][]Signature` field; v2+ uses
`SignaturesRaw` (raw variant vector bytes).

## File Changes

| File | Action |
|------|--------|
| `types/transaction.go` | Add `TxInputZC`, `SignaturesRaw` field |
| `wire/transaction.go` | Fix suffix, add InputTypeZC, add tag handlers, fix signed_parts |
| `wire/transaction_v2_test.go` | New — v2 round-trip tests with real testnet data |

## Verification

1. Fetch block 101 coinbase blob from testnet via RPC
2. Decode blob to Transaction struct
3. Re-encode Transaction to bytes
4. Assert byte-for-byte identity with original blob
5. Hash prefix with Keccak-256, compare to known tx hash
6. `go test -race ./...` — all tests pass
