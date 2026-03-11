---
title: P2P Networking
description: Levin wire protocol, peer discovery, block/transaction propagation, and chain synchronisation.
---

# P2P Networking

The Lethean P2P network uses the **Levin protocol**, a binary TCP protocol inherited from the CryptoNote/epee library. The Go implementation spans two packages:

- **`go-p2p/node/levin/`** -- Wire format: 33-byte header, portable storage serialisation, framed TCP connections.
- **`go-blockchain/p2p/`** -- Application-level command semantics: handshake, sync, block/tx relay.

## Levin Wire Format

Every message is prefixed with a 33-byte header:

```
Offset  Size   Field               Description
------  ----   -----               -----------
0       8      m_signature         Magic: 0x0101010101012101 (LE)
8       8      m_cb                Payload length in bytes (LE)
16      1      m_have_to_return    Boolean: expects a response
17      4      m_command           Command ID (LE uint32)
21      4      m_return_code       Return code (LE int32)
25      4      m_flags             Packet type flags (LE uint32)
29      4      m_protocol_version  Protocol version (LE uint32)
```

### Packet Types

| Flag | Meaning |
|------|---------|
| `0x00000001` | Request (expects response if `m_have_to_return` is set) |
| `0x00000002` | Response to a prior request |

### Header Validation

On receiving a message, a node:

1. Reads 8 bytes and verifies they match `0x0101010101012101`.
2. Reads the remaining 25 bytes of the header.
3. Validates payload length does not exceed `P2P_DEFAULT_PACKET_MAX_SIZE` (50 MB).
4. Reads `m_cb` bytes of payload.
5. Dispatches based on `m_command`.

## Payload Serialisation

Payloads use the **epee portable storage** binary format -- a TLV-style key-value serialisation supporting nested objects, arrays, integers, strings, and binary blobs.

This is distinct from the consensus binary serialisation used for blocks and transactions in the `wire/` package. Notably, portable storage varints use a 2-bit size mark in the low bits (1/2/4/8 byte encoding), **not** the 7-bit LEB128 varints used on the consensus wire.

## Command IDs

Commands are divided into two pools, re-exported in the `p2p/` package:

```go
const (
    CommandHandshake       = 1001  // P2P handshake
    CommandTimedSync       = 1002  // Periodic state sync
    CommandPing            = 1003  // Connectivity check
    CommandNewBlock        = 2001  // Block propagation
    CommandNewTransactions = 2002  // Transaction propagation
    CommandRequestObjects  = 2003  // Request blocks/txs by hash
    CommandResponseObjects = 2004  // Response with blocks/txs
    CommandRequestChain    = 2006  // Request chain skeleton
    CommandResponseChain   = 2007  // Response with chain entry
)
```

## Handshake Protocol

### Go Types

```go
type NodeData struct {
    NetworkID [16]byte
    PeerID    uint64
    LocalTime int64
    MyPort    uint32
}

type HandshakeRequest struct {
    NodeData    NodeData
    PayloadData CoreSyncData
}

type HandshakeResponse struct {
    NodeData     NodeData
    PayloadData  CoreSyncData
    PeerlistBlob []byte  // Packed 24-byte entries
}
```

### Connection Flow

1. **Initiator** connects via TCP and sends `COMMAND_HANDSHAKE` (1001) as a request.
2. **Responder** validates the network ID, client version, and peer uniqueness.
3. **Responder** replies with its own node data, sync state, and peer list.
4. Both sides begin periodic `COMMAND_TIMED_SYNC` (1002) every 60 seconds.

### Handshake Request Payload

```
node_data {
    network_id    [16]byte   // Derived from CURRENCY_FORMATION_VERSION
    peer_id       uint64     // Random peer identifier
    local_time    int64      // Node's Unix timestamp
    my_port       uint32     // Listening port for inbound connections
}
payload_data {
    current_height           uint64
    top_id                   [32]byte   // Top block hash
    last_checkpoint_height   uint64
    core_time                uint64
    client_version           string     // e.g. "6.0.1.2[go-blockchain]"
    non_pruning_mode_enabled bool
}
```

### Handshake Rejection

The responder rejects the handshake if:
- `network_id` does not match (different chain or testnet/mainnet mismatch)
- `client_version` is below the minimum for the current hardfork era
- The peer ID is already connected (duplicate connection)

### Network Identity

| Network | Formation Version | Network ID (byte 15) |
|---------|-------------------|---------------------|
| Mainnet | 84 | `0x54` |
| Testnet | 100 | `0x64` |

```go
var NetworkIDMainnet = [16]byte{
    0x11, 0x10, 0x01, 0x11, 0x01, 0x01, 0x11, 0x01,
    0x10, 0x11, 0x01, 0x11, 0x01, 0x11, 0x21, 0x54,
}
```

## Peer List Management

### Peer Entry Format

