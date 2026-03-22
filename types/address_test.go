// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package types

import (
	"testing"

	"dappco.re/go/core/blockchain/config"
)

// makeTestAddress creates an address with deterministic test data.
func makeTestAddress(flags uint8) *Address {
	addr := &Address{Flags: flags}
	// Fill keys with recognisable patterns.
	for i := 0; i < 32; i++ {
		addr.SpendPublicKey[i] = byte(i)
		addr.ViewPublicKey[i] = byte(32 + i)
	}
	return addr
}

func TestAddressEncodeDecodeRoundTrip_Good(t *testing.T) {
	tests := []struct {
		name   string
		prefix uint64
		flags  uint8
	}{
		{"standard_address", config.AddressPrefix, 0x00},
		{"integrated_address", config.IntegratedAddressPrefix, 0x00},
		{"auditable_address", config.AuditableAddressPrefix, FlagAuditable},
		{"auditable_integrated", config.AuditableIntegratedAddressPrefix, FlagAuditable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := makeTestAddress(tt.flags)
			encoded := original.Encode(tt.prefix)

			if len(encoded) == 0 {
				t.Fatal("Encode returned empty string")
			}

			decoded, decodedPrefix, err := DecodeAddress(encoded)
			if err != nil {
				t.Fatalf("DecodeAddress failed: %v", err)
			}

			if decodedPrefix != tt.prefix {
				t.Errorf("prefix mismatch: got 0x%x, want 0x%x", decodedPrefix, tt.prefix)
			}

			if decoded.SpendPublicKey != original.SpendPublicKey {
				t.Errorf("SpendPublicKey mismatch: got %s, want %s",
					decoded.SpendPublicKey, original.SpendPublicKey)
			}

			if decoded.ViewPublicKey != original.ViewPublicKey {
				t.Errorf("ViewPublicKey mismatch: got %s, want %s",
					decoded.ViewPublicKey, original.ViewPublicKey)
			}

			if decoded.Flags != original.Flags {
				t.Errorf("Flags mismatch: got 0x%02x, want 0x%02x", decoded.Flags, original.Flags)
			}
		})
	}
}

func TestAddressEncodeDeterministic_Good(t *testing.T) {
	// Encoding the same address twice must produce the same string.
	addr := makeTestAddress(0x00)
	first := addr.Encode(config.AddressPrefix)
	second := addr.Encode(config.AddressPrefix)
	if first != second {
		t.Errorf("non-deterministic encoding:\n  first:  %s\n  second: %s", first, second)
	}
}

func TestAddressIsAuditable_Good(t *testing.T) {
	addr := makeTestAddress(FlagAuditable)
	if !addr.IsAuditable() {
		t.Error("address with FlagAuditable should report IsAuditable() == true")
	}

	nonAuditable := makeTestAddress(0x00)
	if nonAuditable.IsAuditable() {
		t.Error("address without FlagAuditable should report IsAuditable() == false")
	}
}

func TestIsIntegratedPrefix_Good(t *testing.T) {
	if !IsIntegratedPrefix(config.IntegratedAddressPrefix) {
		t.Error("IntegratedAddressPrefix should be recognised as integrated")
	}
	if !IsIntegratedPrefix(config.AuditableIntegratedAddressPrefix) {
		t.Error("AuditableIntegratedAddressPrefix should be recognised as integrated")
	}
	if IsIntegratedPrefix(config.AddressPrefix) {
		t.Error("AddressPrefix should not be recognised as integrated")
	}
}

func TestDecodeAddress_Bad(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty_string", ""},
		{"invalid_base58_char", "0OIl"},
		{"too_short", "1111"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := DecodeAddress(tt.input)
			if err == nil {
				t.Error("expected error for invalid input, got nil")
			}
		})
	}
}

func TestDecodeAddressChecksumCorruption_Bad(t *testing.T) {
	addr := makeTestAddress(0x00)
	encoded := addr.Encode(config.AddressPrefix)

	// Corrupt the last character of the encoded string to break the checksum.
	corrupted := []byte(encoded)
	lastChar := corrupted[len(corrupted)-1]
	if lastChar == '1' {
		corrupted[len(corrupted)-1] = '2'
	} else {
		corrupted[len(corrupted)-1] = '1'
	}

	_, _, err := DecodeAddress(string(corrupted))
	if err == nil {
		t.Error("expected checksum error for corrupted address, got nil")
	}
}

func TestBase58RoundTrip_Good(t *testing.T) {
	// Test the underlying base58 encode/decode with known data.
	testData := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	encoded := base58Encode(testData)
	decoded, err := base58Decode(encoded)
	if err != nil {
		t.Fatalf("base58 round-trip failed: %v", err)
	}
	if len(decoded) != len(testData) {
		t.Fatalf("base58 round-trip length mismatch: got %d, want %d", len(decoded), len(testData))
	}
	for i := range testData {
		if decoded[i] != testData[i] {
			t.Errorf("base58 round-trip byte %d: got 0x%02x, want 0x%02x", i, decoded[i], testData[i])
		}
	}
}

func TestBase58Empty_Ugly(t *testing.T) {
	// Encoding empty data should produce an empty string.
	result := base58Encode(nil)
	if result != "" {
		t.Errorf("base58Encode(nil) = %q, want empty string", result)
	}

	// Decoding empty string should return an error.
	_, err := base58Decode("")
	if err == nil {
		t.Error("base58Decode(\"\") should return an error")
	}
}
