// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wallet

import (
	"strings"
	"testing"
)

func TestAccountGenerate(t *testing.T) {
	acc, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	var zero [32]byte
	if acc.SpendSecretKey == zero {
		t.Fatal("spend secret is zero")
	}
	if acc.ViewSecretKey == zero {
		t.Fatal("view secret is zero")
	}
	if acc.SpendPublicKey == zero {
		t.Fatal("spend public is zero")
	}
	if acc.ViewPublicKey == zero {
		t.Fatal("view public is zero")
	}
}

func TestAccountSeedRoundTrip(t *testing.T) {
	acc1, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	phrase, err := acc1.ToSeed()
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Fields(phrase)
	if len(words) != 25 {
		t.Fatalf("seed has %d words, want 25", len(words))
	}

	acc2, err := RestoreFromSeed(phrase)
	if err != nil {
		t.Fatal(err)
	}
	if acc1.SpendSecretKey != acc2.SpendSecretKey {
		t.Fatal("spend secret mismatch")
	}
	if acc1.ViewSecretKey != acc2.ViewSecretKey {
		t.Fatal("view secret mismatch")
	}
	if acc1.SpendPublicKey != acc2.SpendPublicKey {
		t.Fatal("spend public mismatch")
	}
	if acc1.ViewPublicKey != acc2.ViewPublicKey {
		t.Fatal("view public mismatch")
	}
}

func TestAccountViewOnly(t *testing.T) {
	acc, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	view, err := RestoreViewOnly(acc.ViewSecretKey, acc.SpendPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if view.ViewPublicKey != acc.ViewPublicKey {
		t.Fatal("view public mismatch")
	}
	if view.SpendPublicKey != acc.SpendPublicKey {
		t.Fatal("spend public mismatch")
	}

	var zero [32]byte
	if view.SpendSecretKey != zero {
		t.Fatal("view-only should have zero spend secret")
	}
}

func TestAccountSaveLoad(t *testing.T) {
	s := newTestStore(t)

	acc1, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	if err := acc1.Save(s, "test-password-123"); err != nil {
		t.Fatal(err)
	}

	acc2, err := LoadAccount(s, "test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	if acc1.SpendSecretKey != acc2.SpendSecretKey {
		t.Fatal("spend secret mismatch")
	}
	if acc1.ViewSecretKey != acc2.ViewSecretKey {
		t.Fatal("view secret mismatch")
	}
	if acc1.SpendPublicKey != acc2.SpendPublicKey {
		t.Fatal("spend public mismatch")
	}
	if acc1.ViewPublicKey != acc2.ViewPublicKey {
		t.Fatal("view public mismatch")
	}
}

func TestAccountLoadWrongPassword(t *testing.T) {
	s := newTestStore(t)

	acc, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	if err := acc.Save(s, "correct"); err != nil {
		t.Fatal(err)
	}

	_, err = LoadAccount(s, "wrong")
	if err == nil {
		t.Fatal("expected error with wrong password")
	}
}

func TestAccountAddress(t *testing.T) {
	acc, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	addr := acc.Address()
	if addr.SpendPublicKey != acc.SpendPublicKey {
		t.Fatal("address spend public key mismatch")
	}
	if addr.ViewPublicKey != acc.ViewPublicKey {
		t.Fatal("address view public key mismatch")
	}
}
