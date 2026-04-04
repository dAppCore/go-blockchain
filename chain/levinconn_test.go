// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"testing"

	"dappco.re/go/core/blockchain/p2p"
	"dappco.re/go/core/blockchain/types"
	levinpkg "dappco.re/go/core/p2p/node/levin"
)

// --- Good (happy path) ---

func TestLevinconn_NewLevinP2PConn_Good(t *testing.T) {
	syncData := p2p.CoreSyncData{
		CurrentHeight: 1000,
		TopID:         types.Hash{0xab, 0xcd},
	}
	conn := NewLevinP2PConn(nil, 5000, syncData)
	if conn == nil {
		t.Fatal("NewLevinP2PConn returned nil")
	}
	if conn.peerHeight != 5000 {
		t.Errorf("peerHeight: got %d, want 5000", conn.peerHeight)
	}
}

func TestLevinconn_PeerHeight_Good(t *testing.T) {
	syncData := p2p.CoreSyncData{CurrentHeight: 42}
	conn := NewLevinP2PConn(nil, 12345, syncData)

	height := conn.PeerHeight()
	if height != 12345 {
		t.Errorf("PeerHeight: got %d, want 12345", height)
	}
}

func TestLevinconn_NewLevinP2PConn_PreservesLocalSync_Good(t *testing.T) {
	syncData := p2p.CoreSyncData{
		CurrentHeight: 999,
		TopID:         types.Hash{0x01, 0x02, 0x03},
		ClientVersion: "test-v1.0",
	}
	conn := NewLevinP2PConn(nil, 500, syncData)
	if conn.localSync.CurrentHeight != 999 {
		t.Errorf("localSync.CurrentHeight: got %d, want 999", conn.localSync.CurrentHeight)
	}
	if conn.localSync.TopID != syncData.TopID {
		t.Error("localSync.TopID mismatch")
	}
	if conn.localSync.ClientVersion != "test-v1.0" {
		t.Errorf("localSync.ClientVersion: got %q, want %q", conn.localSync.ClientVersion, "test-v1.0")
	}
}

func TestLevinconn_handleMessage_SkipsUnknownCommand_Good(t *testing.T) {
	syncData := p2p.CoreSyncData{}
	conn := NewLevinP2PConn(nil, 100, syncData)

	// handleMessage with a non-timed-sync command should be silently ignored.
	header := levinpkg.Header{
		Command:        p2p.CommandNewBlock,
		ExpectResponse: false,
	}
	err := conn.handleMessage(header, []byte{0x01, 0x02})
	if err != nil {
		t.Fatalf("handleMessage should silently skip unknown commands, got: %v", err)
	}
}

// --- Bad (expected errors / wrong input) ---

func TestLevinconn_PeerHeight_ZeroHeight_Bad(t *testing.T) {
	// A peer that reports height 0 is valid but means no blocks synced.
	conn := NewLevinP2PConn(nil, 0, p2p.CoreSyncData{})
	if conn.PeerHeight() != 0 {
		t.Errorf("PeerHeight: got %d, want 0", conn.PeerHeight())
	}
}

func TestLevinconn_handleMessage_SkipsPingCommand_Bad(t *testing.T) {
	conn := NewLevinP2PConn(nil, 100, p2p.CoreSyncData{})

	// Ping commands that are not timed_sync should be silently skipped.
	header := levinpkg.Header{
		Command:        p2p.CommandPing,
		ExpectResponse: false,
	}
	err := conn.handleMessage(header, nil)
	if err != nil {
		t.Fatalf("handleMessage should skip ping, got: %v", err)
	}
}

func TestLevinconn_handleMessage_SkipsNewTransactions_Bad(t *testing.T) {
	conn := NewLevinP2PConn(nil, 100, p2p.CoreSyncData{})

	header := levinpkg.Header{
		Command:        p2p.CommandNewTransactions,
		ExpectResponse: false,
	}
	err := conn.handleMessage(header, []byte{0xde, 0xad})
	if err != nil {
		t.Fatalf("handleMessage should skip new_transactions, got: %v", err)
	}
}

// --- Ugly (edge cases) ---

func TestLevinconn_NewLevinP2PConn_NilConn_Ugly(t *testing.T) {
	// A nil levin connection should not panic during construction.
	// It will fail at read/write time, but construction is safe.
	conn := NewLevinP2PConn(nil, 0, p2p.CoreSyncData{})
	if conn == nil {
		t.Fatal("NewLevinP2PConn returned nil with nil connection")
	}
	if conn.conn != nil {
		t.Error("expected conn field to be nil")
	}
}

func TestLevinconn_PeerHeight_MaxUint64_Ugly(t *testing.T) {
	maxHeight := ^uint64(0)
	conn := NewLevinP2PConn(nil, maxHeight, p2p.CoreSyncData{})
	if conn.PeerHeight() != maxHeight {
		t.Errorf("PeerHeight: got %d, want max uint64 %d", conn.PeerHeight(), maxHeight)
	}
}

func TestLevinconn_NewLevinP2PConn_EmptySyncData_Ugly(t *testing.T) {
	conn := NewLevinP2PConn(nil, 42, p2p.CoreSyncData{})
	if conn.localSync.CurrentHeight != 0 {
		t.Errorf("empty sync CurrentHeight: got %d, want 0", conn.localSync.CurrentHeight)
	}
	if !conn.localSync.TopID.IsZero() {
		t.Error("empty sync TopID should be zero")
	}
}

func TestLevinconn_handleMessage_EmptyData_Ugly(t *testing.T) {
	conn := NewLevinP2PConn(nil, 100, p2p.CoreSyncData{})

	// A non-timed-sync command with empty data should be skipped without error.
	header := levinpkg.Header{
		Command:        p2p.CommandNewBlock,
		ExpectResponse: false,
	}
	err := conn.handleMessage(header, nil)
	if err != nil {
		t.Fatalf("handleMessage with empty data: %v", err)
	}
}

func TestLevinconn_handleMessage_TimedSyncNoExpectResponse_Ugly(t *testing.T) {
	conn := NewLevinP2PConn(nil, 100, p2p.CoreSyncData{})

	// timed_sync command without ExpectResponse flag should be silently skipped
	// (the conditional checks both Command == CommandTimedSync AND ExpectResponse).
	header := levinpkg.Header{
		Command:        p2p.CommandTimedSync,
		ExpectResponse: false,
	}
	err := conn.handleMessage(header, []byte{0x01})
	if err != nil {
		t.Fatalf("handleMessage timed_sync without ExpectResponse should be skipped, got: %v", err)
	}
}
