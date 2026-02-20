// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package p2p

import (
	"encoding/binary"
	"testing"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-p2p/node/levin"
)

func TestEncodeHandshakeRequest_Good_Roundtrip(t *testing.T) {
	req := HandshakeRequest{
		NodeData: NodeData{
			NetworkID: config.NetworkIDTestnet,
			PeerID:    0xDEADBEEF,
			LocalTime: 1708444800,
			MyPort:    46942,
		},
		PayloadData: CoreSyncData{
			CurrentHeight: 100,
			ClientVersion: "test/0.1",
		},
	}
	data, err := EncodeHandshakeRequest(&req)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	s, err := levin.DecodeStorage(data)
	if err != nil {
		t.Fatalf("decode storage: %v", err)
	}

	var got HandshakeRequest
	if err := got.UnmarshalSection(s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.NodeData.NetworkID != config.NetworkIDTestnet {
		t.Errorf("network_id mismatch")
	}
	if got.NodeData.PeerID != 0xDEADBEEF {
		t.Errorf("peer_id: got %x, want DEADBEEF", got.NodeData.PeerID)
	}
	if got.NodeData.LocalTime != 1708444800 {
		t.Errorf("local_time: got %d, want 1708444800", got.NodeData.LocalTime)
	}
	if got.NodeData.MyPort != 46942 {
		t.Errorf("my_port: got %d, want 46942", got.NodeData.MyPort)
	}
	if got.PayloadData.CurrentHeight != 100 {
		t.Errorf("height: got %d, want 100", got.PayloadData.CurrentHeight)
	}
	if got.PayloadData.ClientVersion != "test/0.1" {
		t.Errorf("client_version: got %q, want %q", got.PayloadData.ClientVersion, "test/0.1")
	}
}

func TestDecodeHandshakeResponse_Good_WithPeerlist(t *testing.T) {
	// Build a response section manually.
	nodeData := levin.Section{
		"network_id": levin.StringVal(config.NetworkIDTestnet[:]),
		"peer_id":    levin.Uint64Val(42),
		"local_time": levin.Int64Val(1708444800),
		"my_port":    levin.Uint32Val(46942),
	}
	syncData := CoreSyncData{
		CurrentHeight: 6300,
		ClientVersion: "Zano/2.0",
	}
	// Pack 2 peerlist entries into a single blob.
	peerBlob := make([]byte, 48) // 2 x 24 bytes
	// Entry 1: ip=10.0.0.1 (0x0100000A LE), port=46942, id=1, last_seen=1000
	binary.LittleEndian.PutUint32(peerBlob[0:4], 0x0100000A) // 10.0.0.1
	binary.LittleEndian.PutUint32(peerBlob[4:8], 46942)
	binary.LittleEndian.PutUint64(peerBlob[8:16], 1)
	binary.LittleEndian.PutUint64(peerBlob[16:24], 1000)
	// Entry 2: ip=192.168.1.1, port=36942, id=2, last_seen=2000
	binary.LittleEndian.PutUint32(peerBlob[24:28], 0x0101A8C0) // 192.168.1.1
	binary.LittleEndian.PutUint32(peerBlob[28:32], 36942)
	binary.LittleEndian.PutUint64(peerBlob[32:40], 2)
	binary.LittleEndian.PutUint64(peerBlob[40:48], 2000)

	s := levin.Section{
		"node_data":      levin.ObjectVal(nodeData),
		"payload_data":   levin.ObjectVal(syncData.MarshalSection()),
		"local_peerlist": levin.StringVal(peerBlob),
	}
	data, err := levin.EncodeStorage(s)
	if err != nil {
		t.Fatalf("encode storage: %v", err)
	}

	var resp HandshakeResponse
	if err := resp.Decode(data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.NodeData.PeerID != 42 {
		t.Errorf("peer_id: got %d, want 42", resp.NodeData.PeerID)
	}
	if resp.PayloadData.CurrentHeight != 6300 {
		t.Errorf("height: got %d, want 6300", resp.PayloadData.CurrentHeight)
	}
	if len(resp.PeerlistBlob) != 48 {
		t.Fatalf("peerlist: got %d bytes, want 48", len(resp.PeerlistBlob))
	}

	// Decode the peerlist
	entries := DecodePeerlist(resp.PeerlistBlob)
	if len(entries) != 2 {
		t.Fatalf("peerlist entries: got %d, want 2", len(entries))
	}
	if entries[0].IP != 0x0100000A {
		t.Errorf("entry[0].ip: got %x, want 0100000A", entries[0].IP)
	}
	if entries[0].Port != 46942 {
		t.Errorf("entry[0].port: got %d, want 46942", entries[0].Port)
	}
	if entries[0].ID != 1 {
		t.Errorf("entry[0].id: got %d, want 1", entries[0].ID)
	}
	if entries[1].LastSeen != 2000 {
		t.Errorf("entry[1].last_seen: got %d, want 2000", entries[1].LastSeen)
	}
}

func TestNodeData_Good_NetworkIDBlob(t *testing.T) {
	nd := NodeData{NetworkID: config.NetworkIDTestnet}
	s := nd.MarshalSection()
	blob, err := s["network_id"].AsString()
	if err != nil {
		t.Fatalf("network_id: %v", err)
	}
	if len(blob) != 16 {
		t.Fatalf("network_id blob: got %d bytes, want 16", len(blob))
	}
	// Byte 10 = testnet flag = 1
	if blob[10] != 0x01 {
		t.Errorf("testnet flag: got %x, want 0x01", blob[10])
	}
	// Byte 15 = version = 0x64 (100)
	if blob[15] != 0x64 {
		t.Errorf("version byte: got %x, want 0x64", blob[15])
	}
}

func TestDecodePeerlist_Good_EmptyBlob(t *testing.T) {
	entries := DecodePeerlist(nil)
	if len(entries) != 0 {
		t.Errorf("empty peerlist: got %d entries, want 0", len(entries))
	}
}
