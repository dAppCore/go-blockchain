// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import "fmt"

// GetLastBlockHeader returns the header of the most recent block.
func (c *Client) GetLastBlockHeader() (*BlockHeader, error) {
	var resp struct {
		BlockHeader BlockHeader `json:"block_header"`
		Status      string     `json:"status"`
	}
	if err := c.call("getlastblockheader", struct{}{}, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "OK" {
		return nil, fmt.Errorf("getlastblockheader: status %q", resp.Status)
	}
	return &resp.BlockHeader, nil
}

// GetBlockHeaderByHeight returns the block header at the given height.
func (c *Client) GetBlockHeaderByHeight(height uint64) (*BlockHeader, error) {
	params := struct {
		Height uint64 `json:"height"`
	}{Height: height}
	var resp struct {
		BlockHeader BlockHeader `json:"block_header"`
		Status      string     `json:"status"`
	}
	if err := c.call("getblockheaderbyheight", params, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "OK" {
		return nil, fmt.Errorf("getblockheaderbyheight: status %q", resp.Status)
	}
	return &resp.BlockHeader, nil
}

// GetBlockHeaderByHash returns the block header with the given hash.
func (c *Client) GetBlockHeaderByHash(hash string) (*BlockHeader, error) {
	params := struct {
		Hash string `json:"hash"`
	}{Hash: hash}
	var resp struct {
		BlockHeader BlockHeader `json:"block_header"`
		Status      string     `json:"status"`
	}
	if err := c.call("getblockheaderbyhash", params, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "OK" {
		return nil, fmt.Errorf("getblockheaderbyhash: status %q", resp.Status)
	}
	return &resp.BlockHeader, nil
}

// GetBlocksDetails returns full block details starting at the given height.
func (c *Client) GetBlocksDetails(heightStart, count uint64) ([]BlockDetails, error) {
	params := struct {
		HeightStart        uint64 `json:"height_start"`
		Count              uint64 `json:"count"`
		IgnoreTransactions bool   `json:"ignore_transactions"`
	}{HeightStart: heightStart, Count: count}
	var resp struct {
		Blocks []BlockDetails `json:"blocks"`
		Status string         `json:"status"`
	}
	if err := c.call("get_blocks_details", params, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "OK" {
		return nil, fmt.Errorf("get_blocks_details: status %q", resp.Status)
	}
	return resp.Blocks, nil
}
