// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wire

import (
	"encoding/binary"
	"io"
)

// Encoder writes consensus-critical binary data to an io.Writer.
// It uses a sticky error pattern: after the first write error, all
// subsequent writes become no-ops. Call Err() after a complete
// encoding sequence to check for failures.
// Usage: var value wire.Encoder
type Encoder struct {
	w   io.Writer
	err error
	buf [10]byte // scratch for LE integers and varints
}

// NewEncoder creates a new Encoder writing to w.
// Usage: wire.NewEncoder(...)
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// Err returns the first error encountered during encoding.
// Usage: value.Err(...)
func (e *Encoder) Err() error { return e.err }

// WriteUint8 writes a single byte.
// Usage: value.WriteUint8(...)
func (e *Encoder) WriteUint8(v uint8) {
	if e.err != nil {
		return
	}
	e.buf[0] = v
	_, e.err = e.w.Write(e.buf[:1])
}

// WriteUint64LE writes a uint64 in little-endian byte order.
// Usage: value.WriteUint64LE(...)
func (e *Encoder) WriteUint64LE(v uint64) {
	if e.err != nil {
		return
	}
	binary.LittleEndian.PutUint64(e.buf[:8], v)
	_, e.err = e.w.Write(e.buf[:8])
}

// WriteVarint writes a uint64 as a CryptoNote varint (LEB128).
// Usage: value.WriteVarint(...)
func (e *Encoder) WriteVarint(v uint64) {
	if e.err != nil {
		return
	}
	b := EncodeVarint(v)
	_, e.err = e.w.Write(b)
}

// WriteBytes writes raw bytes with no length prefix.
// Usage: value.WriteBytes(...)
func (e *Encoder) WriteBytes(b []byte) {
	if e.err != nil {
		return
	}
	if len(b) == 0 {
		return
	}
	_, e.err = e.w.Write(b)
}

// WriteBlob32 writes a 32-byte fixed-size blob (hash, public key, key image).
// Usage: value.WriteBlob32(...)
func (e *Encoder) WriteBlob32(b *[32]byte) {
	if e.err != nil {
		return
	}
	_, e.err = e.w.Write(b[:])
}

// WriteBlob64 writes a 64-byte fixed-size blob (signature).
// Usage: value.WriteBlob64(...)
func (e *Encoder) WriteBlob64(b *[64]byte) {
	if e.err != nil {
		return
	}
	_, e.err = e.w.Write(b[:])
}

// WriteVariantTag writes a single-byte variant discriminator.
// Usage: value.WriteVariantTag(...)
func (e *Encoder) WriteVariantTag(tag uint8) {
	e.WriteUint8(tag)
}
