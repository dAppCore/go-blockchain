# Phase 4: RPC Client Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement a typed JSON-RPC 2.0 client for querying the Lethean C++ daemon, covering 10 core chain query endpoints.

**Architecture:** Single `rpc/` package with a `Client` struct wrapping `net/http`. 8 endpoints use JSON-RPC 2.0 via `/json_rpc`. 2 endpoints (`getheight`, `gettransactions`) use legacy JSON POST to dedicated URI paths. Response types are shared Go structs with JSON tags.

**Tech Stack:** Go stdlib only (`net/http`, `encoding/json`, `net/http/httptest` for tests). No new module dependencies.

**Design doc:** `docs/plans/2026-02-20-rpc-client-design.md`

---

## Important: Endpoint Transport Discovery

Research of the C++ source (`core_rpc_server.h`) revealed that 2 of the 10 endpoints are **not available via `/json_rpc`**:

| Endpoint | Path | Transport |
|----------|------|-----------|
| `getblockcount` | `/json_rpc` | JSON-RPC 2.0 |
| `getheight` | `/getheight` | Legacy JSON POST |
| `getlastblockheader` | `/json_rpc` | JSON-RPC 2.0 |
| `getblockheaderbyheight` | `/json_rpc` | JSON-RPC 2.0 |
| `getblockheaderbyhash` | `/json_rpc` | JSON-RPC 2.0 |
| `getinfo` | `/json_rpc` | JSON-RPC 2.0 |
| `get_blocks_details` | `/json_rpc` | JSON-RPC 2.0 |
| `get_tx_details` | `/json_rpc` | JSON-RPC 2.0 |
| `gettransactions` | `/gettransactions` | Legacy JSON POST |
| `submitblock` | `/json_rpc` | JSON-RPC 2.0 |

The client needs both a `call()` method (JSON-RPC) and a `legacyCall()` method (plain POST to a URI path).

---

### Task 1: Client Transport + JSON-RPC 2.0

**Files:**
- Create: `rpc/client.go`
- Create: `rpc/client_test.go`

**Context:**

The JSON-RPC 2.0 protocol wraps every call in an envelope:

```json
Request:  {"jsonrpc":"2.0","id":"0","method":"getblockcount","params":{}}
Response: {"jsonrpc":"2.0","id":"0","result":{"count":6300,"status":"OK"}}
Error:    {"jsonrpc":"2.0","id":"0","error":{"code":-2,"message":"TOO_BIG_HEIGHT"}}
```

The client also needs a legacy call method for endpoints registered via
`MAP_URI_AUTO_JON2` (plain POST to a URI path, no JSON-RPC envelope):

```json
POST /getheight
Request:  {}
Response: {"height":6300,"status":"OK"}
```

**Step 1: Write the failing test**

