# Phase 3: P2P Levin Protocol Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the CryptoNote Levin binary protocol so go-blockchain nodes can handshake with and relay blocks/transactions to the live C++ daemon network.

**Architecture:** Work spans two repos. `go-p2p` gains a `node/levin/` sub-package with the wire format (header, portable storage, framed TCP connection). `go-blockchain` gains a `p2p/` package with command handlers and CORE_SYNC_DATA types. Bottom-up: wire format → storage → connection → commands → integration test.

**Tech Stack:** Go 1.25, raw TCP (`net` stdlib), no new dependencies in go-p2p. go-blockchain adds `forge.lthn.ai/core/go-p2p` as a module dependency.

**Design doc:** `docs/plans/2026-02-20-p2p-levin-design.md`

---

## Part 1: go-p2p — Levin Wire Format

**Working directory:** `/home/claude/Code/core/go-p2p/`

All Part 1 tasks create files under `node/levin/`.

---

### Task 1: Levin Header Encode/Decode

**Files:**
- Create: `node/levin/header.go`
- Create: `node/levin/header_test.go`

**Context:**

The Levin protocol wraps every message in a 33-byte packed header (little-endian):

```
Offset  Size  Field            Type
0       8     Signature        uint64 LE = 0x0101010101012101
8       8     PayloadSize      uint64 LE
16      1     ExpectResponse   bool (0x01 = request, 0x00 = response/notification)
17      4     Command          uint32 LE
21      4     ReturnCode       int32 LE (0 = success, negative = error)
25      4     Flags            uint32 LE (reserved, set to 0)
29      4     ProtocolVersion  uint32 LE (reserved, set to 0)
```

Total: 8+8+1+4+4+4+4 = 33 bytes.

**Step 1: Write the failing test**

Create `node/levin/header_test.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package levin

import (
	"encoding/binary"
	"testing"
)

func TestEncodeHeader_Good_KnownBytes(t *testing.T) {
	h := Header{
		PayloadSize:    42,
		ExpectResponse: true,
		Command:        1001, // handshake
		ReturnCode:     0,
	}
	got := EncodeHeader(&h)

	// Signature at offset 0
	sig := binary.LittleEndian.Uint64(got[0:8])
	if sig != Signature {
		t.Fatalf("signature: got %#x, want %#x", sig, Signature)
	}

	// Payload size at offset 8
	cb := binary.LittleEndian.Uint64(got[8:16])
	if cb != 42 {
		t.Fatalf("payload size: got %d, want 42", cb)
	}

	// ExpectResponse at offset 16
	if got[16] != 1 {
		t.Fatalf("expect_response: got %d, want 1", got[16])
	}

	// Command at offset 17
	cmd := binary.LittleEndian.Uint32(got[17:21])
	if cmd != 1001 {
		t.Fatalf("command: got %d, want 1001", cmd)
	}

	// ReturnCode at offset 21
	rc := int32(binary.LittleEndian.Uint32(got[21:25]))
	if rc != 0 {
		t.Fatalf("return_code: got %d, want 0", rc)
	}
}

func TestDecodeHeader_Good_Roundtrip(t *testing.T) {
	original := Header{
		PayloadSize:    65535,
		ExpectResponse: false,
		Command:        2001, // NOTIFY_NEW_BLOCK
		ReturnCode:     -7,   // LEVIN_ERROR_FORMAT
	}
	encoded := EncodeHeader(&original)
	decoded, err := DecodeHeader(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.PayloadSize != original.PayloadSize {
		t.Errorf("payload_size: got %d, want %d", decoded.PayloadSize, original.PayloadSize)
	}
	if decoded.ExpectResponse != original.ExpectResponse {
		t.Errorf("expect_response: got %v, want %v", decoded.ExpectResponse, original.ExpectResponse)
	}
	if decoded.Command != original.Command {
		t.Errorf("command: got %d, want %d", decoded.Command, original.Command)
	}
	if decoded.ReturnCode != original.ReturnCode {
		t.Errorf("return_code: got %d, want %d", decoded.ReturnCode, original.ReturnCode)
	}
}

func TestDecodeHeader_Bad_Signature(t *testing.T) {
	var buf [HeaderSize]byte
	binary.LittleEndian.PutUint64(buf[0:8], 0xDEADBEEF) // wrong signature
	_, err := DecodeHeader(buf)
	if err == nil {
		t.Fatal("expected error for bad signature")
	}
}

func TestHeaderSize_Is33(t *testing.T) {
	if HeaderSize != 33 {
		t.Fatalf("HeaderSize: got %d, want 33", HeaderSize)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./node/levin/`
Expected: FAIL — package does not exist yet

**Step 3: Write the implementation**

Create `node/levin/header.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

// Package levin implements the CryptoNote Levin binary protocol.
//
// The Levin protocol is the peer-to-peer wire format used by CryptoNote-based
// blockchains. Every message is framed with a 33-byte header containing a
// fixed signature, payload size, command ID, and return code. Payloads are
// serialised using the epee portable storage format.
package levin

import (
	"encoding/binary"
	"errors"
)

// HeaderSize is the size of a Levin packet header in bytes.
const HeaderSize = 33

// Signature is the magic value at the start of every Levin packet.
const Signature uint64 = 0x0101010101012101

// MaxPayloadSize is the default maximum payload size (100 MB).
const MaxPayloadSize uint64 = 100 * 1024 * 1024

// Return codes.
const (
	ReturnOK                = 0
	ReturnErrConnection     = -1
	ReturnErrFormat         = -7
	ReturnErrSignature      = -13
)

// Command IDs — P2P layer (1000 series).
const (
	CommandHandshake  uint32 = 1001
	CommandTimedSync  uint32 = 1002
	CommandPing       uint32 = 1003
)

// Command IDs — blockchain layer (2000 series).
const (
	CommandNewBlock         uint32 = 2001
	CommandNewTransactions  uint32 = 2002
	CommandRequestObjects   uint32 = 2003
	CommandResponseObjects  uint32 = 2004
	CommandRequestChain     uint32 = 2006
	CommandResponseChain    uint32 = 2007
)

var (
	ErrBadSignature  = errors.New("levin: invalid header signature")
	ErrPayloadTooBig = errors.New("levin: payload exceeds maximum size")
)

// Header is a Levin packet header.
type Header struct {
	PayloadSize    uint64
	ExpectResponse bool
	Command        uint32
	ReturnCode     int32
	Flags          uint32
	Version        uint32
}

// EncodeHeader serialises a Header into a 33-byte packed buffer.
func EncodeHeader(h *Header) [HeaderSize]byte {
	var buf [HeaderSize]byte
	binary.LittleEndian.PutUint64(buf[0:8], Signature)
	binary.LittleEndian.PutUint64(buf[8:16], h.PayloadSize)
	if h.ExpectResponse {
		buf[16] = 1
	}
	binary.LittleEndian.PutUint32(buf[17:21], h.Command)
	binary.LittleEndian.PutUint32(buf[21:25], uint32(h.ReturnCode))
	binary.LittleEndian.PutUint32(buf[25:29], h.Flags)
	binary.LittleEndian.PutUint32(buf[29:33], h.Version)
	return buf
}

// DecodeHeader parses a 33-byte buffer into a Header.
// Returns ErrBadSignature if the signature does not match.
func DecodeHeader(buf [HeaderSize]byte) (Header, error) {
	sig := binary.LittleEndian.Uint64(buf[0:8])
	if sig != Signature {
		return Header{}, ErrBadSignature
	}
	return Header{
		PayloadSize:    binary.LittleEndian.Uint64(buf[8:16]),
		ExpectResponse: buf[16] != 0,
		Command:        binary.LittleEndian.Uint32(buf[17:21]),
		ReturnCode:     int32(binary.LittleEndian.Uint32(buf[21:25])),
		Flags:          binary.LittleEndian.Uint32(buf[25:29]),
		Version:        binary.LittleEndian.Uint32(buf[29:33]),
	}, nil
}
```

**Step 4: Run tests**

Run: `go test -race -v ./node/levin/`
Expected: PASS (4 tests)

Run: `go vet ./node/levin/`
Expected: clean

**Step 5: Commit**

```bash
git add node/levin/header.go node/levin/header_test.go
git commit -m "feat(levin): header encode/decode (33-byte Levin packet framing)

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 2: Portable Storage Varint

**Files:**
- Create: `node/levin/varint.go`
- Create: `node/levin/varint_test.go`

**Context:**

The epee portable storage uses a DIFFERENT varint encoding from CryptoNote's
wire varint (which uses 7-bit LEB128). The portable storage varint packs the
size hint into the lowest 2 bits of the first byte:

```
Bits [1:0]  Total bytes  Max value
00          1            63
01          2            16,383
10          4            1,073,741,823
11          8            4,611,686,018,427,387,903
```

Encode: `raw = (value << 2) | size_mark`; write as little-endian.
Decode: read appropriate bytes; `value = raw >> 2`.

**Step 1: Write the failing test**

Create `node/levin/varint_test.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package levin

import (
	"bytes"
	"testing"
)

func TestPackVarint_Good_OneByte(t *testing.T) {
	// Value 5 → (5 << 2) | 0 = 20 = 0x14, fits in 1 byte
	got := PackVarint(5)
	if len(got) != 1 || got[0] != 0x14 {
		t.Fatalf("pack(5): got %x, want [14]", got)
	}
}

func TestPackVarint_Good_TwoByte(t *testing.T) {
	// Value 100 → (100 << 2) | 1 = 401 = 0x0191
	// Little-endian: [0x91, 0x01]
	got := PackVarint(100)
	if len(got) != 2 || got[0] != 0x91 || got[1] != 0x01 {
		t.Fatalf("pack(100): got %x, want [91 01]", got)
	}
}

func TestPackVarint_Good_FourByte(t *testing.T) {
	// Value 65536 → (65536 << 2) | 2 = 262146 = 0x00040002
	// Little-endian: [0x02, 0x00, 0x04, 0x00]
	got := PackVarint(65536)
	if len(got) != 4 {
		t.Fatalf("pack(65536): got %d bytes, want 4", len(got))
	}
	want := []byte{0x02, 0x00, 0x04, 0x00}
	if !bytes.Equal(got, want) {
		t.Fatalf("pack(65536): got %x, want %x", got, want)
	}
}

