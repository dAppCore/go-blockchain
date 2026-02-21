# Project History

## Origin

go-blockchain implements the Lethean blockchain protocol in pure Go, following
ADR-001 (Go Shell + C++ Crypto Library). The chain lineage is CryptoNote (2014)
to IntenseCoin (2017) to Lethean to a Zano rebase. All consensus parameters are
derived from the canonical C++ source files `currency_config.h.in` and
`default.cmake`.

The package was created as part of the broader effort to rewrite the Lethean
node tooling in Go, keeping protocol logic in Go while deferring only the
mathematically complex cryptographic primitives (ring signatures, bulletproofs,
Zarcanum proofs) to a cleaned C++ library via CGo in later phases.

---

## Phase 0 -- Scaffold

Commit: `4c0b7f2` -- `feat: Phase 0 scaffold -- config, types, wire, difficulty`

Phase 0 established the foundational four packages with complete test suites
and full coverage of the consensus-critical configuration surface.

### Packages implemented

- **config/** -- All chain constants from `currency_config.h.in` and
  `default.cmake`: tokenomics (Coin, BlockReward, fees, premine), address
  prefixes (standard, integrated, auditable, auditable integrated), network
  ports (mainnet 36940-36942, testnet 46940-46942), difficulty parameters
  (window 720, lag 15, cut 60, targets 120s), block and transaction limits,
  version constants, PoS parameters, P2P constants, network identity, currency
  identity, and alias rules. `ChainConfig` struct with pre-populated `Mainnet`
  and `Testnet` globals. Hardfork schedule (HF0-HF6) with `VersionAtHeight()`
  and `IsHardForkActive()` lookup functions.

- **types/** -- Fixed-size cryptographic types: `Hash` (32 bytes), `PublicKey`
  (32 bytes), `SecretKey` (32 bytes), `KeyImage` (32 bytes), `Signature`
  (64 bytes). Hex encode/decode for `Hash` and `PublicKey`. CryptoNote base58
  address encoding with Keccak-256 checksums. `Address` struct with
  `Encode()`/`DecodeAddress()` round-trip. `BlockHeader`, `Block`,
  `Transaction` structs. Input types (`TxInputGenesis`, `TxInputToKey`) and
  output types (`TxOutputBare`, `TxOutputZarcanum`) with wire type tags.
  `TxInput` and `TxOutput` interfaces.

- **wire/** -- CryptoNote varint encoding (7-bit LEB128 with MSB continuation).
  `EncodeVarint()` and `DecodeVarint()` with sentinel errors for overflow and
  empty input. Maximum 10 bytes per uint64.

- **difficulty/** -- LWMA difficulty adjustment algorithm. `NextDifficulty()`
  examines a window of timestamps and cumulative difficulties to compute the
  next target. Handles insufficient data (returns `StarterDifficulty`), zero
  time spans, and negative difficulty deltas.

### Tests added

75 test cases across 5 test files, all passing with the race detector:

- `config/config_test.go` -- 7 test functions validating every constant group
  against C++ source values: tokenomics, address prefixes, ports (mainnet and
  testnet), difficulty parameters, network identity, `ChainConfig` struct fields,
  transaction limits, and transaction version constants.

- `config/hardfork_test.go` -- 7 test functions covering `VersionAtHeight()`
  on both mainnet and testnet fork schedules, `IsHardForkActive()` for boundary
  conditions, unknown version queries, empty fork lists, single-element fork
  lists, and full fork schedule validation for both networks.

- `types/address_test.go` -- 8 test functions covering encode/decode round-trips
  for all four address types, deterministic encoding, auditable flag detection,
  integrated prefix detection, invalid input rejection (empty, invalid base58
  characters, too short), checksum corruption detection, base58 round-trip, and
  base58 edge cases (empty encode/decode).

- `difficulty/difficulty_test.go` -- 7 test functions covering stable difficulty
  with constant intervals, empty input, single entry, fast blocks (difficulty
  increases), slow blocks (difficulty decreases), zero time span handling, and
  algorithm constants.

- `wire/varint_test.go` -- 5 test functions covering encoding known values,
  decoding known values, round-trip across all bit boundaries (0 through
  `MaxUint64`), empty input errors, and overflow detection.

### Coverage

| Package | Coverage |
|---------|----------|
| config | 100.0% |
| difficulty | 81.0% |
| types | 73.4% |
| wire | 95.2% |

`go test -race ./...` passed clean. `go vet ./...` produced no warnings.

---

## Phase 1 -- Wire Serialisation

Phase 1 added consensus-critical binary serialisation for blocks and transactions,
verified to be bit-identical to the C++ daemon output. The definitive proof is
the genesis block hash test: serialising the testnet genesis block and computing
its Keccak-256 hash produces the exact value returned by the C++ daemon
(`cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963`).

### Type corrections from Phase 0

Phase 0 types had several mismatches with the C++ wire format, corrected here:

- `BlockHeader.MinorVersion` changed from `uint8` to `uint64` (varint on wire)
- `BlockHeader.Flags` added (`uint8`, 1 byte fixed)
- `Transaction.Version` changed from `uint8` to `uint64` (varint on wire)
- `Transaction.UnlockTime` removed (lives in extra variants, not top-level)
- All variant tags corrected to match `SET_VARIANT_TAGS` from `currency_basic.h`:
  `InputTypeGenesis=0`, `InputTypeToKey=1`, `OutputTypeBare=36`, `OutputTypeZarcanum=38`
- `TxOutToKey` struct added (public key + mix_attr, 33 bytes packed)
- `TxOutRef` variant type added (global index or ref_by_id)
- `Transaction.Signatures`, `Transaction.Attachment`, `Transaction.Proofs` fields added

### Files added

| File | Purpose |
|------|---------|
| `wire/encoder.go` | Sticky-error streaming encoder |
| `wire/decoder.go` | Sticky-error streaming decoder |
| `wire/block.go` | Block/BlockHeader encode/decode |
| `wire/transaction.go` | Transaction encode/decode (v0/v1 + v2+ stubs) |
| `wire/treehash.go` | Keccak-256 + CryptoNote Merkle tree hash |
| `wire/hash.go` | BlockHash, TransactionPrefixHash, TransactionHash |
| `wire/encoder_test.go` | Encoder round-trip tests |
| `wire/decoder_test.go` | Decoder round-trip tests |
| `wire/block_test.go` | Block header + full block round-trip tests |
| `wire/transaction_test.go` | Coinbase, ToKey, signatures, variant tag tests |
| `wire/treehash_test.go` | Tree hash for 0-8 hashes |
| `wire/hash_test.go` | Genesis block hash verification |

### Key findings

- **Block hash length prefix**: The C++ `get_object_hash(blobdata)` serialises
  the string through `binary_archive` before hashing, prepending `varint(length)`.
  The actual block hash input is `varint(len) || block_hashing_blob`, not just
  the blob itself.

- **Genesis data sources**: The `_genesis_tn.cpp.gen` uint64 array is the
  canonical genesis transaction data, not the `.genesis_tn.txt` hex dump (which
  was stale from a different wallet generation).

- **Extra as raw bytes**: Transaction extra, attachment, and proofs are stored
  as opaque raw wire bytes with tag-level boundary detection. This enables
  bit-identical round-tripping without implementing all 20+ extra variant types.

### Coverage

| Package | Coverage |
|---------|----------|
| config | 100.0% |
| difficulty | 81.0% |
| types | 73.4% |
| wire | 76.8% |

Wire coverage is reduced by v2+ code paths (0% -- Phase 2 scope). Excluding
v2+ stubs, the v0/v1 serialisation code exceeds 85% coverage.

## Phase 2 -- Crypto Bridge

Phase 2 created the `crypto/` package with a CGo bridge to the vendored C++
CryptoNote crypto library. The upstream code (37 files from Zano commit
`fa1608cf`) is built as a static library via CMake, with a thin C API
(`bridge.h`) separating Go from C++ types.

### Files added

| File | Purpose |
|------|---------|
| `crypto/upstream/` | 37 vendored C++ files (unmodified copies) |
| `crypto/compat/` | 11 compatibility stubs replacing epee/Boost |
| `crypto/build/` | CMake build output (libcryptonote.a, ~680KB) |
| `crypto/CMakeLists.txt` | C11/C++17 static library build |
| `crypto/bridge.h` | Stable C API contract (29 functions) |
| `crypto/bridge.cpp` | C→C++ wrappers with memcpy marshalling |
| `crypto/doc.go` | Package documentation + build instructions |
| `crypto/crypto.go` | CGo flags + FastHash binding |
| `crypto/keygen.go` | Key generation, derivation, DerivePublicKey/SecretKey |
| `crypto/keyimage.go` | Key image generation and validation |
| `crypto/signature.go` | Standard + NLSAG ring signatures |
| `crypto/clsag.go` | CLSAG (GG/GGX/GGXXG) + cofactor helpers |
| `crypto/proof.go` | BPP, BPPE, BGE, Zarcanum verification wrappers |
| `crypto/PROVENANCE.md` | Upstream origin mapping + update workflow |
| `crypto/crypto_test.go` | 30 tests (all passing) |

### Key findings

- **CMake build required 5 iterations** to resolve all include paths. The
  upstream files use relative includes (`../currency_core/`, `crypto/crypto-ops.h`)
  that assume the full Zano source tree. Solved with symlinks, additional include
  paths, and compat stubs.

- **eth_signature.cpp excluded** from build -- requires Bitcoin's secp256k1
  library which is not needed for CryptoNote consensus crypto.

- **cn_fast_hash name collision** between bridge.h and hash-ops.h. Resolved by
  renaming the bridge wrapper to `bridge_fast_hash`.

- **Zero key is valid on Ed25519** -- it represents the identity element. Tests
  use `0xFF...FF` for invalid key checks instead.

- **1/8 premultiplication** is critical for CLSAG correctness. On-chain
  commitments are stored as `(1/8)*P`. Generate takes full points; verify takes
  premultiplied values. `PointMul8`/`PointDiv8` helpers convert between forms.

- **Proof verification stubs** return "not implemented" -- the serialisation
  format for BPPE/BGE/Zarcanum proofs requires matching the exact on-chain binary
  layout, which needs real chain data via RPC (Phase 4).

### Tests added

22 test cases in `crypto/crypto_test.go`:

- **Hashing (2):** Known Keccak-256 vector (empty input), non-zero hash
- **Key ops (3):** GenerateKeys round-trip, CheckKey negative, uniqueness
- **Key derivation (2):** ECDH commutativity, output scanning round-trip
  (send → receive → derive ephemeral secret → verify public key match)
- **Key images (3):** Generation, determinism, invalid input rejection
- **Standard sigs (3):** Round-trip, wrong key, wrong message
- **Ring sigs NLSAG (2):** 4-member ring round-trip, wrong message
- **CLSAG GG (2):** Generate+verify round-trip with cofactor handling, wrong message
- **CLSAG size (2):** GGX and GGXXG signature size calculations
- **Proof stubs (3):** Skipped -- pending Phase 4 chain data

All passing with `-race` and `go vet`.

## Phase 3 -- P2P Levin Protocol

Phase 3 implemented the CryptoNote Levin binary protocol for peer-to-peer
communication across two repositories. The go-p2p package gained a `node/levin/`
sub-package with the wire format (header, portable storage, framed TCP
connection). The go-blockchain package gained a `p2p/` package with command
handlers for handshake, timed sync, ping, and block/transaction relay.

### Files added (go-p2p)

| File | Purpose |
|------|---------|
| `node/levin/header.go` | 33-byte Levin header encode/decode |
| `node/levin/header_test.go` | Header tests (9 tests) |
| `node/levin/varint.go` | Portable storage varint (2-bit size mark) |
| `node/levin/varint_test.go` | Varint tests (14 tests) |
| `node/levin/storage.go` | Portable storage section encode/decode |
| `node/levin/storage_test.go` | Storage tests (14 tests) |
| `node/levin/connection.go` | Framed TCP connection |
| `node/levin/connection_test.go` | Connection tests (7 tests) |

### Files added/modified (go-blockchain)

| File | Purpose |
|------|---------|
| `config/config.go` | Added NetworkID constants and ClientVersion |
| `config/config_test.go` | NetworkID validation test |
| `p2p/commands.go` | Command ID re-exports |
| `p2p/sync.go` | CoreSyncData type |
| `p2p/sync_test.go` | CoreSyncData roundtrip test |
| `p2p/ping.go` | Ping encode/decode |
| `p2p/ping_test.go` | Ping tests (2 tests) |
| `p2p/handshake.go` | Handshake command + NodeData + peerlist decoding |
| `p2p/handshake_test.go` | Handshake tests (4 tests) |
| `p2p/timedsync.go` | Timed sync request/response |
| `p2p/relay.go` | Block/tx relay + chain request/response |
| `p2p/relay_test.go` | Relay tests (6 tests) |
| `p2p/integration_test.go` | C++ testnet integration test |

### Key findings

- **Portable storage varint differs from CryptoNote varint.** CryptoNote uses
  7-bit LEB128 (implemented in wire/varint.go). Portable storage uses a 2-bit
  size mark in the low bits of the first byte (1/2/4/8 byte encoding). Both
  implementations exist in separate packages.

- **POD-as-blob serialisation.** Hashes, network IDs, and other fixed-size types
  are encoded as STRING values containing raw bytes, not as typed fields. The
  peerlist is a single STRING blob containing packed 24-byte entries
  (ip:4 + port:4 + id:8 + last_seen:8).

- **Network ID bytes from net_node.inl.** Mainnet: byte 10 = 0x00 (not testnet),
  byte 15 = 0x54 (84 = formation version). Testnet: byte 10 = 0x01, byte 15 =
  0x64 (100). These are validated during handshake.

- **P2P port is 46942, not 46941.** The testnet ports are: 46940 (stratum),
  46941 (RPC), 46942 (P2P).

- **No Transport interface extraction.** The existing go-p2p WebSocket transport
  is tightly coupled to WebSocket semantics. The Levin code lives in
  `node/levin/` as a standalone sub-package rather than sharing an interface.

### Tests added

44 go-p2p tests + 13 go-blockchain p2p tests + 1 integration test = 58 total.
All passing with `-race` and `go vet`.

## Phase 4 -- RPC Client

Phase 4 implemented a typed JSON-RPC 2.0 client for querying the Lethean C++
daemon. The `rpc/` package provides Go methods for 10 core daemon endpoints
covering chain info, block headers, block details, transactions, and mining.

### Files added

| File | Purpose |
|------|---------|
| `rpc/client.go` | Client struct, JSON-RPC 2.0 `call()` + legacy `legacyCall()` |
| `rpc/client_test.go` | Client transport tests (7 tests) |
| `rpc/types.go` | BlockHeader, DaemonInfo, BlockDetails, TxInfo types |
| `rpc/info.go` | GetInfo, GetHeight (legacy), GetBlockCount |
| `rpc/info_test.go` | Info endpoint tests (3 tests) |
| `rpc/blocks.go` | GetLastBlockHeader, GetBlockHeaderByHeight/ByHash, GetBlocksDetails |
| `rpc/blocks_test.go` | Block endpoint tests (5 tests) |
| `rpc/transactions.go` | GetTxDetails, GetTransactions (legacy) |
| `rpc/transactions_test.go` | Transaction endpoint tests (4 tests) |
| `rpc/mining.go` | SubmitBlock |
| `rpc/mining_test.go` | Mining endpoint tests (2 tests) |
| `rpc/integration_test.go` | Build-tagged integration test against C++ testnet |

### Tests added

21 mock tests + 1 integration test = 22 total.

Mock tests use `httptest.Server` with canned JSON responses to verify all
10 endpoints and their error paths. The integration test (`//go:build integration`)
connects to the C++ testnet daemon on `localhost:46941` and verifies the genesis
block hash matches the Phase 1 result (`cb9d5455...`).

All passing with `-race` and `go vet`.

### Key findings

- **Legacy vs JSON-RPC.** Two endpoints (`GetHeight`, `GetTransactions`) use
  plain JSON POST to dedicated URI paths rather than JSON-RPC 2.0. The C++
  daemon registers these with `MAP_URI_AUTO_JON2` (not `MAP_JON_RPC`), so
  they are not accessible via `/json_rpc`. The client provides `legacyCall()`
  for these paths alongside `call()` for standard JSON-RPC.

- **SubmitBlock array params.** The `submitblock` method takes a JSON array
  `["hexblob"]` as its params, not a named object. This is one of the few
  JSON-RPC 2.0 endpoints in the daemon that uses positional parameters.

- **Status field convention.** All daemon responses include a `"status": "OK"`
  field outside the JSON-RPC result envelope. The client checks this after
  successful JSON-RPC decode and returns an error for non-OK status values.

## Phase 5 -- Chain Storage and Sync Client

Commit range: `8cb5cb4`..`23d337e`

Added `chain/` package implementing blockchain storage and RPC sync client.
Uses go-store (pure-Go SQLite) as the persistence backend with five storage
groups mapping to the C++ daemon's core containers.

**Files added:**
- `chain/meta.go` -- `BlockMeta`, `TxMeta`, `outputEntry` types
- `chain/chain.go` -- `Chain` struct, `New()`, `Height()`, `TopBlock()`
- `chain/store.go` -- Block and transaction storage/retrieval
- `chain/index.go` -- Key image and output index operations
- `chain/validate.go` -- Header validation (linkage, height, size)
- `chain/sync.go` -- RPC sync loop with block processing and indexing
- `chain/chain_test.go` -- Storage round-trip tests (6 tests)
- `chain/validate_test.go` -- Validation tests (5 tests)
- `chain/sync_test.go` -- Sync loop tests with mock RPC (2 tests)
- `chain/integration_test.go` -- C++ testnet integration test

**Storage schema:**
- `blocks` -- by height (zero-padded), JSON meta + hex blob
- `block_index` -- hash to height
- `transactions` -- tx hash to meta + blob
- `spent_keys` -- key image to height
- `outputs:{amount}` -- per-amount global output index

**Dependencies added:** `forge.lthn.ai/core/go-store` (local replace).

## Phase 6 -- Wallet Core

Commit range: `5b677d1`..`11b50d0`

Added `wallet/` package implementing full send+receive wallet functionality
with interface-driven design for v1/v2+ extensibility.

### Files added

| File | Purpose |
|------|---------|
| `wallet/wordlist.go` | 1626-word Electrum mnemonic dictionary |
| `wallet/mnemonic.go` | 25-word seed phrase encode/decode with CRC32 checksum |
| `wallet/mnemonic_test.go` | Mnemonic tests (9 tests) |
| `wallet/extra.go` | TX extra parser for tags 22/14/11 |
| `wallet/extra_test.go` | Extra parsing tests (10 tests) |
| `wallet/account.go` | Account key management with Argon2id+AES-256-GCM encryption |
| `wallet/account_test.go` | Account tests (6 tests) |
| `wallet/transfer.go` | Transfer type and go-store persistence |
| `wallet/transfer_test.go` | Transfer tests (15 tests) |
| `wallet/scanner.go` | Scanner interface + V1Scanner (ECDH output detection) |
| `wallet/scanner_test.go` | Scanner tests (7 tests) |
| `wallet/signer.go` | Signer interface + NLSAGSigner (CGo ring signatures) |
| `wallet/signer_test.go` | Signer tests (4 tests) |
| `wallet/ring.go` | RingSelector interface + RPCRingSelector |
| `wallet/ring_test.go` | Ring selection tests (3 tests) |
| `wallet/builder.go` | Builder interface + V1Builder (TX construction) |
| `wallet/builder_test.go` | Builder tests (3 tests) |
| `wallet/wallet.go` | Wallet orchestrator (sync, balance, send) |
| `wallet/wallet_test.go` | Wallet orchestrator tests (2 tests) |
| `wallet/integration_test.go` | C++ testnet integration test |
| `rpc/wallet.go` | GetRandomOutputs + SendRawTransaction RPC endpoints |
| `rpc/wallet_test.go` | RPC wallet endpoint tests (4 tests) |

### Files modified

| File | Change |
|------|--------|
| `types/types.go` | Added `PublicKey.IsZero()` method |
| `crypto/bridge.h` | Added `cn_sc_reduce32()` declaration |
| `crypto/bridge.cpp` | Added `cn_sc_reduce32()` implementation |
| `crypto/crypto.go` | Added `ScReduce32()` Go wrapper |

### Key findings

- **View key derivation requires sc_reduce32.** The raw Keccak-256 of the
  spend secret key must be reduced modulo the Ed25519 group order before it is
  a valid scalar. Added `cn_sc_reduce32` to the CGo bridge.

- **Interface-driven design.** Four core interfaces (Scanner, Signer, Builder,
  RingSelector) decouple v1 implementations from the orchestrator. Future v2+
  (Zarcanum/CLSAG) implementations slot in by implementing the same interfaces.

- **go-store GetAll.** Transfer listing uses `store.GetAll(group)` which returns
  all key-value pairs in a group, rather than iterating with individual Gets.

- **CryptoNote mnemonic encoding.** The 1626-word Electrum dictionary encodes
  4 bytes into 3 words using modular arithmetic: `val = w1 + n*((n-w1+w2)%n) +
  n*n*((n-w2+w3)%n)` where n=1626. The 25th word is a CRC32 checksum.

### Tests added

63 unit tests + 1 integration test = 64 total across wallet/ and rpc/wallet.
All passing with `-race` and `go vet`.

## Phase 7 -- Consensus Rules

Commit range: `fa1c127`..`112da0e` (12 commits)

Added standalone `consensus/` package implementing three-layer validation
(structural, economic, cryptographic) for blocks and transactions. Vendored
RandomX PoW hash function into the CGo crypto bridge. Integrated consensus
validation into the chain sync pipeline.

### Files added

| File | Purpose |
|------|---------|
| `consensus/doc.go` | Package documentation |
| `consensus/errors.go` | 21 sentinel error variables |
| `consensus/errors_test.go` | Error uniqueness tests |
| `consensus/reward.go` | Block reward with 128-bit size penalty |
| `consensus/reward_test.go` | Reward calculation tests |
| `consensus/fee.go` | Fee extraction with overflow detection |
| `consensus/fee_test.go` | Fee tests |
| `consensus/tx.go` | Transaction semantic validation (8 checks) |
| `consensus/tx_test.go` | TX validation tests |
| `consensus/block.go` | Block validation (timestamp, miner tx, reward) |
| `consensus/block_test.go` | Block validation tests |
| `consensus/pow.go` | PoW difficulty check (256-bit comparison) |
| `consensus/pow_test.go` | PoW tests |
| `consensus/verify.go` | Signature verification scaffold |
| `consensus/verify_test.go` | Signature verification tests |
| `consensus/integration_test.go` | Build-tagged integration test |
| `crypto/randomx/` | Vendored RandomX source (26 files) |
| `crypto/pow.go` | CGo RandomX hash wrapper |
| `crypto/pow_test.go` | RandomX hash test |

### Files modified

| File | Change |
|------|--------|
| `crypto/CMakeLists.txt` | Added RandomX static library build |
| `crypto/bridge.h` | Added `bridge_randomx_hash()` declaration |
| `crypto/bridge.cpp` | Added `bridge_randomx_hash()` implementation |
| `crypto/crypto.go` | Added RandomX CGo flags |
| `config/config.go` | Added `MaxTransactionBlobSize` constant |
| `chain/sync.go` | Added `SyncOptions`, consensus validation calls |
| `chain/sync_test.go` | Updated for new `Sync()` signature |

### Key findings

- **RandomX integration.** Vendored the full RandomX source (26 files including
  x86_64 JIT compiler) into `crypto/randomx/`. Built as a separate static
  library (`librandomx.a`) linked into the CGo bridge. Cache key is
  `"LetheanRandomXv1"`, input is `header_hash(32B) || nonce(8B LE)`.

- **128-bit arithmetic for size penalty.** The block reward penalty formula
  uses `math/bits.Mul64` and `bits.Div64` for 128-bit intermediate products,
  matching the C++ 128-bit unsigned arithmetic exactly.

- **Standalone package.** `consensus/` has zero dependencies on `chain/` or
  any storage layer. All functions are pure: take types + config + height,
  return errors. The `RingOutputsFn` callback type decouples signature
  verification from chain storage.

- **Hardfork-aware validation.** Every validation function accepts the fork
  schedule as a parameter. Pre-HF4 and post-HF4 code paths are implemented
  throughout (input types, fee treatment, output requirements).

### Tests added

37 unit tests + 1 integration test = 38 total in consensus/.
Coverage: 82.1% of statements.

All passing with `-race` and `go vet`.

## Phase 8 -- Mining

Solo PoW miner in `mining/` package. Fetches block templates from the C++
daemon, computes header mining hash (nonce=0 Keccak-256), grinds nonces with
RandomX, submits solutions. Single-threaded loop with poll-based template
refresh.

**Commits:** `8735e53..f9ff8ad` on `feat/phase8-mining`

**Known limitations:**
- Single-threaded (CGo RandomX bridge uses static global cache/VM).
- Solo mining only (no stratum protocol).

---

## Known Limitations

**v2+ transaction serialisation is stubbed.** The v0/v1 wire format is complete
and verified. The v2+ (Zarcanum) code paths compile but are untested -- they
will be validated in Phase 2 when post-HF4 transactions appear on-chain.

**BPP range proof verification tested with real data.** The `cn_bpp_verify`
bridge function (Bulletproofs++, 1 delta, `bpp_crypto_trait_ZC_out`) is verified
against a real testnet coinbase transaction from block 101 (post-HF4). The
`cn_bppe_verify` function (Bulletproofs++ Enhanced, 2 deltas,
`bpp_crypto_trait_Zarcanum`) is wired but untested with real data -- it is used
for Zarcanum PoS range proofs, not regular transaction output proofs. BGE
(one-out-of-many) proofs are wired but untested (coinbase transactions have
empty BGE proof vectors). CLSAG GGX/GGXXG verify functions are similarly wired
but untested without real ring data. Zarcanum proof verification remains
stubbed -- the bridge API needs extending to pass kernel_hash, ring,
last_pow_block_id, stake_ki, and pos_difficulty.

**CGo toolchain required.** The `crypto/` package requires CMake, GCC/Clang,
OpenSSL, and Boost headers to build `libcryptonote.a`. Pure Go packages
(`config/`, `types/`, `wire/`, `difficulty/`) remain buildable without a C
toolchain.

**Base58 uses math/big.** The CryptoNote base58 implementation converts each
8-byte block via `big.Int` arithmetic. This is correct but not optimised for
high-throughput scenarios. A future phase may replace this with a lookup-table
approach if profiling identifies it as a bottleneck.

**Difficulty coverage at 81%.** The window-capping branch in `NextDifficulty()`
that limits the window to `BlocksCount` (735) entries is not fully exercised by
the current test suite. Adding test vectors with 1,000+ block entries would
cover this path.

**Types coverage at 73.4%.** Unexported base58 helper functions have branches
that are exercised indirectly through the public API but are not fully reached
by the current test vectors. Additional edge-case address strings would improve
coverage.

**Future forks are placeholders.** HF3 through HF6 are defined with activation
height 999,999,999 on mainnet. These heights will be updated when each fork is
scheduled for activation on the live network.
