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
| `crypto/proof.go` | BPPE, BGE, Zarcanum verification stubs |
| `crypto/PROVENANCE.md` | Upstream origin mapping + update workflow |
| `crypto/crypto_test.go` | 22 tests (19 pass, 3 skipped proof stubs) |

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

## Phase 3 -- P2P Levin Protocol (Planned)

Implement the Levin binary protocol in `p2p/` for peer-to-peer communication.
Handshake, ping, timed sync, and block/transaction relay. Integrate with
`go-p2p` for connection management.

## Phase 4 -- RPC Layer (Planned)

Implement `rpc/` with daemon JSON-RPC (get_block, get_transaction, submit_block)
and wallet JSON-RPC (transfer, get_balance, get_address). Provide both client
and server implementations.

## Phase 5 -- Chain Storage and Validation (Planned)

Implement `chain/` with blockchain storage (using `go-store` for persistence),
block validation, transaction verification, and mempool management. UTXO set
tracking with output index.

## Phase 6 -- Wallet Core (Planned)

Implement `wallet/` with key management, output scanning, transaction
construction, and balance calculation. Deterministic key derivation from seed
phrases. Support for all address types.

## Phase 7 -- Consensus Rules (Planned)

Implement `consensus/` with hardfork-aware block reward calculation, fee
policy enforcement, and full block/transaction validation rules per hardfork
version.

## Phase 8 -- Mining (Planned)

PoW mining support with stratum protocol client. PoS staking with kernel
hash computation and coinstake transaction construction.

---

## Known Limitations

**v2+ transaction serialisation is stubbed.** The v0/v1 wire format is complete
and verified. The v2+ (Zarcanum) code paths compile but are untested -- they
will be validated in Phase 2 when post-HF4 transactions appear on-chain.

**Proof verification not yet wired.** Key generation, derivation, signatures
(standard, NLSAG, CLSAG GG) are fully operational. BPPE, BGE, and Zarcanum
proof verification stubs exist but return "not implemented" -- the deserialisation
of on-chain proof blobs requires matching the exact binary format from chain data
(Phase 4). CLSAG GGX/GGXXG verify functions are wired but untested without real
ring data.

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
