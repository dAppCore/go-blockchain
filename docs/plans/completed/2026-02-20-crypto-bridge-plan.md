# Phase 2: CGo Crypto Bridge Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Bridge Go to the upstream C++ CryptoNote crypto library via CGo, providing key generation, derivation, key images, signatures (standard, NLSAG, CLSAG), range proofs, and Zarcanum verification.

**Architecture:** Vendor upstream C++ files into `crypto/upstream/`, build as a static library via CMake, expose through a thin C API (`bridge.h`). Go bindings call only bridge.h. Provenance tracking enables easy upstream Zano updates.

**Tech Stack:** Go 1.25+, CGo, CMake, C++17, OpenSSL (system), Boost.Multiprecision (header-only, vendored)

**Upstream repo:** `~/Code/LetheanNetwork/blockchain/` at commit `fa1608cf`

---

## Task 1: Create Directory Structure and Extract Upstream Files

**Files:**
- Create: `crypto/upstream/` (directory)
- Create: `crypto/compat/` (directory)
- Create: `crypto/PROVENANCE.md`

**Step 1: Create directory structure**

```bash
cd /home/claude/Code/core/go-blockchain
mkdir -p crypto/upstream crypto/compat crypto/build
```

**Step 2: Copy pure C files from upstream**

These are bit-identical copies — no modifications needed.

```bash
UPSTREAM=~/Code/LetheanNetwork/blockchain/src/crypto
cp $UPSTREAM/crypto-ops.c      crypto/upstream/
cp $UPSTREAM/crypto-ops-data.c crypto/upstream/
cp $UPSTREAM/crypto-ops.h      crypto/upstream/
cp $UPSTREAM/keccak.c          crypto/upstream/
cp $UPSTREAM/hash.c            crypto/upstream/
cp $UPSTREAM/hash-ops.h        crypto/upstream/
cp $UPSTREAM/random.c          crypto/upstream/
cp $UPSTREAM/random.h          crypto/upstream/
cp $UPSTREAM/blake2.h          crypto/upstream/
cp $UPSTREAM/blake2b-ref.c     crypto/upstream/
```

**Step 3: Copy C++ files from upstream**

These are also bit-identical copies. The compat layer handles include path differences.

```bash
cp $UPSTREAM/crypto.h          crypto/upstream/
cp $UPSTREAM/crypto.cpp        crypto/upstream/
cp $UPSTREAM/crypto-sugar.h    crypto/upstream/
cp $UPSTREAM/crypto-sugar.cpp  crypto/upstream/
cp $UPSTREAM/clsag.h           crypto/upstream/
cp $UPSTREAM/clsag.cpp         crypto/upstream/
cp $UPSTREAM/zarcanum.h        crypto/upstream/
cp $UPSTREAM/zarcanum.cpp      crypto/upstream/
cp $UPSTREAM/range_proofs.h    crypto/upstream/
cp $UPSTREAM/range_proof_bppe.h crypto/upstream/
cp $UPSTREAM/one_out_of_many_proofs.h  crypto/upstream/
cp $UPSTREAM/one_out_of_many_proofs.cpp crypto/upstream/
cp $UPSTREAM/msm.h             crypto/upstream/
cp $UPSTREAM/msm.cpp           crypto/upstream/
cp $UPSTREAM/generic-ops.h     crypto/upstream/
cp $UPSTREAM/eth_signature.h   crypto/upstream/
cp $UPSTREAM/eth_signature.cpp crypto/upstream/
cp $UPSTREAM/RIPEMD160.c       crypto/upstream/
cp $UPSTREAM/RIPEMD160.h       crypto/upstream/
cp $UPSTREAM/RIPEMD160_helper.h  crypto/upstream/
cp $UPSTREAM/RIPEMD160_helper.cpp crypto/upstream/
```

**Step 4: Copy external dependency files**

```bash
# crypto_config.h — hash domain separation constants (55 lines, pure #defines)
cp ~/Code/LetheanNetwork/blockchain/src/currency_core/crypto_config.h crypto/compat/currency_core/

# common/pod-class.h — trivial: #define POD_CLASS struct
mkdir -p crypto/compat/common
cp ~/Code/LetheanNetwork/blockchain/src/common/pod-class.h crypto/compat/common/

# common/varint.h — header-only template varint (used by crypto.cpp)
cp ~/Code/LetheanNetwork/blockchain/src/common/varint.h crypto/compat/common/
```

Note: Create `compat/currency_core/` directory first: `mkdir -p crypto/compat/currency_core`

**Step 5: Write PROVENANCE.md**

Create `crypto/PROVENANCE.md` with this content:

```markdown
# Provenance

Vendored from `~/Code/LetheanNetwork/blockchain/` at commit `fa1608cf`.

## Upstream Files (unmodified copies)

| Local Path | Upstream Path | Modified |
|-----------|---------------|----------|
| upstream/crypto-ops.c | src/crypto/crypto-ops.c | no |
| upstream/crypto-ops-data.c | src/crypto/crypto-ops-data.c | no |
| upstream/crypto-ops.h | src/crypto/crypto-ops.h | no |
| upstream/crypto.cpp | src/crypto/crypto.cpp | no |
| upstream/crypto.h | src/crypto/crypto.h | no |
| upstream/crypto-sugar.h | src/crypto/crypto-sugar.h | no |
| upstream/crypto-sugar.cpp | src/crypto/crypto-sugar.cpp | no |
| upstream/clsag.h | src/crypto/clsag.h | no |
| upstream/clsag.cpp | src/crypto/clsag.cpp | no |
| upstream/zarcanum.h | src/crypto/zarcanum.h | no |
| upstream/zarcanum.cpp | src/crypto/zarcanum.cpp | no |
| upstream/range_proofs.h | src/crypto/range_proofs.h | no |
| upstream/range_proof_bppe.h | src/crypto/range_proof_bppe.h | no |
| upstream/one_out_of_many_proofs.h | src/crypto/one_out_of_many_proofs.h | no |
| upstream/one_out_of_many_proofs.cpp | src/crypto/one_out_of_many_proofs.cpp | no |
| upstream/msm.h | src/crypto/msm.h | no |
| upstream/msm.cpp | src/crypto/msm.cpp | no |
| upstream/keccak.c | src/crypto/keccak.c | no |
| upstream/hash.c | src/crypto/hash.c | no |
| upstream/hash-ops.h | src/crypto/hash-ops.h | no |
| upstream/random.c | src/crypto/random.c | no |
| upstream/random.h | src/crypto/random.h | no |
| upstream/blake2.h | src/crypto/blake2.h | no |
| upstream/blake2b-ref.c | src/crypto/blake2b-ref.c | no |
| upstream/generic-ops.h | src/crypto/generic-ops.h | no |
| upstream/eth_signature.h | src/crypto/eth_signature.h | no |
| upstream/eth_signature.cpp | src/crypto/eth_signature.cpp | no |
| upstream/RIPEMD160.c | src/crypto/RIPEMD160.c | no |
| upstream/RIPEMD160.h | src/crypto/RIPEMD160.h | no |
| upstream/RIPEMD160_helper.h | src/crypto/RIPEMD160_helper.h | no |
| upstream/RIPEMD160_helper.cpp | src/crypto/RIPEMD160_helper.cpp | no |

## Compat Files (extracted subsets or stubs)

| Local Path | Origin | Notes |
|-----------|--------|-------|
| compat/currency_core/crypto_config.h | src/currency_core/crypto_config.h | unmodified copy |
| compat/common/pod-class.h | src/common/pod-class.h | unmodified copy |
| compat/common/varint.h | src/common/varint.h | unmodified copy |
| compat/warnings.h | N/A | stub — no-op macros |
| compat/epee/include/misc_log_ex.h | N/A | stub — no-op logging macros |
| compat/common/crypto_stream_operators.h | N/A | stub — empty |

## Update Workflow

1. Note the current provenance commit (`fa1608cf`)
2. In the upstream repo, `git diff fa1608cf..HEAD -- src/crypto/`
3. Copy changed files into `upstream/`
4. Rebuild: `cmake --build crypto/build --parallel`
5. Run tests: `go test -race ./crypto/...`
6. Update this file with new commit hash
```

