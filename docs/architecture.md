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
crypto/       CGo bridge to vendored C++ libcryptonote (keys, signatures, proofs)
p2p/          CryptoNote P2P command types (handshake, sync, relay)
rpc/          Daemon JSON-RPC 2.0 client (12 endpoints)
chain/        Chain storage, indexing, and sync client (go-store backed)
consensus/    Three-layer block/transaction validation (structural, economic, crypto)
wallet/       Wallet core: key management, scanning, signing, TX construction
mining/      Solo PoW miner (daemon RPC, RandomX nonce grinding)
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

### p2p/

CryptoNote P2P protocol command types for peer-to-peer communication. This
package provides encode/decode for all Levin protocol commands, built on the
`node/levin/` sub-package in go-p2p.

The package depends on `forge.lthn.ai/core/go-p2p/node/levin` for the Levin
wire format (33-byte header, portable storage serialisation, framed TCP
connections) and defines the application-level command semantics:

- **handshake.go** -- COMMAND_HANDSHAKE (1001): NodeData (network ID, peer ID,
  local time, port) + CoreSyncData exchange. Peerlist decoding from packed
  24-byte entries.
- **timedsync.go** -- COMMAND_TIMED_SYNC (1002): periodic blockchain state sync.
- **ping.go** -- COMMAND_PING (1003): simple liveness check.
- **relay.go** -- Block relay (2001), transaction relay (2002), chain
  request/response (2006/2007). `BlockCompleteEntry` for block + transaction
  blob pairs. `BlockContextInfo` for chain entry response objects.
- **getobjects.go** -- `RequestGetObjects` (2003) and `ResponseGetObjects`
  (2004) types for fetching blocks by hash during P2P sync.
- **sync.go** -- CoreSyncData type (current_height, top_id, checkpoint,
  core_time, client_version, pruning mode).
- **commands.go** -- Command ID re-exports from the levin package.
- **integration_test.go** -- Build-tagged (`//go:build integration`) test that
  TCP-connects to the C++ testnet daemon on localhost:46942 and performs a full
  handshake + ping exchange.

The Levin wire format in go-p2p includes:
- **node/levin/header.go** -- 33-byte packed header with signature validation.
- **node/levin/varint.go** -- Portable storage varint (2-bit size mark, NOT the
  same as CryptoNote LEB128 varints in wire/).
- **node/levin/storage.go** -- Portable storage section encode/decode (epee KV
  format with 12 type tags).
- **node/levin/connection.go** -- Framed TCP connection with header + payload
  read/write.

### rpc/

Typed JSON-RPC 2.0 client for querying the Lethean daemon. The `Client` struct
wraps `net/http` and provides Go methods for 11 core daemon endpoints.

Eight endpoints use JSON-RPC 2.0 via `/json_rpc`. Two endpoints (`GetHeight`,
`GetTransactions`) use legacy JSON POST to dedicated URI paths (`/getheight`,
`/gettransactions`), as the C++ daemon registers these with `MAP_URI_AUTO_JON2`
rather than `MAP_JON_RPC`.

**Client transport:**
- `client.go` -- `Client` struct with `call()` (JSON-RPC 2.0) and `legacyCall()`
  (plain JSON POST). `RPCError` type for daemon error codes.
- `types.go` -- `BlockHeader`, `DaemonInfo`, `BlockDetails`, `TxInfo` shared types.

**Endpoints:**
- `info.go` -- `GetInfo`, `GetHeight` (legacy), `GetBlockCount`.
- `blocks.go` -- `GetLastBlockHeader`, `GetBlockHeaderByHeight`,
  `GetBlockHeaderByHash`, `GetBlocksDetails`.
- `transactions.go` -- `GetTxDetails`, `GetTransactions` (legacy).
- `mining.go` -- `GetBlockTemplate`, `SubmitBlock`.

**Wallet endpoints:**
- `wallet.go` -- `GetRandomOutputs` (for ring decoy selection via `/getrandom_outs1`)
  and `SendRawTransaction` (for transaction submission via `/sendrawtransaction`).

