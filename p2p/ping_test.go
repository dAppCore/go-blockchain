// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package p2p

import (
	"testing"

	"dappco.re/go/core/p2p/node/levin"
)

func TestPing_EncodePingRequest_EmptySection_Good(t *testing.T) {
	data, err := EncodePingRequest()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	s, err := levin.DecodeStorage(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(s) != 0 {
		t.Errorf("ping request should be empty, got %d fields", len(s))
	}
}

func TestPing_DecodePingResponse_Good(t *testing.T) {
	s := levin.Section{
		"status":  levin.StringVal([]byte("OK")),
		"peer_id": levin.Uint64Val(12345),
	}
	data, _ := levin.EncodeStorage(s)
	status, peerID, err := DecodePingResponse(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status != "OK" {
		t.Errorf("status: got %q, want %q", status, "OK")
	}
	if peerID != 12345 {
		t.Errorf("peer_id: got %d, want 12345", peerID)
	}
}
