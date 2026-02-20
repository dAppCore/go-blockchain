# go-blockchain

Go implementation of the Lethean blockchain protocol. Pure Go package providing
chain configuration, core data types, wire serialisation, and difficulty
calculation for the Lethean CryptoNote/Zano-fork chain.

This package follows ADR-001: Go Shell + C++ Crypto Library. Protocol logic
lives in Go; only the mathematically complex cryptographic primitives (ring
signatures, bulletproofs, Zarcanum proofs) are delegated to a cleaned C++
library via CGo in later phases.

Lineage: CryptoNote -> IntenseCoin (2017) -> Lethean -> Zano rebase.

Licence: EUPL-1.2

## Build and Test

```bash
go build ./...
go test -v -race ./...
go vet ./...
```

## Package Layout

```
config/       Chain parameters (mainnet/testnet), hardfork schedule
types/        Core data types: Hash, PublicKey, Address, Block, Transaction
wire/         Binary serialisation (CryptoNote varint encoding)
difficulty/   PoW + PoS difficulty adjustment (LWMA variant)
crypto/       (future) CGo bridge to libcryptonote
p2p/          (future) Levin TCP protocol
rpc/          (future) Daemon and wallet JSON-RPC
chain/        (future) Blockchain storage, validation, mempool
wallet/       (future) Key management, output scanning, tx construction
consensus/    (future) Hardfork rules, block reward, fee policy
```

## Coding Standards

- **Language:** UK English in all comments and documentation (colour, organisation, centre)
- **Commits:** Conventional commits (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`)
- **Co-Author:** All commits include `Co-Authored-By: Charon <charon@lethean.io>`
- **Test naming:** `_Good` (happy path), `_Bad` (expected errors), `_Ugly` (panics/edge cases)
- **Imports:** stdlib, then `golang.org/x`, then `forge.lthn.ai`, each separated by a blank line
- **No emojis** in code or comments