**Testing:**
- Mock HTTP server tests for all endpoints and error paths.
- Build-tagged integration test (`//go:build integration`) against C++ testnet
  daemon on `localhost:46941`. Verifies genesis block hash matches Phase 1
  result (`cb9d5455...`).

### chain/

Stores and indexes the Lethean blockchain by syncing from a C++ daemon via RPC.
Uses go-store (pure-Go SQLite) for persistence with five storage groups mapping
to the C++ daemon's core containers.

**Storage schema:**
- `blocks` -- blocks by height (zero-padded key), JSON metadata + hex blob.
- `block_index` -- hash-to-height reverse index.
- `transactions` -- tx hash to JSON metadata + hex blob.
- `spent_keys` -- key image to block height (double-spend index).
- `outputs:{amount}` -- global output index per amount.

**Core operations:**
- `chain.go` -- `Chain` struct with `New()`, `Height()`, `TopBlock()`.
- `store.go` -- `PutBlock`, `GetBlockByHeight/Hash`, `PutTransaction`,
  `GetTransaction`, `HasTransaction`.
- `index.go` -- `MarkSpent`, `IsSpent`, `PutOutput`, `GetOutput`, `OutputCount`.
- `validate.go` -- Header validation (previous block linkage, height sequence,
  block size).
- `sync.go` -- `Sync(client)` blocking RPC poll loop. Fetches blocks in batches
  of 10, decodes wire blobs, validates headers, indexes transactions/outputs/
  key images, verifies block hashes. `processBlockBlobs()` shared validation
  path for both RPC and P2P sync. `resolveBlockBlobs()` reconstructs block data
  from daemon JSON responses. Context cancellation and progress logging for
  long syncs.

**P2P sync:**
- `P2PConnection` interface abstracting peer communication (request/response
  over any transport).
- `LevinP2PConn` adapter wrapping a Levin TCP connection to satisfy
  `P2PConnection`.
- `P2PSync()` state machine running the REQUEST_CHAIN / REQUEST_GET_OBJECTS
  loop until the peer has no more blocks.
- `SparseChainHistory()` builds exponentially-spaced block hash lists matching
  the C++ `get_short_chain_history()` algorithm.
- `GetRingOutputs()` callback for ring signature verification during sync.

**Testing:**
- Unit tests with go-store `:memory:` for all CRUD operations and validation.
- Mock RPC server sync tests.
- Build-tagged integration test (`//go:build integration`) syncing first 10
  blocks from C++ testnet daemon on `localhost:46941`.
- P2P sync integration test against C++ testnet daemon on `localhost:46942`.

### consensus/

Standalone consensus validation package implementing three-layer block and
transaction verification. All functions are pure — they take types, config,
and height, returning errors. No dependency on `chain/` or any storage layer.

**Layer 1 — Structural (no crypto):**
- `ValidateTransaction()` — 8 ordered checks matching C++ `validate_tx_semantic()`:
  blob size, input count, input types, output validation, money overflow,
  key image uniqueness, extra parsing, balance check.

**Layer 2 — Economic:**
- `BaseReward()` / `BlockReward()` / `MinerReward()` — fixed 1 LTHN/block with
  size penalty using 128-bit arithmetic (`math/bits.Mul64`). Pre-HF4 fees go to
  miner; post-HF4 fees are burned.
- `TxFee()` — fee extraction with overflow detection.
- `ValidateBlockReward()` — miner output sum vs expected reward.

**Layer 3 — Cryptographic (CGo):**
- `CheckDifficulty()` / `CheckPoWHash()` — 256-bit PoW hash comparison via
  RandomX (vendored in `crypto/randomx/`, key `"LetheanRandomXv1"`).
- `VerifyTransactionSignatures()` — NLSAG (pre-HF4) and CLSAG (post-HF4)
  signature verification. `verifyV1Signatures()` performs real NLSAG ring
  signature verification via CGo, using a `RingOutputsFn` callback to fetch
  ring member public keys from chain storage.

