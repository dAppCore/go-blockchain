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

include(CheckCCompilerFlag)
include(CheckCXXCompilerFlag)
include(CheckIncludeFile)

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

# --- RandomX PoW library ---
if(CMAKE_SYSTEM_PROCESSOR STREQUAL "")
    set(CMAKE_SYSTEM_PROCESSOR ${CMAKE_HOST_SYSTEM_PROCESSOR})
endif()
string(TOLOWER "${CMAKE_SYSTEM_PROCESSOR}" RANDOMX_ARCH_ID)

set(RANDOMX_SOURCES
    randomx/aes_hash.cpp
    randomx/argon2_avx2.c
    randomx/argon2_ref.c
    randomx/argon2_ssse3.c
    randomx/bytecode_machine.cpp
    randomx/cpu.cpp
    randomx/dataset.cpp
    randomx/soft_aes.cpp
    randomx/virtual_memory.c
    randomx/vm_interpreted.cpp
    randomx/allocator.cpp
    randomx/instruction.cpp
    randomx/randomx.cpp
    randomx/superscalar.cpp
    randomx/vm_compiled.cpp
    randomx/vm_interpreted_light.cpp
    randomx/argon2_core.c
    randomx/blake2_generator.cpp
    randomx/instructions_portable.cpp
    randomx/reciprocal.c
    randomx/virtual_machine.cpp
    randomx/vm_compiled_light.cpp
    randomx/blake2/blake2b.c
)

add_library(randomx STATIC ${RANDOMX_SOURCES})
target_include_directories(randomx PRIVATE
    ${CMAKE_CURRENT_SOURCE_DIR}/randomx
)
set_property(TARGET randomx PROPERTY POSITION_INDEPENDENT_CODE ON)
set_property(TARGET randomx PROPERTY CXX_STANDARD 11)
set_property(TARGET randomx PROPERTY CXX_STANDARD_REQUIRED ON)

# Platform-specific flags for RandomX.
if(RANDOMX_ARCH_ID STREQUAL "x86_64" OR RANDOMX_ARCH_ID STREQUAL "x86-64" OR RANDOMX_ARCH_ID STREQUAL "amd64")
    enable_language(ASM)
    target_sources(randomx PRIVATE
        randomx/assembly_generator_x86.cpp
        randomx/jit_compiler_x86.cpp
        randomx/jit_compiler_x86_static.S
    )

    check_cxx_compiler_flag(-maes HAVE_AES_CXX)
    check_c_compiler_flag(-maes HAVE_AES_C)
    if(HAVE_AES_CXX)
        target_compile_options(randomx PRIVATE $<$<COMPILE_LANGUAGE:CXX>:-maes>)
    endif()
    if(HAVE_AES_C)
        target_compile_options(randomx PRIVATE $<$<COMPILE_LANGUAGE:C>:-maes>)
    endif()

    check_c_compiler_flag(-mssse3 HAVE_SSSE3)
    if(HAVE_SSSE3)
        set_source_files_properties(randomx/argon2_ssse3.c PROPERTIES COMPILE_FLAGS -mssse3)
    endif()

    check_c_compiler_flag(-mavx2 HAVE_AVX2)
    if(HAVE_AVX2)
        set_source_files_properties(randomx/argon2_avx2.c PROPERTIES COMPILE_FLAGS -mavx2)
    endif()
elseif(RANDOMX_ARCH_ID STREQUAL "aarch64" OR RANDOMX_ARCH_ID STREQUAL "arm64" OR RANDOMX_ARCH_ID STREQUAL "armv8-a")
    enable_language(ASM)
    target_sources(randomx PRIVATE
        randomx/jit_compiler_a64.cpp
        randomx/jit_compiler_a64_static.S
    )

    check_include_file(asm/hwcap.h HAVE_HWCAP)
    if(HAVE_HWCAP)
        target_compile_definitions(randomx PRIVATE HAVE_HWCAP)
    endif()

    check_cxx_compiler_flag(-march=armv8-a+crypto HAVE_ARMV8_CRYPTO_CXX)
    check_c_compiler_flag(-march=armv8-a+crypto HAVE_ARMV8_CRYPTO_C)
    if(HAVE_ARMV8_CRYPTO_CXX)
        target_compile_options(randomx PRIVATE $<$<COMPILE_LANGUAGE:CXX>:-march=armv8-a+crypto>)
    endif()
    if(HAVE_ARMV8_CRYPTO_C)
        target_compile_options(randomx PRIVATE $<$<COMPILE_LANGUAGE:C>:-march=armv8-a+crypto>)
    endif()
endif()

target_compile_options(randomx PRIVATE
    -Wno-unused-variable
    -Wno-unused-function
    -Wno-sign-compare
    -Wno-unused-parameter
    -Wno-implicit-fallthrough
)

# --- Find system dependencies ---
find_package(OpenSSL REQUIRED)
find_package(Boost REQUIRED)

# --- Static library ---
add_library(cryptonote STATIC ${C_SOURCES} ${CXX_SOURCES})

target_include_directories(cryptonote PRIVATE
    ${CMAKE_CURRENT_SOURCE_DIR}/upstream
    ${CMAKE_CURRENT_SOURCE_DIR}/compat
    ${CMAKE_CURRENT_SOURCE_DIR}/randomx
    ${OPENSSL_INCLUDE_DIR}
    ${Boost_INCLUDE_DIRS}
)

target_link_libraries(cryptonote PRIVATE
    OpenSSL::Crypto
    randomx
)

# Suppress warnings from upstream code (we don't modify it)
target_compile_options(cryptonote PRIVATE
    -Wno-unused-variable
    -Wno-unused-function
    -Wno-sign-compare
    -Wno-unused-parameter
    -Wno-deprecated-declarations
)
