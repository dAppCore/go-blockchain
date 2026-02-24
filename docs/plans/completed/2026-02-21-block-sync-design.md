# Block Sync Design

Date: 2026-02-21

## Overview

Three deliverables in dependency order:

1. **NLSAG signature verification** -- wire `crypto.CheckRingSignature()` into
   `consensus.verifyV1Signatures()` so the chain validates real ring signatures.
2. **Full RPC sync to tip** -- run `chain.Sync()` against testnet/mainnet, fix
   breakages, confirm the entire chain validates end-to-end.
3. **P2P block sync** -- replace the RPC transport with native P2P using Levin
   commands 2003/2004/2006/2007.

## Part 1: NLSAG Signature Verification

### Current State

- `crypto.CheckRingSignature(hash, keyImage, pubs, sigs)` -- CGo bridge exists,
  calls C++ `check_ring_signature()`.
- `consensus.verifyV1Signatures()` -- structural scaffold with `RingOutputsFn`
  callback. Returns nil when callback is provided (TODO).
- `chain.Sync()` -- passes `nil` as `getRingOutputs`, structural checks only.

### Changes

**`consensus/verify.go`** -- `verifyV1Signatures()`:

```
for each TxInputToKey input:
  1. Convert relative key_offsets to absolute offsets
  2. Call getRingOutputs(amount, absoluteOffsets) -> ring public keys
  3. Compute prefixHash = wire.TransactionPrefixHash(tx)
  4. Call crypto.CheckRingSignature(prefixHash, keyImage, ringPubs, sigs[i])
  5. Return error if verification fails
```

**`chain/sync.go`** -- provide a real `RingOutputsFn`:

```go
func (c *Chain) getRingOutputs(amount uint64, offsets []uint64) ([]types.PublicKey, error)
```

Looks up outputs from the chain store via `chain.GetOutput(amount, globalIndex)`,
retrieves the transaction, and extracts the stealth address from the output at
the stored output index.

### CLSAG (v2+)

The CGo bridge exists (`VerifyCLSAGGG`, `VerifyCLSAGGGX`, `VerifyCLSAGGGXXG`)
but v2+ transactions need parsed signatures rather than raw bytes. Since mainnet
is pre-HF4 (no v2+ spending txs yet), defer full CLSAG wiring. The structural
scaffold in `verifyV2Signatures` stays.

## Part 2: Full RPC Sync to Tip

### Current State

`chain.Sync()` works for the first ~10 blocks (integration test passes). Syncing
the full chain will stress wire decode robustness, output indexing, difficulty
accumulation, and memory.

### Changes

- **Progress logging** -- log every N blocks during sync.
- **Context cancellation** -- accept `context.Context` for graceful shutdown.
- **TxInputZC handling** -- `indexOutputs` and key image tracking for v2+ inputs.
- **PoS block validation** -- ensure header validation handles Flags bit 0.
- **Integration test** -- sync to tip with `VerifySignatures: true`.

### Testing

Run against the C++ testnet daemon (localhost:46941). The testnet has ~500+ blocks
with both pre-HF4 and post-HF4 transactions. All blocks must validate, all
hashes must match.

## Part 3: P2P Block Sync

### Protocol

```
Handshake (1001) -> learn peer height via CoreSyncData
    |
REQUEST_CHAIN (2006) -> send sparse block ID history
    |
RESPONSE_CHAIN_ENTRY (2007) -> receive missing block hashes + start_height
    |
REQUEST_GET_OBJECTS (2003) -> request blocks by hash (batches of ~200)
    |
RESPONSE_GET_OBJECTS (2004) -> receive block blobs + tx blobs
    |
Validate -> Store -> Repeat from REQUEST_CHAIN if more blocks
```

### Existing Infrastructure

- `p2p/` has types for RequestChain (2006), ResponseChainEntry (2007),
  NewBlockNotification (2001), handshake, peerlist, timed_sync.
- `go-p2p/node/levin` has the TCP connection layer with frame read/write.
- `p2p/integration_test.go` proves TCP handshake works against testnet.

### New Types

**`p2p/sync.go`** (extend existing):

- `RequestGetObjects` (2003): `blocks []Hash`, `txs []Hash`
- `ResponseGetObjects` (2004): `blocks []BlockCompleteEntry`,
  `missed_ids []Hash`, `current_blockchain_height uint64`
- `BlockCompleteEntry`: `Block []byte`, `Txs [][]byte`

### New Sync Engine

**`chain/p2psync.go`**:

- `P2PSync(ctx, conn, opts)` -- state machine running the protocol above.
- Sparse chain history builder: genesis, then exponentially-spaced block hashes
  from tip backwards (matches C++ `get_short_chain_history()`).
- Block processing reuses `processBlockFromBlobs(blockBlob, txBlobs)` -- same
  validation as RPC path but without JSON/RPC wrapper.
- Single peer connection for Phase 1. Multi-peer is future work.

### Shared Processing

`processBlock` logic is shared between RPC and P2P sync -- only the transport
differs. Refactor the existing `processBlock` to accept raw blobs:

```go
func (c *Chain) processBlockBlobs(blockBlob []byte, txBlobs [][]byte, opts SyncOptions) error
```

The RPC path decodes hex and calls this. The P2P path passes raw bytes directly.

## Architecture

```
                 +-------------+
                 |  chain.Sync |  (existing, RPC)
                 +------+------+
                        |
    +-------------------+-------------------+
    |                   |                   |
    v                   v                   v
chain.P2PSync    processBlockBlobs     RingOutputsFn
(new, P2P)       (shared logic)        (chain store)
    |                   |                   |
    v                   v                   v
 p2p/levin         wire.Decode*       consensus.Verify*
                                            |
                                            v
                                       crypto.Check*
```

## Phasing

1. Signature verification -- small, self-contained, testable with a spending tx
2. RPC sync to tip -- validates the full chain end-to-end
3. P2P sync -- replaces the transport, same validation underneath
