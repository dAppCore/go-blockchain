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

var testBlockHeaderJSON = `{
	"major_version": 1,
	"minor_version": 0,
	"timestamp": 1770897600,
	"prev_hash": "0000000000000000000000000000000000000000000000000000000000000000",
	"nonce": 101011010221,
	"orphan_status": false,
	"height": 0,
	"depth": 6300,
	"hash": "cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963",
	"difficulty": "1",
	"reward": 1000000000000
}`

func blockHeaderResponse() jsonRPCResponse {
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"0"`),
		Result:  json.RawMessage(`{"block_header":` + testBlockHeaderJSON + `,"status":"OK"}`),
	}
}

func TestGetLastBlockHeader_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(blockHeaderResponse())
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	hdr, err := c.GetLastBlockHeader()
	if err != nil {
		t.Fatalf("GetLastBlockHeader: %v", err)
	}
	if hdr.Height != 0 {
		t.Errorf("height: got %d, want 0", hdr.Height)
	}
	if hdr.Hash != "cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963" {
		t.Errorf("hash: got %q", hdr.Hash)
	}
	if hdr.MajorVersion != 1 {
		t.Errorf("major_version: got %d, want 1", hdr.MajorVersion)
	}
}

func TestGetBlockHeaderByHeight_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(blockHeaderResponse())
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	hdr, err := c.GetBlockHeaderByHeight(0)
	if err != nil {
		t.Fatalf("GetBlockHeaderByHeight: %v", err)
	}
	if hdr.Height != 0 {
		t.Errorf("height: got %d, want 0", hdr.Height)
	}
}

func TestGetBlockHeaderByHash_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(blockHeaderResponse())
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	hdr, err := c.GetBlockHeaderByHash("cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963")
	if err != nil {
		t.Fatalf("GetBlockHeaderByHash: %v", err)
	}
	if hdr.Reward != 1000000000000 {
		t.Errorf("reward: got %d, want 1000000000000", hdr.Reward)
	}
}

func TestGetBlocksDetails_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Result: json.RawMessage(`{
				"blocks": [{
					"height": 0,
					"timestamp": 1770897600,
					"base_reward": 1000000000000,
					"id": "cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963",
					"type": 1,
					"is_orphan": false,
					"transactions_details": []
				}],
				"status": "OK"
			}`),
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	blocks, err := c.GetBlocksDetails(0, 1)
	if err != nil {
		t.Fatalf("GetBlocksDetails: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("blocks: got %d, want 1", len(blocks))
	}
	if blocks[0].Height != 0 {
		t.Errorf("height: got %d, want 0", blocks[0].Height)
	}
	if blocks[0].ID != "cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963" {
		t.Errorf("id: got %q", blocks[0].ID)
	}
}

func TestGetBlockHeaderByHeight_Bad_TooBig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Error:   &jsonRPCError{Code: -2, Message: "TOO_BIG_HEIGHT"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.GetBlockHeaderByHeight(999999999)
	if err == nil {
		t.Fatal("expected error")
	}
	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("expected *RPCError, got %T", err)
	}
	if rpcErr.Code != -2 {
		t.Errorf("code: got %d, want -2", rpcErr.Code)
	}
}
