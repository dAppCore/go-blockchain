// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTransactions_GetTxDetails_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      rawJSON(`"0"`),
			Result: rawJSON(`{
				"status": "OK",
				"tx_info": {
					"id": "a6e8da986858e6825fce7a192097e6afae4e889cabe853a9c29b964985b23da8",
					"blob_size": 6794,
					"fee": 1000000000,
					"amount": 18999000000000,
					"timestamp": 1557345925,
					"keeper_block": 51,
					"blob": "ARMB..."
				}
			}`),
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	tx, err := c.GetTxDetails("a6e8da986858e6825fce7a192097e6afae4e889cabe853a9c29b964985b23da8")
	if err != nil {
		t.Fatalf("GetTxDetails: %v", err)
	}
	if tx.ID != "a6e8da986858e6825fce7a192097e6afae4e889cabe853a9c29b964985b23da8" {
		t.Errorf("id: got %q", tx.ID)
	}
	if tx.Fee != 1000000000 {
		t.Errorf("fee: got %d, want 1000000000", tx.Fee)
	}
	if tx.KeeperBlock != 51 {
		t.Errorf("keeper_block: got %d, want 51", tx.KeeperBlock)
	}
}

func TestTransactions_GetTxDetails_NotFound_Bad(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      rawJSON(`"0"`),
			Error:   &jsonRPCError{Code: -14, Message: "NOT_FOUND"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.GetTxDetails("0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTransactions_GetTransactions_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gettransactions" {
			t.Errorf("path: got %s, want /gettransactions", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"txs_as_hex": ["01020304"],
			"missed_tx": ["abcd1234"],
			"status": "OK"
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	found, missed, err := c.GetTransactions([]string{"deadbeef", "abcd1234"})
	if err != nil {
		t.Fatalf("GetTransactions: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("found: got %d, want 1", len(found))
	}
	if found[0] != "01020304" {
		t.Errorf("found[0]: got %q, want %q", found[0], "01020304")
	}
	if len(missed) != 1 {
		t.Fatalf("missed: got %d, want 1", len(missed))
	}
	if missed[0] != "abcd1234" {
		t.Errorf("missed[0]: got %q, want %q", missed[0], "abcd1234")
	}
}

func TestTransactions_GetTransactions_AllFound_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"txs_as_hex":["aa","bb"],"missed_tx":[],"status":"OK"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	found, missed, err := c.GetTransactions([]string{"hash1", "hash2"})
	if err != nil {
		t.Fatalf("GetTransactions: %v", err)
	}
	if len(found) != 2 {
		t.Errorf("found: got %d, want 2", len(found))
	}
	if len(missed) != 0 {
		t.Errorf("missed: got %d, want 0", len(missed))
	}
}
