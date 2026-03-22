// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wallet

import (
	"testing"

	"dappco.re/go/core/blockchain/crypto"
	"dappco.re/go/core/blockchain/types"
	"dappco.re/go/core/blockchain/wire"
)

// makeTestTransaction creates a v0 coinbase tx with one output sent to destAddr.
// It returns the transaction, its hash, and the ephemeral secret key used to
// construct the output (for test verification).
func makeTestTransaction(t *testing.T, destAddr *Account) (*types.Transaction, types.Hash, [32]byte) {
	t.Helper()

	txPub, txSec, err := crypto.GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}

	derivation, err := crypto.GenerateKeyDerivation(
		[32]byte(destAddr.ViewPublicKey), txSec)
	if err != nil {
		t.Fatal(err)
	}

	ephPub, err := crypto.DerivePublicKey(
		derivation, 0, [32]byte(destAddr.SpendPublicKey))
	if err != nil {
		t.Fatal(err)
	}

	tx := &types.Transaction{
		Version: 0,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 0}},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 1000,
				Target: types.TxOutToKey{Key: types.PublicKey(ephPub)},
			},
		},
		Extra:      BuildTxExtra(types.PublicKey(txPub)),
		Attachment: wire.EncodeVarint(0),
		Signatures: [][]types.Signature{{}},
	}

	txHash := wire.TransactionHash(tx)
	return tx, txHash, txSec
}

func TestV1ScannerDetectsOwnedOutput(t *testing.T) {
	acc, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	tx, txHash, _ := makeTestTransaction(t, acc)
	extra, err := ParseTxExtra(tx.Extra)
	if err != nil {
		t.Fatal(err)
	}

	scanner := NewV1Scanner(acc)
	transfers, err := scanner.ScanTransaction(tx, txHash, 1, extra)
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 1 {
		t.Fatalf("got %d transfers, want 1", len(transfers))
	}
	if transfers[0].Amount != 1000 {
		t.Fatalf("amount = %d, want 1000", transfers[0].Amount)
	}
	if transfers[0].OutputIndex != 0 {
		t.Fatalf("output index = %d, want 0", transfers[0].OutputIndex)
	}
	if transfers[0].TxHash != txHash {
		t.Fatal("tx hash mismatch")
	}
	if transfers[0].BlockHeight != 1 {
		t.Fatalf("block height = %d, want 1", transfers[0].BlockHeight)
	}

	var zeroKI types.KeyImage
	if transfers[0].KeyImage == zeroKI {
		t.Fatal("key image should be non-zero")
	}

	var zeroPK types.PublicKey
	if transfers[0].EphemeralKey.Public == zeroPK {
		t.Fatal("ephemeral public key should be non-zero")
	}

	var zeroSK types.SecretKey
	if transfers[0].EphemeralKey.Secret == zeroSK {
		t.Fatal("ephemeral secret key should be non-zero")
	}
}

func TestV1ScannerRejectsNonOwned(t *testing.T) {
	acc1, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	acc2, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	tx, txHash, _ := makeTestTransaction(t, acc1)
	extra, err := ParseTxExtra(tx.Extra)
	if err != nil {
		t.Fatal(err)
	}

	scanner := NewV1Scanner(acc2)
	transfers, err := scanner.ScanTransaction(tx, txHash, 1, extra)
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 0 {
		t.Fatalf("got %d transfers, want 0", len(transfers))
	}
}

func TestV1ScannerNoTxPubKey(t *testing.T) {
	acc, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	tx := &types.Transaction{
		Version:    0,
		Vin:        []types.TxInput{types.TxInputGenesis{Height: 0}},
		Vout:       []types.TxOutput{types.TxOutputBare{Amount: 100}},
		Extra:      wire.EncodeVarint(0),
		Attachment: wire.EncodeVarint(0),
		Signatures: [][]types.Signature{{}},
	}
	txHash := wire.TransactionHash(tx)
	extra, err := ParseTxExtra(tx.Extra)
	if err != nil {
		t.Fatal(err)
	}

	scanner := NewV1Scanner(acc)
	transfers, err := scanner.ScanTransaction(tx, txHash, 1, extra)
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 0 {
		t.Fatalf("expected 0 transfers for missing tx pub key, got %d", len(transfers))
	}
}

