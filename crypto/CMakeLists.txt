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
# eth_signature.cpp excluded — needs bitcoin-secp256k1, not required for consensus crypto.
# The header (eth_signature.h) is still included for type definitions via crypto-sugar.h.
set(CXX_SOURCES
    upstream/crypto.cpp
    upstream/crypto-sugar.cpp
    upstream/clsag.cpp
    upstream/zarcanum.cpp
    upstream/range_proofs.cpp
    upstream/one_out_of_many_proofs.cpp
    upstream/msm.cpp
    upstream/RIPEMD160_helper.cpp
    bridge.cpp
)

# --- Find system dependencies ---
find_package(OpenSSL REQUIRED)
find_package(Boost REQUIRED)

# --- Static library ---
add_library(cryptonote STATIC ${C_SOURCES} ${CXX_SOURCES})

target_include_directories(cryptonote PRIVATE
    ${CMAKE_CURRENT_SOURCE_DIR}/upstream
    ${CMAKE_CURRENT_SOURCE_DIR}/compat
    ${OPENSSL_INCLUDE_DIR}
    ${Boost_INCLUDE_DIRS}
)

target_link_libraries(cryptonote PRIVATE
    OpenSSL::Crypto
)

# Suppress warnings from upstream code (we don't modify it)
target_compile_options(cryptonote PRIVATE
    -Wno-unused-variable
    -Wno-unused-function
    -Wno-sign-compare
    -Wno-unused-parameter
    -Wno-deprecated-declarations
)