func TestPackVarint_Good_EightByte(t *testing.T) {
	// Value > 1,073,741,823 → 8-byte encoding
	v := uint64(2_000_000_000)
	got := PackVarint(v)
	if len(got) != 8 {
		t.Fatalf("pack(%d): got %d bytes, want 8", v, len(got))
	}
	// Low 2 bits must be 11 (0x03)
	if got[0]&0x03 != 0x03 {
		t.Fatalf("pack(%d): low 2 bits = %d, want 3", v, got[0]&0x03)
	}
}

func TestPackVarint_Good_Zero(t *testing.T) {
	got := PackVarint(0)
	if len(got) != 1 || got[0] != 0x00 {
		t.Fatalf("pack(0): got %x, want [00]", got)
	}
}

func TestPackVarint_Good_MaxOneByte(t *testing.T) {
	// Max 1-byte value = 63
	got := PackVarint(63)
	if len(got) != 1 {
		t.Fatalf("pack(63): got %d bytes, want 1", len(got))
	}
}

func TestPackVarint_Good_MinTwoByte(t *testing.T) {
	// 64 requires 2 bytes
	got := PackVarint(64)
	if len(got) != 2 {
		t.Fatalf("pack(64): got %d bytes, want 2", len(got))
	}
}

func TestUnpackVarint_Good_Roundtrip(t *testing.T) {
	values := []uint64{0, 1, 63, 64, 100, 16383, 16384, 1073741823, 1073741824, 4611686018427387903}
	for _, v := range values {
		encoded := PackVarint(v)
		decoded, n, err := UnpackVarint(encoded)
		if err != nil {
			t.Fatalf("unpack(%d): %v", v, err)
		}
		if n != len(encoded) {
			t.Fatalf("unpack(%d): consumed %d bytes, want %d", v, n, len(encoded))
		}
		if decoded != v {
			t.Fatalf("roundtrip(%d): got %d", v, decoded)
		}
	}
}

func TestUnpackVarint_Bad_Empty(t *testing.T) {
	_, _, err := UnpackVarint(nil)
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestUnpackVarint_Bad_TooShort(t *testing.T) {
	// Size mark says 2 bytes, but only 1 byte provided
	_, _, err := UnpackVarint([]byte{0x01}) // low bits = 01 → need 2 bytes
	if err == nil {
		t.Fatal("expected error for truncated input")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./node/levin/ -run Varint`
Expected: FAIL — undefined: PackVarint, UnpackVarint

**Step 3: Write the implementation**

Create `node/levin/varint.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package levin

import (
	"encoding/binary"
	"errors"
)

// Portable storage varint encoding constants.
// The lowest 2 bits of the first byte indicate the total encoded size.
const (
	varintMask   = 0x03
	varintByte   = 0x00 // 1 byte,  max 63
	varintWord   = 0x01 // 2 bytes, max 16,383
	varintDword  = 0x02 // 4 bytes, max 1,073,741,823
	varintQword  = 0x03 // 8 bytes, max 4,611,686,018,427,387,903

	varintMaxByte  = 63
	varintMaxWord  = 16383
	varintMaxDword = 1073741823
)

var errVarintTruncated = errors.New("levin: varint truncated")

// PackVarint encodes a uint64 as a portable storage varint.
func PackVarint(v uint64) []byte {
	switch {
	case v <= varintMaxByte:
		return []byte{byte(v<<2) | varintByte}
	case v <= varintMaxWord:
		var buf [2]byte
		binary.LittleEndian.PutUint16(buf[:], uint16(v<<2)|uint16(varintWord))
		return buf[:]
	case v <= varintMaxDword:
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], uint32(v<<2)|uint32(varintDword))
		return buf[:]
	default:
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], (v<<2)|uint64(varintQword))
		return buf[:]
	}
}

// UnpackVarint decodes a portable storage varint from buf.
// Returns the value, the number of bytes consumed, and any error.
func UnpackVarint(buf []byte) (uint64, int, error) {
	if len(buf) == 0 {
		return 0, 0, errVarintTruncated
	}
	mark := buf[0] & varintMask
	switch mark {
	case varintByte:
		return uint64(buf[0]) >> 2, 1, nil
	case varintWord:
		if len(buf) < 2 {
			return 0, 0, errVarintTruncated
		}
		return uint64(binary.LittleEndian.Uint16(buf[:2])) >> 2, 2, nil
	case varintDword:
		if len(buf) < 4 {
			return 0, 0, errVarintTruncated
		}
		return uint64(binary.LittleEndian.Uint32(buf[:4])) >> 2, 4, nil
	default: // varintQword
		if len(buf) < 8 {
			return 0, 0, errVarintTruncated
		}
		return binary.LittleEndian.Uint64(buf[:8]) >> 2, 8, nil
	}
}
```

**Step 4: Run tests**

Run: `go test -race -v ./node/levin/ -run Varint`
Expected: PASS (10 tests)

Run: `go vet ./node/levin/`
Expected: clean

**Step 5: Commit**

```bash
git add node/levin/varint.go node/levin/varint_test.go
git commit -m "feat(levin): portable storage varint encode/decode

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 3: Portable Storage Section Encode/Decode

**Files:**
- Create: `node/levin/storage.go`
- Create: `node/levin/storage_test.go`

**Context:**

Every Levin payload is a portable storage blob. Format:

```
[9 bytes]  Storage header (SignatureA=0x01011101, SignatureB=0x01020101, Ver=1)
[section]  Root section
```

A section is:
```
[varint]   Entry count
For each entry:
  [uint8]   Name length
  [bytes]   Name (UTF-8)
  [uint8]   Type tag (see below)
  [value]   Type-dependent data
```

Type tags:
```
INT64=1  INT32=2  INT16=3  INT8=4  UINT64=5  UINT32=6  UINT16=7  UINT8=8
DOUBLE=9  STRING=10  BOOL=11  OBJECT=12  ARRAY_FLAG=0x80
```

Arrays use `type_tag | 0x80` followed by `varint(count)` then packed elements.

Strings are `varint(length)` then raw bytes.

Objects (nested sections) are recursively encoded sections.

**Reference files (do NOT modify, read-only):**
- `~/Code/LetheanNetwork/blockchain/contrib/epee/include/storages/portable_storage_to_bin.h`
- `~/Code/LetheanNetwork/blockchain/contrib/epee/include/storages/portable_storage_from_bin.h`

**Step 1: Write the failing test**

Create `node/levin/storage_test.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package levin

import (
	"bytes"
	"testing"
)

func TestStorageEncode_Good_EmptySection(t *testing.T) {
	s := Section{}
	data, err := EncodeStorage(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	// 9-byte header + varint(0) = 10 bytes
	if len(data) != 10 {
		t.Fatalf("len: got %d, want 10", len(data))
	}
	// Check signatures
	if data[0] != 0x01 || data[1] != 0x11 || data[2] != 0x01 || data[3] != 0x01 {
		t.Fatalf("sig_a: got %x", data[0:4])
	}
	if data[4] != 0x01 || data[5] != 0x01 || data[6] != 0x02 || data[7] != 0x01 {
		t.Fatalf("sig_b: got %x", data[4:8])
	}
	if data[8] != 1 {
		t.Fatalf("version: got %d, want 1", data[8])
	}
	// Entry count = 0
	if data[9] != 0x00 {
		t.Fatalf("count: got %x, want 00", data[9])
	}
}

func TestStorageDecode_Good_Roundtrip_Primitives(t *testing.T) {
	s := Section{
		"u64": Uint64Val(12345),
		"u32": Uint32Val(42),
		"u16": Uint16Val(256),
		"u8":  Uint8Val(7),
		"i64": Int64Val(-100),
		"i32": Int32Val(-1),
		"b":   BoolVal(true),
		"str": StringVal([]byte("hello")),
	}
	data, err := EncodeStorage(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := DecodeStorage(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if v, _ := got["u64"].AsUint64(); v != 12345 {
		t.Errorf("u64: got %d, want 12345", v)
	}
	if v, _ := got["u32"].AsUint32(); v != 42 {
		t.Errorf("u32: got %d, want 42", v)
	}
	if v, _ := got["u16"].AsUint16(); v != 256 {
		t.Errorf("u16: got %d, want 256", v)
	}
	if v, _ := got["u8"].AsUint8(); v != 7 {
		t.Errorf("u8: got %d, want 7", v)
	}
	if v, _ := got["i64"].AsInt64(); v != -100 {
		t.Errorf("i64: got %d, want -100", v)
	}
	if v, _ := got["i32"].AsInt32(); v != -1 {
		t.Errorf("i32: got %d, want -1", v)
	}
	if v, _ := got["b"].AsBool(); v != true {
		t.Errorf("bool: got %v, want true", v)
	}
	if v, _ := got["str"].AsString(); !bytes.Equal(v, []byte("hello")) {
		t.Errorf("str: got %q, want %q", v, "hello")
	}
}

func TestStorageDecode_Good_Roundtrip_NestedObject(t *testing.T) {
	inner := Section{
		"depth": Uint32Val(1),
	}
	outer := Section{
		"name":  StringVal([]byte("test")),
		"child": ObjectVal(inner),
	}
	data, err := EncodeStorage(outer)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := DecodeStorage(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	child, err := got["child"].AsSection()
	if err != nil {
		t.Fatalf("child: %v", err)
	}
	if v, _ := child["depth"].AsUint32(); v != 1 {
		t.Errorf("depth: got %d, want 1", v)
	}
}

func TestStorageDecode_Good_Roundtrip_Array(t *testing.T) {
	s := Section{
		"nums": Uint64ArrayVal([]uint64{10, 20, 30}),
	}
	data, err := EncodeStorage(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := DecodeStorage(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	arr, err := got["nums"].AsUint64Array()
	if err != nil {
		t.Fatalf("nums: %v", err)
	}
	if len(arr) != 3 || arr[0] != 10 || arr[1] != 20 || arr[2] != 30 {
		t.Errorf("nums: got %v, want [10 20 30]", arr)
	}
}

func TestStorageDecode_Good_Roundtrip_StringArray(t *testing.T) {
	s := Section{
		"tags": StringArrayVal([][]byte{[]byte("foo"), []byte("bar")}),
	}
	data, err := EncodeStorage(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := DecodeStorage(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	arr, err := got["tags"].AsStringArray()
	if err != nil {
		t.Fatalf("tags: %v", err)
	}
	if len(arr) != 2 || !bytes.Equal(arr[0], []byte("foo")) || !bytes.Equal(arr[1], []byte("bar")) {
		t.Errorf("tags: got %v", arr)
	}
}

func TestStorageDecode_Good_Roundtrip_ObjectArray(t *testing.T) {
	objs := []Section{
		{"id": Uint32Val(1)},
		{"id": Uint32Val(2)},
	}
	s := Section{
		"items": ObjectArrayVal(objs),
	}
	data, err := EncodeStorage(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := DecodeStorage(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	arr, err := got["items"].AsSectionArray()
	if err != nil {
		t.Fatalf("items: %v", err)
	}
	if len(arr) != 2 {
		t.Fatalf("items: got %d elements, want 2", len(arr))
	}
	if v, _ := arr[0]["id"].AsUint32(); v != 1 {
		t.Errorf("items[0].id: got %d, want 1", v)
	}
	if v, _ := arr[1]["id"].AsUint32(); v != 2 {
		t.Errorf("items[1].id: got %d, want 2", v)
	}
}

func TestStorageDecode_Bad_WrongSignature(t *testing.T) {
	_, err := DecodeStorage([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01})
	if err == nil {
		t.Fatal("expected error for wrong signature")
	}
}

func TestStorageDecode_Bad_TooShort(t *testing.T) {
	_, err := DecodeStorage([]byte{0x01, 0x11})
	if err == nil {
		t.Fatal("expected error for truncated header")
	}
}

func TestStorageEncode_Good_ByteIdentical(t *testing.T) {
	// Encode → decode → re-encode must produce identical bytes
	s := Section{
		"version": Uint8Val(1),
		"data": ObjectVal(Section{
			"height": Uint64Val(100),
			"hash":   StringVal(make([]byte, 32)),
		}),
	}
	data1, _ := EncodeStorage(s)
	decoded, _ := DecodeStorage(data1)
	data2, _ := EncodeStorage(decoded)
	if !bytes.Equal(data1, data2) {
		t.Fatalf("re-encode mismatch:\n  first:  %x\n  second: %x", data1, data2)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./node/levin/ -run Storage`
