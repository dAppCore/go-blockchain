# V2+ Transaction Serialisation — Implementation Plan

Design: `docs/plans/2026-02-21-v2-tx-serialisation-design.md`

## Step 1: Fix types

**File:** `types/transaction.go`

- Add `TxInputZC` struct with `KeyOffsets []TxOutRef`, `KeyImage KeyImage`, `EtcDetails []byte`
- Add `InputType()` method returning `InputTypeZC`
- Add `SignaturesRaw []byte` field to `Transaction` struct
- Update doc comment on Transaction to mention SignaturesRaw for v2+

**Tests:** Compile-only — new types have no logic to test independently.

## Step 2: Fix v2 suffix

**File:** `wire/transaction.go`

Fix `encodeSuffixV2`:
```go
func encodeSuffixV2(enc *Encoder, tx *types.Transaction) {
    enc.WriteBytes(tx.Attachment)
    enc.WriteBytes(tx.SignaturesRaw)
    enc.WriteBytes(tx.Proofs)
}
```

Fix `decodeSuffixV2`:
```go
func decodeSuffixV2(dec *Decoder, tx *types.Transaction) {
    tx.Attachment = decodeRawVariantVector(dec)
    tx.SignaturesRaw = decodeRawVariantVector(dec)
    tx.Proofs = decodeRawVariantVector(dec)
}
```

## Step 3: Add InputTypeZC handling

**File:** `wire/transaction.go`

In `encodeInputs`, add case for `types.TxInputZC`:
```go
case types.TxInputZC:
    encodeKeyOffsets(enc, v.KeyOffsets)
    enc.WriteBlob32((*[32]byte)(&v.KeyImage))
    enc.WriteBytes(v.EtcDetails)
```

In `decodeInputs`, add case for `types.InputTypeZC`:
```go
case types.InputTypeZC:
    var in types.TxInputZC
    in.KeyOffsets = decodeKeyOffsets(dec)
    dec.ReadBlob32((*[32]byte)(&in.KeyImage))
    in.EtcDetails = decodeRawVariantVector(dec)
    vin = append(vin, in)
```

## Step 4: Fix tagSignedParts handler

**File:** `wire/transaction.go`

Change `tagSignedParts` from reading 4 fixed bytes to reading two varints:
```go
case tagSignedParts:
    return readSignedParts(dec)
```

New function:
```go
func readSignedParts(dec *Decoder) []byte {
    v1 := dec.ReadVarint() // n_outs
    if dec.err != nil { return nil }
    raw := EncodeVarint(v1)
    v2 := dec.ReadVarint() // n_extras
    if dec.err != nil { return nil }
    raw = append(raw, EncodeVarint(v2)...)
    return raw
}
```

## Step 5: Add tagZarcanumTxDataV1 handler

**File:** `wire/transaction.go`

Add case in `readVariantElementData`:
```go
case tagZarcanumTxDataV1:
    v := dec.ReadVarint() // fee
    if dec.err != nil { return nil }
    return EncodeVarint(v)
```

## Step 6: Add proof tag handlers (46, 47, 48)

**File:** `wire/transaction.go`

New constants:
```go
const (
    tagZCAssetSurjectionProof = 46
    tagZCOutsRangeProof       = 47
    tagZCBalanceProof          = 48
)
```

New reader functions:
- `readBGEProof(dec)` — A(32)+B(32)+vec(32)+vec(32)+y(32)+z(32)
- `readBPPSerialized(dec)` — vec(32)+vec(32)+192 bytes
- `readAggregationProof(dec)` — vec(32)+vec(32)+vec(32)+32 bytes

Tag handlers:
- 46: varint(count) + count * readBGEProof
- 47: readBPPSerialized + readAggregationProof
- 48: 96 fixed bytes

## Step 7: Add signature tag handlers (42, 43, 44, 45)

**File:** `wire/transaction.go`

New constants:
```go
const (
    tagNLSAGSig    = 42
    tagZCSig       = 43
    tagVoidSig     = 44
    tagZarcanumSig = 45
)
```

New reader functions:
- `readCLSAG_GGX(dec)` — 32+vec(32)+vec(32)+64 bytes
- `readCLSAG_GGXXG(dec)` — 32+vec(32)+vec(32)+128 bytes
- `readBPPESerialized(dec)` — vec(32)+vec(32)+224 bytes

Tag handlers:
- 42: readVariantVectorFixed(dec, 64)
- 43: 64 bytes + readCLSAG_GGX
- 44: 0 bytes (empty)
- 45: 320 bytes + readBPPESerialized + 32 bytes + readCLSAG_GGXXG

## Step 8: Test with real testnet data

**New file:** `wire/transaction_v2_test.go`

1. `TestV2CoinbaseRoundTrip` — fetch block 101 coinbase blob from testnet
   (or embed the hex literal), decode to Transaction, re-encode, assert
   byte-for-byte identity.

2. `TestV2CoinbaseTxHash` — decode prefix, hash with Keccak-256, compare
   to known tx hash `543bc3c29e9f4c5d1fc566be03fb4da1f2ce2d70d4312fdcc3e4eed7ca3b61e0`.

3. `TestV2SuffixFieldCount` — decode a v2 coinbase, verify that
   `Attachment` is a zero-count vector, `SignaturesRaw` is a zero-count
   vector, and `Proofs` is a 3-element vector.

## Step 9: Verify

```bash
go test -race ./...
go vet ./...
```

All tests pass, no race conditions, no vet warnings.

## Step 10: Update docs

Update `docs/history.md` with v2+ serialisation completion.
