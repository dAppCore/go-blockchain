# HF5 Confidential Assets Support

**Date:** 2026-03-16
**Author:** Charon
**Package:** `dappco.re/go/core/blockchain`
**Status:** Draft
**Depends on:** HF1 (types refactor), HF3 (block version), HF4 (Zarcanum — already implemented)

## Context

HF5 introduces confidential assets — the ability to deploy, emit, update, and burn custom asset types on the Lethean chain. This is the Zano asset system: every output has a `blinded_asset_id` that proves (via BGE surjection proofs) it corresponds to a legitimate input asset without revealing which one.

On mainnet, HF5 is at height 999,999,999 (future). On testnet, HF5 activates at height 200.

**What's already implemented:**
- BGE surjection proof verification (`crypto.VerifyBGE`) — crypto bridge done
- BGE proof parsing (`readBGEProof`, `readZCAssetSurjectionProof`) — wire done
- `verifyBGEProofs` in consensus/verify.go — verification logic done
- Transaction version 3 wire format with `hardfork_id` field — wire done
- `VersionPostHF5` constant and `decodePrefixV2` hardfork_id handling — done

**What's NOT implemented:**
- Asset operation types in extra/attachment fields
- Asset descriptor structures
- Consensus validation for asset operations
- Pre-hardfork transaction freeze (60 blocks before HF5 activation)
- Minimum build version enforcement

## Scope

### Phase A: Asset descriptor types (types/)

New types for the `asset_descriptor_operation` extra variant:

```go
// AssetDescriptorBase holds the core asset metadata.
type AssetDescriptorBase struct {
    Ticker       string    // max 6 chars
    FullName     string    // max 64 chars
    TotalMaxSupply uint64  // maximum supply cap
    CurrentSupply  uint64  // current circulating supply
    DecimalPoint   uint8   // display precision
    MetaInfo     string    // arbitrary metadata (JSON)
    OwnerKey     PublicKey // asset owner's public key
    // etc: reserved variant vector for future fields
    Etc          []byte   // opaque
}

// AssetDescriptorOperation represents a deploy/emit/update/burn operation.
type AssetDescriptorOperation struct {
    Version       uint8   // currently 0 or 1
    OperationType uint8   // ASSET_DESCRIPTOR_OPERATION_REGISTER, _EMIT, _UPDATE, _BURN, _PUBLIC_BURN
    Descriptor    *AssetDescriptorBase // present for register and update
    AssetID       Hash    // target asset ID (absent for register)
    AmountToEmit  uint64  // for emit operations
    AmountToBurn  uint64  // for burn operations
    Etc           []byte  // opaque
}
```

Operation type constants:
```go
const (
    AssetOpRegister   uint8 = 0 // deploy new asset
    AssetOpEmit       uint8 = 1 // emit additional supply
    AssetOpUpdate     uint8 = 2 // update metadata
    AssetOpBurn       uint8 = 3 // burn supply (with proof)
    AssetOpPublicBurn uint8 = 4 // burn supply (public amount)
)
```

### Phase B: Wire encoding for asset operations (wire/)

The `asset_descriptor_operation` appears as a variant element in the tx extra field (tag 40 in the C++ SET_VARIANT_TAGS).

Add to `readVariantElementData`:
```
case tagAssetDescriptorOperation (40):
    read version transition header
    read operation_type (uint8)
    read opt_asset_id (optional hash)
    read opt_descriptor (optional AssetDescriptorBase)
    read amount_to_emit/burn (varint)
    read etc (opaque vector)
```

This is stored as raw bytes in the extra field (same opaque pattern as everything else), but we need the wire reader to not choke on tag 40 during deserialization.

### Phase C: Asset operation proof types (wire/)

New proof variant tags for HF5:

```
tagAssetOperationProof           = 49  // asset_operation_proof
tagAssetOperationOwnershipProof  = 50  // asset_operation_ownership_proof
tagAssetOperationOwnershipETH    = 51  // asset_operation_ownership_proof_eth
```

Each needs a reader in `readVariantElementData`. The proof structures contain crypto elements (Schnorr signatures, public keys) that are fixed-size.

### Phase D: Consensus validation (consensus/)

**Transaction version enforcement:**
- After HF5: transaction version must be 3 (not 2)
- `hardfork_id` field must be present and match current hardfork

**Pre-hardfork freeze:**
- 60 blocks before HF5 activation, reject non-coinbase transactions
- `config.PreHardforkTxFreezePeriod = 60` already defined

**Asset operation validation:**
- Register: descriptor must be valid (ticker length, supply caps, owner key non-zero)
- Emit: asset_id must exist, caller must prove ownership
- Update: asset_id must exist, caller must prove ownership
- Burn: amount must not exceed current supply

**Minimum build version:**
- C++ enforces `MINIMUM_REQUIRED_BUILD_VERSION = 601` for mainnet, 2 for testnet
- Go equivalent: reject connections from peers with build version below threshold

### Phase E: Asset state tracking (chain/)

Need to track:
- Asset registry: asset_id → AssetDescriptorBase
- Current supply per asset
- Asset ownership proofs

This requires new storage groups in `chain/store.go`.

## What can be deferred

- **Full asset operation validation** — complex, needs ownership proof verification. Can accept blocks containing asset operations structurally (wire parsing) without deep validation initially, then add validation incrementally.
- **Asset state tracking** — needed for wallet/explorer, not strictly for block sync if we trust the C++ daemon's validation.
- **Wallet asset support** — separate design.

## Recommended approach

**Minimum viable HF5:** Wire parsing only. Add tag 40 and the asset proof tags to `readVariantElementData` so the Go node can deserialise HF5 blocks without crashing. Store asset operations as opaque bytes in the extra field (existing pattern). Gate transaction version 3 on HF5.

This follows the same pattern used for extra, attachment, etc_details — opaque bytes for bit-identical round-tripping. Deep validation can layer on top.

## Testing

- Wire round-trip tests with constructed v3 transactions containing asset operations
- Testnet block parsing past height 200 (HF5 activation)
- Version enforcement tests (reject v2 after HF5, accept v3)
- Pre-hardfork freeze tests (reject non-coinbase 60 blocks before activation)

## Out of scope

- Wallet asset management (deploy/emit/burn CLI)
- Asset explorer UI
- Asset whitelist management
- Cross-asset atomic swaps
- HF6 block time halving (separate spec)
