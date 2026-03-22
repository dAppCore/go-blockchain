// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wallet

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dappco.re/go/core/blockchain/rpc"
	"dappco.re/go/core/blockchain/types"
)

func TestRPCRingSelector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type entry struct {
			GlobalIndex uint64 `json:"global_index"`
			PublicKey    string `json:"public_key"`
		}
		resp := struct {
			Outs   []entry `json:"outs"`
			Status string  `json:"status"`
		}{Status: "OK"}

		for i := 0; i < 15; i++ {
			var key types.PublicKey
			key[0] = byte(i + 1)
			resp.Outs = append(resp.Outs, entry{
				GlobalIndex: uint64((i + 1) * 100),
				PublicKey:    hex.EncodeToString(key[:]),
			})
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := rpc.NewClient(srv.URL)
	selector := NewRPCRingSelector(client)

	members, err := selector.SelectRing(1000, 500, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 10 {
		t.Fatalf("got %d ring members, want 10", len(members))
	}

	seen := make(map[uint64]bool)
	for _, m := range members {
		if seen[m.GlobalIndex] {
			t.Fatalf("duplicate global index %d", m.GlobalIndex)
		}
		seen[m.GlobalIndex] = true
	}

	for _, m := range members {
		if m.GlobalIndex == 500 {
			t.Fatal("real global index should be excluded from decoys")
		}
	}
}

func TestRPCRingSelectorExcludesReal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type entry struct {
			GlobalIndex uint64 `json:"global_index"`
			PublicKey    string `json:"public_key"`
		}
		resp := struct {
			Outs   []entry `json:"outs"`
			Status string  `json:"status"`
		}{Status: "OK"}

		for i := 0; i < 15; i++ {
			var key types.PublicKey
			key[0] = byte(i + 1)
			gidx := uint64((i + 1) * 100)
			if i == 3 {
				gidx = 42 // this is the real output
			}
			resp.Outs = append(resp.Outs, entry{
				GlobalIndex: gidx,
				PublicKey:    hex.EncodeToString(key[:]),
			})
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := rpc.NewClient(srv.URL)
	selector := NewRPCRingSelector(client)

	members, err := selector.SelectRing(1000, 42, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range members {
		if m.GlobalIndex == 42 {
			t.Fatal("real output should be excluded")
		}
	}
}

func TestRPCRingSelectorInsufficientDecoys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			Outs   []struct{} `json:"outs"`
			Status string     `json:"status"`
		}{Status: "OK"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := rpc.NewClient(srv.URL)
	selector := NewRPCRingSelector(client)

	_, err := selector.SelectRing(1000, 0, 10)
	if err == nil {
		t.Fatal("expected insufficient decoys error")
	}
}