**Block validation:**
- `CheckTimestamp()` — future time limit (7200s PoW, 1200s PoS) + median-of-60.
- `ValidateMinerTx()` — coinbase structure (1 input = PoW, 2 inputs = PoS).
- `ValidateBlock()` — orchestrator combining timestamp, miner tx, and reward.
- `IsPoS()` — flags bit 0 check.

All validation is hardfork-aware via `config.IsHardForkActive()` with the fork
schedule passed as a parameter. The `chain/sync.go` integration calls
`ValidateMinerTx()` and `ValidateTransaction()` during sync, with optional
signature verification via `SyncOptions.VerifySignatures`.

### wallet/

Full send+receive wallet for the Lethean blockchain. Uses interface-driven design
with four core abstractions so v1 (NLSAG) implementations ship now and v2+
(Zarcanum/CLSAG) slot in later without changing callers.

**Core interfaces:**
- `Scanner` -- detects outputs belonging to a wallet using ECDH derivation.
  `V1Scanner` implements v0/v1 output detection.
- `Signer` -- produces ring signatures for transaction inputs. `NLSAGSigner`
  wraps the CGo NLSAG ring signature primitives.
- `Builder` -- constructs signed transactions. `V1Builder` handles v1
  transactions with decoy ring selection and ECDH output derivation.
- `RingSelector` -- picks decoy outputs for ring signatures.
  `RPCRingSelector` fetches random outputs from the daemon via RPC.

**Account management:**
- `account.go` -- `Account` struct with spend and view key pairs. Key
  derivation: `viewSecret = sc_reduce32(Keccak256(spendSecret))`, matching
  the C++ `account_base::generate()` pattern. Three creation methods:
  `GenerateAccount()`, `RestoreFromSeed()`, `RestoreViewOnly()`.
- `mnemonic.go` -- 25-word CryptoNote mnemonic encoding/decoding using the
  Electrum 1626-word dictionary. CRC32 checksum word.
- Persistence with Argon2id (time=3, mem=64MB, threads=4) + AES-256-GCM
  encryption via go-store.

**Transaction extra:**
- `extra.go` -- minimal parser for three wallet-critical variant tags: tx
  public key (tag 22), unlock time (tag 14), derivation hint (tag 11).
  Unknown tags skipped. Raw bytes preserved for round-tripping.

**Wallet orchestrator:**
- `wallet.go` -- `Wallet` struct tying scanning, building, and sending.
  `Sync()` scans from last checkpoint to chain tip, detecting owned outputs
  and spent key images. `Balance()` returns confirmed vs locked amounts.
  `Send()` performs largest-first coin selection, builds, signs, and submits
  transactions via RPC. `Transfers()` lists all tracked outputs.

**Transfer storage:**
- `transfer.go` -- `Transfer` struct persisted as JSON in go-store group
  `transfers`, keyed by key image hex. `IsSpendable()` checks spend status,
  coinbase maturity (MinedMoneyUnlockWindow=10), and unlock time.

**Testing:**
- Unit tests with go-store `:memory:` for all components.
- Build-tagged integration test (`//go:build integration`) syncing from
  C++ testnet daemon and verifying balance/transfers.

### mining/

Solo PoW miner that talks to a C++ daemon via JSON-RPC. Single-threaded
mining loop: fetches block templates, computes a header mining hash once
per template, then grinds nonces with RandomX until a solution is found
or the chain advances.

**Core types:**
- `Config` -- daemon URL, wallet address, poll interval, callbacks.
- `Miner` -- mining loop with `Start(ctx)` (blocking) and `Stats()` (lock-free).
- `TemplateProvider` -- interface satisfied by `rpc.Client` for testability.

**Mining flow:**
1. `GetBlockTemplate(walletAddr)` -- fetches template from daemon.
2. `HeaderMiningHash(block)` -- Keccak-256 of `BlockHashingBlob` with nonce=0.
3. Nonce loop: `RandomXHash("LetheanRandomXv1", headerHash || nonce_LE)`.
4. `CheckDifficulty(powHash, difficulty)` -- solution found?
5. `SubmitBlock(hexBlob)` -- submits the solved block.

