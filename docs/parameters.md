---
title: Chain Parameters
description: Consensus-critical constants, tokenomics, emission schedule, and hardfork schedule.
---

# Chain Parameters

All values are sourced from the C++ `currency_config.h.in` and `default.cmake`, implemented in the `config/` package. These are consensus-critical constants.

## Tokenomics

| Parameter | Value | Go Constant |
|-----------|-------|-------------|
| Atomic units per coin | 10^12 | `config.Coin` |
| Display decimal places | 12 | `config.DisplayDecimalPoint` |
| Block reward | 1.0 LTHN (10^12 atomic) | `config.BlockReward` |
| Default tx fee | 0.01 LTHN | `config.DefaultFee` |
| Minimum tx fee | 0.01 LTHN | `config.MinimumFee` |
| Premine | 10,000,000 LTHN | `config.Premine` |
| Dust threshold | 0 | `config.DefaultDustThreshold` |
| Ticker | LTHN | `config.CurrencyNameAbbreviation` |

### Supply Model

- **Block reward:** Fixed at 1 LTHN per block (no halving, but see HF6 for block time doubling).
- **Premine:** 10,000,000 LTHN reserved at genesis (coinswap allocation + initial premine).
- **Fee model:** Default and minimum fee of 0.01 LTHN. Pre-HF4, fees go to the miner. Post-HF4, fees are burned.

```go
// Genesis block returns the premine; all others return 1 LTHN.
func BaseReward(height uint64) uint64 {
    if height == 0 {
        return config.Premine
    }
    return config.BlockReward
}
```

## Address Prefixes

| Type | Prefix | Base58 starts with | Go Constant |
|------|--------|-------------------|-------------|
| Standard | `0x1eaf7` | `iTHN` | `config.AddressPrefix` |
| Integrated | `0xdeaf7` | `iTHn` | `config.IntegratedAddressPrefix` |
| Auditable | `0x3ceff7` | `iThN` | `config.AuditableAddressPrefix` |
| Auditable integrated | `0x8b077` | `iThn` | `config.AuditableIntegratedAddressPrefix` |

Addresses are encoded using CryptoNote base58 with a 4-byte Keccak-256 checksum. The prefix is varint-encoded before the spend and view public keys (32 bytes each).

## Block Timing

| Parameter | Value | Go Constant |
|-----------|-------|-------------|
| PoW block target | 120 seconds | `config.DifficultyPowTarget` |
| PoS block target | 120 seconds | `config.DifficultyPosTarget` |
| Combined target | 60 seconds | `config.DifficultyTotalTarget` |
| Blocks per day | ~1,440 | -- |
| PoS active from | Block 0 | `config.PosStartHeight` |

### Post-HF6 Timing (Future)

| Parameter | Value | Go Constant |
|-----------|-------|-------------|
| PoW target | 240 seconds | `config.DifficultyPowTargetHF6` |
| PoS target | 240 seconds | `config.DifficultyPosTargetHF6` |
| Combined target | 120 seconds | `config.DifficultyTotalTargetHF6` |
| Blocks per day | ~720 | -- |

## Difficulty

The difficulty adjustment uses the **LWMA** (Linear Weighted Moving Average) algorithm. Each solve-time interval is weighted linearly by recency -- more recent intervals have greater influence.

```go
// LWMA formula: next_D = total_work * T * (n+1) / (2 * weighted_solvetimes * n)
func NextDifficulty(timestamps []uint64, cumulativeDiffs []*big.Int, target uint64) *big.Int
```

| Parameter | Value | Go Constant |
|-----------|-------|-------------|
| Window | 720 blocks | `config.DifficultyWindow` |
| Lag | 15 blocks | `config.DifficultyLag` |
| Cut | 60 timestamps | `config.DifficultyCut` |
| Blocks count | 735 (Window + Lag) | `config.DifficultyBlocksCount` |
| Initial PoW difficulty | 1 | `config.DifficultyPowStarter` |
| Initial PoS difficulty | 1 | `config.DifficultyPosStarter` |
| LWMA window (N) | 60 intervals | `difficulty.LWMAWindow` |

