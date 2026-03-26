// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import (
	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
)

// SubmitBlock submits a mined block to the daemon.
// The hexBlob is the hex-encoded serialised block.
// Note: submitblock takes a JSON array as params, not an object.
// Usage: value.SubmitBlock(...)
func (c *Client) SubmitBlock(hexBlob string) error {
	// submitblock expects params as an array: ["hexblob"]
	params := []string{hexBlob}
	var resp struct {
		Status string `json:"status"`
	}
	if err := c.call("submitblock", params, &resp); err != nil {
		return err
	}
	if resp.Status != "OK" {
		return coreerr.E("Client.SubmitBlock", core.Sprintf("submitblock: status %q", resp.Status), nil)
	}
	return nil
}

// BlockTemplateResponse is the daemon's response to getblocktemplate.
// Usage: var value rpc.BlockTemplateResponse
type BlockTemplateResponse struct {
	Difficulty            string `json:"difficulty"`
	Height                uint64 `json:"height"`
	BlockTemplateBlob     string `json:"blocktemplate_blob"`
	PrevHash              string `json:"prev_hash"`
	BlockRewardWithoutFee uint64 `json:"block_reward_without_fee"`
	BlockReward           uint64 `json:"block_reward"`
	TxsFee                uint64 `json:"txs_fee"`
	Status                string `json:"status"`
}

// GetBlockTemplate requests a block template from the daemon for mining.
// Usage: value.GetBlockTemplate(...)
func (c *Client) GetBlockTemplate(walletAddr string) (*BlockTemplateResponse, error) {
	params := struct {
		WalletAddress string `json:"wallet_address"`
	}{WalletAddress: walletAddr}
	var resp BlockTemplateResponse
	if err := c.call("getblocktemplate", params, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "OK" {
		return nil, coreerr.E("Client.GetBlockTemplate", core.Sprintf("getblocktemplate: status %q", resp.Status), nil)
	}
	return &resp, nil
}
