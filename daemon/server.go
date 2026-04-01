// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

// Package daemon provides a JSON-RPC server backed by the Go chain.
package daemon

import (
	"encoding/json"
	"io"
	"net/http"

	"dappco.re/go/core"

	"dappco.re/go/core/blockchain/chain"
	"dappco.re/go/core/blockchain/config"
)

// Server serves the Lethean daemon JSON-RPC API backed by a Go chain.
//
//	server := daemon.NewServer(myChain, myConfig)
//	http.ListenAndServe(":46941", server)
type Server struct {
	chain  *chain.Chain
	config *config.ChainConfig
	mux    *http.ServeMux
}

// NewServer creates a JSON-RPC server for the given chain.
//
//	server := daemon.NewServer(c, cfg)
func NewServer(c *chain.Chain, cfg *config.ChainConfig) *Server {
	s := &Server{chain: c, config: cfg, mux: http.NewServeMux()}
	s.mux.HandleFunc("/json_rpc", s.handleJSONRPC)
	s.mux.HandleFunc("/getheight", s.handleGetHeight)
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	s.mux.ServeHTTP(w, r)
}

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcErr         `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, nil, -32700, "parse error")
		return
	}

	var req jsonRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, nil, -32700, "parse error")
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch req.Method {
	case "getinfo":
		s.rpcGetInfo(w, req)
	case "getheight":
		s.rpcGetHeight(w, req)
	case "getblockheaderbyheight":
		s.rpcGetBlockHeaderByHeight(w, req)
	case "getlastblockheader":
		s.rpcGetLastBlockHeader(w, req)
	case "get_all_alias_details":
		s.rpcGetAllAliasDetails(w, req)
	case "get_alias_details":
		s.rpcGetAliasDetails(w, req)
	case "getblockcount":
		s.rpcGetBlockCount(w, req)
	case "get_asset_info":
		s.rpcGetAssetInfo(w, req)
	default:
		writeError(w, req.ID, -32601, core.Sprintf("method %s not found", req.Method))
	}
}

func (s *Server) handleGetHeight(w http.ResponseWriter, r *http.Request) {
	height, _ := s.chain.Height()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"height": height,
		"status": "OK",
	})
}

func (s *Server) rpcGetInfo(w http.ResponseWriter, req jsonRPCRequest) {
	height, _ := s.chain.Height()
	_, meta, _ := s.chain.TopBlock()

	aliases := s.chain.GetAllAliases()

	result := map[string]interface{}{
		"height":               height,
		"difficulty":           meta.Difficulty,
		"alias_count":          len(aliases),
		"tx_pool_size":         0,
		"daemon_network_state": 2,
		"status":               "OK",
		"pos_allowed":          height > 0,
		"is_hardfok_active":    buildHardforkArray(height, s.config),
	}

	writeResult(w, req.ID, result)
}

func (s *Server) rpcGetHeight(w http.ResponseWriter, req jsonRPCRequest) {
	height, _ := s.chain.Height()
	writeResult(w, req.ID, map[string]interface{}{
		"height": height,
		"status": "OK",
	})
}

func (s *Server) rpcGetBlockHeaderByHeight(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		Height uint64 `json:"height"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	blk, meta, err := s.chain.GetBlockByHeight(params.Height)
	if err != nil {
		writeError(w, req.ID, -1, core.Sprintf("block not found at height %d", params.Height))
		return
	}

	header := map[string]interface{}{
		"hash":          meta.Hash.String(),
		"height":        meta.Height,
		"timestamp":     blk.Timestamp,
		"difficulty":    core.Sprintf("%d", meta.Difficulty),
		"major_version": blk.MajorVersion,
		"minor_version": blk.MinorVersion,
		"nonce":         blk.Nonce,
		"prev_hash":     blk.PrevID.String(),
	}

	writeResult(w, req.ID, map[string]interface{}{
		"block_header": header,
		"status":       "OK",
	})
}

func (s *Server) rpcGetLastBlockHeader(w http.ResponseWriter, req jsonRPCRequest) {
	blk, meta, err := s.chain.TopBlock()
	if err != nil {
		writeError(w, req.ID, -1, "no blocks")
		return
	}

	header := map[string]interface{}{
		"hash":          meta.Hash.String(),
		"height":        meta.Height,
		"timestamp":     blk.Timestamp,
		"difficulty":    core.Sprintf("%d", meta.Difficulty),
		"major_version": blk.MajorVersion,
	}

	writeResult(w, req.ID, map[string]interface{}{
		"block_header": header,
		"status":       "OK",
	})
}

func (s *Server) rpcGetAllAliasDetails(w http.ResponseWriter, req jsonRPCRequest) {
	aliases := s.chain.GetAllAliases()
	result := make([]map[string]string, len(aliases))
	for i, a := range aliases {
		result[i] = map[string]string{
			"alias":   a.Name,
			"address": a.Address,
			"comment": a.Comment,
		}
	}
	writeResult(w, req.ID, map[string]interface{}{
		"aliases": result,
		"status":  "OK",
	})
}

func buildHardforkArray(height uint64, cfg *config.ChainConfig) []bool {
	var forks []config.HardFork
	if cfg.IsTestnet {
		forks = config.TestnetForks
	} else {
		forks = config.MainnetForks
	}
	result := make([]bool, 7)
	for _, f := range forks {
		if f.Version < 7 {
			result[f.Version] = height >= f.Height
		}
	}
	return result
}

func writeResult(w http.ResponseWriter, id json.RawMessage, result interface{}) {
	json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func writeError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcErr{Code: code, Message: message},
	})
}

func (s *Server) rpcGetAliasDetails(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		Alias string `json:"alias"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	alias, err := s.chain.GetAlias(params.Alias)
	if err != nil {
		writeError(w, req.ID, -1, core.Sprintf("alias %s not found", params.Alias))
		return
	}

	writeResult(w, req.ID, map[string]interface{}{
		"alias_details": map[string]string{
			"alias":   alias.Name,
			"address": alias.Address,
			"comment": alias.Comment,
		},
		"status": "OK",
	})
}

func (s *Server) rpcGetAliasCount(w http.ResponseWriter, req jsonRPCRequest) {
	aliases := s.chain.GetAllAliases()
	writeResult(w, req.ID, map[string]interface{}{
		"count":  len(aliases),
		"status": "OK",
	})
}

func (s *Server) rpcGetBlockCount(w http.ResponseWriter, req jsonRPCRequest) {
	height, _ := s.chain.Height()
	writeResult(w, req.ID, map[string]interface{}{
		"count":  height,
		"status": "OK",
	})
}

func (s *Server) rpcGetAssetInfo(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		AssetID string `json:"asset_id"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	// For the native LTHN asset, return hardcoded descriptor
	if params.AssetID == "LTHN" || params.AssetID == "d6329b5b1f7c0805b5c345f4957554002a2f557845f64d7645dae0e051a6498a" {
		writeResult(w, req.ID, map[string]interface{}{
			"asset_descriptor": map[string]interface{}{
				"ticker":           "LTHN",
				"full_name":        "Lethean",
				"total_max_supply": 0,
				"current_supply":   0,
				"decimal_point":    12,
				"hidden_supply":    false,
			},
			"asset_id": "d6329b5b1f7c0805b5c345f4957554002a2f557845f64d7645dae0e051a6498a",
			"status":   "OK",
		})
		return
	}

	// For other assets, return not found (until asset index is built)
	writeError(w, req.ID, -1, core.Sprintf("asset %s not found", params.AssetID))
}
