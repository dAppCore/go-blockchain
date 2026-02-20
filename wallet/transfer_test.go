// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wallet

import (
	"testing"

	store "forge.lthn.ai/core/go-store"

	"forge.lthn.ai/core/go-blockchain/types"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestTransferPutGet(t *testing.T) {
	s := newTestStore(t)

	var ki types.KeyImage
	ki[0] = 0x42
	tr := Transfer{
		TxHash:      types.Hash{1},
		OutputIndex: 0,
		Amount:      1000,
		BlockHeight: 10,
		KeyImage:    ki,
	}

	if err := putTransfer(s, &tr); err != nil {
		t.Fatal(err)
	}

	got, err := getTransfer(s, ki)
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != 1000 {
		t.Fatalf("amount = %d, want 1000", got.Amount)
	}
	if got.TxHash != tr.TxHash {
		t.Fatalf("tx hash mismatch: got %s", got.TxHash)
	}
	if got.KeyImage != ki {
		t.Fatalf("key image mismatch: got %s", got.KeyImage)
	}
}

func TestTransferGetNotFound(t *testing.T) {
	s := newTestStore(t)

	var ki types.KeyImage
	ki[0] = 0xFF
	_, err := getTransfer(s, ki)
	if err == nil {
		t.Fatal("expected error for missing transfer")
	}
}

func TestTransferOverwrite(t *testing.T) {
	s := newTestStore(t)

	var ki types.KeyImage
	ki[0] = 0x01
	tr := Transfer{Amount: 500, BlockHeight: 5, KeyImage: ki}
	if err := putTransfer(s, &tr); err != nil {
		t.Fatal(err)
	}

	tr.Amount = 999
	if err := putTransfer(s, &tr); err != nil {
		t.Fatal(err)
	}

	got, err := getTransfer(s, ki)
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != 999 {
		t.Fatalf("amount = %d, want 999 after overwrite", got.Amount)
	}
}

func TestTransferMarkSpent(t *testing.T) {
	s := newTestStore(t)

	var ki types.KeyImage
	ki[0] = 0x43
	tr := Transfer{Amount: 500, BlockHeight: 5, KeyImage: ki}
	if err := putTransfer(s, &tr); err != nil {
		t.Fatal(err)
	}

	if err := markTransferSpent(s, ki, 20); err != nil {
		t.Fatal(err)
	}

	got, err := getTransfer(s, ki)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Spent {
		t.Fatal("should be spent")
	}
	if got.SpentHeight != 20 {
		t.Fatalf("spent height = %d, want 20", got.SpentHeight)
	}
}

func TestTransferMarkSpentNotFound(t *testing.T) {
	s := newTestStore(t)

	var ki types.KeyImage
	ki[0] = 0xDE
	if err := markTransferSpent(s, ki, 10); err == nil {
		t.Fatal("expected error marking non-existent transfer as spent")
	}
}

func TestTransferList(t *testing.T) {
	s := newTestStore(t)

	for i := byte(0); i < 3; i++ {
		var ki types.KeyImage
		ki[0] = i + 1
		tr := Transfer{
			Amount:      uint64(i+1) * 100,
			BlockHeight: uint64(i),
			KeyImage:    ki,
		}
		if err := putTransfer(s, &tr); err != nil {
			t.Fatal(err)
		}
	}

	// Mark second transfer as spent.
	var ki types.KeyImage
	ki[0] = 2
	if err := markTransferSpent(s, ki, 10); err != nil {
		t.Fatal(err)
	}

	transfers, err := listTransfers(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 3 {
		t.Fatalf("got %d transfers, want 3", len(transfers))
	}

	unspent := 0
	for _, tr := range transfers {
		if !tr.Spent {
			unspent++
		}
	}
	if unspent != 2 {
		t.Fatalf("got %d unspent, want 2", unspent)
	}
}

func TestTransferListEmpty(t *testing.T) {
	s := newTestStore(t)

	transfers, err := listTransfers(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 0 {
		t.Fatalf("got %d transfers, want 0 for empty store", len(transfers))
	}
}

func TestTransferSpendableBasic(t *testing.T) {
	tr := Transfer{Amount: 1000, BlockHeight: 5}
	if !tr.IsSpendable(20, false) {
		t.Fatal("unspent transfer should be spendable")
	}
}

func TestTransferSpendableSpent(t *testing.T) {
	tr := Transfer{Amount: 1000, BlockHeight: 5, Spent: true}
	if tr.IsSpendable(20, false) {
		t.Fatal("spent transfer should not be spendable")
	}
}

func TestTransferSpendableCoinbaseImmature(t *testing.T) {
	tr := Transfer{Amount: 1000, BlockHeight: 5, Coinbase: true}
	// MinedMoneyUnlockWindow is 10, so block 5 + 10 = 15 > 10.
	if tr.IsSpendable(10, false) {
		t.Fatal("immature coinbase should not be spendable")
	}
}

func TestTransferSpendableCoinbaseMature(t *testing.T) {
	tr := Transfer{Amount: 1000, BlockHeight: 5, Coinbase: true}
	// MinedMoneyUnlockWindow is 10, so block 5 + 10 = 15 <= 20.
	if !tr.IsSpendable(20, false) {
		t.Fatal("mature coinbase should be spendable")
	}
}

func TestTransferSpendableCoinbaseBoundary(t *testing.T) {
	tr := Transfer{Amount: 1000, BlockHeight: 5, Coinbase: true}
	// Exact boundary: 5 + 10 = 15 == 15, not greater, so spendable.
	if !tr.IsSpendable(15, false) {
		t.Fatal("coinbase at exact maturity boundary should be spendable")
	}
}

func TestTransferSpendableUnlockTime(t *testing.T) {
	tr := Transfer{Amount: 1000, BlockHeight: 5, UnlockTime: 50}
	if tr.IsSpendable(30, false) {
		t.Fatal("transfer with future unlock time should not be spendable")
	}
	if !tr.IsSpendable(50, false) {
		t.Fatal("transfer at exact unlock height should be spendable")
	}
	if !tr.IsSpendable(100, false) {
		t.Fatal("transfer past unlock height should be spendable")
	}
}

func TestTransferSpendableUnlockTimeZero(t *testing.T) {
	tr := Transfer{Amount: 1000, BlockHeight: 5, UnlockTime: 0}
	if !tr.IsSpendable(1, false) {
		t.Fatal("transfer with zero unlock time should be spendable")
	}
}

func TestTransferKeyPairFields(t *testing.T) {
	s := newTestStore(t)

	var ki types.KeyImage
	ki[0] = 0x99
	var pub types.PublicKey
	pub[0] = 0xAA
	var sec types.SecretKey
	sec[0] = 0xBB

	tr := Transfer{
		Amount:       2000,
		BlockHeight:  50,
		KeyImage:     ki,
		EphemeralKey: KeyPair{Public: pub, Secret: sec},
	}

	if err := putTransfer(s, &tr); err != nil {
		t.Fatal(err)
	}

	got, err := getTransfer(s, ki)
	if err != nil {
		t.Fatal(err)
	}
	if got.EphemeralKey.Public != pub {
		t.Fatal("ephemeral public key mismatch")
	}
	if got.EphemeralKey.Secret != sec {
		t.Fatal("ephemeral secret key mismatch")
	}
}