Create `rpc/client_test.go`:

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Good_JSONRPCCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request format.
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}
		if r.URL.Path != "/json_rpc" {
			t.Errorf("path: got %s, want /json_rpc", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var req jsonRPCRequest
		json.Unmarshal(body, &req)
		if req.JSONRPC != "2.0" {
			t.Errorf("jsonrpc: got %q, want %q", req.JSONRPC, "2.0")
		}
		if req.Method != "getblockcount" {
			t.Errorf("method: got %q, want %q", req.Method, "getblockcount")
		}

		// Return a valid response.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Result:  json.RawMessage(`{"count":6300,"status":"OK"}`),
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	var result struct {
		Count  uint64 `json:"count"`
		Status string `json:"status"`
	}
	err := c.call("getblockcount", struct{}{}, &result)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if result.Count != 6300 {
		t.Errorf("count: got %d, want 6300", result.Count)
	}
	if result.Status != "OK" {
		t.Errorf("status: got %q, want %q", result.Status, "OK")
	}
}

func TestClient_Good_LegacyCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getheight" {
			t.Errorf("path: got %s, want /getheight", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"height":6300,"status":"OK"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	var result struct {
		Height uint64 `json:"height"`
		Status string `json:"status"`
	}
	err := c.legacyCall("/getheight", struct{}{}, &result)
	if err != nil {
		t.Fatalf("legacyCall: %v", err)
	}
	if result.Height != 6300 {
		t.Errorf("height: got %d, want 6300", result.Height)
	}
}

func TestClient_Bad_RPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Error: &jsonRPCError{
				Code:    -2,
				Message: "TOO_BIG_HEIGHT",
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	var result struct{}
	err := c.call("getblockheaderbyheight", struct{ Height uint64 }{999999999}, &result)
	if err == nil {
		t.Fatal("expected error")
	}
	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("expected *RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != -2 {
		t.Errorf("code: got %d, want -2", rpcErr.Code)
	}
}

func TestClient_Bad_ConnectionRefused(t *testing.T) {
	c := NewClient("http://127.0.0.1:1") // Unlikely to be listening
	var result struct{}
	err := c.call("getblockcount", struct{}{}, &result)
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestClient_Good_URLAppendPath(t *testing.T) {
	// NewClient should append /json_rpc if path is empty.
	c := NewClient("http://localhost:46941")
	if c.url != "http://localhost:46941/json_rpc" {
		t.Errorf("url: got %q, want %q", c.url, "http://localhost:46941/json_rpc")
	}

	// If path already present, leave it alone.
	c2 := NewClient("http://localhost:46941/json_rpc")
	if c2.url != "http://localhost:46941/json_rpc" {
		t.Errorf("url: got %q, want %q", c2.url, "http://localhost:46941/json_rpc")
	}
}

func TestClient_Bad_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	var result struct{}
	err := c.call("getblockcount", struct{}{}, &result)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestClient_Bad_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	var result struct{}
	err := c.call("getblockcount", struct{}{}, &result)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./rpc/ -run Client`
Expected: FAIL — package not found

**Step 3: Write the implementation**

Create `rpc/client.go`:

```go
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
	JSONRPC string           `json:"jsonrpc"`
	ID      json.RawMessage  `json:"id"`
	Result  json.RawMessage  `json:"result"`
	Error   *jsonRPCError    `json:"error,omitempty"`
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
		return fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("post %s: %w", method, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d from %s", resp.StatusCode, method)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	if rpcResp.Error != nil {
		return &RPCError{Code: rpcResp.Error.Code, Message: rpcResp.Error.Message}
	}

	if result != nil && len(rpcResp.Result) > 0 {
		if err := json.Unmarshal(rpcResp.Result, result); err != nil {
			return fmt.Errorf("unmarshal result: %w", err)
		}
	}
	return nil
}

// legacyCall makes a plain JSON POST to a legacy URI path (e.g. /getheight).
func (c *Client) legacyCall(path string, params any, result any) error {
	reqBody, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + path
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("post %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d from %s", resp.StatusCode, path)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}
	return nil
}
```

**Step 4: Run tests**

Run: `go test -race -v ./rpc/ -run Client`
Expected: PASS (7 tests)

Run: `go vet ./rpc/`
Expected: clean

**Step 5: Commit**

```bash
git add rpc/client.go rpc/client_test.go
git commit -m "feat(rpc): JSON-RPC 2.0 client transport with legacy call support

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 2: Response Types

**Files:**
- Create: `rpc/types.go`

**Context:**

Shared response types used across multiple endpoints. All field names match
the C++ daemon's JSON output exactly (via struct tags).

**Step 1: Write the types**

Create `rpc/types.go`:

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

// BlockHeader is a block header as returned by daemon RPC.
// Returned by getlastblockheader, getblockheaderbyheight, getblockheaderbyhash.
type BlockHeader struct {
	MajorVersion uint8  `json:"major_version"`
	MinorVersion uint8  `json:"minor_version"`
	Timestamp    uint64 `json:"timestamp"`
	PrevHash     string `json:"prev_hash"`
	Nonce        uint64 `json:"nonce"`
	OrphanStatus bool   `json:"orphan_status"`
	Height       uint64 `json:"height"`
	Depth        uint64 `json:"depth"`
	Hash         string `json:"hash"`
	Difficulty   string `json:"difficulty"`
	Reward       uint64 `json:"reward"`
}

