# Phase 2: CGo Crypto Bridge Design

**Date:** 2026-02-20
**Status:** Approved
**Depends on:** Phase 1 (wire serialisation) — complete

## Context

Phase 1 delivered bit-identical wire serialisation verified by genesis block hash.
Phase 2 adds cryptographic operations via CGo bridge to the upstream C++ library.

Lethean's chain lineage: CryptoNote → IntenseCoin (2017) → Lethean → Zano rebase.
The C++ codebase is "Monero messy" — inherited complexity from multiple forks. The
goal is to extract the specific crypto files needed, build a stable interface, and
make upstream Zano algorithm updates a file-swap rather than a Go rewrite.

## Architecture

```
go-blockchain/crypto/
├── CMakeLists.txt              # Builds libcryptonote.a from vendored C++
├── bridge.h                    # Stable C API (the contract)
├── bridge.cpp                  # Thin C++ → C wrappers
├── PROVENANCE.md               # Maps each file to upstream location + commit
├── upstream/                   # Vendored C++ (mirror of blockchain/src/crypto/)
│   ├── crypto-ops.c            # Ed25519 curve operations (4,473 lines, pure C)
│   ├── crypto-ops-data.c       # Precomputed curve constants (859 lines, pure C)
│   ├── crypto-ops.h
│   ├── crypto.cpp              # Key gen, derivation, key image, signatures
│   ├── crypto.h                # C++ POD type definitions
│   ├── crypto-sugar.h          # scalar_t, point_t, operator overloads
│   ├── crypto-sugar.cpp        # Point/scalar math, precomp tables
│   ├── clsag.h / clsag.cpp    # CLSAG ring signatures (4 variants)
│   ├── zarcanum.h / .cpp       # PoS proof system
│   ├── range_proofs.h          # Bulletproofs+ traits
│   ├── range_proof_bppe.h      # Bulletproofs+ implementation
│   ├── one_out_of_many_proofs.h/.cpp  # BGE proofs
│   ├── msm.h / msm.cpp        # Multi-scalar multiplication
│   ├── keccak.c / hash.c       # Hash functions
│   ├── hash-ops.h
│   ├── random.c / random.h     # RNG
│   └── ...                     # Additional files as needed
├── compat/
│   └── crypto_config.h         # Extracted hardfork enums (subset of currency_core)
├── crypto.go                   # CGo link directives
├── keygen.go                   # Key generation, derivation
├── keyimage.go                 # Key image computation
├── signature.go                # Ring signatures (NLSAG, CLSAG)
├── proof.go                    # Range proofs, BGE, Zarcanum
├── crypto_test.go              # Test vectors, round-trip tests
└── doc.go                      # Package documentation
```

### Key Principles

1. **`upstream/` is the mirror** — vendored C++ stays close to Zano source
2. **`bridge.h` is the contract** — stable C API that Go talks to exclusively
3. **`PROVENANCE.md` tracks origins** — each file maps to upstream path + commit
4. **When Zano updates** — diff provenance commit vs new upstream, pull files, rebuild

### Build Flow (MLX Pattern)

```bash
go:generate cmake -S crypto -B crypto/build -DCMAKE_BUILD_TYPE=Release
go:generate cmake --build crypto/build --parallel
```

CMake compiles vendored C/C++ into `libcryptonote.a`. CGo links statically:

```go
// crypto.go
/*
#cgo CPPFLAGS: -I${SRCDIR}/upstream -I${SRCDIR}/compat
#cgo LDFLAGS: -L${SRCDIR}/build -lcryptonote -lstdc++
#include "bridge.h"
*/
import "C"
```

## C API Bridge (`bridge.h`)

All functions take raw `uint8_t*` pointers matching Go's `[32]byte` / `[64]byte`.
No C++ types leak through. Return `int` (0 = success, non-zero = error).

### Key Operations

```c
int cn_generate_keys(uint8_t pub[32], uint8_t sec[32]);
int cn_secret_to_public(const uint8_t sec[32], uint8_t pub[32]);
int cn_check_key(const uint8_t pub[32]);
```

### Key Derivation (One-Time Addresses)

```c
int cn_generate_key_derivation(const uint8_t pub[32], const uint8_t sec[32],
                               uint8_t derivation[32]);
int cn_derive_public_key(const uint8_t derivation[32], uint64_t index,
                         const uint8_t base[32], uint8_t derived[32]);
int cn_derive_secret_key(const uint8_t derivation[32], uint64_t index,
                         const uint8_t base[32], uint8_t derived[32]);
```

