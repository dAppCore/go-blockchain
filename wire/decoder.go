// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wire

import (
	"encoding/binary"
	"fmt"
	"io"

	coreerr "forge.lthn.ai/core/go-log"
)

// Decoder reads consensus-critical binary data from an io.Reader.
// It uses the same sticky error pattern as Encoder.
type Decoder struct {
	r   io.Reader
	err error
	buf [10]byte
}

// NewDecoder creates a new Decoder reading from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

// Err returns the first error encountered during decoding.
func (d *Decoder) Err() error { return d.err }

// ReadUint8 reads a single byte.
func (d *Decoder) ReadUint8() uint8 {
	if d.err != nil {
		return 0
	}
	_, d.err = io.ReadFull(d.r, d.buf[:1])
	return d.buf[0]
}

// ReadUint64LE reads a uint64 in little-endian byte order.
func (d *Decoder) ReadUint64LE() uint64 {
	if d.err != nil {
		return 0
	}
	_, d.err = io.ReadFull(d.r, d.buf[:8])
	if d.err != nil {
		return 0
	}
	return binary.LittleEndian.Uint64(d.buf[:8])
}

// ReadVarint reads a CryptoNote varint (LEB128).
func (d *Decoder) ReadVarint() uint64 {
	if d.err != nil {
		return 0
	}
	var val uint64
	var shift uint
	for range MaxVarintLen {
		_, d.err = io.ReadFull(d.r, d.buf[:1])
		if d.err != nil {
			return 0
		}
		b := d.buf[0]
		val |= uint64(b&0x7F) << shift
		if b&0x80 == 0 {
			return val
		}
		shift += 7
	}
	d.err = ErrVarintOverflow
	return 0
}

// ReadBytes reads exactly n bytes as a raw blob.
func (d *Decoder) ReadBytes(n int) []byte {
	if d.err != nil {
		return nil
	}
	if n == 0 {
		return nil
	}
	if n < 0 || n > MaxBlobSize {
		d.err = coreerr.E("Decoder.ReadBytes", fmt.Sprintf("wire: blob size %d exceeds maximum %d", n, MaxBlobSize), nil)
		return nil
	}
	buf := make([]byte, n)
	_, d.err = io.ReadFull(d.r, buf)
	if d.err != nil {
		return nil
	}
	return buf
}

// ReadBlob32 reads a 32-byte fixed-size blob into dst.
func (d *Decoder) ReadBlob32(dst *[32]byte) {
	if d.err != nil {
		return
	}
	_, d.err = io.ReadFull(d.r, dst[:])
}

// ReadBlob64 reads a 64-byte fixed-size blob into dst.
func (d *Decoder) ReadBlob64(dst *[64]byte) {
	if d.err != nil {
		return
	}
	_, d.err = io.ReadFull(d.r, dst[:])
}

// ReadVariantTag reads a single-byte variant discriminator.
func (d *Decoder) ReadVariantTag() uint8 {
	return d.ReadUint8()
}

// MaxBlobSize is the maximum byte count allowed for a single ReadBytes call.
const MaxBlobSize = 50 * 1024 * 1024 // 50 MiB
