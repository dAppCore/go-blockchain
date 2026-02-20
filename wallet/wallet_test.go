// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// You may obtain a copy of the licence at:
//
//     https://joinup.ec.europa.eu/software/page/eupl/licence-eupl
//
// SPDX-License-Identifier: EUPL-1.2

package wallet

import (
	"testing"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/chain"
	"forge.lthn.ai/core/go-blockchain/crypto"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

func makeTestBlock(t *testing.T, height uint64, prevHash types.Hash,
	destAccount *Account) (*types.Block, types.Hash) {
	t.Helper()

	txPub, txSec, _ := crypto.GenerateKeys()
	derivation, _ := crypto.GenerateKeyDerivation(
		[32]byte(destAccount.ViewPublicKey), txSec)
	ephPub, _ := crypto.DerivePublicKey(
		derivation, 0, [32]byte(destAccount.SpendPublicKey))

	minerTx := types.Transaction{
		Version: 0,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: height}},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 1_000_000_000_000, // 1 LTHN
				Target: types.TxOutToKey{Key: types.PublicKey(ephPub)},
			},
		},
		Extra:      BuildTxExtra(types.PublicKey(txPub)),
		Attachment: wire.EncodeVarint(0),
	}
	minerTx.Signatures = [][]types.Signature{{}}

	blk := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Timestamp:    1770897600 + height*120,
			PrevID:       prevHash,
		},
		MinerTx: minerTx,
	}

	hash := wire.BlockHash(blk)
	return blk, hash
}

func TestWalletSyncAndBalance(t *testing.T) {
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	acc, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	c := chain.New(s)

	// Store 3 blocks, each paying 1 LTHN to our account.
	var prevHash types.Hash
	for h := uint64(0); h < 3; h++ {
		blk, hash := makeTestBlock(t, h, prevHash, acc)
		meta := &chain.BlockMeta{Hash: hash, Height: h}
		if err := c.PutBlock(blk, meta); err != nil {
			t.Fatal(err)
		}
		// Index the output.
		txHash := wire.TransactionHash(&blk.MinerTx)
		c.PutOutput(1_000_000_000_000, txHash, 0)
		prevHash = hash
	}

	w := NewWallet(acc, s, c, nil)
	if err := w.Sync(); err != nil {
		t.Fatal(err)
	}

	confirmed, locked, err := w.Balance()
	if err != nil {
		t.Fatal(err)
	}

	// All 3 blocks are coinbase with MinedMoneyUnlockWindow=10.
	// Chain height = 3, so all 3 are locked (height + 10 > 3).
	if locked != 3_000_000_000_000 {
		t.Fatalf("locked = %d, want 3_000_000_000_000", locked)
	}
	if confirmed != 0 {
		t.Fatalf("confirmed = %d, want 0 (all locked)", confirmed)
	}
}

func TestWalletTransfers(t *testing.T) {
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	acc, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	c := chain.New(s)

	blk, hash := makeTestBlock(t, 0, types.Hash{}, acc)
	meta := &chain.BlockMeta{Hash: hash, Height: 0}
	c.PutBlock(blk, meta)
	txHash := wire.TransactionHash(&blk.MinerTx)
	c.PutOutput(1_000_000_000_000, txHash, 0)

	w := NewWallet(acc, s, c, nil)
	w.Sync()

	transfers, err := w.Transfers()
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 1 {
		t.Fatalf("got %d transfers, want 1", len(transfers))
	}
}
