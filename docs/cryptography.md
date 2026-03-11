---
title: Cryptography
description: Cryptographic primitives provided by the CGo bridge to libcryptonote.
---

# Cryptography

All cryptographic operations are provided by the `crypto/` package, which bridges Go to the vendored C++ CryptoNote library via CGo. This is the only package in the codebase that crosses the CGo boundary.

## Build Requirements

The C++ library must be built before running tests that require crypto:

```bash
cmake -S crypto -B crypto/build -DCMAKE_BUILD_TYPE=Release
cmake --build crypto/build --parallel
```

This produces `libcryptonote.a` (~680KB) and `librandomx.a`. CGo links against both.

Packages that do not require crypto (`config/`, `types/`, `wire/`, `difficulty/`) remain buildable without a C toolchain.

## Hash Functions

### Keccak-256 (`FastHash`)

The primary hash function throughout the protocol. Uses the original Keccak-256 (pre-NIST), **not** SHA3-256. The two differ in padding and produce different outputs.

```go
hash := crypto.FastHash(data)  // returns [32]byte
```

Used for: transaction hashes, block hashes, key derivations, address checksums, and proof construction.

### Tree Hash

Merkle tree hash over a set of transaction hashes within a block. Uses Keccak-256 as the leaf/node hash. Implemented in `wire/treehash.go` as a direct port of the C++ `crypto/tree-hash.c`.

### RandomX (PoW)

Proof-of-Work hash function. The vendored RandomX source (26 files including x86_64 JIT compiler) is built as a separate static library.

```go
hash := crypto.RandomXHash(key, input)  // key = "LetheanRandomXv1"
```

Input format: `header_hash(32 bytes) || nonce(8 bytes LE)`.

## Curve Operations

All public keys, key images, commitments, and signatures are built on the **Ed25519** curve (Twisted Edwards form of Curve25519), providing 128-bit security.

### Key Types

| Type | Size | Description |
|------|------|-------------|
| `PublicKey` | 32 bytes | Compressed Ed25519 point |
| `SecretKey` | 32 bytes | Scalar modulo group order _l_ |
| `KeyImage` | 32 bytes | Double-spend detection tag |
| `KeyDerivation` | 32 bytes | Diffie-Hellman shared secret point |

### Key Generation

```go
pub, sec, err := crypto.GenerateKeys()          // Random key pair
pub, err := crypto.SecretToPublic(sec)           // Derive public from secret
valid := crypto.CheckKey(pub)                    // Validate curve point
```

### Scalar Reduction

```go
crypto.ScReduce32(&key)  // Reduce 32-byte value modulo group order l
```

Required when converting a hash output to a valid secret key scalar. Used in wallet view key derivation: `viewSecret = sc_reduce32(Keccak256(spendSecret))`.

## Stealth Addresses

CryptoNote stealth addresses ensure every output appears at a unique one-time address that only the recipient can identify and spend.

### Key Derivation

```go
// Sender: compute shared secret from recipient's view public key and tx secret key
derivation, err := crypto.GenerateKeyDerivation(viewPubKey, txSecretKey)

// Derive per-output one-time public key
outputKey, err := crypto.DerivePublicKey(derivation, outputIndex, spendPubKey)
```

### Output Scanning (Recipient)

```go
// Compute shared secret from tx public key and own view secret key
derivation, err := crypto.GenerateKeyDerivation(txPubKey, viewSecretKey)

// Derive expected output public key
expected, err := crypto.DerivePublicKey(derivation, outputIndex, spendPubKey)

// If expected == actual output key, this output belongs to us
// Then derive the spending secret key:
outputSecKey, err := crypto.DeriveSecretKey(derivation, outputIndex, spendSecretKey)
```

## Key Images

Each output can only be spent once. When spent, its key image is revealed and recorded on chain. Any subsequent attempt to spend the same output produces an identical key image, which the network rejects.

```go
keyImage, err := crypto.GenerateKeyImage(outputPubKey, outputSecKey)
valid := crypto.ValidateKeyImage(keyImage)
```

Key images are deterministic (same output always produces the same image) and unlinkable (the image cannot be traced back to the output without the secret key).

## Signatures

### Standard Signatures

Non-ring Ed25519 signatures for general-purpose message signing:

```go
sig, err := crypto.GenerateSignature(messageHash, pubKey, secKey)
valid := crypto.CheckSignature(messageHash, pubKey, sig)
```

### NLSAG Ring Signatures (Pre-HF4)

Classic CryptoNote ring signatures. The spender proves ownership of one output within a ring of decoys without revealing which output is theirs.

Ring size: 10 decoys + 1 real (pre-HF4, configurable via `config.DefaultDecoySetSize`).

```go
// Generate a ring signature
sigs, err := crypto.GenerateRingSignature(
    txHash,         // Message being signed
    keyImage,       // Key image of the real input
    ringPubKeys,    // Public keys of all ring members ([][32]byte)
    realSecretKey,  // Secret key of the actual signer
    realIndex,      // Position of the real key in the ring
)

// Verify a ring signature
valid := crypto.CheckRingSignature(txHash, keyImage, ringPubKeys, sigs)
```