**Template refresh:** polls `GetInfo()` every `PollInterval` (default 3s) to
detect new blocks. Re-fetches the template when the chain height advances.

**Testing:** Mock `TemplateProvider` for unit tests. Build-tagged integration
test against C++ testnet daemon on `localhost:46941`.

### tui/ — Terminal Dashboard

Model library for the blockchain TUI dashboard:

| File | Purpose |
|------|---------|
| `node.go` | Node wrapper — polls chain for status snapshots |
| `status_model.go` | StatusModel (FrameModel) — chain sync header bar |
| `explorer_model.go` | ExplorerModel (FrameModel) — block list / block detail / tx detail |
| `keyhints_model.go` | KeyHintsModel (FrameModel) — context-sensitive key hints |
| `messages.go` | Custom bubbletea message types |

### cmd/chain/ — TUI Binary

Thin wiring: creates Node, StatusModel, ExplorerModel, KeyHintsModel, wires into
core/cli Frame ("HCF" layout), starts P2P sync in background, runs Frame.

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

### crypto/ -- CGo Bridge to libcryptonote

Bridges Go to the upstream CryptoNote C++ crypto library via CGo. The C++ code
is vendored in `crypto/upstream/` (37 files from Zano commit `fa1608cf`) and
built as a static library (`libcryptonote.a`) via CMake.

**Build flow:** `CMakeLists.txt` → `cmake --build` → `libcryptonote.a` → CGo links.

**C API contract:** `bridge.h` defines the stable C boundary. Go code calls ONLY
these functions -- no C++ types cross the boundary. All parameters are raw
`uint8_t*` pointers with explicit sizes. This is the same CGo pattern used by
`core/go-ai` for the MLX backend.

**Compat layer:** `crypto/compat/` provides minimal stubs replacing epee/Boost
dependencies (logging macros, warnings pragmas, zero-init, profile tools). The
upstream files are unmodified copies; all adaptation lives in the compat headers.

**Provenance:** `crypto/PROVENANCE.md` maps each vendored file to its upstream
origin path and modification status, with an update workflow for tracking Zano
upstream changes.

**Exposed operations:**

| Category | Functions |
|----------|-----------|
| Hashing | `FastHash` (Keccak-256) |
| Key ops | `GenerateKeys`, `SecretToPublic`, `CheckKey` |
| Key derivation | `GenerateKeyDerivation`, `DerivePublicKey`, `DeriveSecretKey` |
| Key images | `GenerateKeyImage`, `ValidateKeyImage` |
| Standard sigs | `GenerateSignature`, `CheckSignature` |
| Ring sigs (NLSAG) | `GenerateRingSignature`, `CheckRingSignature` |
| CLSAG (HF4+) | `GenerateCLSAGGG`, `VerifyCLSAGGG`, `VerifyCLSAGGGX`, `VerifyCLSAGGGXXG` |
| Point helpers | `PointMul8`, `PointDiv8` (cofactor 1/8 premultiplication) |
| Proof verification | `VerifyBPPE`, `VerifyBGE`, `VerifyZarcanum` (stubs -- Phase 4) |

**Ring buffer convention:** Ring entries are flat byte arrays. CLSAG ring entries
pack 32-byte public keys per dimension (GG=64B, GGX=96B, GGXXG=128B per entry).
Signatures are serialised as flat buffers with documented layouts in `bridge.h`.

**1/8 premultiplication:** On-chain commitments are stored premultiplied by the
cofactor inverse (1/8). The `PointMul8`/`PointDiv8` helpers convert between
representations. CLSAG generate takes full points; CLSAG verify takes
premultiplied values.

### ADR-001: Go Shell + C++ Crypto Library

This package follows ADR-001. All protocol logic, data types, serialisation,
and configuration live in pure Go. The mathematically complex cryptographic
primitives (ring signatures, bulletproofs, Zarcanum proofs) are delegated to
the vendored C++ library in `crypto/` via CGo. This boundary keeps the pure Go
code testable without a C toolchain while preserving access to battle-tested
cryptographic implementations.