Expected: FAIL — undefined functions

**Step 3: Write the implementation**

Create `node/levin/storage.go`. This is the largest file. Key structures:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package levin

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
)

// Portable storage signatures.
const (
	storageSignatureA uint32 = 0x01011101
	storageSignatureB uint32 = 0x01020101
	storageVersion    uint8  = 1
	storageHeaderSize        = 9
)

// Type tags for portable storage values.
const (
	TypeInt64  uint8 = 1
	TypeInt32  uint8 = 2
	TypeInt16  uint8 = 3
	TypeInt8   uint8 = 4
	TypeUint64 uint8 = 5
	TypeUint32 uint8 = 6
	TypeUint16 uint8 = 7
	TypeUint8  uint8 = 8
	TypeDouble uint8 = 9
	TypeString uint8 = 10
	TypeBool   uint8 = 11
	TypeObject uint8 = 12

	TypeArrayFlag uint8 = 0x80
)

var (
	errStorageTruncated = errors.New("levin: storage data truncated")
	errStorageBadSig    = errors.New("levin: storage signature mismatch")
	errStorageBadVer    = errors.New("levin: storage version unsupported")
	errStorageBadType   = errors.New("levin: unknown type tag")
	errStorageTypeMismatch = errors.New("levin: value type mismatch")
)

// Section is an ordered map of field names to values.
// Field order is preserved via sorted keys for deterministic encoding.
type Section map[string]Value

// Value is a portable storage value. The Type field indicates which data field
// is populated. For arrays, Type has TypeArrayFlag set and the Array field
// holds the elements.
type Value struct {
	Type   uint8
	data   [8]byte   // scalar storage (up to 8 bytes)
	str    []byte     // string data
	obj    Section    // nested object
	array  []Value    // array elements
}

// --- Constructors ---

func Uint64Val(v uint64) Value {
	var val Value
	val.Type = TypeUint64
	binary.LittleEndian.PutUint64(val.data[:], v)
	return val
}

func Uint32Val(v uint32) Value {
	var val Value
	val.Type = TypeUint32
	binary.LittleEndian.PutUint32(val.data[:], v)
	return val
}

func Uint16Val(v uint16) Value {
	var val Value
	val.Type = TypeUint16
	binary.LittleEndian.PutUint16(val.data[:], v)
	return val
}

func Uint8Val(v uint8) Value {
	var val Value
	val.Type = TypeUint8
	val.data[0] = v
	return val
}

func Int64Val(v int64) Value {
	var val Value
	val.Type = TypeInt64
	binary.LittleEndian.PutUint64(val.data[:], uint64(v))
	return val
}

func Int32Val(v int32) Value {
	var val Value
	val.Type = TypeInt32
	binary.LittleEndian.PutUint32(val.data[:], uint32(v))
	return val
}

func Int16Val(v int16) Value {
	var val Value
	val.Type = TypeInt16
	binary.LittleEndian.PutUint16(val.data[:], uint16(v))
	return val
}

func Int8Val(v int8) Value {
	var val Value
	val.Type = TypeInt8
	val.data[0] = byte(v)
	return val
}

func BoolVal(v bool) Value {
	var val Value
	val.Type = TypeBool
	if v {
		val.data[0] = 1
	}
	return val
}

func DoubleVal(v float64) Value {
	var val Value
	val.Type = TypeDouble
	binary.LittleEndian.PutUint64(val.data[:], math.Float64bits(v))
	return val
}

func StringVal(v []byte) Value {
	return Value{Type: TypeString, str: v}
}

func ObjectVal(s Section) Value {
	return Value{Type: TypeObject, obj: s}
}

func Uint64ArrayVal(vs []uint64) Value {
	arr := make([]Value, len(vs))
	for i, v := range vs {
		arr[i] = Uint64Val(v)
	}
	return Value{Type: TypeUint64 | TypeArrayFlag, array: arr}
}

func StringArrayVal(vs [][]byte) Value {
	arr := make([]Value, len(vs))
	for i, v := range vs {
		arr[i] = StringVal(v)
	}
	return Value{Type: TypeString | TypeArrayFlag, array: arr}
}

func ObjectArrayVal(vs []Section) Value {
	arr := make([]Value, len(vs))
	for i, v := range vs {
		arr[i] = ObjectVal(v)
	}
	return Value{Type: TypeObject | TypeArrayFlag, array: arr}
}

// --- Accessors ---

func (v Value) AsUint64() (uint64, error) {
	if v.Type != TypeUint64 {
		return 0, errStorageTypeMismatch
	}
	return binary.LittleEndian.Uint64(v.data[:]), nil
}

func (v Value) AsUint32() (uint32, error) {
	if v.Type != TypeUint32 {
		return 0, errStorageTypeMismatch
	}
	return binary.LittleEndian.Uint32(v.data[:]), nil
}

func (v Value) AsUint16() (uint16, error) {
	if v.Type != TypeUint16 {
		return 0, errStorageTypeMismatch
	}
	return binary.LittleEndian.Uint16(v.data[:]), nil
}

func (v Value) AsUint8() (uint8, error) {
	if v.Type != TypeUint8 {
		return 0, errStorageTypeMismatch
	}
	return v.data[0], nil
}

func (v Value) AsInt64() (int64, error) {
	if v.Type != TypeInt64 {
		return 0, errStorageTypeMismatch
	}
	return int64(binary.LittleEndian.Uint64(v.data[:])), nil
}

func (v Value) AsInt32() (int32, error) {
	if v.Type != TypeInt32 {
		return 0, errStorageTypeMismatch
	}
	return int32(binary.LittleEndian.Uint32(v.data[:])), nil
}

func (v Value) AsBool() (bool, error) {
	if v.Type != TypeBool {
		return false, errStorageTypeMismatch
	}
	return v.data[0] != 0, nil
}

func (v Value) AsDouble() (float64, error) {
	if v.Type != TypeDouble {
		return 0, errStorageTypeMismatch
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(v.data[:])), nil
}

func (v Value) AsString() ([]byte, error) {
	if v.Type != TypeString {
		return nil, errStorageTypeMismatch
	}
	return v.str, nil
}

func (v Value) AsSection() (Section, error) {
	if v.Type != TypeObject {
		return nil, errStorageTypeMismatch
	}
	return v.obj, nil
}

func (v Value) AsUint64Array() ([]uint64, error) {
	if v.Type != TypeUint64|TypeArrayFlag {
		return nil, errStorageTypeMismatch
	}
	out := make([]uint64, len(v.array))
	for i, el := range v.array {
		out[i] = binary.LittleEndian.Uint64(el.data[:])
	}
	return out, nil
}

func (v Value) AsStringArray() ([][]byte, error) {
	if v.Type != TypeString|TypeArrayFlag {
		return nil, errStorageTypeMismatch
	}
	out := make([][]byte, len(v.array))
	for i, el := range v.array {
		out[i] = el.str
	}
	return out, nil
}

func (v Value) AsSectionArray() ([]Section, error) {
	if v.Type != TypeObject|TypeArrayFlag {
		return nil, errStorageTypeMismatch
	}
	out := make([]Section, len(v.array))
	for i, el := range v.array {
		out[i] = el.obj
	}
	return out, nil
}

// --- Encoder ---

// EncodeStorage serialises a Section into a portable storage blob.
// Field order is deterministic (sorted by key name).
func EncodeStorage(s Section) ([]byte, error) {
	var buf []byte
	// Write 9-byte header.
	var hdr [storageHeaderSize]byte
	binary.LittleEndian.PutUint32(hdr[0:4], storageSignatureA)
	binary.LittleEndian.PutUint32(hdr[4:8], storageSignatureB)
	hdr[8] = storageVersion
	buf = append(buf, hdr[:]...)
	// Write root section.
	buf = encodeSection(buf, s)
	return buf, nil
}

func encodeSection(buf []byte, s Section) []byte {
	// Sort keys for deterministic output.
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	buf = append(buf, PackVarint(uint64(len(keys)))...)
	for _, k := range keys {
		v := s[k]
		// Field name: [uint8 len][bytes]
		buf = append(buf, byte(len(k)))
		buf = append(buf, k...)
		// Type tag
		buf = append(buf, v.Type)
		// Value
		buf = encodeValue(buf, v)
	}
	return buf
}

