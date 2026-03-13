# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Go implementation of the Lethean blockchain protocol (CryptoNote/Zano-fork) with a CGo crypto bridge.

Module: `forge.lthn.ai/core/go-blockchain`
Licence: EUPL-1.2 (every source file carries the copyright header)

## Build

```bash
# First time: build the C++ crypto library (requires cmake, g++/clang, libssl-dev, libboost-dev)
cmake -S crypto -B crypto/build -DCMAKE_BUILD_TYPE=Release
cmake --build crypto/build --parallel

# Run tests (race detector required before commit)
go test -race ./...
go vet ./...
```

Pure Go packages (`config/`, `types/`, `wire/`, `difficulty/`) build without a C toolchain. Packages that use CGo (`crypto/`, `consensus/`, `chain/`, `wallet/`, `mining/`) require `libcryptonote.a` from the cmake step.

## Commands

```bash
go test ./...                                   # all tests
go test -race ./...                             # race detector (required before commit)
go test -v -run TestName ./pkg/                 # single test
go test -v -run "TestName/subtest" ./pkg/       # single subtest
go vet ./...                                    # vet check
go mod tidy                                     # must produce no changes before commit
cmake --build crypto/build --parallel           # rebuild after bridge.cpp changes
go test -tags integration ./...                 # integration tests (need C++ testnet daemon)
```

## Standards

- UK English throughout (colour, serialise, behaviour — in identifiers, comments, docs)
- `go test -race ./...`, `go vet ./...`, and `go mod tidy` must all pass before commit
- Conventional commits: `type(scope): description`
- Co-Author trailer: `Co-Authored-By: Charon <charon@lethean.io>`
- Error strings: `package: description` format (e.g. `types: invalid hex for hash`)
- Error wrapping: `fmt.Errorf("package: description: %w", err)`
- Import order: stdlib, then `golang.org/x`, then `forge.lthn.ai`, blank lines between groups
- No emojis in code or comments

## Test Conventions

- Tests use `_Good`, `_Bad`, `_Ugly` suffix pattern (happy path / expected errors / edge cases)
- Table-driven subtests with `t.Run()` are the norm
- Use stdlib `testing` package directly (`t.Error`, `t.Errorf`, `t.Fatal`) — testify is a dependency but tests in core packages use stdlib assertions
- Integration tests are build-tagged (`//go:build integration`) and need a C++ testnet daemon on `localhost:46941`

## Architecture

```
consensus  ←  standalone validation (pure functions, no storage dependency)
    ↓
  chain    ←  persistent storage (go-store/SQLite), RPC + P2P sync
    ↓
p2p / rpc / wallet  ←  network and wallet layer
    ↓
  wire     ←  consensus-critical binary serialisation
    ↓
types / config  ←  leaf packages (stdlib only, no internal deps)
    ↓
 crypto    ←  CGo boundary (libcryptonote.a + librandomx.a)
```

**Key design decisions:**
- `consensus/` is pure — takes types + config + height, returns errors. No dependency on `chain/` or storage.
- `wire/` stores extra, attachment, and proof fields as opaque raw bytes for bit-identical round-tripping without implementing all variant types.
- `crypto/` follows ADR-001: Go shell + C++ crypto library. 29-function C API in `bridge.h`, only `uint8_t*` pointers cross the boundary. Upstream C++ from Zano commit `fa1608cf` in `crypto/upstream/`.
- On-chain curve points are stored premultiplied by cofactor inverse (1/8). `PointMul8`/`PointDiv8` convert between representations.
- Transaction wire format differs between versions: v0/v1 is `version, vin, vout, extra, [signatures]`; v2+ is `version, vin, extra, vout, [hardfork_id], [attachment, signatures, proofs]`.
- Block hash includes a varint length prefix: `Keccak256(varint(len) || block_hashing_blob)`.
- Two P2P varint formats exist: CryptoNote LEB128 (`wire/`) and portable storage 2-bit size mark (`go-p2p/node/levin/`).

**Binary:** `cmd/core-chain/` — cobra CLI via `forge.lthn.ai/core/cli`. Subcommands: `chain sync` (P2P block sync) and `chain explorer` (TUI dashboard).

**Local replace directives:** `go.mod` uses local `replace` for sibling `forge.lthn.ai/core/*` modules.

## Docs

- `docs/architecture.md` — package structure, key types, wire format details
- `docs/development.md` — prerequisites, test patterns, coding standards, coverage
- `docs/history.md` — completed phases with commit hashes, known limitations
