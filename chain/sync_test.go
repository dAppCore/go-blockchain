// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/rpc"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

// makeGenesisBlockBlob creates a minimal genesis block and returns its hex blob and hash.
func makeGenesisBlockBlob() (hexBlob string, hash types.Hash) {
	blk := types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Nonce:        101011010221,
			Timestamp:    1770897600,
		},
		MinerTx: types.Transaction{
			Version: 1,
			Vin:     []types.TxInput{types.TxInputGenesis{Height: 0}},
			Vout: []types.TxOutput{
				types.TxOutputBare{
					Amount: 1000000000000,
					Target: types.TxOutToKey{Key: types.PublicKey{0x01}},
				},
			},
			Extra:      wire.EncodeVarint(0),
			Attachment: wire.EncodeVarint(0),
		},
	}
	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	wire.EncodeBlock(enc, &blk)
	hexBlob = hex.EncodeToString(buf.Bytes())
	hash = wire.BlockHash(&blk)
	return
}

func TestSync_Good_SingleBlock(t *testing.T) {
	genesisBlob, genesisHash := makeGenesisBlockBlob()

	// Override genesis hash for this test.
	orig := GenesisHash
	GenesisHash = genesisHash.String()
	t.Cleanup(func() { GenesisHash = orig })

	// Mock RPC server.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/getheight" {
			json.NewEncoder(w).Encode(map[string]any{
				"height": 1,
				"status": "OK",
			})
			return
		}

		// JSON-RPC dispatcher.
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		switch req.Method {
		case "get_blocks_details":
			result := map[string]any{
				"blocks": []map[string]any{{
					"height":               uint64(0),
					"timestamp":            uint64(1770897600),
					"base_reward":          uint64(1000000000000),
					"id":                   genesisHash.String(),
					"difficulty":           "1",
					"type":                 uint64(1),
					"blob":                 genesisBlob,
					"transactions_details": []any{},
				}},
				"status": "OK",
			}
			resultBytes, _ := json.Marshal(result)
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      "0",
				"result":  json.RawMessage(resultBytes),
			})
		}
	}))
	defer srv.Close()

	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	client := rpc.NewClient(srv.URL)

	err := c.Sync(client)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	h, _ := c.Height()
	if h != 1 {
		t.Errorf("height after sync: got %d, want 1", h)
	}

	blk, meta, err := c.GetBlockByHeight(0)
	if err != nil {
		t.Fatalf("GetBlockByHeight(0): %v", err)
	}
	if blk.MajorVersion != 1 {
		t.Errorf("major_version: got %d, want 1", blk.MajorVersion)
	}
	if meta.Hash != genesisHash {
		t.Errorf("hash: got %s, want %s", meta.Hash, genesisHash)
	}
}

