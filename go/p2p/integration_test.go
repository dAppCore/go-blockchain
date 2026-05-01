//go:build integration

// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package p2p

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"net"
	"testing"
	"time"

	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/p2p/node/levin"
	"github.com/stretchr/testify/require"
)

const testnetP2PAddr = "localhost:46942"

// TestIntegration_Handshake connects to the C++ testnet daemon,
// performs a full handshake, and verifies the response.
func TestIntegration_Handshake_Good(t *testing.T) {
	conn, err := net.DialTimeout("tcp", testnetP2PAddr, 10*time.Second)
	if err != nil {
		t.Skipf("testnet daemon not reachable at %s: %v", testnetP2PAddr, err)
	}
	defer conn.Close()

	lc := levin.NewConnection(conn)

	// Generate a random peer ID.
	var peerIDBuf [8]byte
	rand.Read(peerIDBuf[:])
	peerID := binary.LittleEndian.Uint64(peerIDBuf[:])

	// Build handshake request.
	req := HandshakeRequest{
		NodeData: NodeData{
			NetworkID: config.NetworkIDTestnet,
			PeerID:    peerID,
			LocalTime: time.Now().Unix(),
			MyPort:    0, // We're not listening
		},
		PayloadData: CoreSyncData{
			CurrentHeight:  1,
			ClientVersion:  config.ClientVersion,
			NonPruningMode: true,
		},
	}
	payload, err := EncodeHandshakeRequest(&req)
	if err != nil {
		t.Fatalf("encode handshake: %v", err)
	}

	// Send handshake request.
	if err := lc.WritePacket(CommandHandshake, payload, true); err != nil {
		t.Fatalf("write handshake: %v", err)
	}

	// Read handshake response.
	hdr, data, err := lc.ReadPacket()
	if err != nil {
		t.Fatalf("read handshake response: %v", err)
	}
	if hdr.Command != CommandHandshake {
		t.Fatalf("response command: got %d, want %d", hdr.Command, CommandHandshake)
	}
	// The CryptoNote/Zano daemon handler returns 1 (not 0) on success.
	if hdr.ReturnCode < 0 {
		t.Fatalf("return code: got %d (negative = error)", hdr.ReturnCode)
	}

	// Parse response.
	var resp HandshakeResponse
	if err := resp.Decode(data); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify network ID matches testnet.
	if resp.NodeData.NetworkID != config.NetworkIDTestnet {
		t.Errorf("network_id: got %x, want %x", resp.NodeData.NetworkID, config.NetworkIDTestnet)
	}

	// Verify we got a chain height > 0.
	if resp.PayloadData.CurrentHeight == 0 {
		t.Error("current_height is 0 — daemon may not be synced")
	}
	t.Logf("testnet height: %d", resp.PayloadData.CurrentHeight)
	t.Logf("testnet top_id: %x", resp.PayloadData.TopID)
	t.Logf("testnet version: %s", resp.PayloadData.ClientVersion)
	t.Logf("peerlist: %d bytes (%d entries)", len(resp.PeerlistBlob), len(resp.PeerlistBlob)/PeerlistEntrySize)

	// --- Ping test ---
	pingPayload, _ := EncodePingRequest()
	if err := lc.WritePacket(CommandPing, pingPayload, true); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	hdr, data, err = lc.ReadPacket()
	if err != nil {
		t.Fatalf("read ping response: %v", err)
	}
	status, remotePeerID, err := DecodePingResponse(data)
	if err != nil {
		t.Fatalf("decode ping: %v", err)
	}
	if status != "OK" {
		t.Errorf("ping status: got %q, want %q", status, "OK")
	}
	t.Logf("ping OK, remote peer_id: %x", remotePeerID)
}

// TestIntegration_RequestChainAndGetObjects performs a full chain sync
// sequence: handshake, REQUEST_CHAIN with the genesis hash, then
// REQUEST_GET_OBJECTS with the first block hash from the chain response.
func TestIntegration_RequestChainAndGetObjects_Good(t *testing.T) {
	conn, err := net.DialTimeout("tcp", testnetP2PAddr, 10*time.Second)
	if err != nil {
		t.Skipf("testnet daemon not reachable: %v", err)
	}
	defer conn.Close()

	lc := levin.NewConnection(conn)

	// --- Handshake first ---
	var peerIDBuf [8]byte
	rand.Read(peerIDBuf[:])
	peerID := binary.LittleEndian.Uint64(peerIDBuf[:])

	req := HandshakeRequest{
		NodeData: NodeData{
			NetworkID: config.NetworkIDTestnet,
			PeerID:    peerID,
			LocalTime: time.Now().Unix(),
			MyPort:    0,
		},
		PayloadData: CoreSyncData{
			CurrentHeight:  1,
			ClientVersion:  config.ClientVersion,
			NonPruningMode: true,
		},
	}
	payload, err := EncodeHandshakeRequest(&req)
	require.NoError(t, err)
	require.NoError(t, lc.WritePacket(CommandHandshake, payload, true))

	hdr, _, err := lc.ReadPacket()
	require.NoError(t, err)
	require.Equal(t, uint32(CommandHandshake), hdr.Command)

	// --- Request chain ---
	genesisHash, _ := hex.DecodeString("cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963")
	chainReq := RequestChain{
		BlockIDs: [][]byte{genesisHash},
	}
	chainPayload, err := chainReq.Encode()
	require.NoError(t, err)
	require.NoError(t, lc.WritePacket(CommandRequestChain, chainPayload, false))

	// Read until we get RESPONSE_CHAIN_ENTRY. The daemon may send
	// timed_sync or other messages between our request and the response.
	var chainData []byte
	for {
		hdr, data, err := lc.ReadPacket()
		require.NoError(t, err)
		if hdr.Command == CommandResponseChain {
			chainData = data
			break
		}
		t.Logf("skipping command %d", hdr.Command)
	}

	var chainResp ResponseChainEntry
	require.NoError(t, chainResp.Decode(chainData))
	t.Logf("chain response: start=%d, total=%d, block_ids=%d",
		chainResp.StartHeight, chainResp.TotalHeight, len(chainResp.BlockIDs))
	require.Greater(t, len(chainResp.BlockIDs), 0)

	// --- Request first block ---
	firstHash := chainResp.BlockIDs[0]
	if len(firstHash) < 32 {
		t.Fatalf("block hash too short: %d bytes", len(firstHash))
	}

	getReq := RequestGetObjects{
		Blocks: [][]byte{firstHash[:32]},
	}
	getPayload, err := getReq.Encode()
	require.NoError(t, err)
	require.NoError(t, lc.WritePacket(CommandRequestObjects, getPayload, false))

	// Read until RESPONSE_GET_OBJECTS.
	for {
		hdr, data, err := lc.ReadPacket()
		require.NoError(t, err)
		if hdr.Command == CommandResponseObjects {
			var getResp ResponseGetObjects
			require.NoError(t, getResp.Decode(data))
			t.Logf("get_objects response: %d blocks, %d missed, height=%d",
				len(getResp.Blocks), len(getResp.MissedIDs), getResp.CurrentHeight)
			require.Len(t, getResp.Blocks, 1)
			require.Greater(t, len(getResp.Blocks[0].Block), 0)
			t.Logf("block blob: %d bytes", len(getResp.Blocks[0].Block))
			break
		}
		t.Logf("skipping command %d", hdr.Command)
	}
}
