// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import (
	"fmt"

	coreerr "forge.lthn.ai/core/go-log"
)

// GetInfo returns the daemon status.
// Uses flags=0 for the cheapest query (no expensive calculations).
func (c *Client) GetInfo() (*DaemonInfo, error) {
	params := struct {
		Flags uint64 `json:"flags"`
	}{Flags: 0}
	var resp struct {
		DaemonInfo
		Status string `json:"status"`
	}
	if err := c.call("getinfo", params, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "OK" {
		return nil, coreerr.E("Client.GetInfo", fmt.Sprintf("getinfo: status %q", resp.Status), nil)
	}
	return &resp.DaemonInfo, nil
}

// GetHeight returns the current blockchain height.
// Uses the legacy /getheight endpoint (not available via /json_rpc).
func (c *Client) GetHeight() (uint64, error) {
	var resp struct {
		Height uint64 `json:"height"`
		Status string `json:"status"`
	}
	if err := c.legacyCall("/getheight", struct{}{}, &resp); err != nil {
		return 0, err
	}
	if resp.Status != "OK" {
		return 0, coreerr.E("Client.GetHeight", fmt.Sprintf("getheight: status %q", resp.Status), nil)
	}
	return resp.Height, nil
}

// GetBlockCount returns the total number of blocks (height of top block + 1).
func (c *Client) GetBlockCount() (uint64, error) {
	var resp struct {
		Count  uint64 `json:"count"`
		Status string `json:"status"`
	}
	if err := c.call("getblockcount", struct{}{}, &resp); err != nil {
		return 0, err
	}
	if resp.Status != "OK" {
		return 0, coreerr.E("Client.GetBlockCount", fmt.Sprintf("getblockcount: status %q", resp.Status), nil)
	}
	return resp.Count, nil
}
