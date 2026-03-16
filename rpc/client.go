// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

// Package rpc provides a typed client for the Lethean daemon JSON-RPC API.
package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	coreerr "forge.lthn.ai/core/go-log"
)

// Client is a Lethean daemon RPC client.
type Client struct {
	url        string // Base URL with /json_rpc path for JSON-RPC calls.
	baseURL    string // Base URL without path for legacy calls.
	httpClient *http.Client
}

// NewClient creates a client for the daemon at the given URL.
// If the URL has no path, "/json_rpc" is appended automatically.
func NewClient(daemonURL string) *Client {
	return NewClientWithHTTP(daemonURL, &http.Client{Timeout: 30 * time.Second})
}

// NewClientWithHTTP creates a client with a custom http.Client.
func NewClientWithHTTP(daemonURL string, httpClient *http.Client) *Client {
	u, err := url.Parse(daemonURL)
	if err != nil {
		// Fall through with raw URL.
		return &Client{url: daemonURL + "/json_rpc", baseURL: daemonURL, httpClient: httpClient}
	}
	baseURL := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	if u.Path == "" || u.Path == "/" {
		u.Path = "/json_rpc"
	}
	return &Client{
		url:        u.String(),
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// RPCError represents a JSON-RPC error returned by the daemon.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// JSON-RPC 2.0 envelope types.
type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// call makes a JSON-RPC 2.0 call to /json_rpc.
func (c *Client) call(method string, params any, result any) error {
	reqBody, err := json.Marshal(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "0",
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return coreerr.E("Client.call", "marshal request", err)
	}

	resp, err := c.httpClient.Post(c.url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return coreerr.E("Client.call", fmt.Sprintf("post %s", method), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return coreerr.E("Client.call", fmt.Sprintf("http %d from %s", resp.StatusCode, method), nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return coreerr.E("Client.call", "read response", err)
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return coreerr.E("Client.call", "unmarshal response", err)
	}

	if rpcResp.Error != nil {
		return &RPCError{Code: rpcResp.Error.Code, Message: rpcResp.Error.Message}
	}

	if result != nil && len(rpcResp.Result) > 0 {
		if err := json.Unmarshal(rpcResp.Result, result); err != nil {
			return coreerr.E("Client.call", "unmarshal result", err)
		}
	}
	return nil
}

// legacyCall makes a plain JSON POST to a legacy URI path (e.g. /getheight).
func (c *Client) legacyCall(path string, params any, result any) error {
	reqBody, err := json.Marshal(params)
	if err != nil {
		return coreerr.E("Client.legacyCall", "marshal request", err)
	}

	url := c.baseURL + path
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return coreerr.E("Client.legacyCall", fmt.Sprintf("post %s", path), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return coreerr.E("Client.legacyCall", fmt.Sprintf("http %d from %s", resp.StatusCode, path), nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return coreerr.E("Client.legacyCall", "read response", err)
	}

	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return coreerr.E("Client.legacyCall", "unmarshal response", err)
		}
	}
	return nil
}
