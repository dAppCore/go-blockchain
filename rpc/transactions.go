// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import (
	"fmt"

	coreerr "forge.lthn.ai/core/go-log"
)

// GetTxDetails returns detailed information about a transaction.
func (c *Client) GetTxDetails(txHash string) (*TxInfo, error) {
	params := struct {
		TxHash string `json:"tx_hash"`
	}{TxHash: txHash}
	var resp struct {
		TxInfo TxInfo `json:"tx_info"`
		Status string `json:"status"`
	}
	if err := c.call("get_tx_details", params, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "OK" {
		return nil, coreerr.E("Client.GetTxDetails", fmt.Sprintf("get_tx_details: status %q", resp.Status), nil)
	}
	return &resp.TxInfo, nil
}

// GetTransactions fetches transactions by hash.
// Uses the legacy /gettransactions endpoint (not available via /json_rpc).
// Returns hex-encoded transaction blobs and any missed (not found) hashes.
func (c *Client) GetTransactions(hashes []string) (txsHex []string, missed []string, err error) {
	params := struct {
		TxsHashes []string `json:"txs_hashes"`
	}{TxsHashes: hashes}
	var resp struct {
		TxsAsHex []string `json:"txs_as_hex"`
		MissedTx []string `json:"missed_tx"`
		Status   string   `json:"status"`
	}
	if err := c.legacyCall("/gettransactions", params, &resp); err != nil {
		return nil, nil, err
	}
	if resp.Status != "OK" {
		return nil, nil, coreerr.E("Client.GetTransactions", fmt.Sprintf("gettransactions: status %q", resp.Status), nil)
	}
	return resp.TxsAsHex, resp.MissedTx, nil
}
