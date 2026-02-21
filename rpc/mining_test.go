// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSubmitBlock_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req jsonRPCRequest
		json.Unmarshal(body, &req)
		if req.Method != "submitblock" {
			t.Errorf("method: got %q, want %q", req.Method, "submitblock")
		}
		// Verify params is an array.
		raw, _ := json.Marshal(req.Params)
		if raw[0] != '[' {
			t.Errorf("params should be array, got: %s", raw)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Result:  json.RawMessage(`{"status":"OK"}`),
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	err := c.SubmitBlock("0300000001020304")
	if err != nil {
		t.Fatalf("SubmitBlock: %v", err)
	}
}

func TestSubmitBlock_Bad_Rejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Error:   &jsonRPCError{Code: -7, Message: "BLOCK_NOT_ACCEPTED"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	err := c.SubmitBlock("invalid")
	if err == nil {
		t.Fatal("expected error")
	}
	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("expected *RPCError, got %T", err)
	}
	if rpcErr.Code != -7 {
		t.Errorf("code: got %d, want -7", rpcErr.Code)
	}
}

func TestGetBlockTemplate_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req jsonRPCRequest
		json.Unmarshal(body, &req)
		if req.Method != "getblocktemplate" {
			t.Errorf("method: got %q, want %q", req.Method, "getblocktemplate")
		}
		// Verify wallet_address is in params.
		raw, _ := json.Marshal(req.Params)
		if !bytes.Contains(raw, []byte("iTHN")) {
			t.Errorf("params should contain wallet address, got: %s", raw)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Result: json.RawMessage(`{
				"difficulty": "42",
				"height": 100,
				"blocktemplate_blob": "0100000000000000000000000000",
				"prev_hash": "cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963",
				"block_reward_without_fee": 1000000000000,
				"block_reward": 1000000000000,
				"txs_fee": 0,
				"status": "OK"
			}`),
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	resp, err := c.GetBlockTemplate("iTHNtestaddr")
	if err != nil {
		t.Fatalf("GetBlockTemplate: %v", err)
	}
	if resp.Difficulty != "42" {
		t.Errorf("difficulty: got %q, want %q", resp.Difficulty, "42")
	}
	if resp.Height != 100 {
		t.Errorf("height: got %d, want 100", resp.Height)
	}
	if resp.BlockReward != 1000000000000 {
		t.Errorf("block_reward: got %d, want 1000000000000", resp.BlockReward)
	}
}

func TestGetBlockTemplate_Bad_Status(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Result:  json.RawMessage(`{"status":"BUSY"}`),
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.GetBlockTemplate("iTHNtestaddr")
	if err == nil {
		t.Fatal("expected error for BUSY status")
	}
}
