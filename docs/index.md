---
title: Lethean Go Blockchain
description: Pure Go implementation of the Lethean CryptoNote/Zano-fork blockchain protocol.
---

# Lethean Go Blockchain

`go-blockchain` is a Go reimplementation of the Lethean blockchain protocol. It provides pure-Go implementations of chain logic, data structures, consensus rules, wallet operations, and networking, delegating only mathematically complex cryptographic operations (ring signatures, Bulletproofs+, Zarcanum proofs) to a cleaned C++ library via CGo.

**Module path:** `dappco.re/go/core/blockchain`

**Licence:** [European Union Public Licence (EUPL) version 1.2](https://joinup.ec.europa.eu/software/page/eupl/licence-eupl)

## Lineage

```
CryptoNote (van Saberhagen, 2013)
    |
IntenseCoin (2017)
    |
Lethean (2017-present)
    |
Zano rebase (2025)  -- privacy upgrades: Zarcanum, CLSAG, Bulletproofs+, confidential assets
    |
go-blockchain       -- Go reimplementation of the Zano-fork protocol
```

The Lethean mainnet launched on **2026-02-12** with genesis timestamp `1770897600` (12:00 UTC). The chain runs a hybrid PoW/PoS consensus with 120-second block targets.

## Package Structure

```
go-blockchain/
  config/       Chain parameters (mainnet/testnet), hardfork schedule
  types/        Core data types: Hash, PublicKey, Address, Block, Transaction
  wire/         Binary serialisation (consensus-critical, bit-identical to C++)
  crypto/       CGo bridge to libcryptonote (ring sigs, BP+, Zarcanum, stealth)
  difficulty/   PoW + PoS difficulty adjustment (LWMA variant)
  consensus/    Three-layer block/transaction validation
  chain/        Blockchain storage, block/tx validation, mempool
  p2p/          Levin TCP protocol, peer discovery, handshake
  rpc/          Daemon and wallet JSON-RPC client
  wallet/       Key management, output scanning, tx construction
  mining/       Solo PoW miner (RandomX nonce grinding)
  tui/          Terminal dashboard (bubbletea + lipgloss)
```

## Design Principles

1. **Consensus-critical code must be bit-identical** to the C++ implementation. The `wire/` package produces exactly the same binary output as the C++ serialisation for the same input.

2. **No global state.** Chain parameters are passed via `config.ChainConfig` structs, not package-level globals. `Mainnet` and `Testnet` are pre-defined instances.

3. **Interfaces at boundaries.** The `chain/` package defines interfaces for storage backends; the `wallet/` package uses Scanner, Signer, Builder, and RingSelector interfaces for v1/v2+ extensibility.

4. **Test against real chain data.** Wherever possible, tests use actual mainnet block and transaction hex blobs as test vectors, ensuring compatibility with the C++ node.

## Quick Start

```go
import (
    "fmt"

    "dappco.re/go/core/blockchain/config"
    "dappco.re/go/core/blockchain/rpc"
    "dappco.re/go/core/blockchain/types"
)

// Query the daemon
client := rpc.NewClient("http://localhost:36941")
info, err := client.GetInfo()
if err != nil {
    panic(err)
}
fmt.Printf("Height: %d, Difficulty: %d\n", info.Height, info.PowDifficulty)

// Decode an address
addr, prefix, err := types.DecodeAddress("iTHN...")
if err != nil {
    panic(err)
}
fmt.Printf("Spend key: %s\n", addr.SpendPublicKey)
fmt.Printf("Auditable: %v\n", addr.IsAuditable())

// Check hardfork version at a given height
version := config.VersionAtHeight(config.MainnetForks, 15000)
fmt.Printf("Active hardfork at height 15000: HF%d\n", version)
```

## CGo Boundary

The `crypto/` package is the **only** package that crosses the CGo boundary. All other packages are pure Go.

```
Go side                          C++ side (libcryptonote + librandomx)
+---------+                      +---------------------------+
| crypto/ |  --- CGo calls --->  | cn_fast_hash()            |
|         |                      | generate_key_derivation   |
|         |                      | generate_key_image        |
|         |                      | check_ring_signature      |
|         |                      | CLSAG_GG/GGX/GGXXG_verify|
|         |                      | bulletproof_plus_verify   |
|         |                      | zarcanum_verify           |
|         |                      | randomx_hash              |
+---------+                      +---------------------------+
```

When CGo is disabled, stub implementations return errors, allowing the rest of the codebase to compile and run tests that do not require real cryptographic operations.

## Development Phases

The project follows a 9-phase development plan. See the [wiki Development Phases page](https://dappco.re/go/core/blockchain/wiki/Development-Phases) for detailed phase descriptions.

| Phase | Scope | Status |
|-------|-------|--------|
| 0 | Config + Types | Complete |
| 1 | Wire Serialisation | Complete |
| 2 | CGo Crypto Bridge | Complete |
| 3 | P2P Protocol | Complete |
| 4 | RPC Client | Complete |
| 5 | Chain Storage | Complete |
| 6 | Wallet Core | Complete |
| 7 | Consensus Rules | Complete |
| 8 | Mining | Complete |

## Further Reading

- [Architecture](architecture.md) -- Package dependencies, CGo boundary, data structures
- [Cryptography](cryptography.md) -- Crypto primitives, hashing, signatures, proofs
- [Networking](networking.md) -- P2P protocol, peer discovery, message types
- [RPC Reference](rpc.md) -- Daemon and wallet JSON-RPC API
- [Chain Parameters](parameters.md) -- Tokenomics, emission, hardfork schedule
