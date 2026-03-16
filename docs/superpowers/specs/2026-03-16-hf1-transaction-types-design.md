# HF1/HF2 Transaction Type Support

**Date:** 2026-03-16
**Author:** Charon
**Package:** `forge.lthn.ai/core/go-blockchain`
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

## C++ Reference (currency_basic.h)

### txin_htlc (tag 0x22)

Inherits from `txin_to_key`. Wire order: `hltc_origin` (string) serialised BEFORE parent fields.

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
type TxInputHTLC struct {
    HLTCOrigin   string      // serialised BEFORE parent fields
    Amount       uint64
    KeyOffsets   []TxOutRef
    KeyImage     KeyImage
    EtcDetails   []byte      // opaque variant vector
}

func (t TxInputHTLC) InputType() uint8 { return InputTypeHTLC }
```

```go
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
```

Existing `TxOutToKey` gets a `TargetType()` method. New types:

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
    Target TxOutTarget  // was TxOutToKey
}
```

### wire/transaction.go

#### Input decoding (decodeInputs)

Add cases:

```
case InputTypeHTLC:
    read string (hltc_origin)
    read amount (varint), key_offsets, key_image (32 bytes), etc_details (opaque)

case InputTypeMultisig:
    read amount (varint), multisig_out_id (32 bytes), sigs_count (varint), etc_details (opaque)
```

#### Input encoding (encodeInputs)

Mirror decoding for both types.

#### Output target decoding (decodeOutputsV1)

Add cases to target switch:

```
case TargetTypeMultisig:
    read minimum_sigs (varint), keys (varint count + 32*N bytes)

case TargetTypeHTLC:
    read htlc_hash (32 bytes), flags (uint8), expiration (varint),
    pkey_redeem (32 bytes), pkey_refund (32 bytes)
```

#### Output target encoding (encodeOutputsV1)

Match on `TxOutTarget` interface type, encode accordingly.

### consensus/

#### tx.go — checkInputTypes

Accept `TxInputHTLC` and `TxInputMultisig` when `IsHardForkActive(forks, HF1, height)`. Reject pre-HF1.

#### tx.go — checkOutputs

Accept `TxOutMultisig` and `TxOutHTLC` targets when HF1 active. Reject pre-HF1.

#### block.go — ValidateMinerTx

After HF1, validate block major version >= `HF1BlockMajorVersion` (1).

#### block.go — ValidateBlockReward

Update to handle `TxOutMultisig` and `TxOutHTLC` targets when summing outputs (multisig outputs have amounts, HTLC outputs have amounts via the parent tx_out_bare.amount field).

### Breaking change

`TxOutputBare.Target` changes from `TxOutToKey` to `TxOutTarget` interface. All direct field access (`out.Target.Key`) must become type assertions:

```go
if tok, ok := out.Target.(TxOutToKey); ok {
    // use tok.Key, tok.MixAttr
}
```

Affected: `consensus/block.go`, `consensus/verify.go`, `wire/transaction.go`, test files.

## Testing

- Wire round-trip tests: construct HTLC/multisig inputs and outputs, encode, decode, verify equality
- Testnet block parsing: testnet has HF1 at height 0, so all blocks may contain these types
- Consensus gate tests: verify HTLC/multisig rejected pre-HF1, accepted post-HF1
- Breaking change verification: all existing tests must pass after Target interface refactor
- Integration test: sync Go node past HF1 on testnet

## Out of scope

- HTLC redemption/refund logic (wallet layer, not consensus)
- Multisig signing coordination (wallet layer)
- HF3-HF6 changes (separate designs)
- Service attachment parsing (stays opaque)
