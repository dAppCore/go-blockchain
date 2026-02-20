# Architecture

go-blockchain is a pure Go implementation of the Lethean blockchain protocol. It
provides chain configuration, core cryptographic data types, consensus-critical
wire serialisation, and difficulty adjustment for the Lethean CryptoNote/Zano-fork
chain.

Module path: `forge.lthn.ai/core/go-blockchain`

---

## Package Structure

```
config/       Chain parameters (mainnet/testnet), hardfork schedule
types/        Core data types: Hash, PublicKey, Address, Block, Transaction
wire/         Binary serialisation (CryptoNote varint encoding)
difficulty/   PoW + PoS difficulty adjustment (LWMA variant)
```

### config/

Defines every consensus-critical constant for the Lethean chain, derived directly
from the canonical C++ source files `currency_config.h.in` and `default.cmake`.
Constants cover tokenomics, address prefixes, network ports, difficulty parameters,
block and transaction limits, version numbers, PoS parameters, P2P constants,
network identity, currency identity, and alias rules.

The `ChainConfig` struct aggregates all parameters into a single value.
Pre-populated `Mainnet` and `Testnet` variables are provided as package-level
globals. The hardfork schedule is defined in `hardfork.go` with lookup functions
for querying active versions at any block height.

### types/

Fixed-size byte array types matching the CryptoNote specification:

- `Hash` (32 bytes) -- Keccak-256 hash values
- `PublicKey` (32 bytes) -- Ed25519 public keys
- `SecretKey` (32 bytes) -- Ed25519 secret keys
- `KeyImage` (32 bytes) -- double-spend detection images
- `Signature` (64 bytes) -- cryptographic signatures

Also contains the full address encoding/decoding implementation (CryptoNote
base58 with Keccak-256 checksums), block header and block structures, and all
transaction types across versions 0 through 3.

### wire/

Consensus-critical binary serialisation for blocks, transactions, and all wire
primitives. All encoding is bit-identical to the C++ reference implementation.

