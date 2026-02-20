//go:build integration

// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import (
	"net/http"
	"testing"
	"time"
)

const testnetRPCAddr = "http://localhost:46941"

// TestIntegration_RPC connects to the C++ testnet daemon and queries
// all Tier 1 endpoints.
func TestIntegration_RPC(t *testing.T) {
	c := NewClientWithHTTP(testnetRPCAddr, &http.Client{Timeout: 10 * time.Second})

	// --- GetHeight ---
	height, err := c.GetHeight()
	if err != nil {
		t.Skipf("testnet daemon not reachable at %s: %v", testnetRPCAddr, err)
	}
	if height == 0 {
		t.Error("height is 0 — daemon may not be synced")
	}
	t.Logf("testnet height: %d", height)

	// --- GetBlockCount ---
	count, err := c.GetBlockCount()
	if err != nil {
		t.Fatalf("GetBlockCount: %v", err)
	}
	// count should be height or height+1
	if count < height {
		t.Errorf("count %d < height %d", count, height)
	}
	t.Logf("testnet block count: %d", count)

	// --- GetInfo ---
	info, err := c.GetInfo()
	if err != nil {
		t.Fatalf("GetInfo: %v", err)
	}
	if info.Height == 0 {
		t.Error("info.height is 0")
	}
	t.Logf("testnet info: height=%d, tx_count=%d, connections=%d+%d",
		info.Height, info.TxCount,
		info.OutgoingConnectionsCount, info.IncomingConnectionsCount)

	// --- GetLastBlockHeader ---
	lastHdr, err := c.GetLastBlockHeader()
	if err != nil {
		t.Fatalf("GetLastBlockHeader: %v", err)
	}
	if lastHdr.Hash == "" {
		t.Error("last block hash is empty")
	}
	t.Logf("testnet last block: height=%d, hash=%s", lastHdr.Height, lastHdr.Hash)

	// --- GetBlockHeaderByHeight (genesis) ---
	genesis, err := c.GetBlockHeaderByHeight(0)
	if err != nil {
		t.Fatalf("GetBlockHeaderByHeight(0): %v", err)
	}
	const expectedGenesisHash = "cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963"
	if genesis.Hash != expectedGenesisHash {
		t.Errorf("genesis hash: got %q, want %q", genesis.Hash, expectedGenesisHash)
	}
	t.Logf("genesis block hash verified: %s", genesis.Hash)

	// --- GetBlockHeaderByHash (using genesis hash) ---
	byHash, err := c.GetBlockHeaderByHash(expectedGenesisHash)
	if err != nil {
		t.Fatalf("GetBlockHeaderByHash: %v", err)
	}
	if byHash.Height != 0 {
		t.Errorf("genesis by hash: height got %d, want 0", byHash.Height)
	}

	// --- GetBlocksDetails (genesis block) ---
	blocks, err := c.GetBlocksDetails(0, 1)
	if err != nil {
		t.Fatalf("GetBlocksDetails: %v", err)
	}
	if len(blocks) == 0 {
		t.Fatal("GetBlocksDetails returned 0 blocks")
	}
	if blocks[0].ID != expectedGenesisHash {
		t.Errorf("block[0].id: got %q, want %q", blocks[0].ID, expectedGenesisHash)
	}
	t.Logf("genesis block details: reward=%d, type=%d", blocks[0].BaseReward, blocks[0].Type)
}
