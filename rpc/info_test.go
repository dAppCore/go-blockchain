// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetInfo_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      rawJSON(`"0"`),
			Result: rawJSON(`{
				"height": 6300,
				"tx_count": 12345,
				"tx_pool_size": 3,
				"outgoing_connections_count": 8,
				"incoming_connections_count": 4,
				"synchronized_connections_count": 7,
				"daemon_network_state": 2,
				"pow_difficulty": 1000000,
				"block_reward": 1000000000000,
				"default_fee": 10000000000,
				"minimum_fee": 10000000000,
				"last_block_hash": "abc123",
				"total_coins": "17500000000000000000",
				"pos_allowed": true,
				"status": "OK"
			}`),
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	info, err := c.GetInfo()
	if err != nil {
		t.Fatalf("GetInfo: %v", err)
	}
	if info.Height != 6300 {
		t.Errorf("height: got %d, want 6300", info.Height)
	}
	if info.TxCount != 12345 {
		t.Errorf("tx_count: got %d, want 12345", info.TxCount)
	}
	if info.BlockReward != 1000000000000 {
		t.Errorf("block_reward: got %d, want 1000000000000", info.BlockReward)
	}
	if !info.PosAllowed {
		t.Error("pos_allowed: got false, want true")
	}
}

func TestGetHeight_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getheight" {
			t.Errorf("path: got %s, want /getheight", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"height":6300,"status":"OK"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	height, err := c.GetHeight()
	if err != nil {
		t.Fatalf("GetHeight: %v", err)
	}
	if height != 6300 {
		t.Errorf("height: got %d, want 6300", height)
	}
}

func TestGetBlockCount_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      rawJSON(`"0"`),
			Result:  rawJSON(`{"count":6301,"status":"OK"}`),
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	count, err := c.GetBlockCount()
	if err != nil {
		t.Fatalf("GetBlockCount: %v", err)
	}
	if count != 6301 {
		t.Errorf("count: got %d, want 6301", count)
	}
}
