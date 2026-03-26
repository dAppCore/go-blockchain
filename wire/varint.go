// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// You may obtain a copy of the licence at:
//
//     https://joinup.ec.europa.eu/software/page/eupl/licence-eupl
//
// SPDX-License-Identifier: EUPL-1.2

// Package wire provides binary serialisation primitives for the CryptoNote
// wire protocol. All encoding is consensus-critical and must be bit-identical
// to the C++ reference implementation.
package wire

import "dappco.re/go/core"

// MaxVarintLen is the maximum number of bytes a CryptoNote varint can occupy.
// A uint64 requires at most 10 bytes of 7-bit encoding (64 bits / 7 = ~9.14,
// so values above 2^63-1 need a 10th byte).
const MaxVarintLen = 10

// ErrVarintOverflow is returned when a varint exceeds the maximum allowed
// length of 10 bytes.
var ErrVarintOverflow = core.E("", "wire: varint overflow (exceeds 10 bytes)", nil)

// ErrVarintEmpty is returned when attempting to decode a varint from an
// empty byte slice.
var ErrVarintEmpty = core.E("", "wire: cannot decode varint from empty data", nil)

// EncodeVarint encodes a uint64 value as a CryptoNote variable-length integer.
//
// The encoding uses 7 bits per byte, with the most significant bit (MSB) set
// to 1 to indicate that more bytes follow. This is the same scheme as protobuf
// varints but limited to 9 bytes maximum for uint64 values.
func EncodeVarint(v uint64) []byte {
	if v == 0 {
		return []byte{0x00}
	}
	var buf [MaxVarintLen]byte
	n := 0
	for v > 0 {
		buf[n] = byte(v & 0x7f)
		v >>= 7
		if v > 0 {
			buf[n] |= 0x80
		}
		n++
	}
	return append([]byte(nil), buf[:n]...)
}

// DecodeVarint decodes a CryptoNote variable-length integer from the given
// byte slice. It returns the decoded value, the number of bytes consumed,
// and any error encountered.
func DecodeVarint(data []byte) (uint64, int, error) {
	if len(data) == 0 {
		return 0, 0, ErrVarintEmpty
	}
	var v uint64
	for i := 0; i < len(data) && i < MaxVarintLen; i++ {
		v |= uint64(data[i]&0x7f) << (7 * uint(i))
		if data[i]&0x80 == 0 {
			return v, i + 1, nil
		}
	}
	// If we read MaxVarintLen bytes and the last one still has the
	// continuation bit set, or if we ran out of data with the continuation
	// bit still set, that is an overflow.
	if len(data) >= MaxVarintLen {
		return 0, 0, ErrVarintOverflow
	}
	return 0, 0, ErrVarintOverflow
}