**Step 6: No commit yet — continue to Task 2**

---

## Task 2: Create Compat Layer (Stubs for External Dependencies)

**Files:**
- Create: `crypto/compat/warnings.h`
- Create: `crypto/compat/epee/include/misc_log_ex.h`
- Create: `crypto/compat/common/crypto_stream_operators.h`

The upstream C++ files include headers from outside `src/crypto/`. Rather than
pulling the full Zano tree, we provide minimal stubs. These stubs are NOT upstream
files — they're our own code.

**Step 1: Create warnings.h stub**

The upstream `contrib/epee/include/warnings.h` uses Boost.Preprocessor for
warning macros. We provide a simplified version using `_Pragma` directly.

Create `crypto/compat/warnings.h`:

```c
// SPDX-Licence-Identifier: EUPL-1.2
// Compat stub for CryptoNote warning macros.
// Replaces contrib/epee/include/warnings.h without Boost dependency.
#pragma once

#if defined(_MSC_VER)
#define PUSH_VS_WARNINGS    __pragma(warning(push))
#define POP_VS_WARNINGS     __pragma(warning(pop))
#define DISABLE_VS_WARNINGS(w) __pragma(warning(disable: w))
#define PUSH_GCC_WARNINGS
#define POP_GCC_WARNINGS
#define DISABLE_GCC_WARNING(w)
#define DISABLE_CLANG_WARNING(w)
#define DISABLE_GCC_AND_CLANG_WARNING(w)
#define ATTRIBUTE_UNUSED
#else
#define PUSH_VS_WARNINGS
#define POP_VS_WARNINGS
#define DISABLE_VS_WARNINGS(w)
#define PUSH_GCC_WARNINGS   _Pragma("GCC diagnostic push")
#define POP_GCC_WARNINGS    _Pragma("GCC diagnostic pop")
#if defined(__clang__)
#define DISABLE_GCC_WARNING(w)
#define DISABLE_CLANG_WARNING(w) _Pragma("GCC diagnostic ignored \"-W" #w "\"")
#define DISABLE_GCC_AND_CLANG_WARNING(w) _Pragma("GCC diagnostic ignored \"-W" #w "\"")
#else
#define DISABLE_GCC_WARNING(w) _Pragma("GCC diagnostic ignored \"-W" #w "\"")
#define DISABLE_CLANG_WARNING(w)
#define DISABLE_GCC_AND_CLANG_WARNING(w) _Pragma("GCC diagnostic ignored \"-W" #w "\"")
#endif
#define ATTRIBUTE_UNUSED __attribute__((unused))
#endif
```

**Step 2: Create epee logging stub**

The upstream uses `CHECK_AND_ASSERT_MES`, `LOG_PRINT_*` macros from epee.
We provide no-op stubs.

Create `crypto/compat/epee/include/misc_log_ex.h`:

```c
// SPDX-Licence-Identifier: EUPL-1.2
// Compat stub for epee logging macros.
#pragma once

#include <cassert>

#define LOG_PRINT_RED(msg, level) ((void)0)
#define LOG_PRINT_L0(msg) ((void)0)
#define LOG_PRINT_L1(msg) ((void)0)
#define LOG_PRINT_L2(msg) ((void)0)
#define LOG_PRINT_L3(msg) ((void)0)
#define LOG_ERROR(msg) ((void)0)

#define CHECK_AND_ASSERT_MES(cond, ret, msg) \
  do { if (!(cond)) { return (ret); } } while(0)

#define CHECK_AND_ASSERT_MES_NO_RET(cond, msg) \
  do { if (!(cond)) { return; } } while(0)

#define ASSERT_MES_AND_THROW(msg) do { assert(false && (msg)); } while(0)
```

**Step 3: Create crypto_stream_operators.h stub**

Used by `zarcanum.cpp` — provides `operator<<` for crypto types. Not needed for
our bridge since we never stream crypto types. Empty stub.

Create `crypto/compat/common/crypto_stream_operators.h`:

```cpp
// SPDX-Licence-Identifier: EUPL-1.2
// Compat stub — stream operators not needed for CGo bridge.
#pragma once
```

**Step 4: Create include_base_utils.h stub (if needed)**

Some files may transitively include this. Create a minimal stub:

Create `crypto/compat/include_base_utils.h`:

```cpp
// SPDX-Licence-Identifier: EUPL-1.2
// Compat stub for include_base_utils.h
#pragma once
```

**Step 5: No commit yet — continue to Task 3**

---

## Task 3: Write CMakeLists.txt and Verify Library Builds

**Files:**
- Create: `crypto/CMakeLists.txt`

**Step 1: Write CMakeLists.txt**

Create `crypto/CMakeLists.txt`:

```cmake
# SPDX-Licence-Identifier: EUPL-1.2
#
# Builds libcryptonote.a from vendored upstream C/C++ sources.
# This is the build-system half of the CGo bridge.
#
cmake_minimum_required(VERSION 3.20)
project(cryptonote C CXX)

set(CMAKE_C_STANDARD 11)
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_POSITION_INDEPENDENT_CODE ON)

# Include paths: upstream sources + compat stubs
include_directories(
    ${CMAKE_CURRENT_SOURCE_DIR}/upstream
    ${CMAKE_CURRENT_SOURCE_DIR}/compat
)

# --- Pure C sources (no dependencies beyond themselves) ---
set(C_SOURCES
    upstream/crypto-ops.c
    upstream/crypto-ops-data.c
    upstream/keccak.c
    upstream/hash.c
    upstream/random.c
    upstream/blake2b-ref.c
    upstream/RIPEMD160.c
)

# --- C++ sources (depend on compat layer + Boost headers) ---
set(CXX_SOURCES
    upstream/crypto.cpp
    upstream/crypto-sugar.cpp
    upstream/clsag.cpp
    upstream/zarcanum.cpp
    upstream/one_out_of_many_proofs.cpp
    upstream/msm.cpp
    upstream/eth_signature.cpp
    upstream/RIPEMD160_helper.cpp
    bridge.cpp
)

# --- Find system dependencies ---
find_package(OpenSSL REQUIRED)

# Boost.Multiprecision — header-only, check if system Boost is available
find_package(Boost QUIET COMPONENTS headers)
if(NOT Boost_FOUND)
    message(STATUS "System Boost not found — set BOOST_ROOT if needed")
endif()

# --- Static library ---
add_library(cryptonote STATIC ${C_SOURCES} ${CXX_SOURCES})

target_include_directories(cryptonote PRIVATE
    ${CMAKE_CURRENT_SOURCE_DIR}/upstream
    ${CMAKE_CURRENT_SOURCE_DIR}/compat
    ${OPENSSL_INCLUDE_DIR}
)

target_link_libraries(cryptonote PRIVATE
    OpenSSL::Crypto
)

if(Boost_FOUND)
    target_link_libraries(cryptonote PRIVATE Boost::headers)
endif()

# Suppress warnings from upstream code (we don't modify it)
target_compile_options(cryptonote PRIVATE
    -Wno-unused-variable
    -Wno-unused-function
    -Wno-sign-compare
    -Wno-unused-parameter
)
```

