// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package types

import (
	"strings"
	"testing"
)

func TestHashFromHex_Good(t *testing.T) {
	hexStr := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	h, err := HashFromHex(hexStr)
	if err != nil {
		t.Fatalf("HashFromHex: unexpected error: %v", err)
	}
	if h[0] != 0x01 || h[1] != 0x23 {
		t.Errorf("HashFromHex: got [0]=%02x [1]=%02x, want 01 23", h[0], h[1])
	}
	if h.String() != hexStr {
		t.Errorf("String: got %q, want %q", h.String(), hexStr)
	}
}

func TestHashFromHex_Bad(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"short", "0123"},
		{"invalid_chars", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"},
		{"odd_length", "012"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := HashFromHex(tt.input)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestHash_IsZero_Good(t *testing.T) {
	var zero Hash
	if !zero.IsZero() {
		t.Error("zero hash: IsZero() should be true")
	}

	nonZero := Hash{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	if nonZero.IsZero() {
		t.Error("non-zero hash: IsZero() should be false")
	}
}

func TestPublicKeyFromHex_Good(t *testing.T) {
	hexStr := strings.Repeat("ab", 32)
	pk, err := PublicKeyFromHex(hexStr)
	if err != nil {
		t.Fatalf("PublicKeyFromHex: unexpected error: %v", err)
	}
	for i := range pk {
		if pk[i] != 0xAB {
			t.Fatalf("PublicKeyFromHex: byte %d = %02x, want 0xAB", i, pk[i])
		}
	}
	if pk.String() != hexStr {
		t.Errorf("String: got %q, want %q", pk.String(), hexStr)
	}
}

func TestPublicKeyFromHex_Bad(t *testing.T) {
	_, err := PublicKeyFromHex("tooshort")
	if err == nil {
		t.Error("expected error for short hex")
	}
}

func TestPublicKey_IsZero_Good(t *testing.T) {
	var zero PublicKey
	if !zero.IsZero() {
		t.Error("zero key: IsZero() should be true")
	}
	nonZero := PublicKey{1}
	if nonZero.IsZero() {
		t.Error("non-zero key: IsZero() should be false")
	}
}

func TestSecretKey_String_Good(t *testing.T) {
	sk := SecretKey{0xFF}
	s := sk.String()
	if !strings.HasPrefix(s, "ff") {
		t.Errorf("String: got %q, want prefix ff", s)
	}
}

func TestKeyImage_String_Good(t *testing.T) {
	ki := KeyImage{0xDE}
	s := ki.String()
	if !strings.HasPrefix(s, "de") {
		t.Errorf("String: got %q, want prefix de", s)
	}
}
