// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import (
	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
	"encoding/hex"
)

// TransferDestination is an address+amount pair for a wallet transfer.
//
//	destination := rpc.TransferDestination{Address: "iTHN...", Amount: 1000000000000}
type TransferDestination struct {
	Address string `json:"address"`
	Amount  uint64 `json:"amount"`
}

// TransferParams holds the parameters for a wallet transfer RPC call.
//
//	params := rpc.TransferParams{Destinations: destinations, Fee: 10000000000, Mixin: 15}
type TransferParams struct {
	Destinations []TransferDestination `json:"destinations"`
	Fee          uint64                `json:"fee"`
	Mixin        uint64                `json:"mixin"`
	PaymentID    string                `json:"payment_id,omitempty"`
}

// TransferResult holds the response from a wallet transfer RPC call.
//
//	result, err := client.Transfer(params)
//	core.Print(nil, "TX: %s", result.TxHash)
type TransferResult struct {
	TxHash string `json:"tx_hash"`
}

// Transfer sends LTHN to one or more destinations via the wallet RPC.
//
//	result, err := client.Transfer(rpc.TransferParams{
//		Destinations: []rpc.TransferDestination{{Address: "iTHN...", Amount: 1000000000000}},
//		Fee:          10000000000,
//		Mixin:        15,
//	})
func (c *Client) Transfer(params TransferParams) (*TransferResult, error) {
	var result TransferResult
	if err := c.call("transfer", params, &result); err != nil {
		return nil, coreerr.E("Client.Transfer", "wallet transfer RPC failed", err)
	}
	return &result, nil
}

// RandomOutputEntry is a decoy output returned by getrandom_outs.
// Usage: var value rpc.RandomOutputEntry
type RandomOutputEntry struct {
	GlobalIndex uint64 `json:"global_index"`
	PublicKey   string `json:"public_key"`
}

// GetRandomOutputs fetches random decoy outputs for ring construction.
// Uses the legacy /getrandom_outs1 endpoint (not available via /json_rpc).
// Usage: value.GetRandomOutputs(...)
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
		return nil, coreerr.E("Client.GetRandomOutputs", core.Sprintf("getrandom_outs: status %q", resp.Status), nil)
	}
	return resp.Outs, nil
}

// SendRawTransaction submits a serialised transaction for relay.
// Uses the legacy /sendrawtransaction endpoint (not available via /json_rpc).
// Usage: value.SendRawTransaction(...)
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
		return coreerr.E("Client.SendRawTransaction", core.Sprintf("sendrawtransaction: status %q", resp.Status), nil)
	}
	return nil
}
