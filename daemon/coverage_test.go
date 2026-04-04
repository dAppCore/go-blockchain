// Copyright (c) 2017-2026 Lethean (https://lt.hn)
// SPDX-License-Identifier: EUPL-1.2

package daemon

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestServer_AllMethods_Good tests every registered RPC method returns
// a valid JSON-RPC response (not nil result, not method-not-found error).
func TestServer_AllMethods_Good(t *testing.T) {
	srv := setupTestServer(t)

	methods := []string{
		"getinfo", "getheight", "getblockcount",
		"getblockheaderbyheight", "getlastblockheader",
		"get_all_alias_details", "get_alias_details",
		"get_alias_by_address", "get_asset_info",
		"get_blocks_details", "get_pool_info",
		"getblockchaininfo", "get_version",
		"get_hardfork_status", "get_chain_stats",
		"get_node_info", "get_total_coins",
		"get_coin_supply", "get_network_hashrate",
		"get_est_height_from_date",
		"get_current_core_tx_expiration_median",
		"get_all_pool_tx_list", "get_pool_txs_details",
		"get_pool_txs_brief_details",
		"get_multisig_info", "get_alt_blocks_details",
		"get_votes", "get_peer_list", "get_connections",
		"get_aliases_by_type", "get_gateways",
		"get_service_endpoints", "get_network_topology",
		"get_gateway_endpoints", "get_vpn_gateways",
		"get_dns_gateways", "get_difficulty_history",
		"get_recent_blocks", "get_forge_info",
		"search", "get_alias_reward",
		"getrandom_outs", "getrandom_outs3",
		"get_block_hash_by_height",
		"get_address_type",
		"generate_keys", "fast_hash", "check_key",
	}

	broken := 0
	for _, method := range methods {
		body := `{"jsonrpc":"2.0","id":"1","method":"` + method + `","params":{}}`
		req := httptest.NewRequest("POST", "/json_rpc", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		var resp struct {
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Errorf("%s: invalid JSON response: %v", method, err)
			broken++
			continue
		}

		if resp.Error != nil && resp.Error.Code == -32601 {
			t.Errorf("%s: method not found (not routed)", method)
			broken++
			continue
		}

		if resp.Result == nil && resp.Error == nil {
			t.Errorf("%s: nil result AND nil error (empty handler)", method)
			broken++
			continue
		}
	}

	t.Logf("%d/%d methods working, %d broken", len(methods)-broken, len(methods), broken)
}