func TestSync_Good_TwoBlocks_WithRegularTx(t *testing.T) {
	// --- Build genesis block (block 0) ---
	genesisBlob, genesisHash := makeGenesisBlockBlob()

	// --- Build regular transaction for block 1 ---
	regularTx := types.Transaction{
		Version: 1,
		Vin: []types.TxInput{
			types.TxInputToKey{
				Amount: 1000000000000,
				KeyOffsets: []types.TxOutRef{{
					Tag:         types.RefTypeGlobalIndex,
					GlobalIndex: 0,
				}},
				KeyImage:   types.KeyImage{0xaa, 0xbb, 0xcc},
				EtcDetails: wire.EncodeVarint(0),
			},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 900000000000,
				Target: types.TxOutToKey{Key: types.PublicKey{0x02}},
			},
		},
		Extra:      wire.EncodeVarint(0),
		Attachment: wire.EncodeVarint(0),
	}

	// Wire-encode the regular tx to get its blob and hash.
	var txBuf bytes.Buffer
	txEnc := wire.NewEncoder(&txBuf)
	wire.EncodeTransaction(txEnc, &regularTx)
	regularTxBlob := hex.EncodeToString(txBuf.Bytes())
	regularTxHash := wire.TransactionHash(&regularTx)

	// --- Build block 1 ---
	minerTx1 := testCoinbaseTx(1)
	block1 := types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Nonce:        42,
			PrevID:       genesisHash,
			Timestamp:    1770897720,
		},
		MinerTx:  minerTx1,
		TxHashes: []types.Hash{regularTxHash},
	}

	var blk1Buf bytes.Buffer
	blk1Enc := wire.NewEncoder(&blk1Buf)
	wire.EncodeBlock(blk1Enc, &block1)
	block1Blob := hex.EncodeToString(blk1Buf.Bytes())
	block1Hash := wire.BlockHash(&block1)

	// Override genesis hash for this test.
	orig := GenesisHash
	GenesisHash = genesisHash.String()
	t.Cleanup(func() { GenesisHash = orig })

	// Track which batch the server has served (to handle the sync loop).
	callCount := 0

	// Mock RPC server returning 2 blocks.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/getheight" {
			json.NewEncoder(w).Encode(map[string]any{
				"height": 2,
				"status": "OK",
			})
			return
		}

		// JSON-RPC dispatcher.
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		switch req.Method {
		case "get_blocks_details":
			var params struct {
				HeightStart uint64 `json:"height_start"`
				Count       uint64 `json:"count"`
			}
			json.Unmarshal(req.Params, &params)

			var blocks []map[string]any

			// Return blocks based on the requested start height.
			if params.HeightStart == 0 {
				callCount++
				blocks = []map[string]any{
					{
						"height":               uint64(0),
						"timestamp":            uint64(1770897600),
						"base_reward":          uint64(1000000000000),
						"id":                   genesisHash.String(),
						"difficulty":           "1",
						"type":                 uint64(1),
						"blob":                 genesisBlob,
						"transactions_details": []any{},
					},
					{
						"height":      uint64(1),
						"timestamp":   uint64(1770897720),
						"base_reward": uint64(1000000),
						"id":          block1Hash.String(),
						"difficulty":  "100",
						"type":        uint64(1),
						"blob":        block1Blob,
						"transactions_details": []map[string]any{
							{
								"id":   regularTxHash.String(),
								"blob": regularTxBlob,
								"fee":  uint64(100000000000),
							},
						},
					},
				}
			}

			result := map[string]any{
				"blocks": blocks,
				"status": "OK",
			}
			resultBytes, _ := json.Marshal(result)
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      "0",
				"result":  json.RawMessage(resultBytes),
			})
		}
	}))
	defer srv.Close()

	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	client := rpc.NewClient(srv.URL)

	err := c.Sync(client)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// Verify height is 2.
	h, _ := c.Height()
	if h != 2 {
		t.Errorf("height after sync: got %d, want 2", h)
	}

	// Verify block 1 can be retrieved by height.
	gotBlk, gotMeta, err := c.GetBlockByHeight(1)
	if err != nil {
		t.Fatalf("GetBlockByHeight(1): %v", err)
	}
	if gotBlk.MajorVersion != 1 {
		t.Errorf("block 1 major_version: got %d, want 1", gotBlk.MajorVersion)
	}
	if gotMeta.Hash != block1Hash {
		t.Errorf("block 1 hash: got %s, want %s", gotMeta.Hash, block1Hash)
	}
	if gotMeta.CumulativeDiff != 101 { // genesis(1) + block1(100)
		t.Errorf("block 1 cumulative_diff: got %d, want 101", gotMeta.CumulativeDiff)
	}

	// Verify block 1 can be retrieved by hash.
	_, gotMeta2, err := c.GetBlockByHash(block1Hash)
	if err != nil {
		t.Fatalf("GetBlockByHash(block1): %v", err)
	}
	if gotMeta2.Height != 1 {
		t.Errorf("block 1 height from hash lookup: got %d, want 1", gotMeta2.Height)
	}

	// Verify the regular transaction was stored.
	gotTx, gotTxMeta, err := c.GetTransaction(regularTxHash)
	if err != nil {
		t.Fatalf("GetTransaction(regularTx): %v", err)
	}
	if gotTx.Version != 1 {
		t.Errorf("regular tx version: got %d, want 1", gotTx.Version)
	}
	if gotTxMeta.KeeperBlock != 1 {
		t.Errorf("regular tx keeper_block: got %d, want 1", gotTxMeta.KeeperBlock)
	}
	if !c.HasTransaction(regularTxHash) {
		t.Error("HasTransaction(regularTx): got false, want true")
	}

	// Verify the key image was marked as spent.
	ki := types.KeyImage{0xaa, 0xbb, 0xcc}
	spent, err := c.IsSpent(ki)
	if err != nil {
		t.Fatalf("IsSpent: %v", err)
	}
	if !spent {
		t.Error("IsSpent(key_image): got false, want true")
	}

	// Verify output indexing: genesis miner tx output + block 1 miner tx output
	// + regular tx output.
	// Genesis miner tx has 1 output at amount 1000000000000.
	// Regular tx has 1 output at amount 900000000000.
	count, err := c.OutputCount(1000000000000)
	if err != nil {
		t.Fatalf("OutputCount(1000000000000): %v", err)
	}
	if count != 1 { // only genesis miner tx
		t.Errorf("output count for 1000000000000: got %d, want 1", count)
	}

	count, err = c.OutputCount(900000000000)
	if err != nil {
		t.Fatalf("OutputCount(900000000000): %v", err)
	}
	if count != 1 { // regular tx output
		t.Errorf("output count for 900000000000: got %d, want 1", count)
	}

	// Verify top block is block 1.
	_, topMeta, err := c.TopBlock()
	if err != nil {
		t.Fatalf("TopBlock: %v", err)
	}
	if topMeta.Height != 1 {
		t.Errorf("top block height: got %d, want 1", topMeta.Height)
	}
}