// DaemonInfo is the daemon status as returned by getinfo.
type DaemonInfo struct {
	Height                       uint64 `json:"height"`
	TxCount                      uint64 `json:"tx_count"`
	TxPoolSize                   uint64 `json:"tx_pool_size"`
	AltBlocksCount               uint64 `json:"alt_blocks_count"`
	OutgoingConnectionsCount     uint64 `json:"outgoing_connections_count"`
	IncomingConnectionsCount     uint64 `json:"incoming_connections_count"`
	SynchronizedConnectionsCount uint64 `json:"synchronized_connections_count"`
	DaemonNetworkState           uint64 `json:"daemon_network_state"`
	SynchronizationStartHeight   uint64 `json:"synchronization_start_height"`
	MaxNetSeenHeight             uint64 `json:"max_net_seen_height"`
	PowDifficulty                uint64 `json:"pow_difficulty"`
	PosDifficulty                string `json:"pos_difficulty"`
	BlockReward                  uint64 `json:"block_reward"`
	DefaultFee                   uint64 `json:"default_fee"`
	MinimumFee                   uint64 `json:"minimum_fee"`
	LastBlockTimestamp           uint64 `json:"last_block_timestamp"`
	LastBlockHash                string `json:"last_block_hash"`
	AliasCount                   uint64 `json:"alias_count"`
	TotalCoins                   string `json:"total_coins"`
	PosAllowed                   bool   `json:"pos_allowed"`
	CurrentMaxAllowedBlockSize   uint64 `json:"current_max_allowed_block_size"`
}

// BlockDetails is a full block with metadata as returned by get_blocks_details.
type BlockDetails struct {
	Height         uint64   `json:"height"`
	Timestamp      uint64   `json:"timestamp"`
	ActualTimestamp uint64  `json:"actual_timestamp"`
	BaseReward     uint64   `json:"base_reward"`
	SummaryReward  uint64   `json:"summary_reward"`
	TotalFee       uint64   `json:"total_fee"`
	ID             string   `json:"id"`
	PrevID         string   `json:"prev_id"`
	Difficulty     string   `json:"difficulty"`
	Type           uint64   `json:"type"`
	IsOrphan       bool     `json:"is_orphan"`
	CumulativeSize uint64   `json:"block_cumulative_size"`
	Blob           string   `json:"blob"`
	ObjectInJSON   string   `json:"object_in_json"`
	Transactions   []TxInfo `json:"transactions_details"`
}

// TxInfo is transaction metadata as returned by get_tx_details.
type TxInfo struct {
	ID           string `json:"id"`
	BlobSize     uint64 `json:"blob_size"`
	Fee          uint64 `json:"fee"`
	Amount       uint64 `json:"amount"`
	Timestamp    uint64 `json:"timestamp"`
	KeeperBlock  int64  `json:"keeper_block"`
	Blob         string `json:"blob"`
	ObjectInJSON string `json:"object_in_json"`
}
```

**Step 2: Verify it compiles**

Run: `go vet ./rpc/`
Expected: clean

**Step 3: Commit**

```bash
git add rpc/types.go
git commit -m "feat(rpc): response types for daemon RPC endpoints

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 3: GetInfo, GetHeight, GetBlockCount

**Files:**
- Create: `rpc/info.go`
- Create: `rpc/info_test.go`

**Context:**

Three simple endpoints for daemon status:

- `GetInfo()` → JSON-RPC `getinfo` with `{flags: 0}` → `*DaemonInfo`
- `GetHeight()` → Legacy POST `/getheight` with `{}` → `uint64`
- `GetBlockCount()` → JSON-RPC `getblockcount` with `{}` → `uint64`

Note: `getheight` is a **legacy endpoint** (not available via `/json_rpc`).

**Step 1: Write the failing test**

Create `rpc/info_test.go`:

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetInfo_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Result: json.RawMessage(`{
				"height": 6300,
				"tx_count": 12345,
				"tx_pool_size": 3,
				"outgoing_connections_count": 8,
				"incoming_connections_count": 4,
				"synchronized_connections_count": 7,
				"daemon_network_state": 2,
				"pow_difficulty": 1000000,
				"block_reward": 1000000000000,
				"default_fee": 10000000000,
				"minimum_fee": 10000000000,
				"last_block_hash": "abc123",
				"total_coins": "17500000000000000000",
				"pos_allowed": true,
				"status": "OK"
			}`),
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	info, err := c.GetInfo()
	if err != nil {
		t.Fatalf("GetInfo: %v", err)
	}
	if info.Height != 6300 {
		t.Errorf("height: got %d, want 6300", info.Height)
	}
	if info.TxCount != 12345 {
		t.Errorf("tx_count: got %d, want 12345", info.TxCount)
	}
	if info.BlockReward != 1000000000000 {
		t.Errorf("block_reward: got %d, want 1000000000000", info.BlockReward)
	}
	if !info.PosAllowed {
		t.Error("pos_allowed: got false, want true")
	}
}