Peer lists are exchanged as packed binary blobs, 24 bytes per entry:

```go
type PeerlistEntry struct {
    IP       uint32   // IPv4 (network byte order)     [4 bytes]
    Port     uint32   // Port number                    [4 bytes]
    ID       uint64   // Peer ID                        [8 bytes]
    LastSeen int64    // Unix timestamp of last contact  [8 bytes]
}

entries := p2p.DecodePeerlist(blob)  // Splits packed blob into entries
```

### Two-Tier Peer Lists

| List | Max Size | Description |
|------|----------|-------------|
| **White list** | 1,000 | Verified peers (successful handshake + ping) |
| **Grey list** | 5,000 | Unverified peers (received from other nodes) |

### Connection Strategy

| Parameter | Value |
|-----------|-------|
| Target outbound connections | 8 |
| White list preference | 70% |
| Peers exchanged per handshake | 250 |

When establishing outbound connections, 70% are made to white list peers and 30% to grey list peers. Successful connections promote grey list peers to the white list.

### Ping Verification

Before adding a peer to the white list:

1. Connect to the peer's advertised IP:port.
2. Send `COMMAND_PING` (1003).
3. Verify the response contains `status: "OK"` and a matching `peer_id`.
4. Only then promote from grey to white list.

### Failure Handling

| Parameter | Value |
|-----------|-------|
| Failures before blocking | 10 |
| Block duration | 24 hours |
| Failed address forget time | 5 minutes |
| Idle connection kill interval | 5 minutes |

## Timed Sync

After handshake, peers exchange `COMMAND_TIMED_SYNC` (1002) every 60 seconds. This keeps peers informed of each other's chain state and propagates peer list updates.

## Block Propagation

When a node mines or stakes a new block:

```
NOTIFY_NEW_BLOCK (2001) {
    b {
        block:                blob          // Serialised block
        txs:                  []blob        // Serialised transactions
    }
    current_blockchain_height: uint64
}
```

This is a notification (no response expected). The receiving node validates the block and relays it to its own peers.

## Transaction Propagation

```
NOTIFY_OR_INVOKE_NEW_TRANSACTIONS (2002) {
    txs: []blob    // Serialised transaction blobs
}
```

Up to `CURRENCY_RELAY_TXS_MAX_COUNT` (5) transactions per message.

## Chain Synchronisation

### Requesting the Chain Skeleton

To sync, a node sends its known block IDs in a sparse pattern:

```
NOTIFY_REQUEST_CHAIN (2006) {
    block_ids: []hash    // Sparse: first 10 sequential, then 2^n offsets, genesis last
}
```

The Go implementation uses `SparseChainHistory()` which matches the C++ `get_short_chain_history()` algorithm:
- The 10 most recent block hashes
- Then every 2nd, 4th, 8th, 16th, 32nd, etc.
- Always ending with the genesis block hash

### Chain Response

```
NOTIFY_RESPONSE_CHAIN_ENTRY (2007) {
    start_height:  uint64
    total_height:  uint64
    m_block_ids:   []block_context_info
}
```

### Fetching Blocks

```
NOTIFY_REQUEST_GET_OBJECTS (2003) {
    txs:    []hash    // Transaction hashes to fetch
    blocks: []hash    // Block hashes to fetch
}

NOTIFY_RESPONSE_GET_OBJECTS (2004) {
    txs:                      []blob
    blocks:                   []block_complete_entry
    missed_ids:               []hash
    current_blockchain_height: uint64
}
```

### P2P Sync State Machine

The `chain.P2PSync()` function implements the full sync loop:

1. Build sparse chain history from local chain state.
2. Send `REQUEST_CHAIN` to the peer.
3. Receive `RESPONSE_CHAIN_ENTRY` with block hashes.
4. Fetch blocks in batches via `REQUEST_GET_OBJECTS`.
5. Validate and store blocks through `processBlockBlobs()`.
6. Repeat until the peer has no more blocks.

The first block in each `REQUEST_GET_OBJECTS` response overlaps with the last known block (to confirm chain continuity) and is skipped during processing.

### Sync Limits

| Parameter | Value |
|-----------|-------|
| Block IDs per chain request | 2,000 |
| Blocks per download batch | 200 |
| Max bytes per sync packet | 2 MB |
| Max blocks per get_objects | 500 |
| Max txs per get_objects | 500 |

## Connection Timeouts

| Parameter | Value |
|-----------|-------|
| TCP connection | 5,000 ms |
| Ping | 2,000 ms |
| Command invoke | 120,000 ms (2 min) |
| Handshake | 10,000 ms |
| Max packet size | 50 MB |

## Network Ports

| Network | P2P | RPC | Stratum |
|---------|-----|-----|---------|
| Mainnet | 36942 | 36941 | 36940 |
| Testnet | 46942 | 46941 | 46940 |