Solve-times are clamped to [-6T, 6T] to limit timestamp manipulation impact. The algorithm returns `StarterDifficulty` (1) when insufficient data is available.

## Transaction Limits

| Parameter | Value | Go Constant |
|-----------|-------|-------------|
| Max inputs | 256 | `config.TxMaxAllowedInputs` |
| Max outputs | 2,000 | `config.TxMaxAllowedOutputs` |
| Min outputs (HF4+) | 2 | `config.TxMinAllowedOutputs` |
| Ring size (pre-HF4) | 10 | `config.DefaultDecoySetSize` |
| Ring size (HF4+) | 15 | `config.HF4MandatoryDecoySetSize` |
| Min coinage (HF4+) | 10 blocks | `config.HF4MandatoryMinCoinage` |
| Coinbase maturity | 10 blocks | `config.MinedMoneyUnlockWindow` |
| Max tx blob size | 374,600 bytes | `config.MaxTransactionBlobSize` |

### Transaction Versions

| Version | Go Constant | Description |
|---------|-------------|-------------|
| 0 | `config.TransactionVersionInitial` | Genesis/coinbase |
| 1 | `config.TransactionVersionPreHF4` | Standard transparent |
| 2 | `config.TransactionVersionPostHF4` | Zarcanum confidential |
| 3 | `config.TransactionVersionPostHF5` | Confidential assets |

## Block Limits

| Parameter | Value | Go Constant |
|-----------|-------|-------------|
| Max block height | 500,000,000 | `config.MaxBlockNumber` |
| Full reward zone | 125,000 bytes | `config.BlockGrantedFullRewardZone` |
| Coinbase reserved | 1,100 bytes | `config.CoinbaseBlobReservedSize` |
| Reward window | 400 blocks | `config.RewardBlocksWindow` |
| Free tx max size | 1,024 bytes | `config.FreeTxMaxBlobSize` |

### Block Size Penalty

Blocks within the full reward zone (125,000 bytes) receive the full base reward. Larger blocks incur a quadratic penalty:

```go
// reward = baseReward * (2*median - size) * size / median^2
// Uses 128-bit arithmetic (math/bits.Mul64) to avoid overflow.
func BlockReward(baseReward, blockSize, medianSize uint64) (uint64, error)
```

Blocks exceeding 2x the median size are rejected entirely.

### Block Versions

| Major Version | Go Constant | Hardfork |
|---------------|-------------|----------|
| 0 | `config.BlockMajorVersionInitial` | Pre-HF1 |
| 1 | `config.HF1BlockMajorVersion` | HF1 |
| 2 | `config.HF3BlockMajorVersion` | HF3 |
| 3 | `config.CurrentBlockMajorVersion` | HF4+ |

## Timestamp Validation

| Parameter | Value | Go Constant |
|-----------|-------|-------------|
| PoW future limit | 7,200 seconds (2 hours) | `config.BlockFutureTimeLimit` |
| PoS future limit | 1,200 seconds (20 min) | `config.PosBlockFutureTimeLimit` |
| Median window | 60 blocks | `config.TimestampCheckWindow` |

## PoS Parameters

| Parameter | Value | Go Constant |
|-----------|-------|-------------|
| Scan window | 600 seconds (10 min) | `config.PosScanWindow` |
| Scan step | 15 seconds | `config.PosScanStep` |
| Modifier interval | 10 | `config.PosModifierInterval` |
| Minimum coinstake age | 10 blocks | `config.PosMinimumCoinstakeAge` |
| Max consecutive PoS | 20 blocks | `config.PosStrictSequenceLimit` |

## P2P Network

### Ports

| Network | P2P | RPC | Stratum |
|---------|-----|-----|---------|
| Mainnet | 36942 | 36941 | 36940 |
| Testnet | 46942 | 46941 | 46940 |

### Peer Management