func TestV1ScannerCoinbaseFlag(t *testing.T) {
	acc, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	tx, txHash, _ := makeTestTransaction(t, acc)
	extra, err := ParseTxExtra(tx.Extra)
	if err != nil {
		t.Fatal(err)
	}

	scanner := NewV1Scanner(acc)
	transfers, err := scanner.ScanTransaction(tx, txHash, 1, extra)
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 1 {
		t.Fatalf("got %d transfers, want 1", len(transfers))
	}
	if !transfers[0].Coinbase {
		t.Fatal("should be marked as coinbase (TxInputGenesis)")
	}
}

func TestV1ScannerUnlockTime(t *testing.T) {
	acc, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	txPub, txSec, err := crypto.GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}

	derivation, err := crypto.GenerateKeyDerivation(
		[32]byte(acc.ViewPublicKey), txSec)
	if err != nil {
		t.Fatal(err)
	}

	ephPub, err := crypto.DerivePublicKey(
		derivation, 0, [32]byte(acc.SpendPublicKey))
	if err != nil {
		t.Fatal(err)
	}

	// Build extra with unlock time.
	extraRaw := buildTestExtra(types.PublicKey(txPub), 500, 0)

	tx := &types.Transaction{
		Version: 0,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 0}},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 2000,
				Target: types.TxOutToKey{Key: types.PublicKey(ephPub)},
			},
		},
		Extra:      extraRaw,
		Attachment: wire.EncodeVarint(0),
		Signatures: [][]types.Signature{{}},
	}
	txHash := wire.TransactionHash(tx)
	extra, err := ParseTxExtra(tx.Extra)
	if err != nil {
		t.Fatal(err)
	}

	scanner := NewV1Scanner(acc)
	transfers, err := scanner.ScanTransaction(tx, txHash, 10, extra)
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 1 {
		t.Fatalf("got %d transfers, want 1", len(transfers))
	}
	if transfers[0].UnlockTime != 500 {
		t.Fatalf("unlock time = %d, want 500", transfers[0].UnlockTime)
	}
}

func TestV1ScannerMultipleOutputs(t *testing.T) {
	acc, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	txPub, txSec, err := crypto.GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}

	derivation, err := crypto.GenerateKeyDerivation(
		[32]byte(acc.ViewPublicKey), txSec)
	if err != nil {
		t.Fatal(err)
	}

	// Create two outputs for our account at indices 0 and 1.
	eph0, err := crypto.DerivePublicKey(derivation, 0, [32]byte(acc.SpendPublicKey))
	if err != nil {
		t.Fatal(err)
	}
	eph1, err := crypto.DerivePublicKey(derivation, 1, [32]byte(acc.SpendPublicKey))
	if err != nil {
		t.Fatal(err)
	}

	tx := &types.Transaction{
		Version: 0,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 0}},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 100,
				Target: types.TxOutToKey{Key: types.PublicKey(eph0)},
			},
			types.TxOutputBare{
				Amount: 200,
				Target: types.TxOutToKey{Key: types.PublicKey(eph1)},
			},
		},
		Extra:      BuildTxExtra(types.PublicKey(txPub)),
		Attachment: wire.EncodeVarint(0),
		Signatures: [][]types.Signature{{}},
	}
	txHash := wire.TransactionHash(tx)
	extra, err := ParseTxExtra(tx.Extra)
	if err != nil {
		t.Fatal(err)
	}

	scanner := NewV1Scanner(acc)
	transfers, err := scanner.ScanTransaction(tx, txHash, 5, extra)
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 2 {
		t.Fatalf("got %d transfers, want 2", len(transfers))
	}
	if transfers[0].OutputIndex != 0 || transfers[0].Amount != 100 {
		t.Fatalf("transfer[0]: index=%d amount=%d, want index=0 amount=100",
			transfers[0].OutputIndex, transfers[0].Amount)
	}
	if transfers[1].OutputIndex != 1 || transfers[1].Amount != 200 {
		t.Fatalf("transfer[1]: index=%d amount=%d, want index=1 amount=200",
			transfers[1].OutputIndex, transfers[1].Amount)
	}

	// Key images must be unique per output.
	if transfers[0].KeyImage == transfers[1].KeyImage {
		t.Fatal("key images should differ between outputs")
	}
}

func TestV1ScannerImplementsInterface(t *testing.T) {
	acc, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	var _ Scanner = NewV1Scanner(acc)
}
