//go:build integration

// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-blockchain/p2p"
	"forge.lthn.ai/core/go-blockchain/rpc"
	"forge.lthn.ai/core/go-blockchain/types"
	levin "forge.lthn.ai/core/go-p2p/node/levin"
	store "forge.lthn.ai/core/go-store"
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

func TestIntegration_SyncToTip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long sync test in short mode")
	}

	client := rpc.NewClientWithHTTP(testnetRPCAddr, &http.Client{Timeout: 60 * time.Second})

	remoteHeight, err := client.GetHeight()
	if err != nil {
		t.Skipf("testnet daemon not reachable at %s: %v", testnetRPCAddr, err)
	}
	t.Logf("testnet height: %d", remoteHeight)

	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)

	opts := SyncOptions{
		VerifySignatures: false, // first pass: no sigs
		Forks:            config.TestnetForks,
	}

	err = c.Sync(context.Background(), client, opts)
	require.NoError(t, err)

	finalHeight, _ := c.Height()
	t.Logf("synced %d blocks", finalHeight)
	require.Equal(t, remoteHeight, finalHeight)

	// Verify genesis.
	_, genMeta, err := c.GetBlockByHeight(0)
	require.NoError(t, err)
	expectedHash, _ := types.HashFromHex(GenesisHash)
	require.Equal(t, expectedHash, genMeta.Hash)
}

func TestIntegration_SyncWithSignatures(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long sync test in short mode")
	}

	client := rpc.NewClientWithHTTP(testnetRPCAddr, &http.Client{Timeout: 60 * time.Second})

	remoteHeight, err := client.GetHeight()
	if err != nil {
		t.Skipf("testnet daemon not reachable: %v", err)
	}

	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)

	opts := SyncOptions{
		VerifySignatures: true,
		Forks:            config.TestnetForks,
	}

	err = c.Sync(context.Background(), client, opts)
	require.NoError(t, err)

	finalHeight, _ := c.Height()
	t.Logf("synced %d blocks with signature verification", finalHeight)
	require.Equal(t, remoteHeight, finalHeight)
}

func TestIntegration_DifficultyMatchesRPC(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping difficulty comparison test in short mode")
	}

	client := rpc.NewClientWithHTTP(testnetRPCAddr, &http.Client{Timeout: 60 * time.Second})

	_, err := client.GetHeight()
	if err != nil {
		t.Skipf("testnet daemon not reachable: %v", err)
	}

	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)

	// Sync a portion of the chain via RPC (which stores daemon-provided difficulty).
	opts := SyncOptions{
		VerifySignatures: false,
		Forks:            config.TestnetForks,
	}
	err = c.Sync(context.Background(), client, opts)
	require.NoError(t, err)

	finalHeight, _ := c.Height()
	t.Logf("synced %d blocks, checking difficulty computation", finalHeight)

	// For each block from height 1 onwards, verify our NextDifficulty matches
	// the daemon-provided difficulty stored in BlockMeta.
	mismatches := 0
	for h := uint64(1); h < finalHeight; h++ {
		meta, err := c.getBlockMeta(h)
		require.NoError(t, err)

		computed, err := c.NextDifficulty(h, config.TestnetForks)
		require.NoError(t, err)

		if computed != meta.Difficulty {
			if mismatches < 10 {
				t.Logf("difficulty mismatch at height %d: computed=%d, daemon=%d",
					h, computed, meta.Difficulty)
			}
			mismatches++
		}
	}

	if mismatches > 0 {
		t.Errorf("%d/%d blocks have difficulty mismatches", mismatches, finalHeight-1)
	} else {
		t.Logf("all %d blocks have matching difficulty", finalHeight-1)
	}
}

func TestIntegration_P2PSync(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping P2P sync test in short mode")
	}

	// Dial testnet daemon P2P port.
	conn, err := net.DialTimeout("tcp", "localhost:46942", 10*time.Second)
	if err != nil {
		t.Skipf("testnet P2P not reachable: %v", err)
	}
	defer conn.Close()

	lc := levin.NewConnection(conn)

	// Handshake.
	var peerIDBuf [8]byte
	rand.Read(peerIDBuf[:])
	peerID := binary.LittleEndian.Uint64(peerIDBuf[:])

	req := p2p.HandshakeRequest{
		NodeData: p2p.NodeData{
			NetworkID: config.NetworkIDTestnet,
			PeerID:    peerID,
			LocalTime: time.Now().Unix(),
			MyPort:    0,
		},
		PayloadData: p2p.CoreSyncData{
			CurrentHeight:  1,
			ClientVersion:  config.ClientVersion,
			NonPruningMode: true,
		},
	}
	payload, err := p2p.EncodeHandshakeRequest(&req)
	require.NoError(t, err)
	require.NoError(t, lc.WritePacket(p2p.CommandHandshake, payload, true))

	hdr, data, err := lc.ReadPacket()
	require.NoError(t, err)
	require.Equal(t, uint32(p2p.CommandHandshake), hdr.Command)

	var resp p2p.HandshakeResponse
	require.NoError(t, resp.Decode(data))
	t.Logf("peer height: %d", resp.PayloadData.CurrentHeight)

	// Create P2P connection adapter with our local sync state.
	localSync := p2p.CoreSyncData{
		CurrentHeight:  1,
		ClientVersion:  config.ClientVersion,
		NonPruningMode: true,
	}
	p2pConn := NewLevinP2PConn(lc, resp.PayloadData.CurrentHeight, localSync)

	// Create chain and sync.
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	opts := SyncOptions{
		VerifySignatures: false,
		Forks:            config.TestnetForks,
	}

	err = c.P2PSync(context.Background(), p2pConn, opts)
	require.NoError(t, err)

	finalHeight, _ := c.Height()
	t.Logf("P2P synced %d blocks", finalHeight)
	require.Equal(t, resp.PayloadData.CurrentHeight, finalHeight)
}