**Primitives:**
- `Encoder` / `Decoder` -- sticky-error streaming codec (call `Err()` once)
- `EncodeVarint` / `DecodeVarint` -- 7-bit LEB128 with MSB continuation
- `Keccak256` -- pre-NIST Keccak-256 (CryptoNote's `cn_fast_hash`)

**Block serialisation:**
- `EncodeBlockHeader` / `DecodeBlockHeader` -- wire order: major, nonce(LE64),
  prev_id, minor(varint), timestamp(varint), flags
- `EncodeBlock` / `DecodeBlock` -- header + miner_tx + tx_hashes
- `BlockHashingBlob` -- serialised header || tree_root || varint(tx_count)
- `BlockHash` -- Keccak-256 of varint(len) + block hashing blob

**Transaction serialisation (v0/v1):**
- `EncodeTransactionPrefix` / `DecodeTransactionPrefix` -- version-dependent
  field ordering (v0/v1: version, vin, vout, extra; v2+: version, vin, extra, vout)
- `EncodeTransaction` / `DecodeTransaction` -- prefix + signatures + attachment
- All variant tags match `SET_VARIANT_TAGS` from `currency_basic.h`
- Extra/attachment stored as raw wire bytes for bit-identical round-tripping

**Hashing:**
- `TreeHash` -- CryptoNote Merkle tree (direct port of `crypto/tree-hash.c`)
- `TransactionPrefixHash` -- Keccak-256 of serialised prefix
- `TransactionHash` -- Keccak-256 of full serialised transaction

### difficulty/

LWMA (Linear Weighted Moving Average) difficulty adjustment algorithm for both
PoW and PoS blocks. Examines a window of recent block timestamps and cumulative
difficulties to calculate the next target difficulty.

---

## Key Types

### ChainConfig

```go
type ChainConfig struct {
    Name                             string
    Abbreviation                     string
    IsTestnet                        bool
    CurrencyFormationVersion         uint64
    Coin                             uint64
    DisplayDecimalPoint              uint8
    BlockReward                      uint64
    DefaultFee                       uint64
    MinimumFee                       uint64
    Premine                          uint64
    AddressPrefix                    uint64
    IntegratedAddressPrefix          uint64
    AuditableAddressPrefix           uint64
    AuditableIntegratedAddressPrefix uint64
    P2PPort                          uint16
    RPCPort                          uint16
    StratumPort                      uint16
    DifficultyPowTarget              uint64
    DifficultyPosTarget              uint64
    DifficultyWindow                 uint64
    DifficultyLag                    uint64
    DifficultyCut                    uint64
    DifficultyPowStarter             uint64
    DifficultyPosStarter             uint64
    MaxBlockNumber                   uint64
    TxMaxAllowedInputs               uint64
    TxMaxAllowedOutputs              uint64
    DefaultDecoySetSize              uint64
    HF4MandatoryDecoySetSize         uint64
    MinedMoneyUnlockWindow           uint64
    P2PMaintainersPubKey             string
}
```

Pre-populated globals `Mainnet` and `Testnet` contain the complete parameter
sets for each network. Mainnet uses ports 36940-36942; testnet uses 46940-46942.

### HardFork

```go
type HardFork struct {
    Version     uint8
    Height      uint64
    Mandatory   bool
    Description string
}
```

Seven hardfork versions are defined (HF0 through HF6). On mainnet, HF0 is
active from genesis, HF1 and HF2 activate after block 10,080, and HF3 through
HF6 are scheduled at height 999,999,999 (effectively future). On testnet, most
forks activate early for testing.

### Address

```go
type Address struct {
    SpendPublicKey PublicKey
    ViewPublicKey  PublicKey
    Flags          uint8
}
```

Four address types are supported via distinct prefixes:

| Type | Prefix | Leading chars |
|------|--------|---------------|
| Standard | `0x1eaf7` | `iTHN` |
| Integrated | `0xdeaf7` | `iTHn` |
| Auditable | `0x3ceff7` | `iThN` |
| Auditable integrated | `0x8b077` | `iThn` |

### Block and Transaction

```go
type BlockHeader struct {
    MajorVersion uint8
    Nonce        uint64
    PrevID       Hash
    MinorVersion uint64   // varint on wire
    Timestamp    uint64   // varint on wire
    Flags        uint8
}

type Block struct {
    BlockHeader
    MinerTx  Transaction
    TxHashes []Hash
}

type Transaction struct {
    Version    uint64   // varint on wire
    Vin        []TxInput
    Vout       []TxOutput
    Extra      []byte   // raw wire bytes (variant vector)
    Signatures [][]Signature  // v0/v1 only
    Attachment []byte         // raw wire bytes (variant vector)
    Proofs     []byte         // raw wire bytes (v2+ only)
    HardforkID uint8          // v3+ only
}
```

Transaction versions progress through the hardfork schedule:

| Version | Era | Description |
|---------|-----|-------------|
| 0 | Genesis | Coinbase transactions |
| 1 | Pre-HF4 | Standard transparent transactions |
| 2 | Post-HF4 | Zarcanum confidential transactions (CLSAG) |
| 3 | Post-HF5 | Confidential assets with surjection proofs |

Input types: `TxInputGenesis` (coinbase, tag `0x00`) and `TxInputToKey` (standard
spend with ring signature, tag `0x01`).

Output types: `TxOutputBare` (transparent, tag `0x24`) and `TxOutputZarcanum`
(confidential with Pedersen commitments, tag `0x26`).

Additional types: `TxOutToKey` (public key + mix_attr, 33 bytes on wire),
`TxOutRef` (variant: global index or ref_by_id).

---

## Design Decisions

### Why CryptoNote Base58

CryptoNote uses its own base58 variant (not Bitcoin's base58check). The alphabet
omits `0`, `O`, `I`, and `l` to avoid visual ambiguity. Data is split into 8-byte
blocks, each encoded independently into 11 base58 characters. The final partial
block produces fewer characters according to a fixed mapping table
(`base58BlockSizes`). This block-based approach differs from Bitcoin's
whole-number division and produces different output for the same input bytes.

The implementation uses `math/big` for block conversion rather than a lookup
table. This is correct for all uint64 ranges but is not optimised for
high-throughput address generation. Performance optimisation is deferred to a
later phase if profiling identifies it as a bottleneck.

### Why Keccak-256 (Not SHA3-256)

CryptoNote predates the NIST SHA-3 standard. It uses the original Keccak-256
submission (`sha3.NewLegacyKeccak256()`), which differs from the finalised
SHA3-256 in padding. This is consensus-critical -- using SHA3-256 would produce
different checksums and break address compatibility with the C++ node.

### Address Encoding Algorithm

1. Encode the address prefix as a CryptoNote varint
2. Append the 32-byte spend public key
3. Append the 32-byte view public key
4. Append the 1-byte flags field
5. Compute Keccak-256 over bytes 1-4, take the first 4 bytes as checksum
6. Append the 4-byte checksum
7. Encode the entire blob using CryptoNote base58

Decoding reverses this process: base58 decode, extract and validate the varint
prefix, verify the Keccak-256 checksum, then extract the two keys and flags.

### Block Hash Length Prefix

The C++ code computes block hashes via `get_object_hash(get_block_hashing_blob(b))`.
Because `get_block_hashing_blob` returns a `blobdata` (std::string) and
`get_object_hash` serialises its argument through `binary_archive` before hashing,
the actual hash input is `varint(len(blob)) || blob` -- the binary archive
prepends a varint length when serialising a string. This CryptoNote convention is
replicated in Go's `BlockHash` function.

### Extra as Raw Bytes

Transaction extra, attachment, and proofs fields are stored as opaque raw wire
bytes rather than being fully parsed into Go structures. The `decodeRawVariantVector`
function reads variant vectors at the tag level to determine element boundaries but
preserves all bytes verbatim. This enables bit-identical round-tripping without
implementing every extra variant type (there are 20+ defined in the C++ code).

### Varint Encoding

The wire format uses 7-bit variable-length integers identical to protobuf
varints. Each byte carries 7 data bits in the low bits with the MSB set to 1
if more bytes follow. A uint64 requires at most 10 bytes. The implementation
provides sentinel errors (`ErrVarintOverflow`, `ErrVarintEmpty`) for malformed
input.

### Hardfork System (Reverse-Scan VersionAtHeight)

`VersionAtHeight()` iterates all hardforks and returns the highest version whose
activation height has been passed. A fork with `Height=0` is active from genesis.
A fork with `Height=N` is active at heights strictly greater than N.

This scan approach (rather than a sorted binary search) is deliberate: the fork
list is small (7 entries) and correctness is trivially verifiable. The same list
drives both `VersionAtHeight()` and `IsHardForkActive()`.

### LWMA Difficulty Adjustment

The difficulty algorithm uses the LWMA (Linear Weighted Moving Average) approach:

```
nextDiff = difficultyDelta * targetInterval / timeSpan
```

The window examines up to 735 blocks (720 window + 15 lag). When fewer blocks
are available (early chain), the algorithm uses whatever data exists. Division
by zero is prevented by clamping the time span to a minimum of 1 second.
`StarterDifficulty` (value 1) is returned when insufficient data is available.

### ADR-001: Go Shell + C++ Crypto Library

This package follows ADR-001. All protocol logic, data types, serialisation,
and configuration live in pure Go. Only the mathematically complex cryptographic
primitives (ring signatures, bulletproofs, Zarcanum proofs) will be delegated to
a cleaned C++ library via CGo in later phases. This boundary keeps the Go code
testable without a C toolchain while preserving access to battle-tested
cryptographic implementations.
