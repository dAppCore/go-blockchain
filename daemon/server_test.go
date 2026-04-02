// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dappco.re/go/core/blockchain/chain"
	"dappco.re/go/core/blockchain/config"
	store "dappco.re/go/core/store"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	s, err := store.New(dir + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	c := chain.New(s)
	cfg := config.Testnet
	return NewServer(c, &cfg)
}

func rpcCall(t *testing.T, srv *Server, method string) map[string]interface{} {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":"1","method":"` + method + `","params":{}}`
	req := httptest.NewRequest("POST", "/json_rpc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var resp struct {
		Result map[string]interface{} `json:"result"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.Result
}

func TestServer_GetInfo_Good(t *testing.T) {
	srv := setupTestServer(t)
	result := rpcCall(t, srv, "getinfo")

	if result["status"] != "OK" {
		t.Errorf("status: got %v, want OK", result["status"])
	}
	if _, ok := result["height"]; !ok {
		t.Error("missing height field")
	}
	if _, ok := result["node_type"]; !ok {
		t.Error("missing node_type (Go-exclusive field)")
	}
}

func TestServer_GetHeight_Good(t *testing.T) {
	srv := setupTestServer(t)
	result := rpcCall(t, srv, "getheight")

	if result["status"] != "OK" {
		t.Errorf("status: got %v, want OK", result["status"])
	}
}

func TestServer_GetAssetInfo_Good(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"jsonrpc":"2.0","id":"1","method":"get_asset_info","params":{"asset_id":"LTHN"}}`
	req := httptest.NewRequest("POST", "/json_rpc", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var resp struct {
		Result map[string]interface{} `json:"result"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	desc := resp.Result["asset_descriptor"].(map[string]interface{})
	if desc["ticker"] != "LTHN" {
		t.Errorf("ticker: got %v, want LTHN", desc["ticker"])
	}
}


func TestServer_UnknownMethod_Bad(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"jsonrpc":"2.0","id":"1","method":"nonexistent"}`
	req := httptest.NewRequest("POST", "/json_rpc", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var resp struct {
		Error *struct{ Message string } `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Error == nil {
		t.Error("expected error for unknown method")
	}
}

func TestServer_Health_Good(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code: got %d, want 200", w.Code)
	}

	var health map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &health)
	if _, ok := health["height"]; !ok {
		t.Error("missing height in health response")
	}
}

func TestServer_OpenAPI_Good(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/openapi", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var doc map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &doc)

	if doc["node"] == nil {
		t.Error("missing node field in openapi")
	}
	methods := doc["rpc_methods"].([]interface{})
	if len(methods) < 30 {
		t.Errorf("expected 30+ methods, got %d", len(methods))
	}
}

func TestServer_Metrics_Good(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "lethean_chain_height") {
		t.Error("missing lethean_chain_height metric")
	}
	if !strings.Contains(body, "lethean_difficulty") {
		t.Error("missing lethean_difficulty metric")
	}
}

func TestServer_RESTInfo_Good(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/info", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var info map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &info)

	if info["node"] != "CoreChain/Go" {
		t.Errorf("node: got %v, want CoreChain/Go", info["node"])
	}
}
