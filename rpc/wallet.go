// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import (
	"encoding/hex"
	"fmt"

	coreerr "dappco.re/go/core/log"
)

// RandomOutputEntry is a decoy output returned by getrandom_outs.
type RandomOutputEntry struct {
	GlobalIndex uint64 `json:"global_index"`
	PublicKey   string `json:"public_key"`
}

// GetRandomOutputs fetches random decoy outputs for ring construction.
// Uses the legacy /getrandom_outs1 endpoint (not available via /json_rpc).
func (c *Client) GetRandomOutputs(amount uint64, count int) ([]RandomOutputEntry, error) {
	params := struct {
		Amount uint64 `json:"amount"`
		Count  int    `json:"outs_count"`
	}{Amount: amount, Count: count}

	var resp struct {
		Outs   []RandomOutputEntry `json:"outs"`
		Status string              `json:"status"`
	}

	if err := c.legacyCall("/getrandom_outs1", params, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "OK" {
		return nil, coreerr.E("Client.GetRandomOutputs", fmt.Sprintf("getrandom_outs: status %q", resp.Status), nil)
	}
	return resp.Outs, nil
}

// SendRawTransaction submits a serialised transaction for relay.
// Uses the legacy /sendrawtransaction endpoint (not available via /json_rpc).
func (c *Client) SendRawTransaction(txBlob []byte) error {
	params := struct {
		TxAsHex string `json:"tx_as_hex"`
	}{TxAsHex: hex.EncodeToString(txBlob)}

	var resp struct {
		Status string `json:"status"`
	}

	if err := c.legacyCall("/sendrawtransaction", params, &resp); err != nil {
		return err
	}
	if resp.Status != "OK" {
		return coreerr.E("Client.SendRawTransaction", fmt.Sprintf("sendrawtransaction: status %q", resp.Status), nil)
	}
	return nil
}
