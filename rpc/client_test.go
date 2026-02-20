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

func TestClient_Good_JSONRPCCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request format.
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}
		if r.URL.Path != "/json_rpc" {
			t.Errorf("path: got %s, want /json_rpc", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var req jsonRPCRequest
		json.Unmarshal(body, &req)
		if req.JSONRPC != "2.0" {
			t.Errorf("jsonrpc: got %q, want %q", req.JSONRPC, "2.0")
		}
		if req.Method != "getblockcount" {
			t.Errorf("method: got %q, want %q", req.Method, "getblockcount")
		}

		// Return a valid response.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Result:  json.RawMessage(`{"count":6300,"status":"OK"}`),
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	var result struct {
		Count  uint64 `json:"count"`
		Status string `json:"status"`
	}
	err := c.call("getblockcount", struct{}{}, &result)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if result.Count != 6300 {
		t.Errorf("count: got %d, want 6300", result.Count)
	}
	if result.Status != "OK" {
		t.Errorf("status: got %q, want %q", result.Status, "OK")
	}
}

func TestClient_Good_LegacyCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getheight" {
			t.Errorf("path: got %s, want /getheight", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"height":6300,"status":"OK"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	var result struct {
		Height uint64 `json:"height"`
		Status string `json:"status"`
	}
	err := c.legacyCall("/getheight", struct{}{}, &result)
	if err != nil {
		t.Fatalf("legacyCall: %v", err)
	}
	if result.Height != 6300 {
		t.Errorf("height: got %d, want 6300", result.Height)
	}
}

func TestClient_Bad_RPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Error: &jsonRPCError{
				Code:    -2,
				Message: "TOO_BIG_HEIGHT",
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	var result struct{}
	err := c.call("getblockheaderbyheight", struct{ Height uint64 }{999999999}, &result)
	if err == nil {
		t.Fatal("expected error")
	}
	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("expected *RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != -2 {
		t.Errorf("code: got %d, want -2", rpcErr.Code)
	}
}

func TestClient_Bad_ConnectionRefused(t *testing.T) {
	c := NewClient("http://127.0.0.1:1") // Unlikely to be listening
	var result struct{}
	err := c.call("getblockcount", struct{}{}, &result)
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestClient_Good_URLAppendPath(t *testing.T) {
	// NewClient should append /json_rpc if path is empty.
	c := NewClient("http://localhost:46941")
	if c.url != "http://localhost:46941/json_rpc" {
		t.Errorf("url: got %q, want %q", c.url, "http://localhost:46941/json_rpc")
	}

	// If path already present, leave it alone.
	c2 := NewClient("http://localhost:46941/json_rpc")
	if c2.url != "http://localhost:46941/json_rpc" {
		t.Errorf("url: got %q, want %q", c2.url, "http://localhost:46941/json_rpc")
	}
}

func TestClient_Bad_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	var result struct{}
	err := c.call("getblockcount", struct{}{}, &result)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestClient_Bad_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	var result struct{}
	err := c.call("getblockcount", struct{}{}, &result)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}
