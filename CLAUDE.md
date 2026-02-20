# CLAUDE.md

Go implementation of the Lethean blockchain protocol with CGo crypto bridge.

Module: `forge.lthn.ai/core/go-blockchain`

## Build

```bash
# First time: build the crypto C++ library (requires cmake, g++, libssl-dev, libboost-dev)
cmake -S crypto -B crypto/build -DCMAKE_BUILD_TYPE=Release
cmake --build crypto/build --parallel

# Then: run tests
go test -race ./...
go vet ./...
```

## Commands

```bash
go test ./...               # run all tests
go test -race ./...         # race detector (required before commit)
go test -v -run Name ./...  # single test
go vet ./...                # vet check
cmake --build crypto/build --parallel  # rebuild C++ after bridge.cpp changes
```

## Standards

- UK English
- `go test -race ./...` and `go vet ./...` must pass before commit
- Conventional commits: `type(scope): description`
- Co-Author: `Co-Authored-By: Charon <charon@lethean.io>`

## Docs

- `docs/architecture.md` -- package structure, key types, design decisions, ADR-001
- `docs/development.md` -- prerequisites, test patterns, coding standards
- `docs/history.md` -- completed phases with commit hashes, known limitations
