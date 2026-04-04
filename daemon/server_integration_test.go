//go:build integration

// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package daemon_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const (
	cppDaemon = "http://127.0.0.1:46941"
	goDaemon  = "http://127.0.0.1:47941"
)

func rpcCall(t *testing.T, url, method string) map[string]interface{} {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":"0","method":"` + method + `","params":{}}`
	resp, err := http.Post(url+"/json_rpc", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("RPC call to %s failed: %v", url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var result struct {
		Result map[string]interface{} `json:"result"`
	}
	json.Unmarshal(raw, &result)
	return result.Result
}

func TestIntegration_GetInfo_MatchesCpp_Good(t *testing.T) {
	cpp := rpcCall(t, cppDaemon, "getinfo")
	go_ := rpcCall(t, goDaemon, "getinfo")

	// Alias count must match
	cppAliases := int(cpp["alias_count"].(float64))
	goAliases := int(go_["alias_count"].(float64))
	if cppAliases != goAliases {
		t.Errorf("alias_count: C++=%d Go=%d", cppAliases, goAliases)
	}

	// Heights should be within 10 blocks (sync delay)
	cppHeight := int(cpp["height"].(float64))
	goHeight := int(go_["height"].(float64))
	diff := cppHeight - goHeight
	if diff < 0 { diff = -diff }
	if diff > 10 {
		t.Errorf("height diff too large: C++=%d Go=%d (diff=%d)", cppHeight, goHeight, diff)
	}
}

func TestIntegration_Aliases_MatchCpp_Good(t *testing.T) {
	cppResp := rpcCall(t, cppDaemon, "get_all_alias_details")
	goResp := rpcCall(t, goDaemon, "get_all_alias_details")

	cppAliases := cppResp["aliases"].([]interface{})
	goAliases := goResp["aliases"].([]interface{})

	if len(cppAliases) != len(goAliases) {
		t.Fatalf("alias count: C++=%d Go=%d", len(cppAliases), len(goAliases))
	}

	// Build name sets
	cppNames := make(map[string]bool)
	for _, a := range cppAliases {
		m := a.(map[string]interface{})
		cppNames[m["alias"].(string)] = true
	}

	for _, a := range goAliases {
		m := a.(map[string]interface{})
		name := m["alias"].(string)
		if !cppNames[name] {
			t.Errorf("Go has alias @%s not in C++", name)
		}
	}
}

func TestIntegration_BlockHeader_MatchesCpp_Good(t *testing.T) {
	// Compare block 11000 (HF4 activation)
	body := `{"jsonrpc":"2.0","id":"0","method":"getblockheaderbyheight","params":{"height":11000}}`

	cppResp, _ := http.Post(cppDaemon+"/json_rpc", "application/json", strings.NewReader(body))
	goResp, _ := http.Post(goDaemon+"/json_rpc", "application/json", strings.NewReader(body))

	cppRaw, _ := io.ReadAll(cppResp.Body)
	goRaw, _ := io.ReadAll(goResp.Body)
	cppResp.Body.Close()
	goResp.Body.Close()

	var cppResult, goResult struct {
		Result struct {
			BlockHeader struct {
				Hash string `json:"hash"`
			} `json:"block_header"`
		} `json:"result"`
	}

	json.Unmarshal(cppRaw, &cppResult)
	json.Unmarshal(goRaw, &goResult)

	if cppResult.Result.BlockHeader.Hash != goResult.Result.BlockHeader.Hash {
		t.Errorf("block 11000 hash mismatch:\n  C++: %s\n  Go:  %s",
			cppResult.Result.BlockHeader.Hash, goResult.Result.BlockHeader.Hash)
	}
}

func TestIntegration_HardforkStatus_Good(t *testing.T) {
	go_ := rpcCall(t, goDaemon, "get_hardfork_status")
	if go_ == nil {
		t.Fatal("get_hardfork_status returned nil")
	}

	forks := go_["hardforks"].([]interface{})
	if len(forks) < 5 {
		t.Errorf("expected 5+ hardforks, got %d", len(forks))
	}

	// HF0-HF4 should all be active
	for i := 0; i < 5; i++ {
		f := forks[i].(map[string]interface{})
		if !f["active"].(bool) {
			t.Errorf("HF%d should be active", i)
		}
	}
}

func TestIntegration_ChainStats_Good(t *testing.T) {
	go_ := rpcCall(t, goDaemon, "get_chain_stats")
	if go_ == nil {
		t.Fatal("get_chain_stats returned nil")
	}

	height := int(go_["height"].(float64))
	if height < 11000 {
		t.Errorf("expected height > 11000, got %d", height)
	}

	aliases := int(go_["total_aliases"].(float64))
	if aliases != 14 {
		t.Errorf("expected 14 aliases, got %d", aliases)
	}
}





func TestIntegration_RESTHealth_Good(t *testing.T) {
	resp, err := http.Get(goDaemon + "/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	defer resp.Body.Close()

	var health map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&health)

	if health["status"] != "ok" {
		t.Errorf("health status: %v", health["status"])
	}
}