func TestGetHeight_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getheight" {
			t.Errorf("path: got %s, want /getheight", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"height":6300,"status":"OK"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	height, err := c.GetHeight()
	if err != nil {
		t.Fatalf("GetHeight: %v", err)
	}
	if height != 6300 {
		t.Errorf("height: got %d, want 6300", height)
	}
}

func TestGetBlockCount_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Result:  json.RawMessage(`{"count":6301,"status":"OK"}`),
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	count, err := c.GetBlockCount()
	if err != nil {
		t.Fatalf("GetBlockCount: %v", err)
	}
	if count != 6301 {
		t.Errorf("count: got %d, want 6301", count)
	}
}
```

**Step 2: Write the implementation**

Create `rpc/info.go`:

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import "fmt"

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
		return nil, fmt.Errorf("getinfo: status %q", resp.Status)
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
		return 0, fmt.Errorf("getheight: status %q", resp.Status)
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
		return 0, fmt.Errorf("getblockcount: status %q", resp.Status)
	}
	return resp.Count, nil
}
```

**Step 3: Run tests**

Run: `go test -race -v ./rpc/ -run "GetInfo|GetHeight|GetBlockCount"`
Expected: PASS (3 tests)

Run: `go vet ./rpc/`
Expected: clean

**Step 4: Commit**

```bash
git add rpc/info.go rpc/info_test.go
git commit -m "feat(rpc): GetInfo, GetHeight, GetBlockCount endpoints

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 4: Block Header Endpoints

**Files:**
- Create: `rpc/blocks.go`
- Create: `rpc/blocks_test.go`

**Context:**

Four block-related endpoints, all JSON-RPC:

- `GetLastBlockHeader()` → `getlastblockheader` with `{}` (the C++ source
  expects `request = list<string>` but an empty object `{}` also works)
- `GetBlockHeaderByHeight(h)` → `getblockheaderbyheight` with `{height: h}`
- `GetBlockHeaderByHash(hash)` → `getblockheaderbyhash` with `{hash: hash}`
- `GetBlocksDetails(start, count)` → `get_blocks_details` with
  `{height_start, count, ignore_transactions: false}`

The first three return `block_header` (a `BlockHeader` struct). The fourth
returns `blocks` (an array of `BlockDetails`).

**Step 1: Write the failing test**

Create `rpc/blocks_test.go`:

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

var testBlockHeaderJSON = `{
	"major_version": 1,
	"minor_version": 0,
	"timestamp": 1770897600,
	"prev_hash": "0000000000000000000000000000000000000000000000000000000000000000",
	"nonce": 101011010221,
	"orphan_status": false,
	"height": 0,
	"depth": 6300,
	"hash": "cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963",
	"difficulty": "1",
	"reward": 1000000000000
}`

func blockHeaderResponse() jsonRPCResponse {
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"0"`),
		Result:  json.RawMessage(`{"block_header":` + testBlockHeaderJSON + `,"status":"OK"}`),
	}
}

func TestGetLastBlockHeader_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(blockHeaderResponse())
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	hdr, err := c.GetLastBlockHeader()
	if err != nil {
		t.Fatalf("GetLastBlockHeader: %v", err)
	}
	if hdr.Height != 0 {
		t.Errorf("height: got %d, want 0", hdr.Height)
	}
	if hdr.Hash != "cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963" {
		t.Errorf("hash: got %q", hdr.Hash)
	}
	if hdr.MajorVersion != 1 {
		t.Errorf("major_version: got %d, want 1", hdr.MajorVersion)
	}
}

func TestGetBlockHeaderByHeight_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(blockHeaderResponse())
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	hdr, err := c.GetBlockHeaderByHeight(0)
	if err != nil {
		t.Fatalf("GetBlockHeaderByHeight: %v", err)
	}
	if hdr.Height != 0 {
		t.Errorf("height: got %d, want 0", hdr.Height)
	}
}

func TestGetBlockHeaderByHash_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(blockHeaderResponse())
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	hdr, err := c.GetBlockHeaderByHash("cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963")
	if err != nil {
		t.Fatalf("GetBlockHeaderByHash: %v", err)
	}
	if hdr.Reward != 1000000000000 {
		t.Errorf("reward: got %d, want 1000000000000", hdr.Reward)
	}
}