func encodeValue(buf []byte, v Value) []byte {
	if v.Type&TypeArrayFlag != 0 {
		// Array: count + packed elements (type tag already written by caller)
		buf = append(buf, PackVarint(uint64(len(v.array)))...)
		for _, el := range v.array {
			buf = encodeValue(buf, el)
		}
		return buf
	}
	switch v.Type {
	case TypeInt64, TypeUint64, TypeDouble:
		buf = append(buf, v.data[:8]...)
	case TypeInt32, TypeUint32:
		buf = append(buf, v.data[:4]...)
	case TypeInt16, TypeUint16:
		buf = append(buf, v.data[:2]...)
	case TypeInt8, TypeUint8, TypeBool:
		buf = append(buf, v.data[0])
	case TypeString:
		buf = append(buf, PackVarint(uint64(len(v.str)))...)
		buf = append(buf, v.str...)
	case TypeObject:
		buf = encodeSection(buf, v.obj)
	}
	return buf
}

// --- Decoder ---

// DecodeStorage parses a portable storage blob into a Section.
func DecodeStorage(data []byte) (Section, error) {
	if len(data) < storageHeaderSize {
		return nil, errStorageTruncated
	}
	sigA := binary.LittleEndian.Uint32(data[0:4])
	sigB := binary.LittleEndian.Uint32(data[4:8])
	ver := data[8]
	if sigA != storageSignatureA || sigB != storageSignatureB {
		return nil, errStorageBadSig
	}
	if ver != storageVersion {
		return nil, errStorageBadVer
	}
	s, _, err := decodeSection(data[storageHeaderSize:])
	return s, err
}

func decodeSection(data []byte) (Section, int, error) {
	pos := 0
	count, n, err := UnpackVarint(data[pos:])
	if err != nil {
		return nil, 0, fmt.Errorf("section count: %w", err)
	}
	pos += n

	s := make(Section, count)
	for i := uint64(0); i < count; i++ {
		// Field name
		if pos >= len(data) {
			return nil, 0, errStorageTruncated
		}
		nameLen := int(data[pos])
		pos++
		if pos+nameLen > len(data) {
			return nil, 0, errStorageTruncated
		}
		name := string(data[pos : pos+nameLen])
		pos += nameLen

		// Type tag
		if pos >= len(data) {
			return nil, 0, errStorageTruncated
		}
		tag := data[pos]
		pos++

		// Value
		val, n, err := decodeValue(data[pos:], tag)
		if err != nil {
			return nil, 0, fmt.Errorf("field %q: %w", name, err)
		}
		pos += n
		s[name] = val
	}
	return s, pos, nil
}

func decodeValue(data []byte, tag uint8) (Value, int, error) {
	if tag&TypeArrayFlag != 0 {
		elemTag := tag & ^TypeArrayFlag
		return decodeArray(data, elemTag, tag)
	}
	return decodeScalar(data, tag)
}

func decodeScalar(data []byte, tag uint8) (Value, int, error) {
	switch tag {
	case TypeInt64, TypeUint64, TypeDouble:
		if len(data) < 8 {
			return Value{}, 0, errStorageTruncated
		}
		var v Value
		v.Type = tag
		copy(v.data[:8], data[:8])
		return v, 8, nil
	case TypeInt32, TypeUint32:
		if len(data) < 4 {
			return Value{}, 0, errStorageTruncated
		}
		var v Value
		v.Type = tag
		copy(v.data[:4], data[:4])
		return v, 4, nil
	case TypeInt16, TypeUint16:
		if len(data) < 2 {
			return Value{}, 0, errStorageTruncated
		}
		var v Value
		v.Type = tag
		copy(v.data[:2], data[:2])
		return v, 2, nil
	case TypeInt8, TypeUint8, TypeBool:
		if len(data) < 1 {
			return Value{}, 0, errStorageTruncated
		}
		var v Value
		v.Type = tag
		v.data[0] = data[0]
		return v, 1, nil
	case TypeString:
		slen, n, err := UnpackVarint(data)
		if err != nil {
			return Value{}, 0, err
		}
		if uint64(len(data)-n) < slen {
			return Value{}, 0, errStorageTruncated
		}
		str := make([]byte, slen)
		copy(str, data[n:n+int(slen)])
		return Value{Type: TypeString, str: str}, n + int(slen), nil
	case TypeObject:
		s, n, err := decodeSection(data)
		if err != nil {
			return Value{}, 0, err
		}
		return Value{Type: TypeObject, obj: s}, n, nil
	default:
		return Value{}, 0, fmt.Errorf("%w: %d", errStorageBadType, tag)
	}
}

func decodeArray(data []byte, elemTag uint8, fullTag uint8) (Value, int, error) {
	count, n, err := UnpackVarint(data)
	if err != nil {
		return Value{}, 0, err
	}
	pos := n
	arr := make([]Value, count)
	for i := uint64(0); i < count; i++ {
		val, vn, err := decodeScalar(data[pos:], elemTag)
		if err != nil {
			return Value{}, 0, fmt.Errorf("array[%d]: %w", i, err)
		}
		arr[i] = val
		pos += vn
	}
	return Value{Type: fullTag, array: arr}, pos, nil
}
```

**Important implementation notes:**

- Section uses sorted keys for deterministic encoding. This matches C++ map
  iteration order and ensures `encode → decode → encode` produces identical bytes.
- Arrays of POD types could be optimised to read/write bulk bytes, but the
  element-by-element approach is correct and simpler. Optimise later if profiling
  shows it matters.
- `Value.data` is an 8-byte inline array to avoid heap allocation for scalars.
  Strings and objects use separate fields.

**Step 4: Run tests**

Run: `go test -race -v ./node/levin/ -run Storage`
Expected: PASS (8 tests)

Run: `go vet ./node/levin/`
Expected: clean

**Step 5: Commit**

```bash
git add node/levin/storage.go node/levin/storage_test.go
git commit -m "feat(levin): portable storage section encode/decode

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 4: Levin Connection — Framed TCP I/O

**Files:**
- Create: `node/levin/connection.go`
- Create: `node/levin/connection_test.go`

**Context:**

A LevinConnection wraps a `net.Conn` and provides framed packet I/O:
- `WritePacket` builds a 33-byte header + payload and writes atomically.
- `ReadPacket` reads exactly 33 header bytes, validates, reads payload.

**Step 1: Write the failing test**

Create `node/levin/connection_test.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package levin

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func TestConnection_Good_Roundtrip(t *testing.T) {
	// Create a pipe pair to simulate a TCP connection.
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	sConn := NewConnection(server)
	cConn := NewConnection(client)

	payload := []byte("hello levin")
	done := make(chan error, 1)

	// Writer goroutine.
	go func() {
		done <- cConn.WritePacket(CommandPing, payload, true)
	}()

	// Reader.
	hdr, data, err := sConn.ReadPacket()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("write: %v", err)
	}
	if hdr.Command != CommandPing {
		t.Errorf("command: got %d, want %d", hdr.Command, CommandPing)
	}
	if !hdr.ExpectResponse {
		t.Error("expect_response: got false, want true")
	}
	if !bytes.Equal(data, payload) {
		t.Errorf("payload: got %q, want %q", data, payload)
	}
}

func TestConnection_Good_EmptyPayload(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	sConn := NewConnection(server)
	cConn := NewConnection(client)

	done := make(chan error, 1)
	go func() {
		done <- cConn.WritePacket(CommandPing, nil, false)
	}()

	hdr, data, err := sConn.ReadPacket()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	<-done
	if hdr.PayloadSize != 0 {
		t.Errorf("payload_size: got %d, want 0", hdr.PayloadSize)
	}
	if len(data) != 0 {
		t.Errorf("payload: got %d bytes, want 0", len(data))
	}
}

func TestConnection_Good_Response(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	sConn := NewConnection(server)
	cConn := NewConnection(client)

	done := make(chan error, 1)
	go func() {
		done <- cConn.WriteResponse(CommandPing, []byte("OK"), ReturnOK)
	}()

	hdr, _, err := sConn.ReadPacket()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	<-done
	if hdr.ExpectResponse {
		t.Error("response should have expect_response=false")
	}
	if hdr.ReturnCode != ReturnOK {
		t.Errorf("return_code: got %d, want %d", hdr.ReturnCode, ReturnOK)
	}
}

func TestConnection_Bad_PayloadTooBig(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	sConn := NewConnection(server)
	sConn.MaxPayloadSize = 10 // Very small limit

	// Write a valid header with PayloadSize > limit
	h := Header{PayloadSize: 100, Command: CommandPing}
	buf := EncodeHeader(&h)
	go func() { client.Write(buf[:]) }()

	_, _, err := sConn.ReadPacket()
	if err == nil {
		t.Fatal("expected error for oversized payload")
	}
}

func TestConnection_Good_SetDeadline(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	sConn := NewConnection(server)
	sConn.ReadTimeout = 50 * time.Millisecond

	// Don't write anything — should timeout
	_, _, err := sConn.ReadPacket()
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./node/levin/ -run Connection`
Expected: FAIL — undefined: NewConnection

**Step 3: Write the implementation**

Create `node/levin/connection.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package levin

import (
	"io"
	"net"
	"sync"
	"time"
)

// Connection wraps a net.Conn with Levin packet framing.
type Connection struct {
	conn           net.Conn
	writeMu        sync.Mutex
	MaxPayloadSize uint64
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
}

// NewConnection wraps an existing net.Conn.
func NewConnection(conn net.Conn) *Connection {
	return &Connection{
		conn:           conn,
		MaxPayloadSize: MaxPayloadSize,
		ReadTimeout:    120 * time.Second,
		WriteTimeout:   30 * time.Second,
	}
}

// WritePacket sends a Levin request or notification.
func (c *Connection) WritePacket(cmd uint32, payload []byte, expectResponse bool) error {
	h := Header{
		PayloadSize:    uint64(len(payload)),
		ExpectResponse: expectResponse,
		Command:        cmd,
	}
	return c.writeRaw(&h, payload)
}

// WriteResponse sends a Levin response.
func (c *Connection) WriteResponse(cmd uint32, payload []byte, returnCode int32) error {
	h := Header{
		PayloadSize: uint64(len(payload)),
		Command:     cmd,
		ReturnCode:  returnCode,
	}
	return c.writeRaw(&h, payload)
}

func (c *Connection) writeRaw(h *Header, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if c.WriteTimeout > 0 {
		c.conn.SetWriteDeadline(time.Now().Add(c.WriteTimeout))
	}
	buf := EncodeHeader(h)
	if _, err := c.conn.Write(buf[:]); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := c.conn.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

// ReadPacket reads one Levin packet from the connection.
// Returns the header and the payload bytes.
func (c *Connection) ReadPacket() (Header, []byte, error) {
	if c.ReadTimeout > 0 {
		c.conn.SetReadDeadline(time.Now().Add(c.ReadTimeout))
	}

	// Read 33-byte header.
	var buf [HeaderSize]byte
	if _, err := io.ReadFull(c.conn, buf[:]); err != nil {
		return Header{}, nil, err
	}
	hdr, err := DecodeHeader(buf)
	if err != nil {
		return Header{}, nil, err
	}
	if hdr.PayloadSize > c.MaxPayloadSize {
		return Header{}, nil, ErrPayloadTooBig
	}

	// Read payload.
	var payload []byte
	if hdr.PayloadSize > 0 {
		payload = make([]byte, hdr.PayloadSize)
		if _, err := io.ReadFull(c.conn, payload); err != nil {
			return Header{}, nil, err
		}
	}
	return hdr, payload, nil
}

// Close closes the underlying connection.
func (c *Connection) Close() error {
	return c.conn.Close()
}

// RemoteAddr returns the remote address.
func (c *Connection) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}
```

