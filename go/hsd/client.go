// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

// Package hsd provides a typed client for the HSD (Handshake) sidechain RPC.
// Used by go-lns for DNS record fetching and tree-root invalidation.
package hsd

import (
	"encoding/base64"
	"io"
	"net/http"
	"time"

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
)

// Client talks to an HSD sidechain node via JSON-RPC.
//
//	client := hsd.NewClient("http://127.0.0.1:14037", "testkey")
type Client struct {
	url    string
	apiKey string
	http   *http.Client
}

// NewClient creates an HSD RPC client.
//
//	client := hsd.NewClient("http://127.0.0.1:14037", "testkey")
func NewClient(url, apiKey string) *Client {
	return &Client{
		url:    url,
		apiKey: apiKey,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

// BlockchainInfo holds the response from getblockchaininfo.
type BlockchainInfo struct {
	Chain    string `json:"chain"`
	Blocks   int    `json:"blocks"`
	Headers  int    `json:"headers"`
	TreeRoot string `json:"treeroot"`
	BestHash string `json:"bestblockhash"`
}

// NameResource holds DNS records for a name from getnameresource.
type NameResource struct {
	Records []Record `json:"records"`
}

// Record is a single DNS record from HSD.
type Record struct {
	Type       string   `json:"type"`       // GLUE4, GLUE6, TXT, NS, DS
	NS         string   `json:"ns"`         // nameserver
	Address    string   `json:"address"`    // IP address
	TXT        []string `json:"txt"`        // text records
	KeyTag     uint16   `json:"keyTag"`     // DS record
	Algorithm  uint8    `json:"algorithm"`  // DS record
	DigestType uint8    `json:"digestType"` // DS record
	Digest     string   `json:"digest"`     // DS record
}

// GetBlockchainInfo returns sidechain state including the tree root hash.
//
//	info, err := client.GetBlockchainInfo()
//	if info.TreeRoot != lastRoot { /* regenerate zone */ }
func (c *Client) GetBlockchainInfo() (*BlockchainInfo, error) {
	var info BlockchainInfo
	if err := c.call("getblockchaininfo", nil, &info); err != nil {
		return nil, coreerr.E("HSD.GetBlockchainInfo", "getblockchaininfo", err)
	}
	return &info, nil
}

// GetNameResource fetches DNS records for a name from the sidechain.
//
//	resource, err := client.GetNameResource("charon")
func (c *Client) GetNameResource(name string) (*NameResource, error) {
	var resource NameResource
	if err := c.call("getnameresource", []interface{}{name}, &resource); err != nil {
		return nil, coreerr.E("HSD.GetNameResource", core.Sprintf("getnameresource %s", name), err)
	}
	return &resource, nil
}

// GetHeight returns the sidechain block height.
//
//	height, err := client.GetHeight()
func (c *Client) GetHeight() (int, error) {
	var info struct {
		Blocks int `json:"blocks"`
	}
	if err := c.call("getinfo", nil, &info); err != nil {
		return 0, coreerr.E("HSD.GetHeight", "getinfo", err)
	}
	return info.Blocks, nil
}

func (c *Client) call(method string, params interface{}, result interface{}) error {
	body := map[string]interface{}{"method": method, "params": params}
	data := core.JSONMarshalString(body)

	req, err := http.NewRequest("POST", c.url, core.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("x:"+c.apiKey)))

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	var rpcResp struct {
		Result interface{} `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	// Unmarshal into generic first to check for error
	core.JSONUnmarshalString(string(raw), &rpcResp)
	if rpcResp.Error != nil {
		return coreerr.E("HSD.call", rpcResp.Error.Message, nil)
	}

	// Re-unmarshal the result into the specific type
	var fullResp struct {
		Result interface{} `json:"result"`
	}
	fullResp.Result = result
	core.JSONUnmarshalString(string(raw), &fullResp)

	return nil
}