func TestGetBlocksDetails_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Result: json.RawMessage(`{
				"blocks": [{
					"height": 0,
					"timestamp": 1770897600,
					"base_reward": 1000000000000,
					"id": "cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963",
					"type": 1,
					"is_orphan": false,
					"transactions_details": []
				}],
				"status": "OK"
			}`),
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	blocks, err := c.GetBlocksDetails(0, 1)
	if err != nil {
		t.Fatalf("GetBlocksDetails: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("blocks: got %d, want 1", len(blocks))
	}
	if blocks[0].Height != 0 {
		t.Errorf("height: got %d, want 0", blocks[0].Height)
	}
	if blocks[0].ID != "cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963" {
		t.Errorf("id: got %q", blocks[0].ID)
	}
}

func TestGetBlockHeaderByHeight_Bad_TooBig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Error:   &jsonRPCError{Code: -2, Message: "TOO_BIG_HEIGHT"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.GetBlockHeaderByHeight(999999999)
	if err == nil {
		t.Fatal("expected error")
	}
	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("expected *RPCError, got %T", err)
	}
	if rpcErr.Code != -2 {
		t.Errorf("code: got %d, want -2", rpcErr.Code)
	}
}
```

**Step 2: Write the implementation**

Create `rpc/blocks.go`:

```go
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
		HeightStart         uint64 `json:"height_start"`
		Count               uint64 `json:"count"`
		IgnoreTransactions  bool   `json:"ignore_transactions"`
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
```

**Step 3: Run tests**

Run: `go test -race -v ./rpc/ -run "BlockHeader|BlocksDetails"`
Expected: PASS (5 tests)

Run: `go vet ./rpc/`
Expected: clean

**Step 4: Commit**

```bash
git add rpc/blocks.go rpc/blocks_test.go
git commit -m "feat(rpc): block header and block details endpoints

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 5: Transaction Endpoints

**Files:**
- Create: `rpc/transactions.go`
- Create: `rpc/transactions_test.go`

**Context:**

Two transaction endpoints:

- `GetTxDetails(hash)` → JSON-RPC `get_tx_details` with `{tx_hash: hash}` →
  response wraps `TxInfo` in a `tx_info` field
- `GetTransactions(hashes)` → Legacy POST `/gettransactions` with
  `{txs_hashes: [...]}` → returns `txs_as_hex` (found) and `missed_tx` (not found)

Note: `gettransactions` is a **legacy endpoint** (not available via `/json_rpc`).

**Step 1: Write the failing test**

Create `rpc/transactions_test.go`:

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetTxDetails_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Result: json.RawMessage(`{
				"status": "OK",
				"tx_info": {
					"id": "a6e8da986858e6825fce7a192097e6afae4e889cabe853a9c29b964985b23da8",
					"blob_size": 6794,
					"fee": 1000000000,
					"amount": 18999000000000,
					"timestamp": 1557345925,
					"keeper_block": 51,
					"blob": "ARMB..."
				}
			}`),
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	tx, err := c.GetTxDetails("a6e8da986858e6825fce7a192097e6afae4e889cabe853a9c29b964985b23da8")
	if err != nil {
		t.Fatalf("GetTxDetails: %v", err)
	}
	if tx.ID != "a6e8da986858e6825fce7a192097e6afae4e889cabe853a9c29b964985b23da8" {
		t.Errorf("id: got %q", tx.ID)
	}
	if tx.Fee != 1000000000 {
		t.Errorf("fee: got %d, want 1000000000", tx.Fee)
	}
	if tx.KeeperBlock != 51 {
		t.Errorf("keeper_block: got %d, want 51", tx.KeeperBlock)
	}
}