**Step 4: Run tests**

Run: `go test -race -v ./node/levin/ -run Connection`
Expected: PASS (5 tests)

Run: `go test -race ./node/levin/`
Expected: PASS (all tests)

Run: `go vet ./node/levin/`
Expected: clean

**Step 5: Commit**

```bash
git add node/levin/connection.go node/levin/connection_test.go
git commit -m "feat(levin): connection with framed TCP packet I/O

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 5: Push go-p2p

**Step 1: Run full test suite**

Run: `go test -race ./...`
Expected: PASS (all packages)

Run: `go vet ./...`
Expected: clean

**Step 2: Push**

```bash
git push origin main
```

**Note:** This makes the levin package available for go-blockchain to import.

---

## Part 2: go-blockchain — P2P Commands

**Working directory:** `/home/claude/Code/core/go-blockchain/`

---

### Task 6: Network Config + go-p2p Dependency

**Files:**
- Modify: `go.mod`
- Modify: `config/config.go`
- Modify: `config/config_test.go`

**Context:**

go-blockchain needs:
1. The go-p2p module as a dependency (for the levin package)
2. Network ID constants (16-byte UUIDs for mainnet/testnet)
3. P2P client version string

The network IDs are defined in the C++ source at
`src/p2p/net_node.inl:36`:

```cpp
const static boost::uuids::uuid P2P_NETWORK_ID = { {
    0x11, 0x10, 0x01, 0x11, 0x01, 0x01, 0x11, 0x01,
    0x10, 0x11, P2P_NETWORK_ID_TESTNET_FLAG, 0x11,
    0x01, 0x11, 0x21, P2P_NETWORK_ID_VER
} };
```

Where:
- Mainnet: `TESTNET_FLAG=0`, `VER=84` (0x54)
- Testnet: `TESTNET_FLAG=1`, `VER=100` (0x64)

**Step 1: Add go-p2p dependency**

Run: `go get forge.lthn.ai/core/go-p2p@latest`

If the Forgejo module proxy hasn't caught up yet, add a local replace:

```bash
# Temporary — remove before final commit
go mod edit -replace=forge.lthn.ai/core/go-p2p=../go-p2p
go mod tidy
```

**Step 2: Add network constants to config**

Add to `config/config.go` after the P2P ports section:

```go
// ---------------------------------------------------------------------------
// P2P network identity
// ---------------------------------------------------------------------------

// CurrencyFormationVersionMainnet is the formation version for mainnet (from cmake).
const CurrencyFormationVersionMainnet uint8 = 84

// CurrencyFormationVersionTestnet is the formation version for testnet (from cmake).
const CurrencyFormationVersionTestnet uint8 = 100

// NetworkIDMainnet is the 16-byte network UUID for mainnet P2P handshake.
// From net_node.inl: {0x11, 0x10, 0x01, 0x11, 0x01, 0x01, 0x11, 0x01,
//                      0x10, 0x11, 0x00, 0x11, 0x01, 0x11, 0x21, 0x54}
var NetworkIDMainnet = [16]byte{
	0x11, 0x10, 0x01, 0x11, 0x01, 0x01, 0x11, 0x01,
	0x10, 0x11, 0x00, 0x11, 0x01, 0x11, 0x21, 0x54,
}

// NetworkIDTestnet is the 16-byte network UUID for testnet P2P handshake.
// Byte 10 = TESTNET_FLAG (1), byte 15 = FORMATION_VERSION (100 = 0x64).
var NetworkIDTestnet = [16]byte{
	0x11, 0x10, 0x01, 0x11, 0x01, 0x01, 0x11, 0x01,
	0x10, 0x11, 0x01, 0x11, 0x01, 0x11, 0x21, 0x64,
}

// ClientVersion is the version string sent in CORE_SYNC_DATA.
const ClientVersion = "Lethean/go-blockchain 0.1.0"
```

Also add `NetworkID [16]byte` to the `ChainConfig` struct, and set it in the
`Mainnet` and `Testnet` globals.

**Step 3: Add test**

Add to `config/config_test.go`:

```go
func TestNetworkID(t *testing.T) {
	// Mainnet: byte 10 = 0 (not testnet), byte 15 = 84 (0x54)
	if NetworkIDMainnet[10] != 0x00 {
		t.Errorf("mainnet testnet flag: got %x, want 0x00", NetworkIDMainnet[10])
	}
	if NetworkIDMainnet[15] != 0x54 {
		t.Errorf("mainnet version: got %x, want 0x54", NetworkIDMainnet[15])
	}
	// Testnet: byte 10 = 1, byte 15 = 100 (0x64)
	if NetworkIDTestnet[10] != 0x01 {
		t.Errorf("testnet testnet flag: got %x, want 0x01", NetworkIDTestnet[10])
	}
	if NetworkIDTestnet[15] != 0x64 {
		t.Errorf("testnet version: got %x, want 0x64", NetworkIDTestnet[15])
	}
	// ChainConfig should have them
	if Mainnet.NetworkID != NetworkIDMainnet {
		t.Error("Mainnet.NetworkID mismatch")
	}
	if Testnet.NetworkID != NetworkIDTestnet {
		t.Error("Testnet.NetworkID mismatch")
	}
}
```

**Step 4: Run tests**

Run: `go test -race ./config/...`
Expected: PASS

**Step 5: Commit**

```bash
git add go.mod go.sum config/config.go config/config_test.go
git commit -m "feat(config): P2P network IDs and go-p2p dependency

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 7: CORE_SYNC_DATA + Command Infrastructure

**Files:**
- Create: `p2p/commands.go`
- Create: `p2p/sync.go`
- Create: `p2p/sync_test.go`

**Context:**

CORE_SYNC_DATA is the blockchain state exchanged in every handshake and
timed_sync. It's serialised as a portable storage section with these fields:

```
"current_height"             → uint64
"top_id"                     → 32-byte hash (STRING containing raw bytes — POD_AS_BLOB)
"last_checkpoint_height"     → uint64
"core_time"                  → uint64
"client_version"             → string
"non_pruning_mode_enabled"   → bool
```

**Step 1: Create command ID constants**

Create `p2p/commands.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

// Package p2p implements the CryptoNote P2P protocol for the Lethean blockchain.
package p2p

import "forge.lthn.ai/core/go-p2p/node/levin"

// Re-export command IDs from the levin package for convenience.
const (
	CommandHandshake        = levin.CommandHandshake        // 1001
	CommandTimedSync        = levin.CommandTimedSync        // 1002
	CommandPing             = levin.CommandPing             // 1003
	CommandNewBlock         = levin.CommandNewBlock         // 2001
	CommandNewTransactions  = levin.CommandNewTransactions  // 2002
	CommandRequestObjects   = levin.CommandRequestObjects   // 2003
	CommandResponseObjects  = levin.CommandResponseObjects  // 2004
	CommandRequestChain     = levin.CommandRequestChain     // 2006
	CommandResponseChain    = levin.CommandResponseChain    // 2007
)
```

**Step 2: Create CORE_SYNC_DATA**

Create `p2p/sync.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package p2p

import (
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-p2p/node/levin"
)

// CoreSyncData is the blockchain state exchanged during handshake and timed sync.
type CoreSyncData struct {
	CurrentHeight        uint64
	TopID                types.Hash
	LastCheckpointHeight uint64
	CoreTime             uint64
	ClientVersion        string
	NonPruningMode       bool
}

// MarshalSection encodes CoreSyncData into a portable storage Section.
func (d *CoreSyncData) MarshalSection() levin.Section {
	return levin.Section{
		"current_height":             levin.Uint64Val(d.CurrentHeight),
		"top_id":                     levin.StringVal(d.TopID[:]),
		"last_checkpoint_height":     levin.Uint64Val(d.LastCheckpointHeight),
		"core_time":                  levin.Uint64Val(d.CoreTime),
		"client_version":             levin.StringVal([]byte(d.ClientVersion)),
		"non_pruning_mode_enabled":   levin.BoolVal(d.NonPruningMode),
	}
}

// UnmarshalSection decodes CoreSyncData from a portable storage Section.
func (d *CoreSyncData) UnmarshalSection(s levin.Section) error {
	if v, ok := s["current_height"]; ok {
		val, err := v.AsUint64()
		if err != nil {
			return err
		}
		d.CurrentHeight = val
	}
	if v, ok := s["top_id"]; ok {
		blob, err := v.AsString()
		if err != nil {
			return err
		}
		if len(blob) == 32 {
			copy(d.TopID[:], blob)
		}
	}
	if v, ok := s["last_checkpoint_height"]; ok {
		val, err := v.AsUint64()
		if err != nil {
			return err
		}
		d.LastCheckpointHeight = val
	}
	if v, ok := s["core_time"]; ok {
		val, err := v.AsUint64()
		if err != nil {
			return err
		}
		d.CoreTime = val
	}
	if v, ok := s["client_version"]; ok {
		blob, err := v.AsString()
		if err != nil {
			return err
		}
		d.ClientVersion = string(blob)
	}
	if v, ok := s["non_pruning_mode_enabled"]; ok {
		val, err := v.AsBool()
		if err != nil {
			return err
		}
		d.NonPruningMode = val
	}
	return nil
}
```

