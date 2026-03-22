// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wallet

import (
	"testing"

	"dappco.re/go/core/blockchain/types"
	"dappco.re/go/core/blockchain/wire"
)

func TestParseTxExtraPublicKey(t *testing.T) {
	var key types.PublicKey
	for i := range key {
		key[i] = byte(i + 1)
	}
	raw := buildTestExtra(key, 0, 0)
	extra, err := ParseTxExtra(raw)
	if err != nil {
		t.Fatal(err)
	}
	if extra.TxPublicKey != key {
		t.Fatalf("tx public key mismatch: got %x", extra.TxPublicKey)
	}
}

func TestParseTxExtraUnlockTime(t *testing.T) {
	var key types.PublicKey
	key[0] = 0xAA
	raw := buildTestExtra(key, 500, 0)
	extra, err := ParseTxExtra(raw)
	if err != nil {
		t.Fatal(err)
	}
	if extra.UnlockTime != 500 {
		t.Fatalf("unlock_time = %d, want 500", extra.UnlockTime)
	}
}

func TestParseTxExtraDerivationHint(t *testing.T) {
	var key types.PublicKey
	key[0] = 0xBB
	raw := buildTestExtra(key, 0, 0x1234)
	extra, err := ParseTxExtra(raw)
	if err != nil {
		t.Fatal(err)
	}
	if extra.DerivationHint != 0x1234 {
		t.Fatalf("derivation_hint = %04x, want 1234", extra.DerivationHint)
	}
}

func TestParseTxExtraAllFields(t *testing.T) {
	var key types.PublicKey
	for i := range key {
		key[i] = byte(0xFF - i)
	}
	raw := buildTestExtra(key, 1000, 0xABCD)
	extra, err := ParseTxExtra(raw)
	if err != nil {
		t.Fatal(err)
	}
	if extra.TxPublicKey != key {
		t.Fatalf("tx public key mismatch: got %x", extra.TxPublicKey)
	}
	if extra.UnlockTime != 1000 {
		t.Fatalf("unlock_time = %d, want 1000", extra.UnlockTime)
	}
	if extra.DerivationHint != 0xABCD {
		t.Fatalf("derivation_hint = %04x, want ABCD", extra.DerivationHint)
	}
}

func TestParseTxExtraEmpty(t *testing.T) {
	raw := wire.EncodeVarint(0)
	extra, err := ParseTxExtra(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !extra.TxPublicKey.IsZero() {
		t.Fatal("expected zero tx public key for empty extra")
	}
	if extra.UnlockTime != 0 {
		t.Fatalf("unlock_time = %d, want 0", extra.UnlockTime)
	}
	if extra.DerivationHint != 0 {
		t.Fatalf("derivation_hint = %d, want 0", extra.DerivationHint)
	}
}

func TestParseTxExtraNilInput(t *testing.T) {
	extra, err := ParseTxExtra(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !extra.TxPublicKey.IsZero() {
		t.Fatal("expected zero tx public key for nil input")
	}
}

func TestParseTxExtraPreservesRaw(t *testing.T) {
	var key types.PublicKey
	key[0] = 0xCC
	raw := buildTestExtra(key, 100, 0x5678)
	extra, err := ParseTxExtra(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(extra.Raw) != len(raw) {
		t.Fatalf("raw length = %d, want %d", len(extra.Raw), len(raw))
	}
	// Verify it is a copy, not a reference to the same backing array.
	raw[0] = 0xFF
	if extra.Raw[0] == 0xFF {
		t.Fatal("Raw should be a defensive copy, not a reference")
	}
}

func TestParseTxExtraTruncatedKey(t *testing.T) {
	// Build a raw extra with a public key tag but only 16 bytes instead of 32.
	raw := wire.EncodeVarint(1)
	raw = append(raw, extraTagPublicKey)
	raw = append(raw, make([]byte, 16)...) // only 16 bytes
	_, err := ParseTxExtra(raw)
	if err == nil {
		t.Fatal("expected error for truncated public key")
	}
}

func TestBuildTxExtraRoundTrip(t *testing.T) {
	var key types.PublicKey
	for i := range key {
		key[i] = byte(i + 10)
	}
	raw := BuildTxExtra(key)
	extra, err := ParseTxExtra(raw)
	if err != nil {
		t.Fatal(err)
	}
	if extra.TxPublicKey != key {
		t.Fatal("round-trip key mismatch")
	}
}

func TestBuildTxExtraLength(t *testing.T) {
	var key types.PublicKey
	key[0] = 0x42
	raw := BuildTxExtra(key)
	// Expected: varint(1) + tag(1) + key(32) = 34 bytes.
	if len(raw) != 34 {
		t.Fatalf("BuildTxExtra length = %d, want 34", len(raw))
	}
}

// buildTestExtra constructs a raw extra with the given fields.
func buildTestExtra(txPubKey types.PublicKey, unlockTime uint64, hint uint16) []byte {
	var count uint64
	var elements []byte

	if !txPubKey.IsZero() {
		count++
		elements = append(elements, extraTagPublicKey)
		elements = append(elements, txPubKey[:]...)
	}
	if unlockTime > 0 {
		count++
		elements = append(elements, extraTagUnlockTime)
		elements = append(elements, wire.EncodeVarint(unlockTime)...)
	}
	if hint > 0 {
		count++
		elements = append(elements, extraTagDerivationHint)
		elements = append(elements, wire.EncodeVarint(2)...)
		elements = append(elements, byte(hint&0xFF), byte(hint>>8))
	}

	raw := wire.EncodeVarint(count)
	raw = append(raw, elements...)
	return raw
}