| Parameter | Value | Go Constant |
|-----------|-------|-------------|
| White peerlist limit | 1,000 | `config.P2PLocalWhitePeerlistLimit` |
| Grey peerlist limit | 5,000 | `config.P2PLocalGrayPeerlistLimit` |
| Default connections | 8 | `config.P2PDefaultConnectionsCount` |
| Handshake interval | 60 seconds | `config.P2PDefaultHandshakeInterval` |
| Max packet size | 50 MB | `config.P2PDefaultPacketMaxSize` |
| IP block time | 24 hours | `config.P2PIPBlockTime` |
| Failures before block | 10 | `config.P2PIPFailsBeforeBlock` |

## Alias System

| Parameter | Value | Go Constant |
|-----------|-------|-------------|
| Max aliases per block | 1,000 | `config.MaxAliasPerBlock` |
| Max name length | 255 | `config.AliasNameMaxLen` |
| Min public short name | 6 characters | `config.AliasMinimumPublicShortNameAllowed` |
| Valid characters | `0-9a-z-.` | `config.AliasValidChars` |
| Max comment size | 400 bytes | `config.AliasCommentMaxSizeBytes` |

## Mempool

| Parameter | Value |
|-----------|-------|
| Transaction lifetime | 345,600 seconds (4 days) |
| Alt block lifetime | ~10,080 blocks (~7 days) |
| Max alt blocks | 43,200 (~30 days) |
| Relay batch size | 5 transactions |

## Hardfork Schedule

Seven hardforks (HF0 through HF6) are defined. Heights are "after height" -- the fork activates at height N+1.

```go
type HardFork struct {
    Version     uint8    // 0-6
    Height      uint64   // Activates at heights > this value
    Mandatory   bool     // Must support to stay on network
    Description string
}
```

### Mainnet Schedule

| HF | Height | Changes |
|----|--------|---------|
| HF0 | 0 (genesis) | CryptoNote base protocol. Hybrid PoW/PoS. Classic ring signatures (NLSAG). Transparent amounts. |
| HF1 | 10,080 (~7 days) | New transaction types: HTLC, multisig, service attachments. |
| HF2 | 10,080 | Block time adjustment. Activates simultaneously with HF1. |
| HF3 | 999,999,999 | Block version 2. Preparation for Zarcanum. Future activation. |
| HF4 | 999,999,999 | **Zarcanum privacy upgrade.** Confidential transactions, CLSAG ring signatures, Bulletproofs+, mandatory ring size 15, minimum 2 outputs per tx. |
| HF5 | 999,999,999 | **Confidential assets.** Asset deployment/emission/burn, BGE surjection proofs, tx version 3. |
| HF6 | 999,999,999 | **Block time halving.** PoW/PoS targets double to 240s, effectively halving emission rate. |

### Testnet Schedule

| HF | Height | Notes |
|----|--------|-------|
| HF0 | 0 | Active from genesis |
| HF1 | 0 | Active from genesis |
| HF2 | 10 | Early activation |
| HF3 | 0 | Active from genesis |
| HF4 | 100 | Zarcanum active early |
| HF5 | 200 | Confidential assets early |
| HF6 | 999,999,999 | Future |

### Querying Hardfork State

```go
// Highest active version at a given height
version := config.VersionAtHeight(config.MainnetForks, 15000)
// Returns: HF2 (since HF1 and HF2 activate after 10,080)

// Check if a specific fork is active
active := config.IsHardForkActive(config.MainnetForks, config.HF4Zarcanum, 15000)
// Returns: false (HF4 not yet activated on mainnet)
```

### Pre-Hardfork Transaction Freeze

From HF5 onwards, a freeze period of 60 blocks (`config.PreHardforkTxFreezePeriod`) applies before each hardfork activation. During this window, no new transactions (other than coinbase) are accepted into the mempool, ensuring chain stabilisation before consensus rule changes.

## Genesis Block

| Parameter | Value |
|-----------|-------|
| Timestamp | 2026-02-12 12:00 UTC |
| Unix timestamp | `1770897600` |
| Genesis nonce | `CURRENCY_FORMATION_VERSION + 101011010121` |
| Block major version | 1 |
| Block minor version | 0 |
