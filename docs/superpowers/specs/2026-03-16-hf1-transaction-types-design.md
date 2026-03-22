# HF1/HF2 Transaction Type Support

**Date:** 2026-03-16
**Author:** Charon
**Package:** `dappco.re/go/core/blockchain`
**Status:** Approved

## Context

Mainnet hardfork 1 activates at block 10,080. The Go node currently only handles genesis, to_key, and ZC input types, and bare (to_key target) and Zarcanum output types. After HF1, blocks may contain HTLC and multisig transactions. The miner tx major version also changes from 0 to 1. Without this work, the Go node will fail to deserialise blocks past HF1.

HF2 activates at the same height (10,080) and adjusts block time parameters. This is handled by the difficulty package via config constants — no new types needed.

## Scope

- Add `TxInputHTLC` and `TxInputMultisig` input types to `types/` and `wire/`
- Add `TxOutMultisig` and `TxOutHTLC` output target types to `types/` and `wire/`
- Refactor `TxOutputBare.Target` from concrete `TxOutToKey` to `TxOutTarget` interface
- Update `consensus/` validation to gate HTLC/multisig on HF1
- Update block major version validation for HF1
- Update all call sites that access `TxOutputBare.Target` fields directly

## C++ Reference (currency_basic.h)

### txin_htlc (tag 0x22)

Inherits from `txin_to_key`. Wire order: `hltc_origin` (string) serialised BEFORE parent fields.

**Note:** The C++ field is named `hltc_origin` (transposed letters). The Go field uses `HTLCOrigin` (corrected acronym) since the type is already `TxInputHTLC`.

```
FIELD(hltc_origin)        // varint length + bytes
FIELDS(*static_cast<txin_to_key*>(this))  // amount, key_offsets, k_image, etc_details
```

### txin_multisig (tag 0x02)

```
VARINT_FIELD(amount)
FIELD(multisig_out_id)    // 32-byte hash
VARINT_FIELD(sigs_count)
FIELD(etc_details)        // variant vector (opaque)
```

### txout_multisig (target tag 0x04)

```
VARINT_FIELD(minimum_sigs)
FIELD(keys)               // vector of 32-byte public keys
```

### txout_htlc (target tag 0x23)

```
FIELD(htlc_hash)          // 32-byte hash
FIELD(flags)              // uint8 (bit 0: 0=SHA256, 1=RIPEMD160)
VARINT_FIELD(expiration)  // block height
FIELD(pkey_redeem)        // 32-byte public key
FIELD(pkey_refund)        // 32-byte public key
```

## Design

### types/transaction.go

#### New input types

```go
// TxInputHTLC extends TxInputToKey with an HTLC origin hash.
// Wire order: HTLCOrigin (string) serialised BEFORE parent fields (C++ quirk).
// Carries Amount, KeyOffsets, KeyImage, EtcDetails — same as TxInputToKey.
type TxInputHTLC struct {
    HTLCOrigin string       // C++ field: hltc_origin (transposed in source)
    Amount     uint64
    KeyOffsets []TxOutRef
    KeyImage   KeyImage
    EtcDetails []byte       // opaque variant vector
}

func (t TxInputHTLC) InputType() uint8 { return InputTypeHTLC }
```

```go
// TxInputMultisig spends from a multisig output.
type TxInputMultisig struct {
    Amount        uint64
    MultisigOutID Hash
    SigsCount     uint64
    EtcDetails    []byte     // opaque variant vector
}

func (t TxInputMultisig) InputType() uint8 { return InputTypeMultisig }
```

#### Output target interface

Replace concrete `TxOutToKey` target with interface:

```go
type TxOutTarget interface {
    TargetType() uint8
}

func (t TxOutToKey) TargetType() uint8 { return TargetTypeToKey }
```

New types:

```go
type TxOutMultisig struct {
    MinimumSigs uint64
    Keys        []PublicKey
}

func (t TxOutMultisig) TargetType() uint8 { return TargetTypeMultisig }
```

```go
type TxOutHTLC struct {
    HTLCHash   Hash
    Flags      uint8
    Expiration uint64
    PKRedeem   PublicKey
    PKRefund   PublicKey
}

func (t TxOutHTLC) TargetType() uint8 { return TargetTypeHTLC }
```

#### TxOutputBare change

```go
type TxOutputBare struct {
    Amount uint64
    Target TxOutTarget  // was TxOutToKey, now interface
}
```

### wire/transaction.go

#### Input decoding (decodeInputs)

Add cases:

```
case InputTypeHTLC (0x22):
    read hltc_origin as string (varint length + bytes)
    read amount (varint), key_offsets, key_image (32 bytes), etc_details (opaque)

case InputTypeMultisig (0x02):
    read amount (varint), multisig_out_id (32 bytes), sigs_count (varint), etc_details (opaque)
```

#### Input encoding (encodeInputs)

Add matching cases for `TxInputHTLC` and `TxInputMultisig`.

#### Output target decoding — BOTH decodeOutputsV1 AND decodeOutputsV2

Add target cases to both v1 and v2 output decoders:

```
case TargetTypeMultisig (0x04):
    read minimum_sigs (varint), keys (varint count + 32*N bytes)

case TargetTypeHTLC (0x23):
    read htlc_hash (32 bytes), flags (uint8), expiration (varint),
    pkey_redeem (32 bytes), pkey_refund (32 bytes)
```

The v2 decoder (`decodeOutputsV2`) also handles `OutputTypeBare` with an inner target tag, so it needs the same target switch updates.

#### Output target encoding — BOTH encodeOutputsV1 AND encodeOutputsV2

Match on `TxOutTarget` interface type, encode accordingly. Both v1 and v2 encoders must handle all three target types.

### consensus/

#### tx.go — Function signature changes

`checkInputTypes` currently receives `hf4Active bool`. Change to receive `forks []config.HardFork` and `height uint64` (or pre-computed `hf1Active` and `hf4Active` bools from the parent `ValidateTransaction`). Same for `checkOutputs`.

#### tx.go — checkInputTypes

Accept `TxInputHTLC` and `TxInputMultisig` when `IsHardForkActive(forks, HF1, height)`. Reject pre-HF1.

#### tx.go — checkOutputs

Accept `TxOutMultisig` and `TxOutHTLC` targets when HF1 active. Reject pre-HF1. Must type-assert `TxOutputBare.Target` to check target types.

#### tx.go — checkKeyImages

Add `TxInputHTLC` to the key image uniqueness check. HTLC inputs carry a `KeyImage` field that must be checked for double-spend prevention, same as `TxInputToKey`.

#### fee.go — sumInputs

Add `TxInputHTLC` and `TxInputMultisig` to the input sum. Both carry `Amount` fields needed for fee calculation and overflow checks. Without this, transactions with HTLC/multisig inputs would appear to have zero input value.

#### block.go — ValidateBlock

Add block major version check: after HF1 height, `blk.MajorVersion` must be >= `HF1BlockMajorVersion` (1). Before HF1, must be 0. This goes in `ValidateBlock` (the block-level entry point), not in `ValidateMinerTx`.

#### block.go — ValidateBlockReward

Update output sum to handle all `TxOutTarget` types via type assertion. The `Amount` field is on `TxOutputBare` (the outer struct), so the sum logic doesn't change for different targets — but the type assertion for accessing the output is needed after the interface refactor.

#### verify.go — verifyV1Signatures

Count `TxInputHTLC` inputs alongside `TxInputToKey` when matching signatures. HTLC inputs use the same NLSAG ring signature scheme. The signature verification loop must handle both types.

### Breaking change: TxOutTarget interface

`TxOutputBare.Target` changes from `TxOutToKey` to `TxOutTarget` interface. All direct field access (`out.Target.Key`) must become type assertions.

**Complete list of affected call sites:**

| File | Line | Current access | Fix |
|------|------|---------------|-----|
| `consensus/block.go` | ValidateBlockReward output sum | `bare.Target` | Type assert to `TxOutToKey` |
| `consensus/verify.go` | Ring output key extraction | `out.Target.Key` | Type assert |
| `wire/transaction.go` | encodeOutputsV1, encodeOutputsV2 | `v.Target.Key`, `v.Target.MixAttr` | Switch on `TargetType()` |
| `chain/ring.go:38` | `out.Target.Key` | Ring output lookup | Type assert |
| `chain/sync.go:280` | Type switch on `TxOutputBare` | Output processing | Type assert target |
| `wallet/scanner.go:67` | `bare.Target.Key` | Output scanning | Type assert |
| `wallet/builder.go:217` | `Target: types.TxOutToKey{...}` | Output construction | No change (constructs TxOutToKey, which satisfies TxOutTarget) |
| `tui/explorer_model.go:328` | `v.Target.Key[:4]` | Display | Type assert |
| All `*_test.go` files | Various | Test assertions | Type assert where accessing Target fields |

### chain/ring.go — GetRingOutputs

Must handle the case where a ring references an output with a multisig or HTLC target. For `TxOutToKey` targets, return the key as before. For `TxOutMultisig`, the relevant key depends on the spending context (not needed for basic sync). For `TxOutHTLC`, return either `PKRedeem` or `PKRefund` depending on whether the HTLC has expired.

## Testing

- Wire round-trip tests: construct HTLC/multisig inputs and outputs, encode, decode, verify equality
- Testnet block parsing: testnet has HF1 at height 0, so all blocks may contain these types
- Consensus gate tests: verify HTLC/multisig rejected pre-HF1, accepted post-HF1
- Key image uniqueness tests: verify HTLC inputs checked for double-spend
- Fee calculation tests: verify sumInputs includes HTLC and multisig amounts
- Signature verification tests: verify verifyV1Signatures handles mixed TxInputToKey + TxInputHTLC
- Breaking change verification: all existing tests must pass after Target interface refactor
- Integration test: sync Go node past HF1 on testnet

## Out of scope

- HTLC redemption/refund logic (wallet layer, not consensus)
- Multisig signing coordination (wallet layer)
- HF3-HF6 changes (separate designs)
- Service attachment parsing (stays opaque)
