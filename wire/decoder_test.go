// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wire

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestDecoder_DecoderUint8_Good(t *testing.T) {
	dec := NewDecoder(bytes.NewReader([]byte{0x42}))
	got := dec.ReadUint8()
	if dec.Err() != nil {
		t.Fatalf("unexpected error: %v", dec.Err())
	}
	if got != 0x42 {
		t.Errorf("got 0x%02x, want 0x42", got)
	}
}

func TestDecoder_DecoderUint64LE_Good(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		want uint64
	}{
		{"zero", "0000000000000000", 0},
		{"one", "0100000000000000", 1},
		{"genesis_nonce", "adb2b98417000000", 101011010221},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := hex.DecodeString(tc.hex)
			dec := NewDecoder(bytes.NewReader(b))
			got := dec.ReadUint64LE()
			if dec.Err() != nil {
				t.Fatalf("unexpected error: %v", dec.Err())
			}
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestDecoder_DecoderVarint_Good(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		want uint64
	}{
		{"zero", "00", 0},
		{"one", "01", 1},
		{"127", "7f", 127},
		{"128", "8001", 128},
		{"300", "ac02", 300},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := hex.DecodeString(tc.hex)
			dec := NewDecoder(bytes.NewReader(b))
			got := dec.ReadVarint()
			if dec.Err() != nil {
				t.Fatalf("unexpected error: %v", dec.Err())
			}
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestDecoder_DecoderBlob32_Good(t *testing.T) {
	var want [32]byte
	want[0] = 0xAB
	want[31] = 0xCD

	dec := NewDecoder(bytes.NewReader(want[:]))
	var got [32]byte
	dec.ReadBlob32(&got)
	if dec.Err() != nil {
		t.Fatalf("unexpected error: %v", dec.Err())
	}
	if got != want {
		t.Error("blob32 mismatch")
	}
}

func TestDecoder_DecoderBlob64_Good(t *testing.T) {
	var want [64]byte
	want[0] = 0x11
	want[63] = 0x99

	dec := NewDecoder(bytes.NewReader(want[:]))
	var got [64]byte
	dec.ReadBlob64(&got)
	if dec.Err() != nil {
		t.Fatalf("unexpected error: %v", dec.Err())
	}
	if got != want {
		t.Error("blob64 mismatch")
	}
}

func TestDecoder_DecoderStickyError_Bad(t *testing.T) {
	dec := NewDecoder(bytes.NewReader(nil))

	got := dec.ReadUint8()
	if dec.Err() == nil {
		t.Fatal("expected error from empty reader")
	}
	if got != 0 {
		t.Errorf("got %d, want 0 on error", got)
	}

	// Subsequent reads should be no-ops.
	_ = dec.ReadUint64LE()
	_ = dec.ReadVarint()
	var h [32]byte
	dec.ReadBlob32(&h)
	if h != ([32]byte{}) {
		t.Error("expected zero blob on sticky error")
	}
}

func TestDecoder_DecoderReadBytes_Good(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04}
	dec := NewDecoder(bytes.NewReader(data))
	got := dec.ReadBytes(4)
	if dec.Err() != nil {
		t.Fatalf("unexpected error: %v", dec.Err())
	}
	if !bytes.Equal(got, data) {
		t.Error("bytes mismatch")
	}
}

func TestDecoder_DecoderReadBytesZero_Good(t *testing.T) {
	dec := NewDecoder(bytes.NewReader(nil))
	got := dec.ReadBytes(0)
	if dec.Err() != nil {
		t.Fatalf("unexpected error: %v", dec.Err())
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestDecoder_DecoderReadBytesOversize_Bad(t *testing.T) {
	dec := NewDecoder(bytes.NewReader(nil))
	_ = dec.ReadBytes(MaxBlobSize + 1)
	if dec.Err() == nil {
		t.Fatal("expected error for oversize blob")
	}
}

func TestDecoder_DecoderVarintOverflow_Ugly(t *testing.T) {
	data := make([]byte, 11)
	for i := range data {
		data[i] = 0x80
	}
	dec := NewDecoder(bytes.NewReader(data))
	_ = dec.ReadVarint()
	if dec.Err() == nil {
		t.Fatal("expected overflow error")
	}
}

func TestDecoder_EncoderDecoderRoundTrip_Good(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	enc.WriteUint8(0x01)
	enc.WriteUint64LE(101011010221)
	enc.WriteVarint(1770897600)
	var h [32]byte
	h[0] = 0xDE
	h[31] = 0xAD
	enc.WriteBlob32(&h)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	if v := dec.ReadUint8(); v != 0x01 {
		t.Errorf("uint8: got 0x%02x, want 0x01", v)
	}
	if v := dec.ReadUint64LE(); v != 101011010221 {
		t.Errorf("uint64: got %d, want 101011010221", v)
	}
	if v := dec.ReadVarint(); v != 1770897600 {
		t.Errorf("varint: got %d, want 1770897600", v)
	}
	var gotH [32]byte
	dec.ReadBlob32(&gotH)
	if gotH != h {
		t.Error("blob32 mismatch")
	}
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}
}
