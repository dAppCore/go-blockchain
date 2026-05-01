// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package daemon

import (
	"encoding/json"
	"io"
	"net/http"

	"dappco.re/go/core"
)

// WalletProxy forwards wallet RPC calls to a C++ wallet-rpc instance.
// This lets the Go daemon serve a unified RPC endpoint for both
// chain queries (native Go) and wallet operations (C++ backend).
//
//	proxy := daemon.NewWalletProxy("http://127.0.0.1:46944")
type WalletProxy struct {
	walletURL string
	client    *http.Client
}

// NewWalletProxy creates a wallet RPC proxy.
//
//	proxy := daemon.NewWalletProxy("http://127.0.0.1:46944")
func NewWalletProxy(walletURL string) *WalletProxy {
	return &WalletProxy{
		walletURL: walletURL,
		client:    &http.Client{},
	}
}

// Forward sends a JSON-RPC request to the C++ wallet and returns the response.
//
//	resp, err := proxy.Forward(method, params)
func (p *WalletProxy) Forward(method string, params json.RawMessage) (json.RawMessage, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "0",
		"method":  method,
	}
	if params != nil {
		var p interface{}
		json.Unmarshal(params, &p)
		reqBody["params"] = p
	}

	data := core.JSONMarshalString(reqBody)
	resp, err := p.client.Post(p.walletURL+"/json_rpc", "application/json", core.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	json.Unmarshal(body, &rpcResp)

	if rpcResp.Error != nil && string(rpcResp.Error) != "null" {
		return rpcResp.Error, nil
	}
	return rpcResp.Result, nil
}

// walletMethods that get proxied to C++ wallet RPC
var walletMethods = map[string]bool{
	"getbalance":             true,
	"getaddress":             true,
	"get_wallet_info":        true,
	"make_integrated_address": true,
	"split_integrated_address": true,
	"transfer":               true,
	"get_bulk_payments":      true,
	"get_recent_txs_and_info": true,
	"store":                  true,
	"get_restore_info":       true,
	"sign_message":           true,
	"deploy_asset":           true,
	"emit_asset":             true,
	"burn_asset":             true,
	"register_alias":         true,
	"update_alias":           true,
}

// IsWalletMethod returns true if the method should be proxied to wallet RPC.
//
//	if daemon.IsWalletMethod("getbalance") { proxy.Forward(...) }
func IsWalletMethod(method string) bool {
	return walletMethods[method]
}
// SetWalletProxy enables wallet RPC forwarding to a C++ wallet.
//
//	server.SetWalletProxy("http://127.0.0.1:46944")
func (s *Server) SetWalletProxy(walletURL string) {
	s.walletProxy = NewWalletProxy(walletURL)
}