**Step 2: Create minimal bridge.cpp (scaffold only)**

Create `crypto/bridge.cpp` — just enough to compile:

```cpp
// SPDX-Licence-Identifier: EUPL-1.2
// Thin C wrappers around CryptoNote C++ crypto library.
// This is the implementation of bridge.h.

#include "bridge.h"

#include <cstring>
#include "crypto.h"
#include "hash.h"

extern "C" {

// Placeholder — will be implemented in subsequent tasks.

void cn_fast_hash(const uint8_t *data, size_t len, uint8_t hash[32]) {
    crypto::cn_fast_hash(data, len, reinterpret_cast<char*>(hash));
}

} // extern "C"
```

**Step 3: Create minimal bridge.h (scaffold only)**

Create `crypto/bridge.h`:

```c
// SPDX-Licence-Identifier: EUPL-1.2
// Stable C API for go-blockchain CGo bindings.
// Go code calls ONLY these functions — no C++ types cross this boundary.
#ifndef CRYPTONOTE_BRIDGE_H
#define CRYPTONOTE_BRIDGE_H

#include <stdint.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

// ── Hashing ───────────────────────────────────────────────
void cn_fast_hash(const uint8_t *data, size_t len, uint8_t hash[32]);

#ifdef __cplusplus
}
#endif
#endif // CRYPTONOTE_BRIDGE_H
```

**Step 4: Build the library**

```bash
cd /home/claude/Code/core/go-blockchain
cmake -S crypto -B crypto/build -DCMAKE_BUILD_TYPE=Release 2>&1
cmake --build crypto/build --parallel 2>&1
```

Expected: `libcryptonote.a` appears in `crypto/build/`.

This step will likely fail on the first attempt due to missing includes or
compilation errors in the upstream C++. Iterate:
1. Read the error message
2. Add the missing compat stub or fix the include path
3. Rebuild

Common issues to expect:
- Missing `warnings.h` — should be resolved by `-I compat/`
- Missing `common/pod-class.h` — should be resolved by `-I compat/`
- Missing `auto_val_init.h` — may need another stub
- Boost.Multiprecision not found — install `libboost-dev` or provide path
- OpenSSL not found — install `libssl-dev`

**Step 5: Verify .a file exists**

```bash
ls -la crypto/build/libcryptonote.a
```

Expected: static archive file exists.

**Step 6: Commit**

```bash
git add crypto/
git commit -m "feat(crypto): Phase 2a scaffold — vendored C++ and CMake build

Extract CryptoNote crypto sources from upstream (fa1608cf).
Build as static libcryptonote.a via CMake.
Compat stubs for external dependencies (warnings, logging, varint).

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 4: Go CGo Link + Hash Smoke Test

**Files:**
- Create: `crypto/doc.go`
- Create: `crypto/crypto.go`
- Create: `crypto/crypto_test.go`

**Step 1: Write the failing test**

Create `crypto/crypto_test.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package crypto_test

import (
    "encoding/hex"
    "testing"

    "forge.lthn.ai/core/go-blockchain/crypto"
)

func TestFastHash_Good_KnownVector(t *testing.T) {
    // Empty input → known Keccak-256 hash.
    // Python: hashlib.new('keccak-256', b'').hexdigest()
    input := []byte{}
    expected := "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"

    got := crypto.FastHash(input)
    if hex.EncodeToString(got[:]) != expected {
        t.Fatalf("FastHash(empty)\n  got:  %x\n  want: %s", got, expected)
    }
}

