// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package p2p

import (
	"testing"

	"dappco.re/go/core/blockchain/types"
	"dappco.re/go/core/p2p/node/levin"
)

func TestCoreSyncData_Good_Roundtrip(t *testing.T) {
	var topID types.Hash
	topID[0] = 0xCB
	topID[31] = 0x63

	original := CoreSyncData{
		CurrentHeight:        6300,
		TopID:                topID,
		LastCheckpointHeight: 0,
		CoreTime:             1708444800,
		ClientVersion:        "Lethean/go-blockchain 0.1.0",
		NonPruningMode:       true,
	}
	section := original.MarshalSection()
	data, err := levin.EncodeStorage(section)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := levin.DecodeStorage(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	var got CoreSyncData
	if err := got.UnmarshalSection(decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.CurrentHeight != original.CurrentHeight {
		t.Errorf("height: got %d, want %d", got.CurrentHeight, original.CurrentHeight)
	}
	if got.TopID != original.TopID {
		t.Errorf("top_id: got %x, want %x", got.TopID, original.TopID)
	}
	if got.ClientVersion != original.ClientVersion {
		t.Errorf("version: got %q, want %q", got.ClientVersion, original.ClientVersion)
	}
	if got.NonPruningMode != original.NonPruningMode {
		t.Errorf("pruning: got %v, want %v", got.NonPruningMode, original.NonPruningMode)
	}
	if got.LastCheckpointHeight != original.LastCheckpointHeight {
		t.Errorf("checkpoint: got %d, want %d", got.LastCheckpointHeight, original.LastCheckpointHeight)
	}
	if got.CoreTime != original.CoreTime {
		t.Errorf("core_time: got %d, want %d", got.CoreTime, original.CoreTime)
	}
}