### Key Images

```c
int cn_generate_key_image(const uint8_t pub[32], const uint8_t sec[32],
                          uint8_t image[32]);
int cn_validate_key_image(const uint8_t image[32]);
```

### Standard Signatures

```c
int cn_generate_signature(const uint8_t hash[32], const uint8_t pub[32],
                          const uint8_t sec[32], uint8_t sig[64]);
int cn_check_signature(const uint8_t hash[32], const uint8_t pub[32],
                       const uint8_t sig[64]);
```

### Ring Signatures (NLSAG — Pre-HF4)

```c
int cn_generate_ring_signature(const uint8_t hash[32], const uint8_t image[32],
                               const uint8_t *const *pubs, size_t pubs_count,
                               const uint8_t sec[32], size_t sec_index,
                               uint8_t *sigs);
int cn_check_ring_signature(const uint8_t hash[32], const uint8_t image[32],
                            const uint8_t *const *pubs, size_t pubs_count,
                            const uint8_t *sigs);
```

### CLSAG (HF4+)

Opaque signature buffers — size returned by `cn_clsag_*_sig_size()`.

```c
size_t cn_clsag_gg_sig_size(size_t ring_size);
int cn_clsag_gg_verify(const uint8_t hash[32], const uint8_t *ring,
                       size_t ring_size, const uint8_t pseudo_out[32],
                       const uint8_t ki[32], const uint8_t *sig);

size_t cn_clsag_ggx_sig_size(size_t ring_size);
int cn_clsag_ggx_verify(/* ... */);

size_t cn_clsag_ggxxg_sig_size(size_t ring_size);
int cn_clsag_ggxxg_verify(/* ... */);
```

### Range Proofs (Bulletproofs+)

```c
int cn_bppe_verify(const uint8_t *sig, size_t sig_len,
                   const uint8_t *commitments, size_t num_commitments,
                   uint8_t proof_type);  // 0=ZC, 1=Zarcanum
```

### BGE One-Out-of-Many

```c
int cn_bge_verify(const uint8_t context[32], const uint8_t *ring,
                  size_t ring_size, const uint8_t *sig, size_t sig_len);
```

### Zarcanum PoS

```c
int cn_zarcanum_verify(const uint8_t hash[32], const uint8_t kernel[32],
                       const uint8_t *ring, size_t ring_size,
                       const uint8_t *proof, size_t proof_len);
```

### Hashing

```c
void cn_fast_hash(const uint8_t *data, size_t len, uint8_t hash[32]);
void cn_tree_hash(const uint8_t *hashes, size_t count, uint8_t root[32]);
```

## Go Bindings

Each Go file wraps a section of bridge.h, reusing existing types from `types/`:

### keygen.go

```go
func GenerateKeys() (types.PublicKey, types.SecretKey, error)
func SecretToPublic(sec types.SecretKey) (types.PublicKey, error)
func CheckKey(pub types.PublicKey) bool
func GenerateKeyDerivation(pub types.PublicKey, sec types.SecretKey) ([32]byte, error)
func DerivePublicKey(d [32]byte, index uint64, base types.PublicKey) (types.PublicKey, error)
func DeriveSecretKey(d [32]byte, index uint64, base types.SecretKey) (types.SecretKey, error)
```

### keyimage.go

```go
func GenerateKeyImage(pub types.PublicKey, sec types.SecretKey) (types.KeyImage, error)
func ValidateKeyImage(ki types.KeyImage) bool
```

### signature.go

```go
func GenerateSignature(hash types.Hash, pub types.PublicKey, sec types.SecretKey) (types.Signature, error)
func CheckSignature(hash types.Hash, pub types.PublicKey, sig types.Signature) bool
func CheckRingSignature(hash types.Hash, image types.KeyImage, pubs []types.PublicKey, sigs []types.Signature) bool
func CheckCLSAG_GG(hash types.Hash, ring []CLSAGInput, pseudoOut types.PublicKey, ki types.KeyImage, sig []byte) bool
```

### proof.go

```go
func VerifyBPPE(sig []byte, commitments []types.PublicKey, proofType ProofType) bool
func VerifyBGE(context types.Hash, ring []types.PublicKey, sig []byte) bool
func VerifyZarcanum(hash types.Hash, kernel types.Hash, ring []ZarcanumInput, proof []byte) bool
```

## Upstream File Extraction

### Source Files (~14,500 lines)