### CLSAG Ring Signatures (Post-HF4)

Compact Linkable Spontaneous Anonymous Group signatures, introduced with HF4 (Zarcanum). Smaller and faster than NLSAG.

Ring size: 15 decoys + 1 real (from HF4, `config.HF4MandatoryDecoySetSize`).

Three variants with increasing complexity:

| Variant | Base Points | Used For |
|---------|-------------|----------|
| CLSAG-GG | G, G | Efficient ring signatures |
| CLSAG-GGX | G, G, X | Confidential transaction signatures |
| CLSAG-GGXXG | G, G, X, X, G | PoS stake signatures |

```go
// Generate CLSAG-GG
sig, err := crypto.GenerateCLSAGGG(hash, ring, ringSize,
    pseudoOut, keyImage, secretX, secretF, secretIndex)

// Verify CLSAG-GG
valid := crypto.VerifyCLSAGGG(hash, ring, ringSize, pseudoOut, keyImage, sig)

// Verify CLSAG-GGX (confidential transactions)
valid := crypto.VerifyCLSAGGGX(hash, ring, ringSize,
    pseudoOutCommitment, pseudoOutAssetID, keyImage, sig)

// Verify CLSAG-GGXXG (PoS staking)
valid := crypto.VerifyCLSAGGGXXG(hash, ring, ringSize,
    pseudoOutCommitment, pseudoOutAssetID, extendedCommitment, keyImage, sig)
```

**Ring buffer convention:** Ring entries are flat byte arrays. Each entry packs 32-byte public keys per dimension:
- GG: 64 bytes per entry (stealth_addr + amount_commitment)
- GGX: 96 bytes per entry (+ blinded_asset_id)
- GGXXG: 128 bytes per entry (+ concealing_point)

## Cofactor Handling (1/8 Premultiplication)

On-chain curve points (commitments, blinded asset IDs) are stored premultiplied by the cofactor inverse (1/8). This is a Zarcanum convention for efficient batch verification.

```go
// Convert full point to on-chain form
onChain, err := crypto.PointDiv8(fullPoint)

// Convert on-chain form back to full point
full, err := crypto.PointMul8(onChainPoint)

// Point subtraction
result, err := crypto.PointSub(a, b)
```

CLSAG **generate** takes full points; CLSAG **verify** takes premultiplied (on-chain) values.

## Range Proofs

### Bulletproofs+ (BP+)

Zero-knowledge range proofs demonstrating that a committed value lies within [0, 2^64) without revealing the value. Used for transaction output amounts from HF4.

```go
valid := crypto.VerifyBPP(proofBlob, commitments)  // commitments premultiplied by 1/8
```

### Bulletproofs++ Enhanced (BPPE)

Extended variant used in Zarcanum PoS proofs. Proves the stake amount meets the minimum threshold without revealing the balance.

```go
valid := crypto.VerifyBPPE(proofBlob, commitments)
```

## Asset Surjection Proofs (BGE)

Bootle-Groth-Esgin one-out-of-many proofs. Each output proves its blinded asset ID corresponds to one of the input asset IDs, without revealing which. Introduced in HF5 (confidential assets).

```go
valid := crypto.VerifyBGE(contextHash, ring, proofBlob)
```

## Zarcanum PoS Proofs

Composite proof for Proof-of-Stake block production. Combines stake commitment proof, Schnorr response scalars, BPPE range proof, and CLSAG-GGXXG ring signature.

```go
valid := crypto.VerifyZarcanum(hash, proofBlob)
```

## Pedersen Commitments

Commits to a value _v_ with blinding factor _r_ as `C = v * H + r * G`. Additively homomorphic: `C(a) + C(b) = C(a+b)`, allowing verification that inputs and outputs balance without revealing amounts.

The native coin asset ID `H` is a fixed curve point. From HF4, all transaction amounts use Pedersen commitments.

## Wallet Encryption

Wallet files are encrypted with ChaCha8 (8-round ChaCha stream cipher). The encryption key is derived from the user's password via Argon2id (time=3, memory=64MB, threads=4) with AES-256-GCM.

## Summary

| Primitive | Purpose | Available From |
|-----------|---------|----------------|
| Keccak-256 | Primary hash | Genesis |
| Tree hash | Tx Merkle root | Genesis |
| RandomX | PoW hash | Genesis |
| Ed25519 ops | Key arithmetic | Genesis |
| Key derivation | Stealth addresses | Genesis |
| Key images | Double-spend prevention | Genesis |
| NLSAG ring sigs | Input privacy (classic) | Genesis |
| CLSAG-GG | Efficient ring sigs | HF4 |
| CLSAG-GGX | Confidential tx sigs | HF4 |
| CLSAG-GGXXG | PoS stake sigs | HF4 |
| Bulletproofs+ | Amount range proofs | HF4 |
| Bulletproofs++ | Stake range proofs | HF4 |
| Zarcanum proofs | PoS composite proofs | HF4 |
| BGE surjection | Asset type proofs | HF5 |
| Pedersen commitments | Hidden amounts | HF4 |
