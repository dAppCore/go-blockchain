// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wire

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"
)

func TestEncoderUint8_Good(t *testing.T) {
	tests := []struct {
		name string
		val  uint8
		want string
	}{
		{"zero", 0, "00"},
		{"one", 1, "01"},
		{"max", 255, "ff"},
		{"mid", 0x42, "42"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			enc := NewEncoder(&buf)
			enc.WriteUint8(tc.val)
			if enc.Err() != nil {
				t.Fatalf("unexpected error: %v", enc.Err())
			}
			got := hex.EncodeToString(buf.Bytes())
			if got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestEncoderUint64LE_Good(t *testing.T) {
	tests := []struct {
		name string
		val  uint64
		want string
	}{
		{"zero", 0, "0000000000000000"},
		{"one", 1, "0100000000000000"},
		{"max", ^uint64(0), "ffffffffffffffff"},
		{"genesis_nonce", 101011010221, "adb2b98417000000"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			enc := NewEncoder(&buf)
			enc.WriteUint64LE(tc.val)
			if enc.Err() != nil {
				t.Fatalf("unexpected error: %v", enc.Err())
			}
			got := hex.EncodeToString(buf.Bytes())
			if got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestEncoderVarint_Good(t *testing.T) {
	tests := []struct {
		name string
		val  uint64
		want string
	}{
		{"zero", 0, "00"},
		{"one", 1, "01"},
		{"127", 127, "7f"},
		{"128", 128, "8001"},
		{"300", 300, "ac02"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			enc := NewEncoder(&buf)
			enc.WriteVarint(tc.val)
			if enc.Err() != nil {
				t.Fatalf("unexpected error: %v", enc.Err())
			}
			got := hex.EncodeToString(buf.Bytes())
			if got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestEncoderBlob32_Good(t *testing.T) {
	var h [32]byte
	h[0] = 0xAB
	h[31] = 0xCD

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	enc.WriteBlob32(&h)
	if enc.Err() != nil {
		t.Fatalf("unexpected error: %v", enc.Err())
	}
	if buf.Len() != 32 {
		t.Fatalf("got %d bytes, want 32", buf.Len())
	}
	if buf.Bytes()[0] != 0xAB || buf.Bytes()[31] != 0xCD {
		t.Errorf("blob32 bytes mismatch")
	}
}

func TestEncoderBlob64_Good(t *testing.T) {
	var s [64]byte
	s[0] = 0x11
	s[63] = 0x99

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	enc.WriteBlob64(&s)
	if enc.Err() != nil {
		t.Fatalf("unexpected error: %v", enc.Err())
	}
	if buf.Len() != 64 {
		t.Fatalf("got %d bytes, want 64", buf.Len())
	}
}

func TestEncoderStickyError_Bad(t *testing.T) {
	w := &failWriter{failAfter: 1}
	enc := NewEncoder(w)

	enc.WriteUint8(0x01) // succeeds
	enc.WriteUint8(0x02) // fails
	enc.WriteUint8(0x03) // should be no-op

	if enc.Err() == nil {
		t.Fatal("expected error after failed write")
	}
}

func TestEncoderEmptyBytes_Good(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	enc.WriteBytes(nil)
	enc.WriteBytes([]byte{})
	if enc.Err() != nil {
		t.Fatalf("unexpected error: %v", enc.Err())
	}
	if buf.Len() != 0 {
		t.Errorf("got %d bytes, want 0", buf.Len())
	}
}

func TestEncoderSequence_Good(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	enc.WriteUint8(0x01)
	enc.WriteUint64LE(42)
	enc.WriteVarint(300)
	enc.WriteVariantTag(0x24)
	if enc.Err() != nil {
		t.Fatalf("unexpected error: %v", enc.Err())
	}
	if buf.Len() != 12 {
		t.Errorf("got %d bytes, want 12", buf.Len())
	}
}

// failWriter fails after writing failAfter bytes.
type failWriter struct {
	written   int
	failAfter int
}

func (fw *failWriter) Write(p []byte) (int, error) {
	if fw.written+len(p) > fw.failAfter {
		return 0, errors.New("write failed")
	}
	fw.written += len(p)
	return len(p), nil
}
