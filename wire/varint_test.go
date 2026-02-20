// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wire

import (
	"math"
	"testing"
)

func TestEncodeVarint_Good(t *testing.T) {
	tests := []struct {
		name  string
		value uint64
		want  []byte
	}{
		{"zero", 0, []byte{0x00}},
		{"one", 1, []byte{0x01}},
		{"max_single_byte", 127, []byte{0x7f}},
		{"128", 128, []byte{0x80, 0x01}},
		{"255", 255, []byte{0xff, 0x01}},
		{"256", 256, []byte{0x80, 0x02}},
		{"16384", 16384, []byte{0x80, 0x80, 0x01}},
		{"65535", 65535, []byte{0xff, 0xff, 0x03}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodeVarint(tt.value)
			if len(got) != len(tt.want) {
				t.Fatalf("EncodeVarint(%d) = %x (len %d), want %x (len %d)",
					tt.value, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("EncodeVarint(%d)[%d] = 0x%02x, want 0x%02x",
						tt.value, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDecodeVarint_Good(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		wantVal  uint64
		wantLen  int
	}{
		{"zero", []byte{0x00}, 0, 1},
		{"one", []byte{0x01}, 1, 1},
		{"127", []byte{0x7f}, 127, 1},
		{"128", []byte{0x80, 0x01}, 128, 2},
		{"16384", []byte{0x80, 0x80, 0x01}, 16384, 3},
		// With trailing data — should only consume the varint bytes.
		{"with_trailing", []byte{0x01, 0xff, 0xff}, 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, n, err := DecodeVarint(tt.data)
			if err != nil {
				t.Fatalf("DecodeVarint(%x) returned error: %v", tt.data, err)
			}
			if val != tt.wantVal {
				t.Errorf("DecodeVarint(%x) value = %d, want %d", tt.data, val, tt.wantVal)
			}
			if n != tt.wantLen {
				t.Errorf("DecodeVarint(%x) length = %d, want %d", tt.data, n, tt.wantLen)
			}
		})
	}
}

func TestVarintRoundTrip_Good(t *testing.T) {
	values := []uint64{
		0, 1, 127, 128, 255, 256, 1000, 65535, 65536,
		1<<14 - 1, 1 << 14, 1<<21 - 1, 1 << 21,
		1<<28 - 1, 1 << 28, 1<<35 - 1, 1 << 35,
		1<<42 - 1, 1 << 42, 1<<49 - 1, 1 << 49,
		1<<56 - 1, 1 << 56, math.MaxUint64,
	}

	for _, v := range values {
		encoded := EncodeVarint(v)
		decoded, n, err := DecodeVarint(encoded)
		if err != nil {
			t.Errorf("round-trip failed for %d: encode→%x, decode error: %v", v, encoded, err)
			continue
		}
		if decoded != v {
			t.Errorf("round-trip failed for %d: encode→%x, decode→%d", v, encoded, decoded)
		}
		if n != len(encoded) {
			t.Errorf("round-trip for %d: consumed %d bytes, encoded %d bytes", v, n, len(encoded))
		}
	}
}

func TestDecodeVarint_Bad(t *testing.T) {
	// Empty input.
	_, _, err := DecodeVarint(nil)
	if err != ErrVarintEmpty {
		t.Errorf("DecodeVarint(nil) error = %v, want ErrVarintEmpty", err)
	}

	_, _, err = DecodeVarint([]byte{})
	if err != ErrVarintEmpty {
		t.Errorf("DecodeVarint([]) error = %v, want ErrVarintEmpty", err)
	}
}

func TestDecodeVarint_Ugly(t *testing.T) {
	// A varint with all continuation bits set for 11 bytes (overflow).
	overflow := make([]byte, 11)
	for i := range overflow {
		overflow[i] = 0x80
	}
	_, _, err := DecodeVarint(overflow)
	if err != ErrVarintOverflow {
		t.Errorf("DecodeVarint(overflow) error = %v, want ErrVarintOverflow", err)
	}
}
