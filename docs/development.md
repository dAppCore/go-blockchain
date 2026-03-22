# Development Guide

## Prerequisites

- Go 1.25 or later (the module declares `go 1.25`)
- `golang.org/x/crypto` for Keccak-256 (legacy, pre-NIST)
- No CGO required

No C toolchain, no system libraries, no external build tools. A plain
`go build ./...` is sufficient.

---

## Build and Test

```bash
# Run all tests
go test ./...

# Run all tests with the race detector (required before every commit)
go test -race ./...

# Run a single test by name
go test -v -run TestVersionAtHeight ./config/

# Run a single subtest
go test -v -run "TestVersionAtHeight_Good/genesis" ./config/

# Check for vet issues
go vet ./...

# Tidy dependencies
go mod tidy
```

All three commands (`go test -race ./...`, `go vet ./...`, and `go mod tidy`)
must produce no errors or warnings before a commit is pushed.

---

## Test Patterns

### File Organisation

Each package has a corresponding `_test.go` file in the same package (white-box
tests):

- `config/config_test.go` -- constant validation against C++ source values
- `config/hardfork_test.go` -- hardfork schedule and version lookup
- `types/address_test.go` -- address encode/decode round-trips, base58, checksums
- `difficulty/difficulty_test.go` -- LWMA algorithm behaviour
- `wire/varint_test.go` -- varint encode/decode round-trips, boundary values

### Naming Convention

All tests follow the `_Good`, `_Bad`, `_Ugly` suffix pattern:

- `_Good` -- happy path (correct inputs produce correct outputs)
- `_Bad` -- expected error conditions (invalid input, checksum corruption)
- `_Ugly` -- edge cases (empty slices, nil inputs, overflow, zero time spans)

Examples:

```
TestAddressEncodeDecodeRoundTrip_Good
TestDecodeAddress_Bad
TestBase58Empty_Ugly
TestIsHardForkActive_Bad
TestVersionAtHeight_Ugly
TestNextDifficulty_Ugly
TestDecodeVarint_Ugly
```

### Table-Driven Subtests

Most test functions use table-driven subtests with `t.Run()`:

```go
tests := []struct {
    name   string
    height uint64
    want   uint8
}{
    {"genesis", 0, HF0Initial},
    {"before_hf1", 10080, HF0Initial},
    {"at_hf1_hf2", 10081, HF2},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got := VersionAtHeight(MainnetForks, tt.height)
        if got != tt.want {
            t.Errorf("got %d, want %d", got, tt.want)
        }
    })
}
```

### Test Helpers

`makeTestAddress(flags uint8)` creates an address with deterministic byte
patterns (sequential values 0-31 for the spend key, 32-63 for the view key).
This produces reproducible base58 output for round-trip testing.

### Assertion Style

Tests use the standard library `testing` package directly with `t.Error`,
`t.Errorf`, and `t.Fatal`. Testify is not used in this package. Error messages
include both the got and want values with descriptive context.

---

## Coverage

Current coverage by package:

| Package | Coverage |
|---------|----------|
| config | 100.0% |
| difficulty | 81.0% |
| types | 73.4% |
| wire | 95.2% |

Total test functions: 75 (across 5 test files).

The lower coverage in `types/` reflects unexported helper functions
(`encodeBlock`, `decodeBlock`, `base58CharIndex`) that are exercised indirectly
through the public API but have branches not fully reached by the current test
vectors. The lower coverage in `difficulty/` reflects the window-capping branch
that only triggers with more than 735 block entries.

`go test -race ./...` passes clean across all packages.

---

## Coding Standards

### Language

UK English throughout: colour, organisation, serialise, initialise, behaviour.
Do not use American spellings in identifiers, comments, or documentation.

### Go Style

- All exported types, functions, and fields must have doc comments
- Error strings use the `package: description` format (e.g. `types: invalid hex
  for hash`)
- Error wrapping uses `fmt.Errorf("types: description: %w", err)`
- Every source file carries the EUPL-1.2 copyright header
- No emojis in code or comments
- Imports are ordered: stdlib, then `golang.org/x`, then `dappco.re`, each
  separated by a blank line

### Dependencies

Direct dependencies are intentionally minimal:

| Dependency | Purpose |
|------------|---------|
| `golang.org/x/crypto` | Keccak-256 (legacy, pre-NIST) for address checksums |
| `golang.org/x/sys` | Indirect, required by `golang.org/x/crypto` |

No test-only dependencies beyond the standard library `testing` package.

---

## Licence

EUPL-1.2. Every source file carries the standard copyright header:

```
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2
```

---

## Commit Convention

Format: `type(scope): description`

Common types: `feat`, `fix`, `test`, `refactor`, `docs`, `perf`, `chore`

Common scopes: `config`, `types`, `wire`, `difficulty`, `address`, `hardfork`

Every commit must include:

```
Co-Authored-By: Charon <charon@lethean.io>
```

Example:

```
feat(types): add Zarcanum confidential output type

Co-Authored-By: Charon <charon@lethean.io>
```

Commits must not be pushed unless `go test -race ./...` and `go vet ./...` both
pass. `go mod tidy` must produce no changes.
