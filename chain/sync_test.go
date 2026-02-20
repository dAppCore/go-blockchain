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
