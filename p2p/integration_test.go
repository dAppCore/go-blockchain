//go:build integration

// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package p2p

import (
	"crypto/rand"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-p2p/node/levin"
)

const testnetP2PAddr = "localhost:46942"

// TestIntegration_Handshake connects to the C++ testnet daemon,
// performs a full handshake, and verifies the response.
func TestIntegration_Handshake(t *testing.T) {
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
	if hdr.ReturnCode != levin.ReturnOK {
		t.Fatalf("return code: got %d, want %d", hdr.ReturnCode, levin.ReturnOK)
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