**Step 3: Write the test**

Create `p2p/sync_test.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package p2p

import (
	"testing"

	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-p2p/node/levin"
)

func TestCoreSyncData_Good_Roundtrip(t *testing.T) {
	var topID types.Hash
	topID[0] = 0xCB
	topID[31] = 0x63

	original := CoreSyncData{
		CurrentHeight:        6300,
		TopID:                topID,
		LastCheckpointHeight: 0,
		CoreTime:             1708444800,
		ClientVersion:        "Lethean/go-blockchain 0.1.0",
		NonPruningMode:       true,
	}
	section := original.MarshalSection()

	// Encode → decode storage to prove wire compatibility.
	data, err := levin.EncodeStorage(section)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := levin.DecodeStorage(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	var got CoreSyncData
	if err := got.UnmarshalSection(decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.CurrentHeight != original.CurrentHeight {
		t.Errorf("height: got %d, want %d", got.CurrentHeight, original.CurrentHeight)
	}
	if got.TopID != original.TopID {
		t.Errorf("top_id: got %x, want %x", got.TopID, original.TopID)
	}
	if got.ClientVersion != original.ClientVersion {
		t.Errorf("version: got %q, want %q", got.ClientVersion, original.ClientVersion)
	}
	if got.NonPruningMode != original.NonPruningMode {
		t.Errorf("pruning: got %v, want %v", got.NonPruningMode, original.NonPruningMode)
	}
}
```

**Step 4: Run tests**

Run: `go test -race -v ./p2p/...`
Expected: PASS

**Step 5: Commit**

```bash
git add p2p/commands.go p2p/sync.go p2p/sync_test.go
git commit -m "feat(p2p): CORE_SYNC_DATA type and command ID constants

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 8: Ping Command

**Files:**
- Create: `p2p/ping.go`
- Create: `p2p/ping_test.go`

**Context:**

Ping is the simplest command — empty request, response with `"status"` = `"OK"`
and `"peer_id"` = uint64. Good smoke test for the whole stack.

**Step 1: Write the test**

Create `p2p/ping_test.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package p2p

import (
	"testing"

	"forge.lthn.ai/core/go-p2p/node/levin"
)