func TestSync_Good_AlreadySynced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"height": 0,
			"status": "OK",
		})
	}))
	defer srv.Close()

	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	client := rpc.NewClient(srv.URL)
	err := c.Sync(client)
	if err != nil {
		t.Fatalf("Sync on empty: %v", err)
	}

	h, _ := c.Height()
	if h != 0 {
		t.Errorf("height: got %d, want 0", h)
	}
}

func TestSync_Bad_GetHeightError(t *testing.T) {
	// Server that returns HTTP 500 on /getheight.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	client := rpc.NewClient(srv.URL)
	err := c.Sync(client)
	if err == nil {
		t.Fatal("Sync: expected error from bad getheight, got nil")
	}
}

func TestSync_Bad_FetchBlocksError(t *testing.T) {
	// Server that succeeds on /getheight but fails on JSON-RPC.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/getheight" {
			json.NewEncoder(w).Encode(map[string]any{
				"height": 1,
				"status": "OK",
			})
			return
		}

		// Return a JSON-RPC error for get_blocks_details.
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      "0",
			"error": map[string]any{
				"code":    -1,
				"message": "internal error",
			},
		})
	}))
	defer srv.Close()

	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	client := rpc.NewClient(srv.URL)
	err := c.Sync(client)
	if err == nil {
		t.Fatal("Sync: expected error from bad get_blocks_details, got nil")
	}
}

func TestSync_Bad_GenesisHashMismatch(t *testing.T) {
	genesisBlob, genesisHash := makeGenesisBlockBlob()

	// Deliberately do NOT override GenesisHash, so it mismatches.
	// The default GenesisHash is a different value.

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/getheight" {
			json.NewEncoder(w).Encode(map[string]any{
				"height": 1,
				"status": "OK",
			})
			return
		}

		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		switch req.Method {
		case "get_blocks_details":
			result := map[string]any{
				"blocks": []map[string]any{{
					"height":               uint64(0),
					"timestamp":            uint64(1770897600),
					"base_reward":          uint64(1000000000000),
					"id":                   genesisHash.String(),
					"difficulty":           "1",
					"type":                 uint64(1),
					"blob":                 genesisBlob,
					"transactions_details": []any{},
				}},
				"status": "OK",
			}
			resultBytes, _ := json.Marshal(result)
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      "0",
				"result":  json.RawMessage(resultBytes),
			})
		}
	}))
	defer srv.Close()

	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	client := rpc.NewClient(srv.URL)
	err := c.Sync(client)
	if err == nil {
		t.Fatal("Sync: expected genesis hash mismatch error, got nil")
	}
}

func TestSync_Bad_BlockHashMismatch(t *testing.T) {
	genesisBlob, genesisHash := makeGenesisBlockBlob()

	// Set genesis hash to a value that differs from the actual computed hash.
	wrongHash := "0000000000000000000000000000000000000000000000000000000000000001"
	orig := GenesisHash
	GenesisHash = wrongHash
	t.Cleanup(func() { GenesisHash = orig })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/getheight" {
			json.NewEncoder(w).Encode(map[string]any{
				"height": 1,
				"status": "OK",
			})
			return
		}

		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		switch req.Method {
		case "get_blocks_details":
			result := map[string]any{
				"blocks": []map[string]any{{
					"height":               uint64(0),
					"timestamp":            uint64(1770897600),
					"base_reward":          uint64(0),
					"id":                   wrongHash, // wrong: doesn't match computed hash
					"difficulty":           "1",
					"type":                 uint64(1),
					"blob":                 genesisBlob,
					"transactions_details": []any{},
				}},
				"status": "OK",
			}
			resultBytes, _ := json.Marshal(result)
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      "0",
				"result":  json.RawMessage(resultBytes),
			})
		}
	}))
	defer srv.Close()

	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	client := rpc.NewClient(srv.URL)
	err := c.Sync(client)
	if err == nil {
		t.Fatal("Sync: expected block hash mismatch error, got nil")
	}

	// Verify the real genesis hash is unaffected.
	_ = genesisHash
}

