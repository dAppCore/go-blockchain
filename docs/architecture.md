---
title: Architecture
description: Package structure, dependency graph, CGo boundary, and core data structures.
---

# Architecture

## Dependency Graph

```
                    +------------+
                    |  consensus |
                    +------+-----+
                           |
              +------------+------------+
              |                         |
        +-----+-----+            +-----+-----+
        |   chain    |            | difficulty |
        +-----+------+           +-----------+
              |
     +--------+---------+
     |        |         |
+----+--+ +---+---+ +---+---+
|  p2p  | |  rpc  | | wallet|
+----+--+ +---+---+ +---+---+
     |        |         |
     +--------+---------+
              |
         +----+----+
         |  wire   |
         +----+----+
              |
      +-------+-------+
      |               |
  +---+---+     +-----+-----+
  | types |     |   config   |
  +---+---+     +-----------+
      |
  +---+---+
  | crypto |  <-- CGo boundary
  +--------+
```

### Key Relationships

- **config** and **types** are leaf packages with no internal dependencies (stdlib only).
- **wire** depends on **types** for struct definitions and **config** for version-specific serialisation rules.
- **crypto** wraps the C++ library via CGo; it is used by **wire** (for hashing), **chain** (for signature verification), and **wallet** (for key derivation and tx signing).
- **p2p**, **rpc**, and **wallet** are higher-level packages that depend on wire-level serialisation.
- **chain** is the central coordinator: it validates blocks using **consensus** rules, adjusts **difficulty**, and stores state.
- **consensus** is standalone -- no dependency on **chain** or any storage layer. All functions are pure: they take types, config, and height, returning errors.

## Package Details

### config/

Every consensus-critical constant, derived from the C++ `currency_config.h.in` and `default.cmake`. The `ChainConfig` struct aggregates all parameters:

```go
// Pre-populated globals for each network.
var Mainnet = config.ChainConfig{
    Name:         "Lethean",
    Abbreviation: "LTHN",
    IsTestnet:    false,
    Coin:         1_000_000_000_000,  // 10^12 atomic units
    BlockReward:  1_000_000_000_000,  // 1 LTHN per block
    P2PPort:      36942,
    RPCPort:      36941,
    // ... all other parameters
}
```

The hardfork schedule is defined separately with lookup functions:

```go
version := config.VersionAtHeight(config.MainnetForks, height)
active := config.IsHardForkActive(config.MainnetForks, config.HF4Zarcanum, height)
```

### types/

Fixed-size byte arrays matching the CryptoNote specification:

```go
type Hash      [32]byte   // Keccak-256 hash
type PublicKey [32]byte   // Ed25519 public key
type SecretKey [32]byte   // Ed25519 secret key
type KeyImage  [32]byte   // Double-spend detection
type Signature [64]byte   // Cryptographic signature
```

All types provide `String()` (hex encoding), `IsZero()`, and `FromHex()` methods.

### Block Structure

```go
type BlockHeader struct {
    MajorVersion uint8    // Consensus rules version (0, 1, 2, 3)
    Nonce        uint64   // PoW nonce (8 bytes LE on wire)
    PrevID       Hash     // Previous block hash (32 bytes)
    MinorVersion uint64   // Soft-fork signalling (varint on wire)
    Timestamp    uint64   // Unix epoch seconds (varint on wire)
    Flags        uint8    // Bit 0: PoS flag (0=PoW, 1=PoS)
}

type Block struct {
    BlockHeader
    MinerTx  Transaction  // Coinbase transaction
    TxHashes []Hash       // Hashes of included transactions
}
```

The wire serialisation order differs from the struct field order. Canonical format: `major_version`, `nonce`, `prev_id`, `minor_version`, `timestamp`, `flags`.

### Transaction Structure

```go
type Transaction struct {
    Version       uint64        // 0=genesis, 1=pre-HF4, 2=post-HF4, 3=post-HF5
    Vin           []TxInput     // Inputs (variant type)
    Vout          []TxOutput    // Outputs (variant type)
    Extra         []byte        // Raw wire bytes for bit-identical round-tripping
    HardforkID    uint8         // v3+ only
    Signatures    [][]Signature // v0/v1 ring signatures
    SignaturesRaw []byte        // v2+ raw signature bytes (CLSAG, etc.)
    Attachment    []byte        // Service attachments
    Proofs        []byte        // v2+ proofs (BP+, balance, surjection)
}
```

Wire format differs between versions:
- **v0/v1:** `version, vin, vout, extra, [signatures, attachment]`
- **v2+:** `version, vin, extra, vout, [hardfork_id], [attachment, signatures, proofs]`

### Input Types

| Type | Tag | Description |
|------|-----|-------------|
| `TxInputGenesis` | `0x00` | Coinbase input (block height only) |
| `TxInputToKey` | `0x01` | Standard spend with ring signature |
| `TxInputZC` | `0x25` | Zarcanum confidential input (no amount field) |

### Output Types

| Type | Tag | Description |
|------|-----|-------------|
| `TxOutputBare` | `0x24` | Transparent output (visible amount) |
| `TxOutputZarcanum` | `0x26` | Confidential output (Pedersen commitment) |

A `TxOutputZarcanum` contains:

```go
type TxOutputZarcanum struct {
    StealthAddress   PublicKey  // One-time stealth address
    ConcealingPoint  PublicKey  // Group element Q (premultiplied by 1/8)
    AmountCommitment PublicKey  // Pedersen commitment (premultiplied by 1/8)
    BlindedAssetID   PublicKey  // Asset type blinding (premultiplied by 1/8)
    EncryptedAmount  uint64     // XOR-encrypted amount
    MixAttr          uint8      // Mixing attribute
}
```

### Address Encoding

Four address types via distinct base58 prefixes:

| Type | Prefix | Starts with | Auditable | Integrated |
|------|--------|-------------|-----------|------------|
| Standard | `0x1eaf7` | `iTHN` | No | No |
| Integrated | `0xdeaf7` | `iTHn` | No | Yes |
| Auditable | `0x3ceff7` | `iThN` | Yes | No |
| Auditable integrated | `0x8b077` | `iThn` | Yes | Yes |

Encoding format:
```
base58(varint(prefix) || spend_pubkey(32) || view_pubkey(32) || flags(1) || keccak256_checksum(4))
```

CryptoNote base58 splits input into 8-byte blocks, each encoded independently into 11 characters. Uses legacy Keccak-256 (pre-NIST), not SHA3-256.

## wire/

Consensus-critical binary serialisation. Key primitives:

- **Varint:** 7-bit LEB128 with MSB continuation (same as protobuf). Max 10 bytes per uint64.
- **Block hash:** `Keccak256(varint(len) || block_hashing_blob)` -- the length prefix comes from the C++ `binary_archive` serialisation of a `blobdata` type.
- **Tree hash:** CryptoNote Merkle tree over transaction hashes (direct port of `crypto/tree-hash.c`).

Extra, attachment, and proof fields are stored as opaque raw wire bytes. This enables bit-identical round-tripping without implementing all 20+ extra variant types.

## consensus/

Three-layer validation, all hardfork-aware:

**Layer 1 -- Structural (no crypto):**
Transaction size, input/output counts, key image uniqueness, extra parsing, version checks.

**Layer 2 -- Economic:**
Block reward (fixed 1 LTHN with size penalty using 128-bit arithmetic), fee extraction, balance checks. Pre-HF4 fees go to miner; post-HF4 fees are burned.

```go
func BaseReward(height uint64) uint64 {
    if height == 0 {
        return config.Premine  // Genesis block: 10M LTHN
    }
    return config.BlockReward  // All other blocks: 1 LTHN
}
```

**Layer 3 -- Cryptographic (CGo):**
PoW hash verification (RandomX, key `"LetheanRandomXv1"`), NLSAG ring signatures (pre-HF4), CLSAG signatures (post-HF4), Bulletproofs+ range proofs.

## chain/

Persistent blockchain storage using go-store (pure-Go SQLite). Five storage groups:

| Group | Key | Value |
|-------|-----|-------|
| `blocks` | Height (zero-padded) | JSON metadata + hex blob |
| `block_index` | Block hash | Height |
| `transactions` | Tx hash | JSON metadata + hex blob |
| `spent_keys` | Key image | Block height |
| `outputs:{amount}` | Index | Global output entry |

Supports two sync modes:
- **RPC sync:** Poll-based fetching from a JSON-RPC daemon.
- **P2P sync:** Levin protocol REQUEST_CHAIN / REQUEST_GET_OBJECTS loop.

Both share a common `processBlockBlobs()` validation path.

## wallet/

Interface-driven design with four core abstractions:

| Interface | Purpose | v1 Implementation |
|-----------|---------|-------------------|
| `Scanner` | Detect owned outputs via ECDH | `V1Scanner` |
| `Signer` | Produce ring signatures | `NLSAGSigner` |
| `Builder` | Construct signed transactions | `V1Builder` |
| `RingSelector` | Pick decoy outputs | `RPCRingSelector` |

Account key derivation: `viewSecret = sc_reduce32(Keccak256(spendSecret))`, matching the C++ `account_base::generate()` pattern. 25-word CryptoNote mnemonic encoding using the Electrum 1626-word dictionary.

## mining/

Solo PoW miner flow:

1. `GetBlockTemplate(walletAddr)` -- fetch template from daemon
2. `HeaderMiningHash(block)` -- Keccak-256 of `BlockHashingBlob` with nonce=0
3. Nonce loop: `RandomXHash("LetheanRandomXv1", headerHash || nonce_LE)`
4. `CheckDifficulty(powHash, difficulty)` -- solution found?
5. `SubmitBlock(hexBlob)` -- submit the solved block

## CGo Bridge Design

The crypto package follows **ADR-001: Go Shell + C++ Crypto Library**:

- `crypto/upstream/` -- 37 vendored C++ files from Zano commit `fa1608cf`
- `crypto/compat/` -- Compatibility stubs replacing epee/Boost dependencies
- `crypto/bridge.h` -- Stable C API (29 functions). Only `uint8_t*` pointers cross the boundary.
- `crypto/randomx/` -- Vendored RandomX source (26 files including x86_64 JIT)

Build: `cmake -S crypto -B crypto/build && cmake --build crypto/build --parallel`

All curve points on chain are stored premultiplied by the cofactor inverse (1/8). The `PointMul8`/`PointDiv8` helpers convert between representations. CLSAG generate takes full points; CLSAG verify takes premultiplied values.