func TestGetTxDetails_Bad_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Error:   &jsonRPCError{Code: -14, Message: "NOT_FOUND"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.GetTxDetails("0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetTransactions_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gettransactions" {
			t.Errorf("path: got %s, want /gettransactions", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"txs_as_hex": ["01020304"],
			"missed_tx": ["abcd1234"],
			"status": "OK"
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	found, missed, err := c.GetTransactions([]string{"deadbeef", "abcd1234"})
	if err != nil {
		t.Fatalf("GetTransactions: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("found: got %d, want 1", len(found))
	}
	if found[0] != "01020304" {
		t.Errorf("found[0]: got %q, want %q", found[0], "01020304")
	}
	if len(missed) != 1 {
		t.Fatalf("missed: got %d, want 1", len(missed))
	}
	if missed[0] != "abcd1234" {
		t.Errorf("missed[0]: got %q, want %q", missed[0], "abcd1234")
	}
}

func TestGetTransactions_Good_AllFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"txs_as_hex":["aa","bb"],"missed_tx":[],"status":"OK"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	found, missed, err := c.GetTransactions([]string{"hash1", "hash2"})
	if err != nil {
		t.Fatalf("GetTransactions: %v", err)
	}
	if len(found) != 2 {
		t.Errorf("found: got %d, want 2", len(found))
	}
	if len(missed) != 0 {
		t.Errorf("missed: got %d, want 0", len(missed))
	}
}
```

**Step 2: Write the implementation**

Create `rpc/transactions.go`:

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import "fmt"

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
		return nil, fmt.Errorf("get_tx_details: status %q", resp.Status)
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
		return nil, nil, fmt.Errorf("gettransactions: status %q", resp.Status)
	}
	return resp.TxsAsHex, resp.MissedTx, nil
}
```

**Step 3: Run tests**

Run: `go test -race -v ./rpc/ -run "TxDetails|Transactions"`
Expected: PASS (4 tests)

Run: `go vet ./rpc/`
Expected: clean

**Step 4: Commit**

```bash
git add rpc/transactions.go rpc/transactions_test.go
git commit -m "feat(rpc): transaction detail and bulk fetch endpoints

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 6: SubmitBlock

**Files:**
- Create: `rpc/mining.go`
- Create: `rpc/mining_test.go`

**Context:**

The `submitblock` JSON-RPC method is special: its params are a **JSON array**
of strings (not an object). The C++ source defines `request = vector<string>`.

```json
{"jsonrpc":"2.0","id":"0","method":"submitblock","params":["03000000..."]}
```

**Step 1: Write the failing test**

Create `rpc/mining_test.go`:

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSubmitBlock_Good(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req jsonRPCRequest
		json.Unmarshal(body, &req)
		if req.Method != "submitblock" {
			t.Errorf("method: got %q, want %q", req.Method, "submitblock")
		}
		// Verify params is an array.
		raw, _ := json.Marshal(req.Params)
		if raw[0] != '[' {
			t.Errorf("params should be array, got: %s", raw)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Result:  json.RawMessage(`{"status":"OK"}`),
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	err := c.SubmitBlock("0300000001020304")
	if err != nil {
		t.Fatalf("SubmitBlock: %v", err)
	}
}

func TestSubmitBlock_Bad_Rejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"0"`),
			Error:   &jsonRPCError{Code: -7, Message: "BLOCK_NOT_ACCEPTED"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	err := c.SubmitBlock("invalid")
	if err == nil {
		t.Fatal("expected error")
	}
	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("expected *RPCError, got %T", err)
	}
	if rpcErr.Code != -7 {
		t.Errorf("code: got %d, want -7", rpcErr.Code)
	}
}
```

**Step 2: Write the implementation**

Create `rpc/mining.go`:

```go
// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import "fmt"

// SubmitBlock submits a mined block to the daemon.
// The hexBlob is the hex-encoded serialised block.
// Note: submitblock takes a JSON array as params, not an object.
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
		return fmt.Errorf("submitblock: status %q", resp.Status)
	}
	return nil
}
```

**Step 3: Run tests**

Run: `go test -race -v ./rpc/ -run SubmitBlock`
Expected: PASS (2 tests)

Run: `go test -race ./rpc/`
Expected: PASS (all rpc tests)

Run: `go vet ./rpc/`
Expected: clean

**Step 4: Commit**

```bash
git add rpc/mining.go rpc/mining_test.go
git commit -m "feat(rpc): SubmitBlock endpoint

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 7: Integration Test Against C++ Testnet

**Files:**
- Create: `rpc/integration_test.go`

**Context:**

The C++ testnet daemon runs on `localhost:46941` (RPC port). This is the Phase 4
equivalent of the genesis block hash test — if we can query the daemon and get
matching block hashes, the entire RPC client is correct.

The testnet genesis block hash is
`cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963` (verified in
Phase 1).

**Step 1: Write the integration test**

