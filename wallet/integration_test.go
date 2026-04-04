//go:build integration

// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wallet

import (
	"testing"

	"dappco.re/go/core/blockchain/crypto"
	"dappco.re/go/core/blockchain/types"
)

func TestIntegration_WalletLifecycle_Good(t *testing.T) {
	// 1. Generate account
	account, err := GenerateAccount()
	if err != nil {
		t.Fatalf("GenerateAccount: %v", err)
	}

	addr := account.Address()
	if addr.SpendPublicKey == (types.PublicKey{}) {
		t.Fatal("spend key is zero")
	}
	if addr.ViewPublicKey == (types.PublicKey{}) {
		t.Fatal("view key is zero")
	}

	// 2. Encode address — should start with iTHN
	encoded := addr.Encode(0x1eaf7)
	if len(encoded) < 90 {
		t.Errorf("encoded address too short: %d chars", len(encoded))
	}
	if encoded[:4] != "iTHN" {
		t.Errorf("address prefix: got %s, want iTHN", encoded[:4])
	}

	// 3. Export and restore seed
	seed, err := account.ToSeed()
	if err != nil {
		t.Fatalf("ToSeed: %v", err)
	}
	words := mnemonicWords(seed)
	if len(words) != 25 {
		t.Errorf("seed words: got %d, want 25", len(words))
	}

	restored, err := RestoreFromSeed(seed)
	if err != nil {
		t.Fatalf("RestoreFromSeed: %v", err)
	}

	if restored.SpendPublicKey != account.SpendPublicKey {
		t.Error("restored spend key doesn't match")
	}
	if restored.ViewPublicKey != account.ViewPublicKey {
		t.Error("restored view key doesn't match")
	}

	// 4. Generate key image from ephemeral keypair
	var ephPub, ephSec [32]byte
	copy(ephPub[:], account.SpendPublicKey[:])
	copy(ephSec[:], account.SpendSecretKey[:])
	ki, err := crypto.GenerateKeyImage(ephPub, ephSec)
	if err != nil {
		t.Fatalf("GenerateKeyImage: %v", err)
	}
	if ki == ([32]byte{}) {
		t.Error("key image is zero")
	}

	// 5. Verify key image is valid
	if !crypto.ValidateKeyImage(ki) {
		t.Error("key image failed validation")
	}

	// 6. Test NLSAG signer exists and has correct version
	signer := &NLSAGSigner{}
	if signer.Version() != 1 {
		t.Errorf("signer version: got %d, want 1", signer.Version())
	}

	// 7. Test scanner creation
	scanner := NewV1Scanner(account)
	if scanner == nil {
		t.Error("scanner is nil")
	}
}

func TestIntegration_IntegratedAddress_Good(t *testing.T) {
	account, _ := GenerateAccount()
	addr := account.Address()

	// Standard address
	standard := addr.Encode(0x1eaf7)
	if standard[:4] != "iTHN" {
		t.Errorf("standard prefix: %s", standard[:4])
	}

	// Integrated address (different prefix)
	integrated := addr.Encode(0xdeaf7)
	if integrated[:4] != "iTHn" {
		t.Errorf("integrated prefix: %s", integrated[:4])
	}

	// Auditable address
	auditable := addr.Encode(0x3ceff7)
	if auditable[:4] != "iThN" {
		t.Errorf("auditable prefix: %s", auditable[:4])
	}

	// All three should be different
	if standard == integrated {
		t.Error("standard == integrated")
	}
	if standard == auditable {
		t.Error("standard == auditable")
	}
}
