// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// You may obtain a copy of the licence at:
//
//     https://joinup.ec.europa.eu/software/page/eupl/licence-eupl
//
// SPDX-License-Identifier: EUPL-1.2

//go:build integration

package wallet

import (
	"testing"

	store "dappco.re/go/core/store"
	"dappco.re/go/core/blockchain/chain"
	"dappco.re/go/core/blockchain/rpc"
)

func TestWalletIntegration(t *testing.T) {
	client := rpc.NewClient("http://localhost:46941")

	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	c := chain.New(s)

	// Sync chain first.
	if err := c.Sync(client, chain.DefaultSyncOptions()); err != nil {
		t.Fatalf("chain sync: %v", err)
	}

	acc, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	w := NewWallet(acc, s, c, client)
	if err := w.Sync(); err != nil {
		t.Fatal(err)
	}

	confirmed, locked, err := w.Balance()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Balance: confirmed=%d, locked=%d", confirmed, locked)

	transfers, err := w.Transfers()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Transfers: %d", len(transfers))
}
