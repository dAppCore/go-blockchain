// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

// Package rpc provides a typed client for the Lethean daemon JSON-RPC API.
package rpc

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"time"

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
)

// Client is a Lethean daemon RPC client.
// Usage: var value rpc.Client
type Client struct {
	url        string // Base URL with /json_rpc path for JSON-RPC calls.
	baseURL    string // Base URL without path for legacy calls.
	httpClient *http.Client
}

// NewClient creates a client for the daemon at the given URL.
// If the URL has no path, "/json_rpc" is appended automatically.
// Usage: rpc.NewClient(...)
func NewClient(daemonURL string) *Client {
	return NewClientWithHTTP(daemonURL, &http.Client{Timeout: 30 * time.Second})
}

// NewClientWithHTTP creates a client with a custom http.Client.
// Usage: rpc.NewClientWithHTTP(...)
func NewClientWithHTTP(daemonURL string, httpClient *http.Client) *Client {
	u, err := url.Parse(daemonURL)
	if err != nil {
		// Fall through with raw URL.
		return &Client{url: daemonURL + "/json_rpc", baseURL: daemonURL, httpClient: httpClient}
	}
	baseURL := core.Sprintf("%s://%s", u.Scheme, u.Host)
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
// Usage: var value rpc.RPCError
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Usage: value.Error(...)
func (e *RPCError) Error() string {
	return core.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// JSON-RPC 2.0 envelope types.
type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type jsonRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      rawJSON       `json:"id"`
	Result  rawJSON       `json:"result"`
	Error   *jsonRPCError `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rawJSON []byte

// Usage: value.UnmarshalJSON(...)
func (r *rawJSON) UnmarshalJSON(data []byte) error {
	*r = append((*r)[:0], data...)
	return nil
}

// Usage: value.MarshalJSON(...)
func (r rawJSON) MarshalJSON() ([]byte, error) {
	if r == nil {
		return []byte("null"), nil
	}
	return []byte(r), nil
}

// call makes a JSON-RPC 2.0 call to /json_rpc.
func (c *Client) call(method string, params any, result any) error {
	reqBody := core.JSONMarshalString(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "0",
		Method:  method,
		Params:  params,
	})

	resp, err := c.httpClient.Post(c.url, "application/json", bytes.NewReader([]byte(reqBody)))
	if err != nil {
		return coreerr.E("Client.call", core.Sprintf("post %s", method), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return coreerr.E("Client.call", core.Sprintf("http %d from %s", resp.StatusCode, method), nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return coreerr.E("Client.call", "read response", err)
	}

	var rpcResp jsonRPCResponse
	if r := core.JSONUnmarshalString(string(body), &rpcResp); !r.OK {
		return coreerr.E("Client.call", "unmarshal response", r.Value.(error))
	}

	if rpcResp.Error != nil {
		return &RPCError{Code: rpcResp.Error.Code, Message: rpcResp.Error.Message}
	}

	if result != nil && len(rpcResp.Result) > 0 {
		if r := core.JSONUnmarshalString(string(rpcResp.Result), result); !r.OK {
			return coreerr.E("Client.call", "unmarshal result", r.Value.(error))
		}
	}
	return nil
}

// legacyCall makes a plain JSON POST to a legacy URI path (e.g. /getheight).
func (c *Client) legacyCall(path string, params any, result any) error {
	reqBody := core.JSONMarshalString(params)

	url := c.baseURL + path
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader([]byte(reqBody)))
	if err != nil {
		return coreerr.E("Client.legacyCall", core.Sprintf("post %s", path), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return coreerr.E("Client.legacyCall", core.Sprintf("http %d from %s", resp.StatusCode, path), nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return coreerr.E("Client.legacyCall", "read response", err)
	}

	if result != nil {
		if r := core.JSONUnmarshalString(string(body), result); !r.OK {
			return coreerr.E("Client.legacyCall", "unmarshal response", r.Value.(error))
		}
	}
	return nil
}
