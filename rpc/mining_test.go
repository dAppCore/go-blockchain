// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import (
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
