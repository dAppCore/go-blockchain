// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

//go:build integration

package consensus

import (
	"testing"

	store "dappco.re/go/core/store"

	"dappco.re/go/core/blockchain/chain"
	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/rpc"
)

func TestConsensusIntegration(t *testing.T) {
	client := rpc.NewClient("http://localhost:46941")

	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	c := chain.New(s)

	// Sync with consensus validation using testnet fork schedule.
	opts := chain.SyncOptions{
		Forks: config.TestnetForks,
	}

	if err := c.Sync(client, opts); err != nil {
		t.Fatalf("sync with consensus validation: %v", err)
	}

	height, err := c.Height()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Synced %d blocks with consensus validation", height)

	if height == 0 {
		t.Fatal("expected at least 1 block synced")
	}
}
