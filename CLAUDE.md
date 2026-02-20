# CLAUDE.md

Pure Go implementation of the Lethean blockchain protocol (config, types, wire, difficulty).

Module: `forge.lthn.ai/core/go-blockchain`

## Commands

```bash
go test ./...               # run all tests
go test -race ./...         # race detector (required before commit)
go test -v -run Name ./...  # single test
go vet ./...                # vet check
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
