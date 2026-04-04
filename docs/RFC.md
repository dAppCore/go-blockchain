---
module: dappco.re/go/core/blockchain
repo: core/go-blockchain
lang: go
tier: lib
depends:
  - code/core/go
tags:
  - blockchain
  - web3
  - crypto
  - ledger
---
# go-blockchain RFC — CryptoNote+ Chain Implementation

> The authoritative spec for the Lethean blockchain package.
> An agent should be able to implement any chain feature from this document and its sub-specs alone.

**Module:** `dappco.re/go/core/blockchain`
**Repository:** `core/go-blockchain` on forge.lthn.ai
**Sub-specs:** [Models](RFC.models.md) | [Commands](RFC.commands.md) | [Difficulty](RFC.difficulty.md) | [Hardforks](RFC.hardforks.md) | [Tokenomics](RFC.tokenomics.md) | [LNS](RFC.lns.md) | [PubSub](RFC.pubsub.md) | [Swap Service](RFC.swap-service.md) | [Asset](asset/RFC.md) | [Blockstore](blockstore/RFC.md) | [Chain](chain/RFC.md) | [Escrow](escrow/RFC.md) | [Wallet](wallet/RFC.md)
**Upstream:** [Imports](RFC.imports.md)

---

## 1. Overview

A Go implementation of the Lethean blockchain node with C++ cryptographic primitives via CGo. The Go layer handles networking, storage, consensus rules, wallet operations, and CLI. The C++ layer handles ring signatures, bulletproofs, key derivation, and zero-knowledge math.

### 1.1 Architecture

```
core CLI (Go)
  core chain [serve|status|sync]
  core wallet [create|balance|transfer|stake]
  core identity [register|lookup]
─────────────────────────────────────────────
  pkg/ (Go)
  chain/      — block storage, validation, sync
  consensus/  — difficulty, PoW/PoS rules
  config/     — chain parameters
  crypto/     — CGo bridge to C++ primitives
  difficulty/ — retarget algorithms
  mining/     — block template, miner management
  p2p/        — peer protocol (Levin wire format)
  rpc/        — JSON-RPC daemon API
  tui/        — terminal dashboard
  types/      — block, transaction, header structs
  wallet/     — key management, transfer construction
  wire/       — binary serialisation (varint, tx, block)
─────────────────────────────────────────────
  CGo bridge (bridge.h — stable C API)
─────────────────────────────────────────────
  libcryptonote (C++, ~14,500 lines)
  ring sigs, bulletproofs, stealth addrs,
  key images, RandomX, consensus crypto
```

### 1.2 Design Decision

Go Shell + C++ Crypto Library. The crypto is battle-tested (Zano upstream, CryptoNote lineage since 2014). The Go layer owns everything above the crypto boundary — networking, storage, consensus validation, and wallet operations are implemented in Go.

### 1.3 Chain Type

Hybrid PoW/PoS. RandomX for proof-of-work (ASIC resistant). CryptoNote+ staking for proof-of-stake. Both consensus types produce blocks and earn rewards.

## 2. Consensus

### 2.1 Difficulty

Two algorithms available, selected by height. See `RFC.difficulty.md` for full specification.

- Pre-HF2 (blocks 0-10,000): broken LWMA-1 with 10s PoW target
- Post-HF2 (blocks 10,001+): Zano `next_difficulty_1` with 120s PoW target

### 2.2 Block Production

| Parameter | Value |
|-----------|-------|
| PoW target | 120s (post-HF2) |
| PoS target | 120s |
| Combined target | 60s (`DIFFICULTY_TOTAL_TARGET`) |
| Blocks per day | ~1,440 |
| Block reward | 1.0 LTHN |

PoW and PoS difficulties are tracked independently. The collection function filters blocks by type — PoW difficulty only considers PoW block timestamps.

### 2.3 Hardforks

Six hardforks defined. See `RFC.hardforks.md` for validation rules per HF, including the schedule and network impact.

## 3. Chain Economics

See `RFC.tokenomics.md` for emission, premine, fees, SWAP mechanics, and address formats.

## 4. Wire Format

Binary serialisation using CryptoNote varint encoding. Blocks, transactions, and headers are serialised to/from byte streams. The `wire/` package handles encoding.

See `RFC.models.md` for all struct definitions.

## 5. RPC Interface

JSON-RPC over HTTP. The daemon exposes both JSON-RPC methods (via `/json_rpc` endpoint) and direct HTTP endpoints (e.g. `/start_mining`, `/stop_mining`, `/getinfo`).

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/json_rpc` | `getinfo` | Chain status, height, difficulty, HF flags |
| `/json_rpc` | `getblockheaderbyheight` | Block header at height |
| `/start_mining` | POST | Start PoW mining with address and thread count |
| `/stop_mining` | POST | Stop mining |

## 6. Daemon Binary

Two binaries from the same source, distinguished by compile-time `TESTNET` flag:

| Binary | Flag | Default Ports |
|--------|------|---------------|
| `lethean-chain-node` | OFF | RPC 36941, P2P 36942, API 36943 |
| `lethean-testnet-chain-node` | ON | RPC 36941, P2P 36942, API 36943 |

Testnet ports are overridden at runtime via systemd service configuration to avoid conflicts with mainnet.

## 7. Reference

| Resource | Location |
|----------|----------|
| Difficulty algorithms | `code/core/go/blockchain/RFC.difficulty.md` |
| Hardfork validation rules | `code/core/go/blockchain/RFC.hardforks.md` |
| Tokenomics | `code/core/go/blockchain/RFC.tokenomics.md` |
| Models (structs) | `code/core/go/blockchain/RFC.models.md` |
| Commands (CLI) | `code/core/go/blockchain/RFC.commands.md` |
| P2P networking | `code/core/go/p2p/RFC.md` |
| Network protocol Layer 0 | `code/core/network/RFC.md` § "Layer 0: Chain Interface" |