func TestFastHash_Good_HelloWorld(t *testing.T) {
    input := []byte("Hello, World!")
    got := crypto.FastHash(input)
    // Non-zero, 32 bytes.
    var zero [32]byte
    if got == zero {
        t.Fatal("FastHash returned zero hash")
    }
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/claude/Code/core/go-blockchain
go test -v -run TestFastHash ./crypto/...
```

Expected: FAIL — `crypto` package doesn't exist yet (or functions missing).

**Step 3: Write doc.go with CGo link directives**

Create `crypto/doc.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

// Package crypto provides CryptoNote cryptographic operations via CGo
// bridge to the vendored upstream C++ library.
//
// Build the C++ library before running tests:
//
//   cmake -S crypto -B crypto/build -DCMAKE_BUILD_TYPE=Release
//   cmake --build crypto/build --parallel
package crypto
```

**Step 4: Write crypto.go with CGo bindings**

Create `crypto/crypto.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package crypto

/*
#cgo CPPFLAGS: -I${SRCDIR}/upstream -I${SRCDIR}/compat
#cgo LDFLAGS: -L${SRCDIR}/build -lcryptonote -lstdc++ -lssl -lcrypto
#include "bridge.h"
*/
import "C"

import "unsafe"

// FastHash computes the CryptoNote fast hash (Keccak-256) of the input.
func FastHash(data []byte) [32]byte {
    var hash [32]byte
    if len(data) == 0 {
        C.cn_fast_hash(nil, 0, (*C.uint8_t)(unsafe.Pointer(&hash[0])))
    } else {
        C.cn_fast_hash((*C.uint8_t)(unsafe.Pointer(&data[0])),
            C.size_t(len(data)),
            (*C.uint8_t)(unsafe.Pointer(&hash[0])))
    }
    return hash
}
```

**Step 5: Run test to verify it passes**

```bash
cd /home/claude/Code/core/go-blockchain
go test -v -run TestFastHash ./crypto/...
```

Expected: PASS. If it fails with linker errors, check:
- `crypto/build/libcryptonote.a` exists
- CGo flags point to correct paths
- OpenSSL is installed (`apt list --installed 2>/dev/null | grep libssl-dev`)

**Step 6: Commit**

```bash
git add crypto/doc.go crypto/crypto.go crypto/crypto_test.go
git commit -m "feat(crypto): CGo bridge smoke test — FastHash via Keccak-256

Verify CGo link to libcryptonote.a works with a known-vector hash test.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 5: Key Generation and Validation

**Files:**
- Create: `crypto/keygen.go`
- Modify: `crypto/bridge.h` (add key ops)
- Modify: `crypto/bridge.cpp` (add key ops)
- Modify: `crypto/crypto_test.go` (add key tests)

**Step 1: Write the failing tests**

Add to `crypto/crypto_test.go`:

```go
func TestGenerateKeys_Good_Roundtrip(t *testing.T) {
    pub, sec, err := crypto.GenerateKeys()
    if err != nil {
        t.Fatalf("GenerateKeys: %v", err)
    }

    // Public key should be valid.
    if !crypto.CheckKey(pub) {
        t.Fatal("generated public key failed CheckKey")
    }

    // Deriving public from secret should match.
    pub2, err := crypto.SecretToPublic(sec)
    if err != nil {
        t.Fatalf("SecretToPublic: %v", err)
    }
    if pub != pub2 {
        t.Fatalf("SecretToPublic mismatch:\n  GenerateKeys: %x\n  SecretToPublic: %x", pub, pub2)
    }
}

func TestCheckKey_Bad_Zero(t *testing.T) {
    var zero [32]byte
    if crypto.CheckKey(zero) {
        t.Fatal("zero key should fail CheckKey")
    }
}

func TestGenerateKeys_Good_Unique(t *testing.T) {
    pub1, _, _ := crypto.GenerateKeys()
    pub2, _, _ := crypto.GenerateKeys()
    if pub1 == pub2 {
        t.Fatal("two GenerateKeys calls returned identical public keys")
    }
}
```

**Step 2: Run tests — should fail (functions don't exist)**

```bash
go test -v -run TestGenerateKeys ./crypto/... 2>&1
go test -v -run TestCheckKey ./crypto/... 2>&1
```

**Step 3: Add key ops to bridge.h**

Add before the `#ifdef __cplusplus` closing:

```c
// ── Key Operations ────────────────────────────────────────
int cn_generate_keys(uint8_t pub[32], uint8_t sec[32]);
int cn_secret_to_public(const uint8_t sec[32], uint8_t pub[32]);
int cn_check_key(const uint8_t pub[32]);
```

**Step 4: Implement in bridge.cpp**

Add to `bridge.cpp`:

```cpp
int cn_generate_keys(uint8_t pub[32], uint8_t sec[32]) {
    crypto::public_key pk;
    crypto::secret_key sk;
    crypto::generate_keys(pk, sk);
    memcpy(pub, &pk, 32);
    memcpy(sec, &sk, 32);
    return 0;
}

int cn_secret_to_public(const uint8_t sec[32], uint8_t pub[32]) {
    crypto::secret_key sk;
    crypto::public_key pk;
    memcpy(&sk, sec, 32);
    bool ok = crypto::secret_key_to_public_key(sk, pk);
    if (!ok) return 1;
    memcpy(pub, &pk, 32);
    return 0;
}

int cn_check_key(const uint8_t pub[32]) {
    crypto::public_key pk;
    memcpy(&pk, pub, 32);
    return crypto::check_key(pk) ? 0 : 1;
}
```

**Step 5: Write Go bindings**

Create `crypto/keygen.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package crypto

/*
#include "bridge.h"
*/
import "C"

import (
    "fmt"
    "unsafe"
)

// GenerateKeys creates a new random key pair.
func GenerateKeys() (pub [32]byte, sec [32]byte, err error) {
    rc := C.cn_generate_keys(
        (*C.uint8_t)(unsafe.Pointer(&pub[0])),
        (*C.uint8_t)(unsafe.Pointer(&sec[0])),
    )
    if rc != 0 {
        err = fmt.Errorf("crypto: generate_keys failed (rc=%d)", rc)
    }
    return
}

// SecretToPublic derives the public key from a secret key.
func SecretToPublic(sec [32]byte) ([32]byte, error) {
    var pub [32]byte
    rc := C.cn_secret_to_public(
        (*C.uint8_t)(unsafe.Pointer(&sec[0])),
        (*C.uint8_t)(unsafe.Pointer(&pub[0])),
    )
    if rc != 0 {
        return pub, fmt.Errorf("crypto: secret_to_public failed (rc=%d)", rc)
    }
    return pub, nil
}

// CheckKey validates that a public key is a valid curve point.
func CheckKey(pub [32]byte) bool {
    return C.cn_check_key((*C.uint8_t)(unsafe.Pointer(&pub[0]))) == 0
}
```

**Step 6: Rebuild library and run tests**

```bash
cmake --build crypto/build --parallel 2>&1
go test -v -run "TestGenerateKeys|TestCheckKey" ./crypto/...
```

Expected: all PASS.

**Step 7: Commit**

```bash
git add crypto/keygen.go crypto/bridge.h crypto/bridge.cpp crypto/crypto_test.go
git commit -m "feat(crypto): key generation, validation, and secret-to-public derivation

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 6: Key Derivation (One-Time Addresses)

**Files:**
- Modify: `crypto/keygen.go` (add derivation functions)
- Modify: `crypto/bridge.h` (add derivation ops)
- Modify: `crypto/bridge.cpp` (add derivation ops)
- Modify: `crypto/crypto_test.go` (add derivation tests)

**Step 1: Write the failing tests**

```go
func TestKeyDerivation_Good_Roundtrip(t *testing.T) {
    // Simulate: sender generates tx, receiver scans outputs.
    // Sender: derivation = ECDH(receiver_view_pub, tx_secret)
    // Receiver: derivation = ECDH(tx_public, receiver_view_sec)
    // Both derivations must be identical.

    viewPub, viewSec, _ := crypto.GenerateKeys()
    txPub, txSec, _ := crypto.GenerateKeys()

    // Sender side.
    d1, err := crypto.GenerateKeyDerivation(viewPub, txSec)
    if err != nil {
        t.Fatalf("sender derivation: %v", err)
    }

    // Receiver side.
    d2, err := crypto.GenerateKeyDerivation(txPub, viewSec)
    if err != nil {
        t.Fatalf("receiver derivation: %v", err)
    }

    if d1 != d2 {
        t.Fatalf("ECDH derivation mismatch:\n  sender:   %x\n  receiver: %x", d1, d2)
    }
}

func TestDerivePublicKey_Good_OutputScanning(t *testing.T) {
    // Generate receiver keys.
    spendPub, spendSec, _ := crypto.GenerateKeys()
    viewPub, viewSec, _ := crypto.GenerateKeys()
    txPub, txSec, _ := crypto.GenerateKeys()

    // Sender creates output for receiver at index 0.
    d, _ := crypto.GenerateKeyDerivation(viewPub, txSec)
    ephPub, err := crypto.DerivePublicKey(d, 0, spendPub)
    if err != nil {
        t.Fatalf("DerivePublicKey: %v", err)
    }

    // Receiver derives the same ephemeral public key.
    d2, _ := crypto.GenerateKeyDerivation(txPub, viewSec)
    ephPub2, _ := crypto.DerivePublicKey(d2, 0, spendPub)
    if ephPub != ephPub2 {
        t.Fatal("ephemeral public key mismatch between sender and receiver")
    }

    // Receiver derives the corresponding secret key.
    ephSec, err := crypto.DeriveSecretKey(d2, 0, spendSec)
    if err != nil {
        t.Fatalf("DeriveSecretKey: %v", err)
    }

    // Verify: SecretToPublic(ephSec) == ephPub.
    derivedPub, _ := crypto.SecretToPublic(ephSec)
    if derivedPub != ephPub {
        t.Fatalf("SecretToPublic(ephSec) != ephPub:\n  got:  %x\n  want: %x", derivedPub, ephPub)
    }
}
```

**Step 2: Add to bridge.h**

```c
// ── Key Derivation ────────────────────────────────────────
int cn_generate_key_derivation(const uint8_t pub[32], const uint8_t sec[32],
                               uint8_t derivation[32]);
int cn_derive_public_key(const uint8_t derivation[32], uint64_t index,
                         const uint8_t base[32], uint8_t derived[32]);
int cn_derive_secret_key(const uint8_t derivation[32], uint64_t index,
                         const uint8_t base[32], uint8_t derived[32]);
```

**Step 3: Implement in bridge.cpp**

```cpp
int cn_generate_key_derivation(const uint8_t pub[32], const uint8_t sec[32],
                               uint8_t derivation[32]) {
    crypto::public_key pk;
    crypto::secret_key sk;
    crypto::key_derivation kd;
    memcpy(&pk, pub, 32);
    memcpy(&sk, sec, 32);
    bool ok = crypto::generate_key_derivation(pk, sk, kd);
    if (!ok) return 1;
    memcpy(derivation, &kd, 32);
    return 0;
}

int cn_derive_public_key(const uint8_t derivation[32], uint64_t index,
                         const uint8_t base[32], uint8_t derived[32]) {
    crypto::key_derivation kd;
    crypto::public_key base_pk, derived_pk;
    memcpy(&kd, derivation, 32);
    memcpy(&base_pk, base, 32);
    bool ok = crypto::derive_public_key(kd, index, base_pk, derived_pk);
    if (!ok) return 1;
    memcpy(derived, &derived_pk, 32);
    return 0;
}

int cn_derive_secret_key(const uint8_t derivation[32], uint64_t index,
                         const uint8_t base[32], uint8_t derived[32]) {
    crypto::key_derivation kd;
    crypto::secret_key base_sk, derived_sk;
    memcpy(&kd, derivation, 32);
    memcpy(&base_sk, base, 32);
    crypto::derive_secret_key(kd, index, base_sk, derived_sk);
    memcpy(derived, &derived_sk, 32);
    return 0;
}
```

**Step 4: Add Go bindings to keygen.go**

```go
// GenerateKeyDerivation computes the ECDH shared secret (key derivation).
func GenerateKeyDerivation(pub [32]byte, sec [32]byte) ([32]byte, error) {
    var d [32]byte
    rc := C.cn_generate_key_derivation(
        (*C.uint8_t)(unsafe.Pointer(&pub[0])),
        (*C.uint8_t)(unsafe.Pointer(&sec[0])),
        (*C.uint8_t)(unsafe.Pointer(&d[0])),
    )
    if rc != 0 {
        return d, fmt.Errorf("crypto: generate_key_derivation failed")
    }
    return d, nil
}

// DerivePublicKey derives an ephemeral public key for a transaction output.
func DerivePublicKey(derivation [32]byte, index uint64, base [32]byte) ([32]byte, error) {
    var derived [32]byte
    rc := C.cn_derive_public_key(
        (*C.uint8_t)(unsafe.Pointer(&derivation[0])),
        C.uint64_t(index),
        (*C.uint8_t)(unsafe.Pointer(&base[0])),
        (*C.uint8_t)(unsafe.Pointer(&derived[0])),
    )
    if rc != 0 {
        return derived, fmt.Errorf("crypto: derive_public_key failed")
    }
    return derived, nil
}

// DeriveSecretKey derives the ephemeral secret key for a received output.
func DeriveSecretKey(derivation [32]byte, index uint64, base [32]byte) ([32]byte, error) {
    var derived [32]byte
    rc := C.cn_derive_secret_key(
        (*C.uint8_t)(unsafe.Pointer(&derivation[0])),
        C.uint64_t(index),
        (*C.uint8_t)(unsafe.Pointer(&base[0])),
        (*C.uint8_t)(unsafe.Pointer(&derived[0])),
    )
    if rc != 0 {
        return derived, fmt.Errorf("crypto: derive_secret_key failed")
    }
    return derived, nil
}
```

**Step 5: Rebuild and test**

```bash
cmake --build crypto/build --parallel 2>&1
go test -v -run "TestKeyDerivation|TestDerivePublicKey" ./crypto/...
```

Expected: PASS.

**Step 6: Commit**

```bash
git add crypto/
git commit -m "feat(crypto): key derivation for one-time addresses

ECDH shared secret, ephemeral public/secret key derivation.
Round-trip tested: sender and receiver produce identical derivations.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 7: Key Images

**Files:**
- Create: `crypto/keyimage.go`
- Modify: `crypto/bridge.h`
- Modify: `crypto/bridge.cpp`
- Modify: `crypto/crypto_test.go`

**Step 1: Write the failing tests**

```go
func TestKeyImage_Good_Roundtrip(t *testing.T) {
    pub, sec, _ := crypto.GenerateKeys()

    ki, err := crypto.GenerateKeyImage(pub, sec)
    if err != nil {
        t.Fatalf("GenerateKeyImage: %v", err)
    }

    // Key image must be non-zero.
    var zero [32]byte
    if ki == zero {
        t.Fatal("key image is all zeros")
    }

    // Key image must be valid.
    if !crypto.ValidateKeyImage(ki) {
        t.Fatal("generated key image failed validation")
    }
}

func TestKeyImage_Good_Deterministic(t *testing.T) {
    pub, sec, _ := crypto.GenerateKeys()

    ki1, _ := crypto.GenerateKeyImage(pub, sec)
    ki2, _ := crypto.GenerateKeyImage(pub, sec)

    if ki1 != ki2 {
        t.Fatal("same keys should produce same key image")
    }
}

func TestKeyImage_Bad_Zero(t *testing.T) {
    var zero [32]byte
    if crypto.ValidateKeyImage(zero) {
        t.Fatal("zero key image should fail validation")
    }
}
```

**Step 2: Add to bridge.h**

```c
// ── Key Images ────────────────────────────────────────────
int cn_generate_key_image(const uint8_t pub[32], const uint8_t sec[32],
                          uint8_t image[32]);
int cn_validate_key_image(const uint8_t image[32]);
```

**Step 3: Implement in bridge.cpp**

```cpp
int cn_generate_key_image(const uint8_t pub[32], const uint8_t sec[32],
                          uint8_t image[32]) {
    crypto::public_key pk;
    crypto::secret_key sk;
    crypto::key_image ki;
    memcpy(&pk, pub, 32);
    memcpy(&sk, sec, 32);
    crypto::generate_key_image(pk, sk, ki);
    memcpy(image, &ki, 32);
    return 0;
}

int cn_validate_key_image(const uint8_t image[32]) {
    crypto::key_image ki;
    memcpy(&ki, image, 32);
    return crypto::validate_key_image(ki) ? 0 : 1;
}
```

**Step 4: Write keyimage.go**

```go
// SPDX-Licence-Identifier: EUPL-1.2

package crypto

/*
#include "bridge.h"
*/
import "C"

import (
    "fmt"
    "unsafe"
)

// GenerateKeyImage computes the key image for a public/secret key pair.
// The key image is used for double-spend detection in ring signatures.
func GenerateKeyImage(pub [32]byte, sec [32]byte) ([32]byte, error) {
    var ki [32]byte
    rc := C.cn_generate_key_image(
        (*C.uint8_t)(unsafe.Pointer(&pub[0])),
        (*C.uint8_t)(unsafe.Pointer(&sec[0])),
        (*C.uint8_t)(unsafe.Pointer(&ki[0])),
    )
    if rc != 0 {
        return ki, fmt.Errorf("crypto: generate_key_image failed")
    }
    return ki, nil
}

// ValidateKeyImage checks that a key image is a valid curve point of the correct order.
func ValidateKeyImage(ki [32]byte) bool {
    return C.cn_validate_key_image((*C.uint8_t)(unsafe.Pointer(&ki[0]))) == 0
}
```

**Step 5: Rebuild and test**

```bash
cmake --build crypto/build --parallel 2>&1
go test -v -run TestKeyImage ./crypto/...
```

Expected: PASS.

**Step 6: Commit sub-phase 2a**

```bash
git add crypto/
go test -race ./crypto/... 2>&1
go vet ./crypto/... 2>&1
git commit -m "feat(crypto): Phase 2a complete — key gen, derivation, key images

Sub-phase 2a delivers the minimum crypto for wallet output scanning:
- Key pair generation and validation
- ECDH key derivation for one-time addresses
- Key image generation and validation
- FastHash (Keccak-256) via CGo bridge

All round-trip tested. Race-free.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 8: Standard Signatures

**Files:**
- Create: `crypto/signature.go`
- Modify: `crypto/bridge.h`
- Modify: `crypto/bridge.cpp`
- Modify: `crypto/crypto_test.go`

**Step 1: Write the failing tests**

```go
func TestSignature_Good_Roundtrip(t *testing.T) {
    pub, sec, _ := crypto.GenerateKeys()

    // Sign a message hash.
    msg := crypto.FastHash([]byte("test message"))
    sig, err := crypto.GenerateSignature(msg, pub, sec)
    if err != nil {
        t.Fatalf("GenerateSignature: %v", err)
    }

    // Verify with correct key.
    if !crypto.CheckSignature(msg, pub, sig) {
        t.Fatal("valid signature failed verification")
    }
}

func TestSignature_Bad_WrongKey(t *testing.T) {
    pub, sec, _ := crypto.GenerateKeys()
    pub2, _, _ := crypto.GenerateKeys()

    msg := crypto.FastHash([]byte("test"))
    sig, _ := crypto.GenerateSignature(msg, pub, sec)

    // Verify with wrong public key should fail.
    if crypto.CheckSignature(msg, pub2, sig) {
        t.Fatal("signature verified with wrong public key")
    }
}

func TestSignature_Bad_WrongMessage(t *testing.T) {
    pub, sec, _ := crypto.GenerateKeys()

    msg1 := crypto.FastHash([]byte("message 1"))
    msg2 := crypto.FastHash([]byte("message 2"))
    sig, _ := crypto.GenerateSignature(msg1, pub, sec)

    if crypto.CheckSignature(msg2, pub, sig) {
        t.Fatal("signature verified with wrong message")
    }
}
```

**Step 2: Add to bridge.h**

```c
// ── Standard Signatures ───────────────────────────────────
int cn_generate_signature(const uint8_t hash[32], const uint8_t pub[32],
                          const uint8_t sec[32], uint8_t sig[64]);
int cn_check_signature(const uint8_t hash[32], const uint8_t pub[32],
                       const uint8_t sig[64]);
```

**Step 3: Implement in bridge.cpp**

```cpp
int cn_generate_signature(const uint8_t hash[32], const uint8_t pub[32],
                          const uint8_t sec[32], uint8_t sig[64]) {
    crypto::hash h;
    crypto::public_key pk;
    crypto::secret_key sk;
    crypto::signature s;
    memcpy(&h, hash, 32);
    memcpy(&pk, pub, 32);
    memcpy(&sk, sec, 32);
    crypto::generate_signature(h, pk, sk, s);
    memcpy(sig, &s, 64);
    return 0;
}

int cn_check_signature(const uint8_t hash[32], const uint8_t pub[32],
                       const uint8_t sig[64]) {
    crypto::hash h;
    crypto::public_key pk;
    crypto::signature s;
    memcpy(&h, hash, 32);
    memcpy(&pk, pub, 32);
    memcpy(&s, sig, 64);
    return crypto::check_signature(h, pk, s) ? 0 : 1;
}
```

**Step 4: Write signature.go**

```go
// SPDX-Licence-Identifier: EUPL-1.2

package crypto

/*
#include "bridge.h"
*/
import "C"

import (
    "fmt"
    "unsafe"
)

// GenerateSignature creates a standard (non-ring) signature.
func GenerateSignature(hash [32]byte, pub [32]byte, sec [32]byte) ([64]byte, error) {
    var sig [64]byte
    rc := C.cn_generate_signature(
        (*C.uint8_t)(unsafe.Pointer(&hash[0])),
        (*C.uint8_t)(unsafe.Pointer(&pub[0])),
        (*C.uint8_t)(unsafe.Pointer(&sec[0])),
        (*C.uint8_t)(unsafe.Pointer(&sig[0])),
    )
    if rc != 0 {
        return sig, fmt.Errorf("crypto: generate_signature failed")
    }
    return sig, nil
}

// CheckSignature verifies a standard signature.
func CheckSignature(hash [32]byte, pub [32]byte, sig [64]byte) bool {
    return C.cn_check_signature(
        (*C.uint8_t)(unsafe.Pointer(&hash[0])),
        (*C.uint8_t)(unsafe.Pointer(&pub[0])),
        (*C.uint8_t)(unsafe.Pointer(&sig[0])),
    ) == 0
}
```

**Step 5: Rebuild and test**

```bash
cmake --build crypto/build --parallel 2>&1
go test -v -run TestSignature ./crypto/...
```

Expected: PASS.

**Step 6: Commit**

```bash
git add crypto/
git commit -m "feat(crypto): standard signature generation and verification

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 9: Ring Signatures (NLSAG — Pre-HF4)

**Files:**
- Modify: `crypto/signature.go` (add ring sig functions)
- Modify: `crypto/bridge.h`
- Modify: `crypto/bridge.cpp`
- Modify: `crypto/crypto_test.go`

**Step 1: Write the failing tests**

```go
func TestRingSignature_Good_Roundtrip(t *testing.T) {
    // Create a ring of 4 public keys. The real signer is at index 1.
    ringSize := 4
    realIndex := 1

    pubs := make([][32]byte, ringSize)
    var realSec [32]byte
    for i := 0; i < ringSize; i++ {
        pub, sec, _ := crypto.GenerateKeys()
        pubs[i] = pub
        if i == realIndex {
            realSec = sec
        }
    }

    // Generate key image for the real key.
    ki, _ := crypto.GenerateKeyImage(pubs[realIndex], realSec)

    // Sign.
    msg := crypto.FastHash([]byte("ring sig test"))
    sigs, err := crypto.GenerateRingSignature(msg, ki, pubs, realSec, realIndex)
    if err != nil {
        t.Fatalf("GenerateRingSignature: %v", err)
    }

    // Verify.
    if !crypto.CheckRingSignature(msg, ki, pubs, sigs) {
        t.Fatal("valid ring signature failed verification")
    }
}

func TestRingSignature_Bad_WrongMessage(t *testing.T) {
    pubs := make([][32]byte, 3)
    var sec [32]byte
    for i := range pubs {
        pub, s, _ := crypto.GenerateKeys()
        pubs[i] = pub
        if i == 0 { sec = s }
    }
    ki, _ := crypto.GenerateKeyImage(pubs[0], sec)

    msg1 := crypto.FastHash([]byte("msg1"))
    msg2 := crypto.FastHash([]byte("msg2"))
    sigs, _ := crypto.GenerateRingSignature(msg1, ki, pubs, sec, 0)

    if crypto.CheckRingSignature(msg2, ki, pubs, sigs) {
        t.Fatal("ring signature verified with wrong message")
    }
}
```

**Step 2: Add to bridge.h**

```c
// ── Ring Signatures (NLSAG) ──────────────────────────────
int cn_generate_ring_signature(const uint8_t hash[32], const uint8_t image[32],
                               const uint8_t *pubs, size_t pubs_count,
                               const uint8_t sec[32], size_t sec_index,
                               uint8_t *sigs);
int cn_check_ring_signature(const uint8_t hash[32], const uint8_t image[32],
                            const uint8_t *pubs, size_t pubs_count,
                            const uint8_t *sigs);
```

Note: `pubs` is a flat array of `pubs_count * 32` bytes. `sigs` is `pubs_count * 64` bytes.
This avoids double-pointer indirection across the CGo boundary.

**Step 3: Implement in bridge.cpp**

The C++ API uses `const public_key *const *pubs` (array of pointers). We flatten
for the C API and reconstruct the pointer array internally.

```cpp
int cn_generate_ring_signature(const uint8_t hash[32], const uint8_t image[32],
                               const uint8_t *pubs, size_t pubs_count,
                               const uint8_t sec[32], size_t sec_index,
                               uint8_t *sigs) {
    crypto::hash h;
    crypto::key_image ki;
    crypto::secret_key sk;
    memcpy(&h, hash, 32);
    memcpy(&ki, image, 32);
    memcpy(&sk, sec, 32);

    // Reconstruct pointer array from flat buffer.
    std::vector<const crypto::public_key*> pk_ptrs(pubs_count);
    std::vector<crypto::public_key> pk_storage(pubs_count);
    for (size_t i = 0; i < pubs_count; i++) {
        memcpy(&pk_storage[i], pubs + i * 32, 32);
        pk_ptrs[i] = &pk_storage[i];
    }

    std::vector<crypto::signature> sig_vec(pubs_count);
    crypto::generate_ring_signature(h, ki, pk_ptrs.data(), pubs_count,
                                    sk, sec_index, sig_vec.data());
    memcpy(sigs, sig_vec.data(), pubs_count * 64);
    return 0;
}

int cn_check_ring_signature(const uint8_t hash[32], const uint8_t image[32],
                            const uint8_t *pubs, size_t pubs_count,
                            const uint8_t *sigs) {
    crypto::hash h;
    crypto::key_image ki;
    memcpy(&h, hash, 32);
    memcpy(&ki, image, 32);

    std::vector<const crypto::public_key*> pk_ptrs(pubs_count);
    std::vector<crypto::public_key> pk_storage(pubs_count);
    for (size_t i = 0; i < pubs_count; i++) {
        memcpy(&pk_storage[i], pubs + i * 32, 32);
        pk_ptrs[i] = &pk_storage[i];
    }

    auto* sig_ptr = reinterpret_cast<const crypto::signature*>(sigs);
    return crypto::check_ring_signature(h, ki, pk_ptrs.data(), pubs_count, sig_ptr) ? 0 : 1;
}
```

**Step 4: Add Go bindings to signature.go**

```go
// GenerateRingSignature creates a ring signature using the given key ring.
// pubs contains the public keys of all ring members.
// sec is the secret key of the actual signer at position secIndex.
// Returns pubs_count signatures (one per ring member).
func GenerateRingSignature(hash [32]byte, image [32]byte, pubs [][32]byte,
    sec [32]byte, secIndex int) ([][64]byte, error) {

    n := len(pubs)
    flatPubs := make([]byte, n*32)
    for i, p := range pubs {
        copy(flatPubs[i*32:], p[:])
    }

    flatSigs := make([]byte, n*64)
    rc := C.cn_generate_ring_signature(
        (*C.uint8_t)(unsafe.Pointer(&hash[0])),
        (*C.uint8_t)(unsafe.Pointer(&image[0])),
        (*C.uint8_t)(unsafe.Pointer(&flatPubs[0])),
        C.size_t(n),
        (*C.uint8_t)(unsafe.Pointer(&sec[0])),
        C.size_t(secIndex),
        (*C.uint8_t)(unsafe.Pointer(&flatSigs[0])),
    )
    if rc != 0 {
        return nil, fmt.Errorf("crypto: generate_ring_signature failed")
    }

    sigs := make([][64]byte, n)
    for i := range sigs {
        copy(sigs[i][:], flatSigs[i*64:])
    }
    return sigs, nil
}

// CheckRingSignature verifies a ring signature.
func CheckRingSignature(hash [32]byte, image [32]byte, pubs [][32]byte,
    sigs [][64]byte) bool {

    n := len(pubs)
    if len(sigs) != n {
        return false
    }

    flatPubs := make([]byte, n*32)
    for i, p := range pubs {
        copy(flatPubs[i*32:], p[:])
    }

    flatSigs := make([]byte, n*64)
    for i, s := range sigs {
        copy(flatSigs[i*64:], s[:])
    }

    return C.cn_check_ring_signature(
        (*C.uint8_t)(unsafe.Pointer(&hash[0])),
        (*C.uint8_t)(unsafe.Pointer(&image[0])),
        (*C.uint8_t)(unsafe.Pointer(&flatPubs[0])),
        C.size_t(n),
        (*C.uint8_t)(unsafe.Pointer(&flatSigs[0])),
    ) == 0
}
```

**Step 5: Rebuild and test**

```bash
cmake --build crypto/build --parallel 2>&1
go test -v -run TestRingSignature ./crypto/...
```

Expected: PASS.

**Step 6: Commit sub-phase 2b**

```bash
git add crypto/
go test -race ./crypto/... 2>&1
git commit -m "feat(crypto): Phase 2b — standard and ring signature (NLSAG)

Standard signature generation/verification.
Ring signature (NLSAG) generation/verification with flat buffer C API.
Round-trip and negative tests pass with race detector.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 10: CLSAG Verification (All 4 Variants)

**Files:**
- Modify: `crypto/signature.go` (add CLSAG verify functions)
- Modify: `crypto/bridge.h` (add CLSAG ops)
- Modify: `crypto/bridge.cpp` (add CLSAG ops)
- Modify: `crypto/crypto_test.go` (add CLSAG tests)

This task is more complex because CLSAG uses `crypto-sugar.h` types internally.
The bridge wraps them into flat byte buffers.

**Step 1: Write the failing tests**

CLSAG verification tests need test vectors from the C++ daemon. For now, write
a basic generation + verification round-trip test for CLSAG_GG (the simplest
variant).

```go
func TestCLSAG_GG_Good_Roundtrip(t *testing.T) {
    // This test calls the C++ CLSAG generation directly via bridge,
    // then verifies. Full test vectors from chain data come in Task 12.
    t.Skip("CLSAG round-trip test — implement after bridge functions exist")
}
```

The actual CLSAG bridge functions are complex — they need to marshal ring
structures. Implementation details will emerge during bridge.cpp coding.
Document the approach and iterate.

**Step 2: Add CLSAG to bridge.h**

```c
// ── CLSAG Verification (HF4+) ────────────────────────────
// Ring entries are packed as: [stealth_addr(32) | amount_commitment(32)] per entry.
// Signatures are opaque blobs — size from cn_clsag_*_sig_size().

size_t cn_clsag_gg_sig_size(size_t ring_size);
int cn_clsag_gg_generate(const uint8_t hash[32], const uint8_t *ring,
                         size_t ring_size, const uint8_t pseudo_out[32],
                         const uint8_t ki[32], const uint8_t secret_x[32],
                         const uint8_t secret_f[32], size_t secret_index,
                         uint8_t *sig);
int cn_clsag_gg_verify(const uint8_t hash[32], const uint8_t *ring,
                       size_t ring_size, const uint8_t pseudo_out[32],
                       const uint8_t ki[32], const uint8_t *sig);
```

**Step 3: Implement in bridge.cpp**

This requires converting flat byte arrays to `CLSAG_GG_input_ref_t` vectors.
The implementation must:
1. Parse the flat ring buffer into `point_t` stealth addresses and commitments
2. Construct `std::vector<CLSAG_GG_input_ref_t>`
3. Convert scalar secrets from raw bytes to `scalar_t`
4. Call `generate_CLSAG_GG()` / `verify_CLSAG_GG()`
5. Serialise the `CLSAG_GG_signature` struct back to bytes

This is the most complex bridge function. Implementation will require reading
`clsag.h` to understand the exact struct layout and field order for serialisation.

**Step 4: Add Go bindings, rebuild, test**

Follow the same pattern as ring signatures. Flat byte buffers across the boundary.

**Step 5: Repeat for GGX and GGXXG variants**

Each adds more layers to the ring structure. The pattern is identical, just
wider ring entries.

**Step 6: Commit sub-phase 2c**

```bash
git add crypto/
go test -race ./crypto/... 2>&1
git commit -m "feat(crypto): Phase 2c — CLSAG ring signature verification (GG, GGX, GGXXG)

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 11: Bulletproofs+, BGE, Zarcanum Verification

**Files:**
- Create: `crypto/proof.go`
- Modify: `crypto/bridge.h`
- Modify: `crypto/bridge.cpp`
- Modify: `crypto/crypto_test.go`

This task depends on Boost.Multiprecision being available for Zarcanum.

**Step 1: Verify Boost is available**

```bash
dpkg -l | grep libboost-dev 2>/dev/null
# If not installed:
sudo apt install -y libboost-dev
```

**Step 2: Add proof verification to bridge.h**

```c
// ── Range Proofs (Bulletproofs+) ──────────────────────────
int cn_bppe_verify(const uint8_t *sig, size_t sig_len,
                   const uint8_t *commitments, size_t num_commitments,
                   uint8_t proof_type);

// ── BGE One-out-of-Many ───────────────────────────────────
int cn_bge_verify(const uint8_t context[32], const uint8_t *ring,
                  size_t ring_size, const uint8_t *sig, size_t sig_len);

// ── Zarcanum PoS ──────────────────────────────────────────
int cn_zarcanum_verify(const uint8_t hash[32], const uint8_t kernel[32],
                       const uint8_t *ring, size_t ring_size,
                       const uint8_t *proof, size_t proof_len);
```

**Step 3: Implement in bridge.cpp**

These functions deserialise opaque proof blobs into C++ structs, then call
the verification functions. The serialisation format must match how the
chain stores proofs in transaction data.

Implementation will require reading `range_proof_bppe.h` and `zarcanum.h`
to understand the exact struct layouts and how they're serialised on-chain.

**Step 4: Write proof.go with Go bindings**

```go
// SPDX-Licence-Identifier: EUPL-1.2

package crypto

/*
#include "bridge.h"
*/
import "C"

import "unsafe"

// ProofType identifies the range proof variant.
type ProofType uint8

const (
    ProofTypeZC       ProofType = 0
    ProofTypeZarcanum ProofType = 1
)

// VerifyBPPE verifies a Bulletproofs+ range proof.
func VerifyBPPE(sig []byte, commitments [][32]byte, proofType ProofType) bool {
    n := len(commitments)
    flat := make([]byte, n*32)
    for i, c := range commitments {
        copy(flat[i*32:], c[:])
    }
    return C.cn_bppe_verify(
        (*C.uint8_t)(unsafe.Pointer(&sig[0])),
        C.size_t(len(sig)),
        (*C.uint8_t)(unsafe.Pointer(&flat[0])),
        C.size_t(n),
        C.uint8_t(proofType),
    ) == 0
}

// VerifyBGE verifies a BGE one-out-of-many proof.
func VerifyBGE(context [32]byte, ring [][32]byte, sig []byte) bool {
    n := len(ring)
    flat := make([]byte, n*32)
    for i, r := range ring {
        copy(flat[i*32:], r[:])
    }
    return C.cn_bge_verify(
        (*C.uint8_t)(unsafe.Pointer(&context[0])),
        (*C.uint8_t)(unsafe.Pointer(&flat[0])),
        C.size_t(n),
        (*C.uint8_t)(unsafe.Pointer(&sig[0])),
        C.size_t(len(sig)),
    ) == 0
}

// VerifyZarcanum verifies a Zarcanum PoS proof.
func VerifyZarcanum(hash [32]byte, kernel [32]byte, ring []byte,
    ringSize int, proof []byte) bool {

    return C.cn_zarcanum_verify(
        (*C.uint8_t)(unsafe.Pointer(&hash[0])),
        (*C.uint8_t)(unsafe.Pointer(&kernel[0])),
        (*C.uint8_t)(unsafe.Pointer(&ring[0])),
        C.size_t(ringSize),
        (*C.uint8_t)(unsafe.Pointer(&proof[0])),
        C.size_t(len(proof)),
    ) == 0
}
```

**Step 5: Write tests using chain data**

These tests will need real proof data from testnet transactions.
Initially write placeholder tests that verify the functions compile and
can be called without crashing. Real verification tests come when we
have a working RPC client (Phase 4).

```go
func TestBPPE_Good_Placeholder(t *testing.T) {
    t.Skip("needs real proof data from testnet — Phase 4")
}

func TestBGE_Good_Placeholder(t *testing.T) {
    t.Skip("needs real proof data from testnet — Phase 4")
}

func TestZarcanum_Good_Placeholder(t *testing.T) {
    t.Skip("needs real proof data from testnet — Phase 4")
}
```

**Step 6: Commit sub-phase 2d**

```bash
git add crypto/
go test -race ./crypto/... 2>&1
git commit -m "feat(crypto): Phase 2d — proof verification stubs (BPPE, BGE, Zarcanum)

Bridge functions for Bulletproofs+, BGE one-out-of-many, and Zarcanum
PoS verification. Tests pending real chain data from Phase 4.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 12: Update Documentation

**Files:**
- Modify: `docs/architecture.md` (add crypto package)
- Modify: `docs/history.md` (add Phase 2 completion)
- Modify: `CLAUDE.md` (add CMake build instructions)

**Step 1: Update architecture.md**

Add a new section for the `crypto/` package describing:
- CGo bridge architecture
- Build flow (CMake → libcryptonote.a → CGo link)
- C API contract (bridge.h)
- Provenance tracking and upstream update workflow

**Step 2: Update history.md**

Add Phase 2 completion entry with:
- Files added/modified
- Test coverage
- Key findings (compat layer, build challenges)
- Known limitations (CLSAG/proof tests need chain data)

**Step 3: Update CLAUDE.md**

Add CMake build prerequisite:

```markdown
## Build

```bash
# First: build the crypto C++ library
cmake -S crypto -B crypto/build -DCMAKE_BUILD_TYPE=Release
cmake --build crypto/build --parallel

# Then: run tests
go test -race ./...
```
```

**Step 4: Run full test suite**

```bash
cd /home/claude/Code/core/go-blockchain
go test -race ./... 2>&1
go vet ./... 2>&1
```

Expected: all PASS, no warnings.

**Step 5: Commit**

```bash
git add docs/ CLAUDE.md
git commit -m "docs: Phase 2 crypto bridge documentation

Updated architecture, history, and CLAUDE.md with CMake build
instructions and crypto package details.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Expected Challenges

1. **Include path issues** — The upstream C++ files have interconnected includes
   that assume the full Zano tree is available. The compat layer must satisfy
   all transitive includes. Expect 2-3 iterations of "build, find missing header,
   add stub, rebuild".

2. **Boost.Multiprecision** — `crypto-sugar.h` line 19: `#include <boost/multiprecision/cpp_int.hpp>`.
   This is a header-only Boost library. Either install system Boost (`apt install libboost-dev`)
   or vendor just the `boost/multiprecision/` headers.

3. **eth_signature.cpp + OpenSSL** — `crypto-sugar.h` includes `eth_signature.h`.
   This pulls in OpenSSL. Ensure `libssl-dev` is installed. If Ethereum signatures
   aren't needed for consensus, consider stubbing `eth_signature.h` with just the
   type definitions.

4. **CLSAG struct serialisation** — The bridge must serialise/deserialise
   `CLSAG_GG_signature` (and variants) between opaque byte buffers and C++ structs.
   The exact field layout needs careful reading of `clsag.h`.

5. **Thread safety** — `crypto.cpp` uses `std::mutex` for the RNG. CGo calls from
   multiple goroutines are safe (Go holds a thread per CGo call), but the C++
   mutex must not deadlock under concurrent access.

6. **CGo overhead** — Each CGo call has ~100ns overhead. For hot paths (batch
   verification), consider batching multiple operations in a single bridge call.

---

## Verification Checklist

After all tasks:

- [ ] `cmake --build crypto/build` succeeds
- [ ] `go test -race ./...` passes (all packages)
- [ ] `go vet ./...` clean
- [ ] Key generation round-trip works
- [ ] ECDH derivation: sender and receiver agree
- [ ] Key image is deterministic and valid
- [ ] Standard signature sign/verify round-trip
- [ ] Ring signature (NLSAG) sign/verify with 4-member ring
- [ ] Wrong message / wrong key → verification fails
- [ ] PROVENANCE.md is accurate and complete
- [ ] docs/architecture.md updated
- [ ] docs/history.md updated