Create `rpc/integration_test.go`:

```go
//go:build integration

// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import (
	"net/http"
	"testing"
	"time"
)

const testnetRPCAddr = "http://localhost:46941"

// TestIntegration_RPC connects to the C++ testnet daemon and queries
// all Tier 1 endpoints.
func TestIntegration_RPC(t *testing.T) {
	c := NewClientWithHTTP(testnetRPCAddr, &http.Client{Timeout: 10 * time.Second})

	// --- GetHeight ---
	height, err := c.GetHeight()
	if err != nil {
		t.Skipf("testnet daemon not reachable at %s: %v", testnetRPCAddr, err)
	}
	if height == 0 {
		t.Error("height is 0 — daemon may not be synced")
	}
	t.Logf("testnet height: %d", height)

	// --- GetBlockCount ---
	count, err := c.GetBlockCount()
	if err != nil {
		t.Fatalf("GetBlockCount: %v", err)
	}
	// count should be height or height+1
	if count < height {
		t.Errorf("count %d < height %d", count, height)
	}
	t.Logf("testnet block count: %d", count)

	// --- GetInfo ---
	info, err := c.GetInfo()
	if err != nil {
		t.Fatalf("GetInfo: %v", err)
	}
	if info.Height == 0 {
		t.Error("info.height is 0")
	}
	t.Logf("testnet info: height=%d, tx_count=%d, connections=%d+%d",
		info.Height, info.TxCount,
		info.OutgoingConnectionsCount, info.IncomingConnectionsCount)

	// --- GetLastBlockHeader ---
	lastHdr, err := c.GetLastBlockHeader()
	if err != nil {
		t.Fatalf("GetLastBlockHeader: %v", err)
	}
	if lastHdr.Hash == "" {
		t.Error("last block hash is empty")
	}
	t.Logf("testnet last block: height=%d, hash=%s", lastHdr.Height, lastHdr.Hash)

	// --- GetBlockHeaderByHeight (genesis) ---
	genesis, err := c.GetBlockHeaderByHeight(0)
	if err != nil {
		t.Fatalf("GetBlockHeaderByHeight(0): %v", err)
	}
	const expectedGenesisHash = "cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963"
	if genesis.Hash != expectedGenesisHash {
		t.Errorf("genesis hash: got %q, want %q", genesis.Hash, expectedGenesisHash)
	}
	t.Logf("genesis block hash verified: %s", genesis.Hash)

	// --- GetBlockHeaderByHash (using genesis hash) ---
	byHash, err := c.GetBlockHeaderByHash(expectedGenesisHash)
	if err != nil {
		t.Fatalf("GetBlockHeaderByHash: %v", err)
	}
	if byHash.Height != 0 {
		t.Errorf("genesis by hash: height got %d, want 0", byHash.Height)
	}

	// --- GetBlocksDetails (genesis block) ---
	blocks, err := c.GetBlocksDetails(0, 1)
	if err != nil {
		t.Fatalf("GetBlocksDetails: %v", err)
	}
	if len(blocks) == 0 {
		t.Fatal("GetBlocksDetails returned 0 blocks")
	}
	if blocks[0].ID != expectedGenesisHash {
		t.Errorf("block[0].id: got %q, want %q", blocks[0].ID, expectedGenesisHash)
	}
	t.Logf("genesis block details: reward=%d, type=%d", blocks[0].BaseReward, blocks[0].Type)
}
```

**Step 2: Run tests**

Run: `go test -race -v -tags integration ./rpc/ -run Integration -timeout 30s`
Expected: Either PASS (daemon running) or SKIP (daemon not reachable)

Run: `go test -race ./...` (without integration tag — all non-integration tests pass)
Expected: PASS

**Step 3: Commit**

```bash
git add rpc/integration_test.go
git commit -m "test(rpc): integration test against C++ testnet daemon

Co-Authored-By: Charon <charon@lethean.io>"
```

---

### Task 8: Documentation

**Files:**
- Modify: `docs/architecture.md`
- Modify: `docs/history.md`

**Context:**

Update project documentation with Phase 4 completion details.

**Step 1: Update architecture.md**

Read `docs/architecture.md` first. Add `rpc/` to the package structure listing
(after `p2p/`):

```
rpc/          Daemon JSON-RPC 2.0 client (10 endpoints)
```

Add a new section `### rpc/` describing the package:

