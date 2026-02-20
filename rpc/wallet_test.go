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

func TestGetRandomOutputs_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getrandom_outs1" {
			t.Errorf("path: got %s, want /getrandom_outs1", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"outs": [
				{"global_index": 10, "public_key": "aa00000000000000000000000000000000000000000000000000000000000000"},
				{"global_index": 20, "public_key": "bb00000000000000000000000000000000000000000000000000000000000000"}
			],
			"status": "OK"
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	outs, err := c.GetRandomOutputs(1000, 2)
	if err != nil {
		t.Fatalf("GetRandomOutputs: %v", err)
	}
	if len(outs) != 2 {
		t.Fatalf("outs: got %d, want 2", len(outs))
	}
	if outs[0].GlobalIndex != 10 {
		t.Errorf("outs[0].GlobalIndex: got %d, want 10", outs[0].GlobalIndex)
	}
	if outs[0].PublicKey != "aa00000000000000000000000000000000000000000000000000000000000000" {
		t.Errorf("outs[0].PublicKey: got %q", outs[0].PublicKey)
	}
	if outs[1].GlobalIndex != 20 {
		t.Errorf("outs[1].GlobalIndex: got %d, want 20", outs[1].GlobalIndex)
	}
}

func TestGetRandomOutputs_Bad_Status(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct{ Status string }{Status: "BUSY"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.GetRandomOutputs(1000, 2)
	if err == nil {
		t.Fatal("expected error for non-OK status")
	}
}

func TestSendRawTransaction_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sendrawtransaction" {
			t.Errorf("path: got %s, want /sendrawtransaction", r.URL.Path)
		}

		var req struct {
			TxAsHex string `json:"tx_as_hex"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.TxAsHex != "0102" {
			t.Errorf("tx_as_hex: got %q, want %q", req.TxAsHex, "0102")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"OK"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	err := c.SendRawTransaction([]byte{0x01, 0x02})
	if err != nil {
		t.Fatalf("SendRawTransaction: %v", err)
	}
}

func TestSendRawTransaction_Bad_Rejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct{ Status string }{Status: "Failed"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	err := c.SendRawTransaction([]byte{0x01})
	if err == nil {
		t.Fatal("expected error for rejected transaction")
	}
}
