# go-blockchain

Pure Go implementation of the Lethean blockchain protocol. Provides chain configuration, core cryptographic data types, CryptoNote wire serialisation, and LWMA difficulty adjustment for the Lethean CryptoNote/Zano-fork chain. Follows ADR-001: protocol logic in Go, cryptographic primitives deferred to a C++ bridge in later phases. Lineage: CryptoNote to IntenseCoin (2017) to Lethean to Zano rebase.

**Module**: `forge.lthn.ai/core/go-blockchain`
**Licence**: EUPL-1.2
**Language**: Go 1.25

## Quick Start

```go
import (
    "forge.lthn.ai/core/go-blockchain/config"
    "forge.lthn.ai/core/go-blockchain/types"
    "forge.lthn.ai/core/go-blockchain/wire"
    "forge.lthn.ai/core/go-blockchain/difficulty"
)

// Query the active hardfork version at a given block height
version := config.VersionAtHeight(config.MainnetForks, 10081) // returns HF2

// Check if a specific hardfork is active
active := config.IsHardForkActive(config.MainnetForks, config.HF4Zarcanum, 50000) // false

// Encode and decode a Lethean address
addr := &types.Address{SpendPublicKey: spendKey, ViewPublicKey: viewKey}
encoded := addr.Encode(config.AddressPrefix)
decoded, prefix, err := types.DecodeAddress(encoded)

// Varint encoding for the wire protocol
buf := wire.EncodeVarint(0x1eaf7)
val, n, err := wire.DecodeVarint(buf)

// Calculate next block difficulty
nextDiff := difficulty.NextDifficulty(timestamps, cumulativeDiffs, 120)
```

## Documentation

- [Architecture](docs/architecture.md) -- package structure, key types, design decisions, ADR-001
- [Development Guide](docs/development.md) -- prerequisites, test patterns, coding standards
- [Project History](docs/history.md) -- completed phases with commit hashes, known limitations

## Build & Test

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

## Licence

European Union Public Licence 1.2 -- see [LICENCE](LICENCE) for details.