func TestEncodePingRequest_Good_EmptySection(t *testing.T) {
	data, err := EncodePingRequest()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	s, err := levin.DecodeStorage(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(s) != 0 {
		t.Errorf("ping request should be empty, got %d fields", len(s))
	}
}

func TestDecodePingResponse_Good(t *testing.T) {
	s := levin.Section{
		"status":  levin.StringVal([]byte("OK")),
		"peer_id": levin.Uint64Val(12345),
	}
	data, _ := levin.EncodeStorage(s)

	status, peerID, err := DecodePingResponse(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status != "OK" {
		t.Errorf("status: got %q, want %q", status, "OK")
	}
	if peerID != 12345 {
		t.Errorf("peer_id: got %d, want 12345", peerID)
	}
}
```

**Step 2: Write the implementation**

Create `p2p/ping.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package p2p

import "forge.lthn.ai/core/go-p2p/node/levin"

// EncodePingRequest returns an encoded empty ping request payload.
func EncodePingRequest() ([]byte, error) {
	return levin.EncodeStorage(levin.Section{})
}

// DecodePingResponse parses a ping response payload.
func DecodePingResponse(data []byte) (status string, peerID uint64, err error) {
	s, err := levin.DecodeStorage(data)
	if err != nil {
		return "", 0, err
	}
	if v, ok := s["status"]; ok {
		blob, err := v.AsString()
		if err != nil {
			return "", 0, err
		}
		status = string(blob)
	}
	if v, ok := s["peer_id"]; ok {
		peerID, err = v.AsUint64()
		if err != nil {
			return "", 0, err
		}
	}
	return status, peerID, nil
}
```

**Step 3: Run tests**

Run: `go test -race -v ./p2p/ -run Ping`
Expected: PASS

**Step 4: Commit**

```bash
git add p2p/ping.go p2p/ping_test.go
git commit -m "feat(p2p): ping command encode/decode

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 9: Handshake Command

**Files:**
- Create: `p2p/handshake.go`
- Create: `p2p/handshake_test.go`

**Context:**

Handshake is the most complex P2P command. Request carries `node_data` +
`payload_data` (CORE_SYNC_DATA). Response carries the same plus `local_peerlist`.

`node_data` is a section with:
```
"network_id"  → 16-byte blob (POD_AS_BLOB)
"peer_id"     → uint64
"local_time"  → int64
"my_port"     → uint32
```

`local_peerlist` is a single STRING blob containing packed 24-byte entries:
```
ip         uint32 (4 bytes)
port       uint32 (4 bytes)
id         uint64 (8 bytes)
last_seen  int64  (8 bytes)
```

**Step 1: Write the test**

Create `p2p/handshake_test.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package p2p

import (
	"testing"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-p2p/node/levin"
)

func TestEncodeHandshakeRequest_Good_Roundtrip(t *testing.T) {
	req := HandshakeRequest{
		NodeData: NodeData{
			NetworkID: config.NetworkIDTestnet,
			PeerID:    0xDEADBEEF,
			LocalTime: 1708444800,
			MyPort:    46942,
		},
		PayloadData: CoreSyncData{
			CurrentHeight: 100,
			ClientVersion: "test/0.1",
		},
	}
	data, err := EncodeHandshakeRequest(&req)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	s, err := levin.DecodeStorage(data)
	if err != nil {
		t.Fatalf("decode storage: %v", err)
	}

	var got HandshakeRequest
	if err := got.UnmarshalSection(s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.NodeData.NetworkID != config.NetworkIDTestnet {
		t.Errorf("network_id mismatch")
	}
	if got.NodeData.PeerID != 0xDEADBEEF {
		t.Errorf("peer_id: got %x, want DEADBEEF", got.NodeData.PeerID)
	}
	if got.PayloadData.CurrentHeight != 100 {
		t.Errorf("height: got %d, want 100", got.PayloadData.CurrentHeight)
	}
}

func TestDecodeHandshakeResponse_Good_WithPeerlist(t *testing.T) {
	// Build a response section manually.
	nodeData := levin.Section{
		"network_id": levin.StringVal(config.NetworkIDTestnet[:]),
		"peer_id":    levin.Uint64Val(42),
		"local_time": levin.Int64Val(1708444800),
		"my_port":    levin.Uint32Val(46942),
	}
	syncData := CoreSyncData{
		CurrentHeight: 6300,
		ClientVersion: "Zano/2.0",
	}
	// Pack 2 peerlist entries into a single blob.
	peerBlob := make([]byte, 48) // 2 × 24 bytes
	// Entry 1: ip=10.0.0.1, port=46942, id=1, last_seen=0
	peerBlob[0] = 10 // ip byte 0
	peerBlob[3] = 1  // ip byte 3
	// (rest is zeros for simplicity)

	s := levin.Section{
		"node_data":      levin.ObjectVal(nodeData),
		"payload_data":   levin.ObjectVal(syncData.MarshalSection()),
		"local_peerlist": levin.StringVal(peerBlob),
	}
	data, _ := levin.EncodeStorage(s)

	var resp HandshakeResponse
	if err := resp.Decode(data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.NodeData.PeerID != 42 {
		t.Errorf("peer_id: got %d, want 42", resp.NodeData.PeerID)
	}
	if resp.PayloadData.CurrentHeight != 6300 {
		t.Errorf("height: got %d, want 6300", resp.PayloadData.CurrentHeight)
	}
	if len(resp.PeerlistBlob) != 48 {
		t.Errorf("peerlist: got %d bytes, want 48", len(resp.PeerlistBlob))
	}
}

func TestNodeData_Good_NetworkIDBlob(t *testing.T) {
	nd := NodeData{NetworkID: config.NetworkIDTestnet}
	s := nd.MarshalSection()
	blob, err := s["network_id"].AsString()
	if err != nil {
		t.Fatalf("network_id: %v", err)
	}
	if len(blob) != 16 {
		t.Fatalf("network_id blob: got %d bytes, want 16", len(blob))
	}
	// Byte 10 = testnet flag = 1
	if blob[10] != 0x01 {
		t.Errorf("testnet flag: got %x, want 0x01", blob[10])
	}
}

var _ types.Hash // ensure types import is used
```

**Step 2: Write the implementation**

Create `p2p/handshake.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package p2p

import (
	"encoding/binary"

	"forge.lthn.ai/core/go-p2p/node/levin"
)

// PeerlistEntrySize is the packed size of a peerlist entry (ip + port + id + last_seen).
const PeerlistEntrySize = 24

// NodeData contains the node identity exchanged during handshake.
type NodeData struct {
	NetworkID [16]byte
	PeerID    uint64
	LocalTime int64
	MyPort    uint32
}

// MarshalSection encodes NodeData into a portable storage Section.
func (n *NodeData) MarshalSection() levin.Section {
	return levin.Section{
		"network_id": levin.StringVal(n.NetworkID[:]),
		"peer_id":    levin.Uint64Val(n.PeerID),
		"local_time": levin.Int64Val(n.LocalTime),
		"my_port":    levin.Uint32Val(n.MyPort),
	}
}

// UnmarshalSection decodes NodeData from a portable storage Section.
func (n *NodeData) UnmarshalSection(s levin.Section) error {
	if v, ok := s["network_id"]; ok {
		blob, err := v.AsString()
		if err != nil {
			return err
		}
		if len(blob) >= 16 {
			copy(n.NetworkID[:], blob[:16])
		}
	}
	if v, ok := s["peer_id"]; ok {
		val, err := v.AsUint64()
		if err != nil {
			return err
		}
		n.PeerID = val
	}
	if v, ok := s["local_time"]; ok {
		val, err := v.AsInt64()
		if err != nil {
			return err
		}
		n.LocalTime = val
	}
	if v, ok := s["my_port"]; ok {
		val, err := v.AsUint32()
		if err != nil {
			return err
		}
		n.MyPort = val
	}
	return nil
}

// PeerlistEntry is a decoded peerlist entry from a handshake response.
type PeerlistEntry struct {
	IP       uint32
	Port     uint32
	ID       uint64
	LastSeen int64
}

// DecodePeerlist splits a packed peerlist blob into entries.
func DecodePeerlist(blob []byte) []PeerlistEntry {
	n := len(blob) / PeerlistEntrySize
	entries := make([]PeerlistEntry, n)
	for i := 0; i < n; i++ {
		off := i * PeerlistEntrySize
		entries[i] = PeerlistEntry{
			IP:       binary.LittleEndian.Uint32(blob[off : off+4]),
			Port:     binary.LittleEndian.Uint32(blob[off+4 : off+8]),
			ID:       binary.LittleEndian.Uint64(blob[off+8 : off+16]),
			LastSeen: int64(binary.LittleEndian.Uint64(blob[off+16 : off+24])),
		}
	}
	return entries
}

// HandshakeRequest is a COMMAND_HANDSHAKE request.
type HandshakeRequest struct {
	NodeData    NodeData
	PayloadData CoreSyncData
}

// MarshalSection encodes the request.
func (r *HandshakeRequest) MarshalSection() levin.Section {
	return levin.Section{
		"node_data":    levin.ObjectVal(r.NodeData.MarshalSection()),
		"payload_data": levin.ObjectVal(r.PayloadData.MarshalSection()),
	}
}

// UnmarshalSection decodes the request.
func (r *HandshakeRequest) UnmarshalSection(s levin.Section) error {
	if v, ok := s["node_data"]; ok {
		obj, err := v.AsSection()
		if err != nil {
			return err
		}
		if err := r.NodeData.UnmarshalSection(obj); err != nil {
			return err
		}
	}
	if v, ok := s["payload_data"]; ok {
		obj, err := v.AsSection()
		if err != nil {
			return err
		}
		if err := r.PayloadData.UnmarshalSection(obj); err != nil {
			return err
		}
	}
	return nil
}

// EncodeHandshakeRequest serialises a handshake request into a storage blob.
func EncodeHandshakeRequest(req *HandshakeRequest) ([]byte, error) {
	return levin.EncodeStorage(req.MarshalSection())
}

// HandshakeResponse is a COMMAND_HANDSHAKE response.
type HandshakeResponse struct {
	NodeData     NodeData
	PayloadData  CoreSyncData
	PeerlistBlob []byte // Raw packed peerlist (24 bytes per entry)
}

// Decode parses a handshake response from a storage blob.
func (r *HandshakeResponse) Decode(data []byte) error {
	s, err := levin.DecodeStorage(data)
	if err != nil {
		return err
	}
	if v, ok := s["node_data"]; ok {
		obj, err := v.AsSection()
		if err != nil {
			return err
		}
		if err := r.NodeData.UnmarshalSection(obj); err != nil {
			return err
		}
	}
	if v, ok := s["payload_data"]; ok {
		obj, err := v.AsSection()
		if err != nil {
			return err
		}
		if err := r.PayloadData.UnmarshalSection(obj); err != nil {
			return err
		}
	}
	if v, ok := s["local_peerlist"]; ok {
		blob, err := v.AsString()
		if err != nil {
			return err
		}
		r.PeerlistBlob = blob
	}
	return nil
}
```

**Step 3: Run tests**

Run: `go test -race -v ./p2p/ -run Handshake`
Expected: PASS

Run: `go test -race -v ./p2p/ -run NodeData`
Expected: PASS

**Step 4: Commit**

```bash
git add p2p/handshake.go p2p/handshake_test.go
git commit -m "feat(p2p): handshake command with NodeData and peerlist decoding

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 10: Timed Sync + Relay Command Types

**Files:**
- Create: `p2p/timedsync.go`
- Create: `p2p/relay.go`
- Create: `p2p/relay_test.go`

**Context:**

Timed sync request carries `payload_data` (CoreSyncData). Response adds
`local_time` and `local_peerlist`.

Relay commands (2001-2007) carry block/transaction blobs. At this stage,
we define the types and encode/decode helpers. The actual relay logic
(connecting to chain storage) comes in Phase 5.

**Step 1: Write timed sync**

Create `p2p/timedsync.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package p2p

import "forge.lthn.ai/core/go-p2p/node/levin"

// TimedSyncRequest is a COMMAND_TIMED_SYNC request.
type TimedSyncRequest struct {
	PayloadData CoreSyncData
}

// Encode serialises the timed sync request.
func (r *TimedSyncRequest) Encode() ([]byte, error) {
	s := levin.Section{
		"payload_data": levin.ObjectVal(r.PayloadData.MarshalSection()),
	}
	return levin.EncodeStorage(s)
}

// TimedSyncResponse is a COMMAND_TIMED_SYNC response.
type TimedSyncResponse struct {
	LocalTime    int64
	PayloadData  CoreSyncData
	PeerlistBlob []byte
}

// Decode parses a timed sync response from a storage blob.
func (r *TimedSyncResponse) Decode(data []byte) error {
	s, err := levin.DecodeStorage(data)
	if err != nil {
		return err
	}
	if v, ok := s["local_time"]; ok {
		r.LocalTime, _ = v.AsInt64()
	}
	if v, ok := s["payload_data"]; ok {
		obj, _ := v.AsSection()
		r.PayloadData.UnmarshalSection(obj)
	}
	if v, ok := s["local_peerlist"]; ok {
		r.PeerlistBlob, _ = v.AsString()
	}
	return nil
}
```

**Step 2: Write relay types**

Create `p2p/relay.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package p2p

import "forge.lthn.ai/core/go-p2p/node/levin"

// NewBlockNotification is NOTIFY_NEW_BLOCK (2001).
type NewBlockNotification struct {
	BlockBlob  []byte   // Serialised block
	TxBlobs    [][]byte // Serialised transactions
	Height     uint64   // Current blockchain height
}

// Encode serialises the notification.
func (n *NewBlockNotification) Encode() ([]byte, error) {
	blockEntry := levin.Section{
		"block": levin.StringVal(n.BlockBlob),
		"txs":   levin.StringArrayVal(n.TxBlobs),
	}
	s := levin.Section{
		"b":                          levin.ObjectVal(blockEntry),
		"current_blockchain_height":  levin.Uint64Val(n.Height),
	}
	return levin.EncodeStorage(s)
}

// Decode parses a new block notification from a storage blob.
func (n *NewBlockNotification) Decode(data []byte) error {
	s, err := levin.DecodeStorage(data)
	if err != nil {
		return err
	}
	if v, ok := s["current_blockchain_height"]; ok {
		n.Height, _ = v.AsUint64()
	}
	if v, ok := s["b"]; ok {
		obj, _ := v.AsSection()
		if blk, ok := obj["block"]; ok {
			n.BlockBlob, _ = blk.AsString()
		}
		if txs, ok := obj["txs"]; ok {
			n.TxBlobs, _ = txs.AsStringArray()
		}
	}
	return nil
}

// NewTransactionsNotification is NOTIFY_OR_INVOKE_NEW_TRANSACTIONS (2002).
type NewTransactionsNotification struct {
	TxBlobs [][]byte
}

// Encode serialises the notification.
func (n *NewTransactionsNotification) Encode() ([]byte, error) {
	s := levin.Section{
		"txs": levin.StringArrayVal(n.TxBlobs),
	}
	return levin.EncodeStorage(s)
}

// Decode parses a new transactions notification.
func (n *NewTransactionsNotification) Decode(data []byte) error {
	s, err := levin.DecodeStorage(data)
	if err != nil {
		return err
	}
	if v, ok := s["txs"]; ok {
		n.TxBlobs, _ = v.AsStringArray()
	}
	return nil
}

// RequestChain is NOTIFY_REQUEST_CHAIN (2006).
type RequestChain struct {
	BlockIDs [][]byte // Array of 32-byte block hashes
}

// Encode serialises the request.
func (r *RequestChain) Encode() ([]byte, error) {
	s := levin.Section{
		"block_ids": levin.StringArrayVal(r.BlockIDs),
	}
	return levin.EncodeStorage(s)
}

// ResponseChainEntry is NOTIFY_RESPONSE_CHAIN_ENTRY (2007).
type ResponseChainEntry struct {
	StartHeight uint64
	TotalHeight uint64
	BlockIDs    [][]byte
}

// Decode parses a chain entry response.
func (r *ResponseChainEntry) Decode(data []byte) error {
	s, err := levin.DecodeStorage(data)
	if err != nil {
		return err
	}
	if v, ok := s["start_height"]; ok {
		r.StartHeight, _ = v.AsUint64()
	}
	if v, ok := s["total_height"]; ok {
		r.TotalHeight, _ = v.AsUint64()
	}
	if v, ok := s["m_block_ids"]; ok {
		r.BlockIDs, _ = v.AsStringArray()
	}
	return nil
}
```

**Step 3: Write tests**

Create `p2p/relay_test.go`:

```go
// SPDX-Licence-Identifier: EUPL-1.2

package p2p

import (
	"bytes"
	"testing"
)

func TestNewBlockNotification_Good_Roundtrip(t *testing.T) {
	original := NewBlockNotification{
		BlockBlob: []byte{0x01, 0x02, 0x03},
		TxBlobs:   [][]byte{{0xAA}, {0xBB, 0xCC}},
		Height:    6300,
	}
	data, err := original.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var got NewBlockNotification
	if err := got.Decode(data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Height != 6300 {
		t.Errorf("height: got %d, want 6300", got.Height)
	}
	if !bytes.Equal(got.BlockBlob, original.BlockBlob) {
		t.Errorf("block: got %x, want %x", got.BlockBlob, original.BlockBlob)
	}
	if len(got.TxBlobs) != 2 {
		t.Fatalf("txs: got %d, want 2", len(got.TxBlobs))
	}
}

func TestNewTransactionsNotification_Good_Roundtrip(t *testing.T) {
	original := NewTransactionsNotification{
		TxBlobs: [][]byte{{0x01}, {0x02}, {0x03}},
	}
	data, _ := original.Encode()
	var got NewTransactionsNotification
	got.Decode(data)
	if len(got.TxBlobs) != 3 {
		t.Errorf("txs: got %d, want 3", len(got.TxBlobs))
	}
}

func TestRequestChain_Good_Roundtrip(t *testing.T) {
	hash := make([]byte, 32)
	hash[0] = 0xFF
	original := RequestChain{BlockIDs: [][]byte{hash}}
	data, _ := original.Encode()

	// Verify it can be decoded back via storage
	var got RequestChain
	s, _ := DecodeStorageHelper(data)
	if v, ok := s["block_ids"]; ok {
		got.BlockIDs, _ = v.AsStringArray()
	}
	if len(got.BlockIDs) != 1 || got.BlockIDs[0][0] != 0xFF {
		t.Errorf("block_ids roundtrip failed")
	}
}

func TestTimedSyncRequest_Good_Encode(t *testing.T) {
	req := TimedSyncRequest{
		PayloadData: CoreSyncData{CurrentHeight: 42},
	}
	data, err := req.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty encoding")
	}
}

// DecodeStorageHelper is a test helper that wraps levin.DecodeStorage.
func DecodeStorageHelper(data []byte) (map[string]interface{ AsStringArray() ([][]byte, error) }, error) {
	// Just use levin.DecodeStorage directly in the real test.
	return nil, nil
}
```

**Note:** The `DecodeStorageHelper` is a placeholder — in the actual implementation,
use `levin.DecodeStorage` directly. Adjust the test to avoid the wrapper:

```go
func TestRequestChain_Good_Roundtrip(t *testing.T) {
	hash := make([]byte, 32)
	hash[0] = 0xFF
	original := RequestChain{BlockIDs: [][]byte{hash}}
	data, _ := original.Encode()

	// Decode back
	s, _ := levin.DecodeStorage(data)
	if v, ok := s["block_ids"]; ok {
		ids, _ := v.AsStringArray()
		if len(ids) != 1 || ids[0][0] != 0xFF {
			t.Errorf("block_ids roundtrip failed")
		}
	}
}
```

**Step 4: Run tests**

Run: `go test -race -v ./p2p/...`
Expected: PASS

**Step 5: Commit**

```bash
git add p2p/timedsync.go p2p/relay.go p2p/relay_test.go
git commit -m "feat(p2p): timed sync and block/tx relay command types

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 11: C++ Testnet Integration Test

**Files:**
- Create: `p2p/integration_test.go`

**Context:**

This is the Phase 3 equivalent of the genesis block hash test. If we can
complete a TCP handshake with the C++ testnet daemon and exchange valid
CORE_SYNC_DATA, the entire wire format stack is correct.

The testnet daemon runs on `localhost:46942` (P2P port, NOT 46941 which is RPC).

**Prerequisites:** The C++ testnet daemon must be running on snider-linux.

**Step 1: Write the integration test**

Create `p2p/integration_test.go`:

```go
//go:build integration

// SPDX-Licence-Identifier: EUPL-1.2

package p2p

import (
	"crypto/rand"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-p2p/node/levin"
)

const testnetP2PAddr = "localhost:46942"

// TestIntegration_Handshake connects to the C++ testnet daemon,
// performs a full handshake, and verifies the response.
func TestIntegration_Handshake(t *testing.T) {
	conn, err := net.DialTimeout("tcp", testnetP2PAddr, 10*time.Second)
	if err != nil {
		t.Skipf("testnet daemon not reachable at %s: %v", testnetP2PAddr, err)
	}
	defer conn.Close()

	lc := levin.NewConnection(conn)

	// Generate a random peer ID.
	var peerIDBuf [8]byte
	rand.Read(peerIDBuf[:])
	peerID := binary.LittleEndian.Uint64(peerIDBuf[:])

	// Build handshake request.
	req := HandshakeRequest{
		NodeData: NodeData{
			NetworkID: config.NetworkIDTestnet,
			PeerID:    peerID,
			LocalTime: time.Now().Unix(),
			MyPort:    0, // We're not listening
		},
		PayloadData: CoreSyncData{
			CurrentHeight: 1,
			ClientVersion: config.ClientVersion,
			NonPruningMode: true,
		},
	}
	payload, err := EncodeHandshakeRequest(&req)
	if err != nil {
		t.Fatalf("encode handshake: %v", err)
	}

	// Send handshake request.
	if err := lc.WritePacket(CommandHandshake, payload, true); err != nil {
		t.Fatalf("write handshake: %v", err)
	}

	// Read handshake response.
	hdr, data, err := lc.ReadPacket()
	if err != nil {
		t.Fatalf("read handshake response: %v", err)
	}
	if hdr.Command != CommandHandshake {
		t.Fatalf("response command: got %d, want %d", hdr.Command, CommandHandshake)
	}
	if hdr.ReturnCode != levin.ReturnOK {
		t.Fatalf("return code: got %d, want %d", hdr.ReturnCode, levin.ReturnOK)
	}

	// Parse response.
	var resp HandshakeResponse
	if err := resp.Decode(data); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify network ID matches testnet.
	if resp.NodeData.NetworkID != config.NetworkIDTestnet {
		t.Errorf("network_id: got %x, want %x", resp.NodeData.NetworkID, config.NetworkIDTestnet)
	}

	// Verify we got a chain height > 0.
	if resp.PayloadData.CurrentHeight == 0 {
		t.Error("current_height is 0 — daemon may not be synced")
	}
	t.Logf("testnet height: %d", resp.PayloadData.CurrentHeight)
	t.Logf("testnet top_id: %x", resp.PayloadData.TopID)
	t.Logf("testnet version: %s", resp.PayloadData.ClientVersion)
	t.Logf("peerlist: %d bytes (%d entries)", len(resp.PeerlistBlob), len(resp.PeerlistBlob)/PeerlistEntrySize)

	// --- Ping test ---
	pingPayload, _ := EncodePingRequest()
	if err := lc.WritePacket(CommandPing, pingPayload, true); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	hdr, data, err = lc.ReadPacket()
	if err != nil {
		t.Fatalf("read ping response: %v", err)
	}
	status, remotePeerID, err := DecodePingResponse(data)
	if err != nil {
		t.Fatalf("decode ping: %v", err)
	}
	if status != "OK" {
		t.Errorf("ping status: got %q, want %q", status, "OK")
	}
	t.Logf("ping OK, remote peer_id: %x", remotePeerID)
}
```

**Step 2: Run (skips if daemon not available)**

Run: `go test -race -v -tags integration ./p2p/ -run Integration`
Expected: Either PASS (daemon running) or SKIP (daemon not reachable)

If daemon IS running, expect output like:
```
testnet height: 6300
testnet top_id: cb9d5455...
testnet version: Zano v2.0.0...
peerlist: 48 bytes (2 entries)
ping OK, remote peer_id: abc123...
```

**Step 3: Commit**

```bash
git add p2p/integration_test.go
git commit -m "test(p2p): integration test against C++ testnet daemon

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 12: Documentation

**Files:**
- Modify: `docs/architecture.md`
- Modify: `docs/history.md`

**Step 1: Update architecture.md**

Add `p2p/` to the package structure listing. Add a section describing the P2P
package, its relationship to go-p2p/node/levin, the command types, and the
testing strategy.

**Step 2: Update history.md**

Add Phase 3 completion section with:
- Files added/modified
- Key findings (network ID bytes, portable storage varint vs CryptoNote varint,
  peerlist-as-blob encoding)
- Tests added (count and categories)
- Coverage
- Integration test result (if daemon was available)

**Step 3: Commit**

```bash
git add docs/architecture.md docs/history.md
git commit -m "docs: Phase 3 P2P Levin protocol documentation

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## File Summary

| # | File | Repo | Action |
|---|------|------|--------|
| 1 | `node/levin/header.go` | go-p2p | create |
| 2 | `node/levin/header_test.go` | go-p2p | create |
| 3 | `node/levin/varint.go` | go-p2p | create |
| 4 | `node/levin/varint_test.go` | go-p2p | create |
| 5 | `node/levin/storage.go` | go-p2p | create |
| 6 | `node/levin/storage_test.go` | go-p2p | create |
| 7 | `node/levin/connection.go` | go-p2p | create |
| 8 | `node/levin/connection_test.go` | go-p2p | create |
| 9 | `go.mod` | go-blockchain | modify |
| 10 | `config/config.go` | go-blockchain | modify |
| 11 | `config/config_test.go` | go-blockchain | modify |
| 12 | `p2p/commands.go` | go-blockchain | create |
| 13 | `p2p/sync.go` | go-blockchain | create |
| 14 | `p2p/sync_test.go` | go-blockchain | create |
| 15 | `p2p/ping.go` | go-blockchain | create |
| 16 | `p2p/ping_test.go` | go-blockchain | create |
| 17 | `p2p/handshake.go` | go-blockchain | create |
| 18 | `p2p/handshake_test.go` | go-blockchain | create |
| 19 | `p2p/timedsync.go` | go-blockchain | create |
| 20 | `p2p/relay.go` | go-blockchain | create |
| 21 | `p2p/relay_test.go` | go-blockchain | create |
| 22 | `p2p/integration_test.go` | go-blockchain | create |
| 23 | `docs/architecture.md` | go-blockchain | modify |
| 24 | `docs/history.md` | go-blockchain | modify |

## Verification

1. `go test -race ./...` in go-p2p — all tests pass
2. `go test -race ./...` in go-blockchain — all tests pass
3. `go vet ./...` in both repos — no warnings
4. `go test -race -tags integration ./p2p/` — handshake + ping against C++ testnet daemon
5. Coverage target: >80% across new files

## Critical References

- `~/Code/LetheanNetwork/blockchain/contrib/epee/include/net/levin_base.h` — header struct
- `~/Code/LetheanNetwork/blockchain/contrib/epee/include/storages/portable_storage_to_bin.h` — encoder
- `~/Code/LetheanNetwork/blockchain/contrib/epee/include/storages/portable_storage_from_bin.h` — decoder
- `~/Code/LetheanNetwork/blockchain/src/p2p/p2p_protocol_defs.h` — P2P commands
- `~/Code/LetheanNetwork/blockchain/src/p2p/net_node.inl:36` — network ID definition
- `~/Code/LetheanNetwork/blockchain/src/currency_protocol/currency_protocol_defs.h` — CORE_SYNC_DATA

## Design Decision: No Transport Interface Extraction

The approved design mentions extracting a `Transport` interface from go-p2p's
existing WebSocket transport. After examining the code, the existing `Transport`
struct is tightly coupled to WebSocket (`*websocket.Conn`, SMSG encryption,
JSON messages). The Levin protocol has fundamentally different wire semantics
(raw TCP, binary framing, no encryption).

Rather than a risky refactor of working code, this plan creates `node/levin/`
as a **standalone sub-package** within go-p2p. It does not import from
`node/` and does not attempt to share a runtime interface with WebSocket.
go-blockchain imports the levin package directly. Interface extraction is
deferred to a future phase if a shared transport abstraction is needed.