func TestSync_Bad_InvalidRegularTxBlob(t *testing.T) {
	genesisBlob, genesisHash := makeGenesisBlockBlob()

	// Build block 1 with a tx hash but the RPC will return a bad tx blob.
	fakeTxHash := types.Hash{0xde, 0xad}
	minerTx1 := testCoinbaseTx(1)
	block1 := types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Nonce:        42,
			PrevID:       genesisHash,
			Timestamp:    1770897720,
		},
		MinerTx:  minerTx1,
		TxHashes: []types.Hash{fakeTxHash},
	}

	var blk1Buf bytes.Buffer
	blk1Enc := wire.NewEncoder(&blk1Buf)
	wire.EncodeBlock(blk1Enc, &block1)
	block1Blob := hex.EncodeToString(blk1Buf.Bytes())
	block1Hash := wire.BlockHash(&block1)

	orig := GenesisHash
	GenesisHash = genesisHash.String()
	t.Cleanup(func() { GenesisHash = orig })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/getheight" {
			json.NewEncoder(w).Encode(map[string]any{
				"height": 2,
				"status": "OK",
			})
			return
		}

		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		switch req.Method {
		case "get_blocks_details":
			result := map[string]any{
				"blocks": []map[string]any{
					{
						"height":               uint64(0),
						"timestamp":            uint64(1770897600),
						"base_reward":          uint64(1000000000000),
						"id":                   genesisHash.String(),
						"difficulty":           "1",
						"type":                 uint64(1),
						"blob":                 genesisBlob,
						"transactions_details": []any{},
					},
					{
						"height":      uint64(1),
						"timestamp":   uint64(1770897720),
						"base_reward": uint64(1000000),
						"id":          block1Hash.String(),
						"difficulty":  "100",
						"type":        uint64(1),
						"blob":        block1Blob,
						"transactions_details": []map[string]any{
							{
								"id":   fakeTxHash.String(),
								"blob": "badc0de", // invalid: odd length hex
								"fee":  uint64(0),
							},
						},
					},
				},
				"status": "OK",
			}
			resultBytes, _ := json.Marshal(result)
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      "0",
				"result":  json.RawMessage(resultBytes),
			})
		}
	}))
	defer srv.Close()

	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	client := rpc.NewClient(srv.URL)
	err := c.Sync(client)
	if err == nil {
		t.Fatal("Sync: expected error from invalid tx blob, got nil")
	}
}

func TestSync_Bad_InvalidBlockBlob(t *testing.T) {
	// Override genesis hash to a value that matches a fake block ID.
	fakeHash := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	orig := GenesisHash
	GenesisHash = fakeHash
	t.Cleanup(func() { GenesisHash = orig })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/getheight" {
			json.NewEncoder(w).Encode(map[string]any{
				"height": 1,
				"status": "OK",
			})
			return
		}

		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		switch req.Method {
		case "get_blocks_details":
			result := map[string]any{
				"blocks": []map[string]any{{
					"height":               uint64(0),
					"timestamp":            uint64(1770897600),
					"base_reward":          uint64(0),
					"id":                   fakeHash,
					"difficulty":           "1",
					"type":                 uint64(1),
					"blob":                 "deadbeef", // invalid wire data
					"transactions_details": []any{},
				}},
				"status": "OK",
			}
			resultBytes, _ := json.Marshal(result)
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      "0",
				"result":  json.RawMessage(resultBytes),
			})
		}
	}))
	defer srv.Close()

	s, _ := store.New(":memory:")
	defer s.Close()
	c := New(s)

	client := rpc.NewClient(srv.URL)
	err := c.Sync(client)
	if err == nil {
		t.Fatal("Sync: expected error from invalid block blob, got nil")
	}
}
