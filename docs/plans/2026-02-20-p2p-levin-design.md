# Phase 3 Design: P2P Levin Protocol

## Summary

Phase 3 adds the CryptoNote Levin binary protocol for peer-to-peer
communication, enabling go-blockchain nodes to connect to, handshake with, and
relay blocks/transactions to the live C++ daemon network.

Work spans two repositories:

- **go-p2p** gains a `levin/` sub-package with the wire format (header,
  portable storage) and a raw TCP transport implementing a new `Transport`
  interface extracted from the existing WebSocket transport.
- **go-blockchain** gains a `p2p/` package with command handlers (handshake,
  timed sync, ping, block/tx relay) and the CORE_SYNC_DATA type.

## Decisions

| Question | Decision |
|----------|----------|
| Where does Levin live? | In go-p2p as a new transport alongside WebSocket |
| Scope | Full P2P stack: wire + storage + commands + relay |
| Interop target | Test against the C++ testnet daemon on localhost:46941 |
| Transport architecture | Extract a Transport interface; Levin implements it |
| Implementation order | Bottom-up: header → storage → transport → commands → relay → integration |

## Package Structure

### go-p2p

```
node/
  transport.go           -- extract Transport interface from existing code
  levin/
    header.go            -- bucket_head2 encode/decode (33 bytes)
    header_test.go
    storage.go           -- portable storage encode/decode (epee KV)
    storage_test.go
    transport.go         -- LevinTransport (raw TCP, implements Transport)
    transport_test.go
    connection.go        -- LevinConnection (framed read/write over net.Conn)
    connection_test.go
```

### go-blockchain

```
p2p/
  sync.go                -- CORE_SYNC_DATA type + portable storage mapping
  handshake.go           -- COMMAND_HANDSHAKE (ID 1001)
  timedsync.go           -- COMMAND_TIMED_SYNC (ID 1002)
  ping.go                -- COMMAND_PING (ID 1003)
  relay.go               -- NOTIFY_NEW_BLOCK (2001), NOTIFY_NEW_TRANSACTIONS (2002)
  objects.go             -- REQUEST/RESPONSE_GET_OBJECTS (2003/2004)
  chain.go               -- REQUEST_CHAIN (2006), RESPONSE_CHAIN_ENTRY (2007)
  node.go                -- P2P node orchestrator
  p2p_test.go            -- unit tests
  integration_test.go    -- C++ testnet integration (build tag)
```

## Levin Wire Format

### Header (33 bytes, little-endian, packed)

```
Offset  Size  Field            Notes
0       8     Signature        0x0101010101012101 (fixed)
8       8     PayloadSize      byte count of payload after header
16      1     ExpectResponse   1 = request, 0 = response/notification
17      4     Command          uint32 command ID
21      4     ReturnCode       int32 (0 = success, negative = error)
25      4     Flags            uint32 (reserved)
29      4     ProtocolVersion  uint32 (reserved)
```

Go types:

```go
type Header struct {
    PayloadSize    uint64
    ExpectResponse bool
    Command        uint32
    ReturnCode     int32
    Flags          uint32
    Version        uint32
}

func EncodeHeader(h *Header) [HeaderSize]byte
func DecodeHeader(b [HeaderSize]byte) (Header, error)
```

Validation rejects mismatched signatures and oversized payloads (default cap:
100 MB, matching the C++ `LEVIN_DEFAULT_MAX_PACKET_SIZE`).

### Return Codes

```
LEVIN_OK                       =  0
LEVIN_ERROR_CONNECTION         = -1
LEVIN_ERROR_FORMAT             = -7
LEVIN_ERROR_SIGNATURE_MISMATCH = -13
```

## Portable Storage (epee KV Serialisation)

### Storage Header (9 bytes)

```
SignatureA  uint32 LE  0x01011101
SignatureB  uint32 LE  0x01020101
Version     uint8      1
```

### Varint Encoding

Different from CryptoNote wire varints. The low 2 bits of the first byte
indicate the total encoded size:

| Bits [1:0] | Total bytes | Max value |
|------------|-------------|-----------|
| 00         | 1           | 63 |
| 01         | 2           | 16,383 |
| 10         | 4           | 1,073,741,823 |
| 11         | 8           | 4,611,686,018,427,387,903 |

Encoding: shift value left 2 bits, OR in size mark, write little-endian.
Decoding: read bytes, shift right 2 bits.

### Type Tags

```
INT64   = 1   INT32  = 2   INT16  = 3   INT8   = 4
UINT64  = 5   UINT32 = 6   UINT16 = 7   UINT8  = 8
DOUBLE  = 9   STRING = 10  BOOL   = 11  OBJECT = 12
ARRAY   = 13

ARRAY_FLAG = 0x80  (ORed with element type for typed arrays)
```

### Section Encoding

```
[varint]  entry count
For each entry:
  [uint8]   name length (1-255)
  [bytes]   name (UTF-8)
  [uint8]   type tag
  [value]   type-dependent payload
```

### Go API

```go
type Section map[string]Value

type Value struct {
    Type   uint8
    Int64  int64
    Uint64 uint64
    Int32  int32
    Uint32 uint32
    Int16  int16
    Uint16 uint16
    Int8   int8
    Uint8  uint8
    Bool   bool
    Str    []byte
    Obj    Section
    Array  []Value
}

func Encode(s Section) ([]byte, error)
func Decode(data []byte) (Section, error)
```

POD-as-blob fields (hashes, keys, peerlist entries) are serialised as strings
containing raw bytes. Conversion between Go types and blobs happens in the
command layer, not in the storage layer.

## Transport Interface (go-p2p refactor)

Extract from the existing concrete `Transport` struct:

```go
type Transport interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Connect(ctx context.Context, addr string) (Connection, error)
    Send(peerID string, data []byte) error
    Broadcast(data []byte) error
    OnMessage(handler func(peerID string, data []byte))
    GetConnection(peerID string) Connection
}

type Connection interface {
    Send(data []byte) error
    Close() error
    RemoteAddr() string
    PeerID() string
}
```

The existing WebSocket code becomes `WSTransport` implementing `Transport`.
The new Levin code becomes `LevinTransport` implementing `Transport`.

## Levin Transport

Raw TCP transport for CryptoNote P2P communication:

```go
type LevinTransport struct {
    listener net.Listener
    conns    map[string]*LevinConnection
    handler  func(peerID string, data []byte)
    config   LevinConfig
    // ...
}

type LevinConfig struct {
    ListenAddr     string        // ":36940" (mainnet) or ":46940" (testnet)
    MaxPayloadSize uint64        // 100 MB default
    DialTimeout    time.Duration // 10s default
    ReadTimeout    time.Duration // 120s default
}
```

### LevinConnection

Wraps `net.Conn` with framed I/O:

```go
type LevinConnection struct {
    conn     net.Conn
    peerID   string
    lastSeen time.Time
    mu       sync.Mutex
}

func (c *LevinConnection) WritePacket(cmd uint32, payload []byte, expectResp bool) error
func (c *LevinConnection) ReadPacket() (Header, []byte, error)
```

### Connection Lifecycle

1. TCP dial to peer address
2. Send COMMAND_HANDSHAKE request with node_data + CORE_SYNC_DATA
3. Receive handshake response with peer's node_data + CORE_SYNC_DATA + peerlist
4. Record peer ID from response
5. Start read loop: read 33-byte header, read payload, dispatch to handler
6. Periodic COMMAND_TIMED_SYNC (every 60s)

## Command Handlers (go-blockchain)

### Command IDs

```
P2P_COMMANDS_POOL_BASE = 1000
BC_COMMANDS_POOL_BASE  = 2000

COMMAND_HANDSHAKE           = 1001
COMMAND_TIMED_SYNC          = 1002
COMMAND_PING                = 1003

NOTIFY_NEW_BLOCK            = 2001
NOTIFY_NEW_TRANSACTIONS     = 2002
NOTIFY_REQUEST_GET_OBJECTS  = 2003
NOTIFY_RESPONSE_GET_OBJECTS = 2004
NOTIFY_REQUEST_CHAIN        = 2006
NOTIFY_RESPONSE_CHAIN_ENTRY = 2007
```