```markdown
### rpc/

Typed JSON-RPC 2.0 client for querying the Lethean daemon. The `Client` struct
wraps `net/http` and provides Go methods for 10 core daemon endpoints.

Eight endpoints use JSON-RPC 2.0 via `/json_rpc`. Two endpoints (`GetHeight`,
`GetTransactions`) use legacy JSON POST to dedicated URI paths (`/getheight`,
`/gettransactions`), as the C++ daemon registers these with `MAP_URI_AUTO_JON2`
rather than `MAP_JON_RPC`.

**Client transport:**
- `client.go` -- `Client` struct with `call()` (JSON-RPC 2.0) and `legacyCall()`
  (plain JSON POST). `RPCError` type for daemon error codes.
- `types.go` -- `BlockHeader`, `DaemonInfo`, `BlockDetails`, `TxInfo` shared types.

**Endpoints:**
- `info.go` -- `GetInfo`, `GetHeight` (legacy), `GetBlockCount`.
- `blocks.go` -- `GetLastBlockHeader`, `GetBlockHeaderByHeight`,
  `GetBlockHeaderByHash`, `GetBlocksDetails`.
- `transactions.go` -- `GetTxDetails`, `GetTransactions` (legacy).
- `mining.go` -- `SubmitBlock`.

**Testing:**
- Mock HTTP server tests for all endpoints and error paths.
- Build-tagged integration test (`//go:build integration`) against C++ testnet
  daemon on `localhost:46941`. Verifies genesis block hash matches Phase 1
  result (`cb9d5455...`).
```

**Step 2: Update history.md**

Read `docs/history.md` first. Replace the "Phase 4 -- RPC Layer (Planned)"
section with the completion record. Include:

- Files added (8 files in `rpc/`)
- Key findings (legacy vs JSON-RPC endpoints, submitblock array params, etc.)
- Tests added (count and categories)
- Coverage numbers
- Integration test result

**Step 3: Run full test suite**

Run: `go test -race ./...`
Expected: PASS (all packages)

Run: `go vet ./...`
Expected: clean

**Step 4: Commit**

```bash
git add docs/architecture.md docs/history.md
git commit -m "docs: Phase 4 RPC client documentation

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## File Summary

| # | File | Action | Purpose |
|---|------|--------|---------|
| 1 | `rpc/client.go` | create | Client struct, JSON-RPC 2.0 + legacy transport |
| 2 | `rpc/client_test.go` | create | Transport tests (7 tests) |
| 3 | `rpc/types.go` | create | BlockHeader, DaemonInfo, BlockDetails, TxInfo |
| 4 | `rpc/info.go` | create | GetInfo, GetHeight, GetBlockCount |
| 5 | `rpc/info_test.go` | create | Info endpoint tests (3 tests) |
| 6 | `rpc/blocks.go` | create | Block header + block details endpoints |
| 7 | `rpc/blocks_test.go` | create | Block endpoint tests (5 tests) |
| 8 | `rpc/transactions.go` | create | GetTxDetails, GetTransactions |
| 9 | `rpc/transactions_test.go` | create | Transaction endpoint tests (4 tests) |
| 10 | `rpc/mining.go` | create | SubmitBlock |
| 11 | `rpc/mining_test.go` | create | SubmitBlock tests (2 tests) |
| 12 | `rpc/integration_test.go` | create | C++ testnet integration test |
| 13 | `docs/architecture.md` | modify | Add rpc/ section |
| 14 | `docs/history.md` | modify | Phase 4 completion record |

## Verification

1. `go test -race ./...` — all tests pass
2. `go vet ./...` — no warnings
3. `go test -race -tags integration ./rpc/` — genesis hash matches `cb9d5455...`
4. Coverage target: >80% across `rpc/` files

## C++ Reference Files

- `~/Code/LetheanNetwork/blockchain/src/rpc/core_rpc_server_commands_defs.h` — struct definitions
- `~/Code/LetheanNetwork/blockchain/src/rpc/core_rpc_server.h` — method registration (`MAP_JON_RPC` vs `MAP_URI_AUTO_JON2`)
- `~/Code/LetheanNetwork/blockchain/src/rpc/core_rpc_server_error_codes.h` — error codes
- `~/Code/LetheanNetwork/zano-docs/docs/build/rpc-api/daemon-rpc-api/` — API documentation
