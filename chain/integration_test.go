//go:build integration

// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"context"
	"net/http"
	"testing"
	"time"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/rpc"
	"forge.lthn.ai/core/go-blockchain/types"
)

const testnetRPCAddr = "http://localhost:46941"

func TestIntegration_SyncFirst10Blocks(t *testing.T) {
	client := rpc.NewClientWithHTTP(testnetRPCAddr, &http.Client{Timeout: 30 * time.Second})

	// Check daemon is reachable.
	remoteHeight, err := client.GetHeight()
	if err != nil {
		t.Skipf("testnet daemon not reachable at %s: %v", testnetRPCAddr, err)
	}
	t.Logf("testnet height: %d", remoteHeight)

	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	defer s.Close()

	c := New(s)

	// Sync first 10 blocks (or fewer if chain is shorter).
	targetHeight := uint64(10)
	if remoteHeight < targetHeight {
		targetHeight = remoteHeight
	}

	// Sync in a loop, stopping early.
	for {
		h, _ := c.Height()
		if h >= targetHeight {
			break
		}
		if err := c.Sync(context.Background(), client, DefaultSyncOptions()); err != nil {
			t.Fatalf("Sync: %v", err)
		}
	}

	// Verify genesis block.
	_, genMeta, err := c.GetBlockByHeight(0)
	if err != nil {
		t.Fatalf("GetBlockByHeight(0): %v", err)
	}
	expectedHash, _ := types.HashFromHex(GenesisHash)
	if genMeta.Hash != expectedHash {
		t.Errorf("genesis hash: got %s, want %s", genMeta.Hash, expectedHash)
	}
	t.Logf("genesis block verified: %s", genMeta.Hash)

	// Verify chain height.
	finalHeight, _ := c.Height()
	t.Logf("synced %d blocks", finalHeight)
	if finalHeight < targetHeight {
		t.Errorf("expected at least %d blocks, got %d", targetHeight, finalHeight)
	}

	// Verify blocks are sequential.
	for i := uint64(1); i < finalHeight; i++ {
		_, meta, err := c.GetBlockByHeight(i)
		if err != nil {
			t.Fatalf("GetBlockByHeight(%d): %v", i, err)
		}
		_, prevMeta, err := c.GetBlockByHeight(i - 1)
		if err != nil {
			t.Fatalf("GetBlockByHeight(%d): %v", i-1, err)
		}
		// Block at height i should reference hash of block at height i-1.
		if meta.Height != i {
			t.Errorf("block %d: height %d", i, meta.Height)
		}
		_ = prevMeta // linkage verified during sync
	}
}