| Upstream File | Lines | Pure C | Purpose |
|--------------|-------|--------|---------|
| crypto-ops.c | 4,473 | yes | Ed25519 curve operations |
| crypto-ops-data.c | 859 | yes | Precomputed constants |
| crypto.cpp | 431 | no | Key gen, derivation, signatures |
| crypto-sugar.h | 1,452 | no | scalar_t, point_t types |
| crypto-sugar.cpp | 1,987 | no | Point/scalar math |
| clsag.h + .cpp | 1,328 | no | CLSAG ring signatures |
| zarcanum.h + .cpp | 749 | no | PoS proof system |
| range_proofs.h | 160 | no | Bulletproofs+ traits |
| range_proof_bppe.h | ~1,500 | no | Bulletproofs+ impl |
| one_out_of_many_proofs.* | 395 | no | BGE proofs |
| msm.h + .cpp | 222 | no | Multi-scalar mult |
| keccak.c | 129 | yes | Keccak-256 |
| hash.c + hash-ops.h | ~100 | yes | Hash dispatch |
| random.c + random.h | ~280 | yes | RNG |

### External Dependencies

- **crypto_config.h** — hardfork enums extracted into `compat/crypto_config.h`
- **OpenSSL** — for `random.c` (RAND_bytes). System library, not vendored.
- **Boost.Multiprecision** — used in Zarcanum for uint256/uint512 PoS math.
  Either vendor the header-only lib or replace with fixed-size C structs.
- **epee logging** — replaced with no-ops or printf in bridge.cpp

### Provenance Tracking

`PROVENANCE.md` maps every vendored file:

```
| Local Path              | Upstream                            | Commit  | Modified |
|-------------------------|-------------------------------------|---------|----------|
| upstream/crypto-ops.c   | src/crypto/crypto-ops.c             | abc1234 | no       |
| upstream/clsag.cpp      | src/crypto/clsag.cpp                | abc1234 | no       |
| compat/crypto_config.h  | src/currency_core/crypto_config.h   | abc1234 | yes      |
```

**Update workflow**: diff provenance commit vs new Zano tag, copy changed files,
rebuild, run tests. The bridge.h contract absorbs internal changes.

## Testing Strategy

### Layer 1 — Known Test Vectors

Generate keys with known seed in C++, verify Go produces same output.
Compute key derivation with known inputs, compare byte-for-byte.

### Layer 2 — Round-Trip Tests

- `GenerateKeys()` → `SecretToPublic()` → compare
- `GenerateKeyDerivation()` → `DerivePublicKey()` → output scanning
- `GenerateSignature()` → `CheckSignature()` → must pass
- `GenerateKeyImage()` → `ValidateKeyImage()` → must pass

### Layer 3 — Real Chain Data

Pull transactions from testnet via RPC. Extract ring, key image, signatures.
Verify in Go — must pass.

### Layer 4 — Cross-Validation

Run same operations in Go (CGo bridge) and C++ (test binary). Compare
outputs byte-for-byte.

### Build Constraint

Tests require built `libcryptonote.a`. Skip gracefully if not built.
CI needs a CMake build step before `go test`.

## Sub-Phases

| Sub-phase | Scope | Verification |
|-----------|-------|-------------|
| 2a | CMake build + bridge scaffold + key gen/derivation/key image | Round-trip tests |
| 2b | Standard signatures + NLSAG ring signatures | Verify testnet txs |
| 2c | CLSAG (all 4 variants) | HF4+ test vectors |
| 2d | Bulletproofs+, BGE, Zarcanum | Proof verification from chain |

Each sub-phase is a commit, testable independently.

## Design Decisions

1. **Vendor C++ with provenance** — self-contained module, easy upstream tracking
2. **Thin C API bridge** — no C++ types cross the boundary, stable contract
3. **CMake static library** — same pattern as go-mlx, well-understood build
4. **Verify-only for advanced schemes** — node needs verification first, generation
   comes in wallet phase
5. **Raw byte pointers** — Go's `[32]byte` maps directly to `uint8_t[32]`, zero-copy
6. **Opaque sig buffers for CLSAG/proofs** — variable-size, Go allocates via size functions

## References

- ADR-001: Go Shell + C++ Crypto Library
- MLX CGo pattern: `forge.lthn.ai/core/go-mlx`
- C++ source: `~/Code/LetheanNetwork/blockchain/src/crypto/`
- Zano docs: `~/Code/LetheanNetwork/zano-docs/`