### CORE_SYNC_DATA

Exchanged in handshake and timed_sync payloads:

```go
type CoreSyncData struct {
    CurrentHeight        uint64     // "current_height"
    TopID                types.Hash // "top_id" (POD as blob)
    LastCheckpointHeight uint64     // "last_checkpoint_height"
    CoreTime             uint64     // "core_time"
    ClientVersion        string     // "client_version"
    NonPruningMode       bool       // "non_pruning_mode_enabled"
}
```

### Handshake (1001)

**Request:**
```
"node_data" → {
    "network_id" → 16-byte UUID (POD as blob)
    "peer_id"    → uint64
    "local_time" → int64
    "my_port"    → uint32
}
"payload_data" → CoreSyncData
```

**Response:** same fields plus `"local_peerlist"` (array of 24-byte packed
peerlist entries as blob).

### Timed Sync (1002)

**Request:** `"payload_data"` (CoreSyncData)
**Response:** `"local_time"`, `"payload_data"`, `"local_peerlist"`

### Ping (1003)

**Request:** empty section
**Response:** `"status"` = `"OK"`, `"peer_id"` = uint64

### Block Relay (2001)

Notification carrying a complete block with transactions:

```
"b" → {
    "block" → string (serialised block blob)
    "txs"   → array of strings (transaction blobs)
}
"current_blockchain_height" → uint64
```

### Transaction Relay (2002)

Notification: `"txs"` = array of transaction blob strings.

### Object Request/Response (2003/2004)

Request: `"blocks"` and `"txs"` as arrays of 32-byte hash blobs.
Response: `"blocks"` (block_complete_entry objects), `"txs"` (blobs),
`"missed_ids"` (hash blobs), `"current_blockchain_height"`.

### Chain Request/Response (2006/2007)

Request: `"block_ids"` = array of hash blobs (first 10 sequential, then
powers-of-2 offsets, last is genesis).
Response: `"start_height"`, `"total_height"`, `"m_block_ids"`.

## Network Identity

The Lethean network ID (16-byte UUID) from `currency_config.h.in`:

```
Mainnet: {0x11, 0x10, 0x01, 0x11, 0x11, 0x00, 0x01, 0x01,
          0x01, 0x01, 0x00, 0x01, 0x01, 0x11, 0x01, 0x00}

Testnet: {0x11, 0x10, 0x01, 0x11, 0x11, 0x00, 0x01, 0x01,
          0x01, 0x01, 0x00, 0x01, 0x01, 0x11, 0x01, 0x01}
```

These are validated during handshake — a peer with the wrong network ID is
rejected.

## Testing Strategy

### Unit Tests (Go-to-Go)

- Levin header encode/decode round-trip with known bytes
- Portable storage varint round-trip across all size ranges
- Portable storage encode/decode for all 12 type tags
- Section nesting (object within object)
- Array encoding for typed arrays
- Command payload round-trips (CoreSyncData, handshake, ping)

### Integration Test (C++ testnet)

Build-tagged `//go:build integration` test that:

1. TCP connects to `localhost:46941` (testnet P2P port)
2. Sends COMMAND_HANDSHAKE with correct network ID and CoreSyncData
3. Verifies handshake response contains valid peer data
4. Sends COMMAND_PING, verifies `"OK"` response
5. Sends COMMAND_TIMED_SYNC, verifies updated chain state

This is the Phase 3 equivalent of the genesis block hash test — if handshake
succeeds with the C++ daemon, the entire wire format stack is correct.

## C++ Reference Files

- `contrib/epee/include/net/levin_base.h` — header struct, constants
- `contrib/epee/include/storages/portable_storage_from_bin.h` — storage decoder
- `contrib/epee/include/storages/portable_storage_to_bin.h` — storage encoder
- `src/p2p/p2p_protocol_defs.h` — P2P command definitions
- `src/currency_protocol/currency_protocol_defs.h` — blockchain command definitions
- `src/currency_core/currency_config.h.in` — network ID, ports
