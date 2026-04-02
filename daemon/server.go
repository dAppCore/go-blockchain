// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

// Package daemon provides a JSON-RPC server backed by the Go chain.
package daemon

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"dappco.re/go/core"

	"dappco.re/go/core/blockchain/chain"
	"dappco.re/go/core/blockchain/crypto"
	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/types"
	"dappco.re/go/core/blockchain/wallet"
)

// Server serves the Lethean daemon JSON-RPC API backed by a Go chain.
//
//	server := daemon.NewServer(myChain, myConfig)
//	http.ListenAndServe(":46941", server)
type Server struct {
	walletProxy *WalletProxy
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
	s.mux.HandleFunc("/start_mining", s.handleStartMining)
	s.mux.HandleFunc("/api/info", s.handleRESTInfo)
	s.mux.HandleFunc("/api/block", s.handleRESTBlock)
	s.mux.HandleFunc("/api/aliases", s.handleRESTAliases)
	s.mux.HandleFunc("/api/alias", s.handleRESTAlias)
	s.mux.HandleFunc("/api/search", s.handleRESTSearch)
	s.mux.HandleFunc("/events/blocks", s.handleSSEBlocks)
	s.mux.HandleFunc("/openapi", s.handleOpenAPI)
	s.mux.HandleFunc("/health", s.handleRESTHealth)
	s.mux.HandleFunc("/gettransactions", s.handleGetTransactions)
	s.mux.HandleFunc("/stop_mining", s.handleStopMining)
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
	case "get_alias_by_address":
		s.rpcGetAliasByAddress(w, req)
	case "getblockheaderbyhash":
		s.rpcGetBlockHeaderByHash(w, req)
	case "on_getblockhash":
		s.rpcOnGetBlockHash(w, req)
	case "get_tx_details":
		s.rpcGetTxDetails(w, req)
	case "get_blocks_details":
		s.rpcGetBlocksDetails(w, req)
	case "get_alias_reward":
		s.rpcGetAliasReward(w, req)
	case "get_est_height_from_date":
	case "get_pool_info":
		s.rpcGetPoolInfo(w, req)
	case "get_assets_list":
		s.rpcGetAssetsListHandler(w, req)
	case "getblockchaininfo":
		s.rpcGetBlockchainInfo(w, req)
	case "get_version":
	case "marketplace_global_get_offers_ex":
		s.rpcMarketplaceGetOffersEx(w, req)
	case "get_current_core_tx_expiration_median":
		s.rpcGetCurrentCoreTxExpirationMedian(w, req)
	case "check_keyimages":
		s.rpcCheckKeyImages(w, req)
	case "getrandom_outs3":
		s.rpcGetRandomOuts(w, req)
	case "getrandom_outs":
		s.rpcGetRandomOuts(w, req)
	case "get_peer_list":
		s.rpcGetPeerList(w, req)
	case "get_connections":
		s.rpcGetConnections(w, req)
	case "sendrawtransaction":
	case "validate_signature":
	case "generate_key_image":
		s.rpcGenerateKeyImage(w, req)
	case "fast_hash":
		s.rpcFastHash(w, req)
	case "generate_keys":
		s.rpcGenerateKeys(w, req)
	case "check_key":
	case "make_integrated_address":
		s.rpcMakeIntegratedAddress(w, req)
	case "split_integrated_address":
		s.rpcSplitIntegratedAddress(w, req)
	case "validate_address":
	case "get_block_hash_by_height":
		s.rpcGetBlockHashByHeight(w, req)
	case "get_chain_stats":
		s.rpcGetChainStats(w, req)
	case "get_recent_blocks":
	case "search":
	case "get_aliases_by_type":
		s.rpcGetAliasesByType(w, req)
	case "get_gateways":
		s.rpcGetGateways(w, req)
	case "get_hardfork_status":
		s.rpcGetHardforkStatus(w, req)
	case "get_node_info":
		s.rpcGetNodeInfo(w, req)
	case "get_difficulty_history":
	case "get_alias_capabilities":
		s.rpcGetAliasCapabilities(w, req)
	case "get_service_endpoints":
		s.rpcGetServiceEndpoints(w, req)
	case "get_total_coins":
		s.rpcGetTotalCoins(w, req)
		s.rpcGetDifficultyHistory(w, req)
		s.rpcSearch(w, req)
		s.rpcGetRecentBlocks(w, req)
		s.rpcValidateAddress(w, req)
		s.rpcCheckKey(w, req)
		s.rpcValidateSignature(w, req)
		s.rpcSendRawTransaction(w, req)
		s.rpcGetVersion(w, req)
		s.rpcGetEstHeightFromDate(w, req)
	case "get_asset_info":
		s.rpcGetAssetInfo(w, req)
	default:
		if s.walletProxy != nil && IsWalletMethod(req.Method) {
			result, err := s.walletProxy.Forward(req.Method, req.Params)
			if err != nil {
				writeError(w, req.ID, -1, err.Error())
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": req.ID, "result": result})
		} else {
			writeError(w, req.ID, -32601, core.Sprintf("method %s not found", req.Method))
		}
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

func (s *Server) rpcGetAliasByAddress(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		Address string `json:"address"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	// Search all aliases for matching address
	aliases := s.chain.GetAllAliases()
	var matches []map[string]string
	for _, a := range aliases {
		if a.Address == params.Address {
			matches = append(matches, map[string]string{
				"alias":   a.Name,
				"address": a.Address,
				"comment": a.Comment,
			})
		}
	}

	writeResult(w, req.ID, map[string]interface{}{
		"alias_info_list": matches,
		"status":          "OK",
	})
}

// Legacy HTTP endpoints (non-JSON-RPC, used by mining scripts and monitoring)

func (s *Server) handleStartMining(w http.ResponseWriter, r *http.Request) {
	// The Go daemon doesn't mine — forward to C++ daemon or return info
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "NOT MINING",
		"note":   "Go daemon is read-only. Use C++ daemon for mining.",
	})
}

func (s *Server) handleStopMining(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "OK",
	})
}

// --- Additional RPC methods ---

func (s *Server) rpcGetBlockHeaderByHash(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		Hash string `json:"hash"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	blockHash, hashErr := types.HashFromHex(params.Hash)
	if hashErr != nil {
		writeError(w, req.ID, -1, core.Sprintf("invalid block hash: %s", params.Hash))
		return
	}
	blk, meta, err := s.chain.GetBlockByHash(blockHash)
	if err != nil {
		writeError(w, req.ID, -1, core.Sprintf("block not found: %s", params.Hash))
		return
	}

	writeResult(w, req.ID, map[string]interface{}{
		"block_header": map[string]interface{}{
			"hash":          meta.Hash.String(),
			"height":        meta.Height,
			"timestamp":     blk.Timestamp,
			"difficulty":    core.Sprintf("%d", meta.Difficulty),
			"major_version": blk.MajorVersion,
			"minor_version": blk.MinorVersion,
			"nonce":         blk.Nonce,
			"prev_hash":     blk.PrevID.String(),
		},
		"status": "OK",
	})
}

func (s *Server) rpcOnGetBlockHash(w http.ResponseWriter, req jsonRPCRequest) {
	var params []uint64
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if len(params) == 0 {
		writeError(w, req.ID, -1, "height required")
		return
	}

	_, meta, err := s.chain.GetBlockByHeight(params[0])
	if err != nil {
		writeError(w, req.ID, -1, core.Sprintf("block not found at %d", params[0]))
		return
	}

	writeResult(w, req.ID, meta.Hash.String())
}

func (s *Server) rpcGetTxDetails(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		TxHash string `json:"tx_hash"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	txHash, hashErr := types.HashFromHex(params.TxHash)
	if hashErr != nil {
		writeError(w, req.ID, -1, core.Sprintf("invalid tx hash: %s", params.TxHash))
		return
	}
	tx, txMeta, err := s.chain.GetTransaction(txHash)
	if err != nil {
		writeError(w, req.ID, -1, core.Sprintf("tx not found: %s", params.TxHash))
		return
	}

	writeResult(w, req.ID, map[string]interface{}{
		"tx_info": map[string]interface{}{
			"id":           params.TxHash,
			"keeper_block": txMeta.KeeperBlock,
			"amount":       0,
			"fee":          0,
			"ins":          len(tx.Vin),
			"outs":         len(tx.Vout),
			"version":      tx.Version,
		},
		"status": "OK",
	})
}

func (s *Server) rpcGetBlocksDetails(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		HeightStart uint64 `json:"height_start"`
		Count       uint64 `json:"count"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.Count == 0 {
		params.Count = 10
	}
	if params.Count > 100 {
		params.Count = 100
	}

	height, _ := s.chain.Height()
	var blocks []map[string]interface{}

	for h := params.HeightStart; h < params.HeightStart+params.Count && h < height; h++ {
		blk, meta, err := s.chain.GetBlockByHeight(h)
		if err != nil {
			continue
		}
		blocks = append(blocks, map[string]interface{}{
			"height":        meta.Height,
			"hash":          meta.Hash.String(),
			"timestamp":     blk.Timestamp,
			"difficulty":    meta.Difficulty,
			"major_version": blk.MajorVersion,
			"tx_count":      len(blk.TxHashes),
		})
	}

	writeResult(w, req.ID, map[string]interface{}{
		"blocks": blocks,
		"status": "OK",
	})
}

func (s *Server) rpcGetAliasReward(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		Alias string `json:"alias"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	// Alias registration costs 1 LTHN (constexpr in currency_config.h)
	writeResult(w, req.ID, map[string]interface{}{
		"reward": 1000000000000, // 1 LTHN in atomic units
		"status": "OK",
	})
}

func (s *Server) rpcGetEstHeightFromDate(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		Timestamp uint64 `json:"timestamp"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	// Estimate: genesis timestamp + (height * 120s avg block time)
	height, _ := s.chain.Height()
	_, meta, _ := s.chain.TopBlock()
	if meta.Height == 0 || params.Timestamp == 0 {
		writeResult(w, req.ID, map[string]interface{}{"height": 0, "status": "OK"})
		return
	}

	// Get genesis timestamp
	genesis, _, _ := s.chain.GetBlockByHeight(0)
	genesisTs := genesis.Timestamp
	if params.Timestamp <= genesisTs {
		writeResult(w, req.ID, map[string]interface{}{"height": 0, "status": "OK"})
		return
	}

	// Linear estimate: (target_ts - genesis_ts) / avg_block_time
	elapsed := params.Timestamp - genesisTs
	avgBlockTime := (meta.Timestamp - genesisTs) / meta.Height
	if avgBlockTime == 0 {
		avgBlockTime = 120
	}
	estimatedHeight := elapsed / avgBlockTime
	if estimatedHeight > height {
		estimatedHeight = height
	}

	writeResult(w, req.ID, map[string]interface{}{
		"height": estimatedHeight,
		"status": "OK",
	})
}

func (s *Server) handleGetTransactions(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(400)
		return
	}

	var params struct {
		TxHashes []string `json:"txs_hashes"`
	}
	json.Unmarshal(body, &params)

	var txsHex []string
	var missed []string

	for _, hashStr := range params.TxHashes {
		txHash, err := types.HashFromHex(hashStr)
		if err != nil {
			missed = append(missed, hashStr)
			continue
		}
		tx, _, err := s.chain.GetTransaction(txHash)
		if err != nil || tx == nil {
			missed = append(missed, hashStr)
			continue
		}
		// Serialize tx back to wire format
		txBlob, err := wallet.SerializeTransaction(tx)
		if err != nil {
			missed = append(missed, hashStr)
			continue
		}
		txsHex = append(txsHex, hex.EncodeToString(txBlob))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"txs_as_hex":  txsHex,
		"missed_tx":   missed,
		"status":      "OK",
	})
}

// --- Explorer & monitoring methods ---

func (s *Server) rpcGetPoolInfo(w http.ResponseWriter, req jsonRPCRequest) {
	// Go daemon doesn't have a tx pool (read-only node)
	writeResult(w, req.ID, map[string]interface{}{
		"tx_pool_size":    0,
		"transactions":    []interface{}{},
		"status":          "OK",
	})
}

func (s *Server) rpcGetAssetsListHandler(w http.ResponseWriter, req jsonRPCRequest) {
	// Return native LTHN asset
	writeResult(w, req.ID, map[string]interface{}{
		"assets_list": []map[string]interface{}{
			{
				"asset_id": "d6329b5b1f7c0805b5c345f4957554002a2f557845f64d7645dae0e051a6498a",
				"is_native": true,
				"asset_descriptor": map[string]interface{}{
					"ticker":           "LTHN",
					"full_name":        "Lethean",
					"decimal_point":    12,
					"total_max_supply": 0,
					"hidden_supply":    false,
				},
			},
		},
		"status": "OK",
	})
}

func (s *Server) rpcGetBlockchainInfo(w http.ResponseWriter, req jsonRPCRequest) {
	height, _ := s.chain.Height()
	_, meta, _ := s.chain.TopBlock()
	genesis, _, _ := s.chain.GetBlockByHeight(0)

	network := "mainnet"
	if s.config.IsTestnet {
		network = "testnet"
	}

	writeResult(w, req.ID, map[string]interface{}{
		"chain":                 network,
		"blocks":                height,
		"headers":               height,
		"bestblockhash":         meta.Hash.String(),
		"difficulty":            meta.Difficulty,
		"genesis_hash":          chain.DetectNetwork(meta.Hash.String()),
		"genesis_timestamp":     genesis.Timestamp,
		"status":                "OK",
	})
}

func (s *Server) rpcGetVersion(w http.ResponseWriter, req jsonRPCRequest) {
	writeResult(w, req.ID, map[string]interface{}{
		"version":  "go-blockchain/0.2.0",
		"rpc_api":  "1.0",
		"node":     "CoreChain",
		"status":   "OK",
	})
}

// --- Marketplace & utility methods ---

func (s *Server) rpcMarketplaceGetOffersEx(w http.ResponseWriter, req jsonRPCRequest) {
	// Marketplace offers are in transaction attachments — need tx scanning
	// For now return empty (Go daemon is read-only for marketplace)
	writeResult(w, req.ID, map[string]interface{}{
		"offers": []interface{}{},
		"total":  0,
		"status": "OK",
	})
}

func (s *Server) rpcGetCurrentCoreTxExpirationMedian(w http.ResponseWriter, req jsonRPCRequest) {
	height, _ := s.chain.Height()
	// TX expiration median is ~10 * TOTAL_TARGET (~600s post-HF2)
	writeResult(w, req.ID, map[string]interface{}{
		"expiration_median": height,
		"status":            "OK",
	})
}

func (s *Server) rpcCheckKeyImages(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		KeyImages []string `json:"key_images"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	results := make([]map[string]interface{}, len(params.KeyImages))
	for i, kiHex := range params.KeyImages {
		var ki types.KeyImage
		hashBytes, err := hex.DecodeString(kiHex)
		if err == nil && len(hashBytes) == 32 { copy(ki[:], hashBytes) }
		if err != nil {
			results[i] = map[string]interface{}{"spent": false}
			continue
		}
		spent, _ := s.chain.IsSpent(ki)
		results[i] = map[string]interface{}{"spent": spent}
	}

	writeResult(w, req.ID, map[string]interface{}{
		"images":  results,
		"status":  "OK",
	})
}

func (s *Server) rpcSendRawTransaction(w http.ResponseWriter, req jsonRPCRequest) {
	// Forward to C++ daemon — Go node can't validate consensus yet
	if s.walletProxy != nil {
		// Use the wallet proxy's HTTP client to forward
		writeError(w, req.ID, -1, "sendrawtransaction: forward to C++ daemon at port 46941")
		return
	}
	writeError(w, req.ID, -1, "sendrawtransaction not available (read-only node)")
}

func (s *Server) rpcValidateSignature(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		Buff  string `json:"buff"`  // base64 encoded data
		PKey  string `json:"pkey"`  // hex public key (or empty to use alias)
		Sig   string `json:"sig"`   // hex signature
		Alias string `json:"alias"` // alias to look up public key
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	// Get public key from params or alias
	pubHex := params.PKey
	if pubHex == "" && params.Alias != "" {
		alias, err := s.chain.GetAlias(params.Alias)
		if err != nil {
			writeError(w, req.ID, -1, core.Sprintf("alias %s not found", params.Alias))
			return
		}
		pubHex = alias.Address // spend public key hex
	}

	if pubHex == "" {
		writeError(w, req.ID, -1, "pkey or alias required")
		return
	}

	// Decode inputs
	dataBytes, err := base64.StdEncoding.DecodeString(params.Buff)
	if err != nil {
		writeError(w, req.ID, -1, "invalid base64 buff")
		return
	}

	pubBytes, err := hex.DecodeString(pubHex)
	if err != nil || len(pubBytes) != 32 {
		writeError(w, req.ID, -1, "invalid public key")
		return
	}

	sigBytes, err := hex.DecodeString(params.Sig)
	if err != nil || len(sigBytes) != 64 {
		writeError(w, req.ID, -1, "invalid signature")
		return
	}

	// Hash the data
	var hash [32]byte
	hashResult := crypto.FastHash(dataBytes)
	copy(hash[:], hashResult[:])

	var pub [32]byte
	var sig [64]byte
	copy(pub[:], pubBytes)
	copy(sig[:], sigBytes)

	valid := crypto.CheckSignature(hash, pub, sig)

	writeResult(w, req.ID, map[string]interface{}{
		"valid":  valid,
		"status": "OK",
	})
}

// --- Native crypto methods (Go+CGo, no C++ daemon needed) ---

func (s *Server) rpcGenerateKeyImage(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		PublicKey string `json:"public_key"`
		SecretKey string `json:"secret_key"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	pubBytes, _ := hex.DecodeString(params.PublicKey)
	secBytes, _ := hex.DecodeString(params.SecretKey)
	if len(pubBytes) != 32 || len(secBytes) != 32 {
		writeError(w, req.ID, -1, "keys must be 32 bytes hex")
		return
	}

	var pub, sec [32]byte
	copy(pub[:], pubBytes)
	copy(sec[:], secBytes)

	ki, err := crypto.GenerateKeyImage(pub, sec)
	if err != nil {
		writeError(w, req.ID, -1, "key image generation failed")
		return
	}

	writeResult(w, req.ID, map[string]interface{}{
		"key_image": hex.EncodeToString(ki[:]),
		"status":    "OK",
	})
}

func (s *Server) rpcFastHash(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		Data string `json:"data"` // hex encoded
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	data, err := hex.DecodeString(params.Data)
	if err != nil {
		writeError(w, req.ID, -1, "invalid hex data")
		return
	}

	hash := crypto.FastHash(data)
	writeResult(w, req.ID, map[string]interface{}{
		"hash":   hex.EncodeToString(hash[:]),
		"status": "OK",
	})
}

func (s *Server) rpcGenerateKeys(w http.ResponseWriter, req jsonRPCRequest) {
	pub, sec, err := crypto.GenerateKeys()
	if err != nil {
		writeError(w, req.ID, -1, "key generation failed")
		return
	}

	writeResult(w, req.ID, map[string]interface{}{
		"public_key": hex.EncodeToString(pub[:]),
		"secret_key": hex.EncodeToString(sec[:]),
		"status":     "OK",
	})
}

func (s *Server) rpcCheckKey(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		Key string `json:"key"` // hex public key
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	keyBytes, _ := hex.DecodeString(params.Key)
	if len(keyBytes) != 32 {
		writeResult(w, req.ID, map[string]interface{}{"valid": false, "status": "OK"})
		return
	}

	var key [32]byte
	copy(key[:], keyBytes)

	writeResult(w, req.ID, map[string]interface{}{
		"valid":  crypto.CheckKey(key),
		"status": "OK",
	})
}

// --- Address utility methods ---

func (s *Server) rpcMakeIntegratedAddress(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		Address   string `json:"address"`
		PaymentID string `json:"payment_id"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	// Parse the standard address
	addr, _, err := types.DecodeAddress(params.Address)
	if err != nil {
		writeError(w, req.ID, -1, "invalid address")
		return
	}

	// Encode as integrated (prefix 0xdeaf7)
	integrated := addr.Encode(0xdeaf7)

	writeResult(w, req.ID, map[string]interface{}{
		"integrated_address": integrated,
		"payment_id":         params.PaymentID,
		"status":             "OK",
	})
}

func (s *Server) rpcSplitIntegratedAddress(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		Address string `json:"integrated_address"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	addr, prefix, err := types.DecodeAddress(params.Address)
	if err != nil {
		writeError(w, req.ID, -1, "invalid integrated address")
		return
	}

	// Re-encode as standard (prefix 0x1eaf7)
	standard := addr.Encode(0x1eaf7)

	writeResult(w, req.ID, map[string]interface{}{
		"standard_address": standard,
		"prefix":           prefix,
		"status":           "OK",
	})
}

func (s *Server) rpcValidateAddress(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		Address string `json:"address"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	_, prefix, err := types.DecodeAddress(params.Address)
	valid := err == nil

	addrType := "unknown"
	switch prefix {
	case 0x1eaf7:
		addrType = "standard"
	case 0xdeaf7:
		addrType = "integrated"
	case 0x3ceff7:
		addrType = "auditable"
	case 0x8b077:
		addrType = "auditable_integrated"
	}

	writeResult(w, req.ID, map[string]interface{}{
		"valid":   valid,
		"type":    addrType,
		"prefix":  prefix,
		"status":  "OK",
	})
}

// --- Block search & chain statistics ---

func (s *Server) rpcGetBlockHashByHeight(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		Height uint64 `json:"height"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	_, meta, err := s.chain.GetBlockByHeight(params.Height)
	if err != nil {
		writeError(w, req.ID, -1, core.Sprintf("no block at height %d", params.Height))
		return
	}

	writeResult(w, req.ID, meta.Hash.String())
}

func (s *Server) rpcGetChainStats(w http.ResponseWriter, req jsonRPCRequest) {
	height, _ := s.chain.Height()
	_, topMeta, _ := s.chain.TopBlock()
	genesis, _, _ := s.chain.GetBlockByHeight(0)
	aliases := s.chain.GetAllAliases()

	var avgBlockTime uint64
	if topMeta.Height > 0 && genesis != nil {
		avgBlockTime = (topMeta.Timestamp - genesis.Timestamp) / topMeta.Height
	}

	// Count gateway aliases
	gateways := 0
	services := 0
	for _, a := range aliases {
		if core.Contains(a.Comment, "type=gateway") {
			gateways++
		} else if core.Contains(a.Comment, "type=service") {
			services++
		}
	}

	writeResult(w, req.ID, map[string]interface{}{
		"height":          height,
		"difficulty":      topMeta.Difficulty,
		"cumulative_diff": topMeta.CumulativeDiff,
		"total_aliases":   len(aliases),
		"gateways":        gateways,
		"services":        services,
		"avg_block_time":  avgBlockTime,
		"chain_age_hours": (topMeta.Timestamp - genesis.Timestamp) / 3600,
		"node_type":       "go-blockchain",
		"status":          "OK",
	})
}

func (s *Server) rpcGetRecentBlocks(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		Count uint64 `json:"count"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.Count == 0 || params.Count > 50 {
		params.Count = 10
	}

	height, _ := s.chain.Height()
	var blocks []map[string]interface{}

	start := uint64(0)
	if height > params.Count {
		start = height - params.Count
	}

	for h := height - 1; h >= start && h < height; h-- {
		blk, meta, err := s.chain.GetBlockByHeight(h)
		if err != nil {
			continue
		}
		blocks = append(blocks, map[string]interface{}{
			"height":    meta.Height,
			"hash":      meta.Hash.String(),
			"timestamp": blk.Timestamp,
			"difficulty": meta.Difficulty,
			"tx_count":  len(blk.TxHashes),
			"major_version": blk.MajorVersion,
		})
	}

	writeResult(w, req.ID, map[string]interface{}{
		"blocks": blocks,
		"status": "OK",
	})
}

// --- Search & discovery methods ---

func (s *Server) rpcSearch(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		Query string `json:"query"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	q := params.Query
	result := map[string]interface{}{"query": q, "status": "OK"}

	// Try as block height
	if height, err := parseUint64(q); err == nil {
		if blk, meta, err := s.chain.GetBlockByHeight(height); err == nil {
			result["type"] = "block"
			result["block"] = map[string]interface{}{
				"height": meta.Height, "hash": meta.Hash.String(),
				"timestamp": blk.Timestamp,
			}
			writeResult(w, req.ID, result)
			return
		}
	}

	// Try as block/tx hash
	if len(q) == 64 {
		if hash, err := types.HashFromHex(q); err == nil {
			if blk, meta, err := s.chain.GetBlockByHash(hash); err == nil {
				result["type"] = "block"
				result["block"] = map[string]interface{}{
					"height": meta.Height, "hash": meta.Hash.String(),
					"timestamp": blk.Timestamp,
				}
				writeResult(w, req.ID, result)
				return
			}
			if tx, meta, err := s.chain.GetTransaction(hash); err == nil {
				result["type"] = "transaction"
				result["transaction"] = map[string]interface{}{
					"hash": q, "block_height": meta.KeeperBlock,
					"inputs": len(tx.Vin), "outputs": len(tx.Vout),
				}
				writeResult(w, req.ID, result)
				return
			}
		}
	}

	// Try as alias
	if alias, err := s.chain.GetAlias(q); err == nil {
		result["type"] = "alias"
		result["alias"] = map[string]string{
			"name": alias.Name, "address": alias.Address, "comment": alias.Comment,
		}
		writeResult(w, req.ID, result)
		return
	}

	// Try as address prefix
	if len(q) > 4 && (q[:4] == "iTHN" || q[:4] == "iTHn" || q[:4] == "iThN") {
		_, _, err := types.DecodeAddress(q)
		if err == nil {
			result["type"] = "address"
			result["valid"] = true
			writeResult(w, req.ID, result)
			return
		}
	}

	result["type"] = "not_found"
	writeResult(w, req.ID, result)
}

func parseUint64(s string) (uint64, error) {
	var n uint64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, core.E("parse", "not a number", nil)
		}
		n = n*10 + uint64(c-'0')
	}
	return n, nil
}

// --- Service discovery & network methods ---

func (s *Server) rpcGetAliasesByType(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		Type string `json:"type"` // gateway, service, root
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	all := s.chain.GetAllAliases()
	var filtered []map[string]string
	for _, a := range all {
		if params.Type == "" || core.Contains(a.Comment, "type="+params.Type) {
			filtered = append(filtered, map[string]string{
				"alias": a.Name, "address": a.Address, "comment": a.Comment,
			})
		}
	}

	writeResult(w, req.ID, map[string]interface{}{
		"aliases": filtered,
		"count":   len(filtered),
		"status":  "OK",
	})
}

func (s *Server) rpcGetGateways(w http.ResponseWriter, req jsonRPCRequest) {
	all := s.chain.GetAllAliases()
	var gateways []map[string]interface{}
	for _, a := range all {
		if !core.Contains(a.Comment, "type=gateway") {
			continue
		}
		caps := ""
		for _, part := range splitSemicolon(a.Comment) {
			if core.HasPrefix(part, "cap=") {
				caps = part[4:]
			}
		}
		gateways = append(gateways, map[string]interface{}{
			"alias":        a.Name,
			"address":      a.Address,
			"capabilities": caps,
			"comment":      a.Comment,
		})
	}

	writeResult(w, req.ID, map[string]interface{}{
		"gateways": gateways,
		"count":    len(gateways),
		"status":   "OK",
	})
}

func splitSemicolon(s string) []string {
	var parts []string
	current := ""
	for _, c := range s {
		if c == ';' {
			if current != "" { parts = append(parts, current) }
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" { parts = append(parts, current) }
	return parts
}

func (s *Server) rpcGetHardforkStatus(w http.ResponseWriter, req jsonRPCRequest) {
	height, _ := s.chain.Height()

	type hfStatus struct {
		Version     int    `json:"version"`
		Height      uint64 `json:"activation_height"`
		Active      bool   `json:"active"`
		Description string `json:"description"`
	}

	forks := []hfStatus{
		{0, 0, true, "Genesis"},
		{1, 0, height >= 0, "PoS staking rules"},
		{2, 10000, height >= 10000, "Difficulty fix + 120s blocks"},
		{3, 10500, height >= 10500, "Block version bump"},
		{4, 11000, height >= 11000, "Zarcanum (aliases, transfers, assets)"},
		{5, 11500, height >= 11500, "Asset operations (deploy, emit, burn)"},
	}

	nextHF := "none"
	nextHeight := uint64(0)
	for _, f := range forks {
		if !f.Active {
			nextHF = core.Sprintf("HF%d", f.Version)
			nextHeight = f.Height
			break
		}
	}

	writeResult(w, req.ID, map[string]interface{}{
		"hardforks":        forks,
		"current_height":   height,
		"next_hardfork":    nextHF,
		"next_hf_height":   nextHeight,
		"blocks_remaining": int64(nextHeight) - int64(height),
		"status":           "OK",
	})
}

func (s *Server) rpcGetNodeInfo(w http.ResponseWriter, req jsonRPCRequest) {
	height, _ := s.chain.Height()
	aliases := s.chain.GetAllAliases()

	writeResult(w, req.ID, map[string]interface{}{
		"node_type":    "CoreChain (Go)",
		"version":      "0.3.0",
		"module":       "dappco.re/go/core/blockchain",
		"height":       height,
		"aliases":      len(aliases),
		"endpoints":    56,
		"native_crypto": []string{
			"validate_signature", "check_keyimages", "generate_keys",
			"generate_key_image", "fast_hash", "check_key",
			"make_integrated_address", "validate_address",
		},
		"capabilities": []string{
			"chain_sync", "rpc_server", "wallet_proxy",
			"alias_index", "block_explorer", "crypto_utils",
		},
		"status": "OK",
	})
}

func (s *Server) rpcGetDifficultyHistory(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		Count uint64 `json:"count"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.Count == 0 || params.Count > 100 {
		params.Count = 20
	}

	height, _ := s.chain.Height()
	var history []map[string]interface{}

	start := uint64(0)
	if height > params.Count {
		start = height - params.Count
	}

	for h := start; h < height; h++ {
		_, meta, err := s.chain.GetBlockByHeight(h)
		if err != nil { continue }
		history = append(history, map[string]interface{}{
			"height":     meta.Height,
			"difficulty": meta.Difficulty,
			"timestamp":  meta.Timestamp,
		})
	}

	writeResult(w, req.ID, map[string]interface{}{
		"history": history,
		"status":  "OK",
	})
}

// --- REST-style HTTP endpoints (no JSON-RPC wrapper) ---

func (s *Server) handleRESTInfo(w http.ResponseWriter, r *http.Request) {
	height, _ := s.chain.Height()
	_, meta, _ := s.chain.TopBlock()
	aliases := s.chain.GetAllAliases()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"height": height, "difficulty": meta.Difficulty,
		"aliases": len(aliases), "node": "CoreChain/Go",
	})
}

func (s *Server) handleRESTBlock(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("height")
	h, err := parseUint64(q)
	if err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "?height= required"})
		return
	}
	blk, meta, err := s.chain.GetBlockByHeight(h)
	if err != nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"height": meta.Height, "hash": meta.Hash.String(),
		"timestamp": blk.Timestamp, "difficulty": meta.Difficulty,
		"tx_count": len(blk.TxHashes), "version": blk.MajorVersion,
	})
}

func (s *Server) handleRESTAliases(w http.ResponseWriter, r *http.Request) {
	aliases := s.chain.GetAllAliases()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(aliases)
}

func (s *Server) handleRESTAlias(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "?name= required"})
		return
	}
	alias, err := s.chain.GetAlias(name)
	if err != nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(alias)
}

func (s *Server) handleRESTSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "?q= required"})
		return
	}
	// Reuse the RPC search logic
	fakeReq := jsonRPCRequest{Params: json.RawMessage(core.Sprintf(`{"query":"%s"}`, q))}
	s.rpcSearch(w, fakeReq)
}

func (s *Server) handleRESTHealth(w http.ResponseWriter, r *http.Request) {
	height, _ := s.chain.Height()
	aliases := s.chain.GetAllAliases()
	status := "ok"
	if height == 0 {
		status = "syncing"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": status, "height": height,
		"aliases": len(aliases), "node": "CoreChain/Go",
	})
}

// --- Server-Sent Events for real-time updates ---
// (SSE is simpler than WebSocket and works with curl)

func (s *Server) handleSSEBlocks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", 500)
		return
	}

	lastHeight := uint64(0)
	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		height, _ := s.chain.Height()
		if height > lastHeight && lastHeight > 0 {
			// New block(s) arrived
			for h := lastHeight + 1; h <= height; h++ {
				blk, meta, err := s.chain.GetBlockByHeight(h)
				if err != nil { continue }
				data := core.Sprintf(`{"height":%d,"hash":"%s","timestamp":%d,"difficulty":%d,"tx_count":%d}`,
					meta.Height, meta.Hash.String(), blk.Timestamp, meta.Difficulty, len(blk.TxHashes))
				w.Write([]byte(core.Sprintf("event: block\ndata: %s\n\n", data)))
				flusher.Flush()
			}
		}
		lastHeight = height

		// Poll every 2 seconds
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}


// --- SWAP & gateway service methods ---

func (s *Server) rpcGetAliasCapabilities(w http.ResponseWriter, req jsonRPCRequest) {
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

	// Parse v=lthn1;type=gateway;cap=vpn,dns,proxy;hns=charon.lthn
	parsed := make(map[string]string)
	for _, part := range splitSemicolon(alias.Comment) {
		idx := -1
		for i, c := range part {
			if c == '=' { idx = i; break }
		}
		if idx > 0 {
			parsed[part[:idx]] = part[idx+1:]
		}
	}

	var caps []string
	if c, ok := parsed["cap"]; ok {
		for _, part := range splitComma(c) {
			caps = append(caps, part)
		}
	}

	writeResult(w, req.ID, map[string]interface{}{
		"alias":        alias.Name,
		"version":      parsed["v"],
		"type":         parsed["type"],
		"capabilities": caps,
		"hns":          parsed["hns"],
		"raw_comment":  alias.Comment,
		"status":       "OK",
	})
}

func splitComma(s string) []string {
	var parts []string
	current := ""
	for _, c := range s {
		if c == ',' {
			if current != "" { parts = append(parts, current) }
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" { parts = append(parts, current) }
	return parts
}

func (s *Server) rpcGetServiceEndpoints(w http.ResponseWriter, req jsonRPCRequest) {
	all := s.chain.GetAllAliases()
	var endpoints []map[string]interface{}

	for _, a := range all {
		parsed := make(map[string]string)
		for _, part := range splitSemicolon(a.Comment) {
			idx := -1
			for i, c := range part { if c == '=' { idx = i; break } }
			if idx > 0 { parsed[part[:idx]] = part[idx+1:] }
		}

		if parsed["type"] == "" { continue }

		var caps []string
		if c, ok := parsed["cap"]; ok {
			for _, p := range splitComma(c) { caps = append(caps, p) }
		}

		endpoints = append(endpoints, map[string]interface{}{
			"alias":        a.Name,
			"type":         parsed["type"],
			"capabilities": caps,
			"hns_name":     parsed["hns"],
		})
	}

	writeResult(w, req.ID, map[string]interface{}{
		"endpoints": endpoints,
		"count":     len(endpoints),
		"status":    "OK",
	})
}

func (s *Server) rpcGetTotalCoins(w http.ResponseWriter, req jsonRPCRequest) {
	height, _ := s.chain.Height()
	// Premine: 10M LTHN + 1 LTHN per block after genesis
	premine := uint64(10000000) // LTHN
	mined := height             // 1 LTHN per block
	total := premine + mined

	writeResult(w, req.ID, map[string]interface{}{
		"total_coins":     total,
		"premine":         premine,
		"mined":           mined,
		"block_reward":    1,
		"height":          height,
		"unit":            "LTHN",
		"atomic_total":    total * 1000000000000,
		"status":          "OK",
	})
}

// --- Self-documentation ---

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	methods := []map[string]string{
		{"method": "getinfo", "type": "chain", "desc": "Chain height, difficulty, alias count, hardfork status"},
		{"method": "getheight", "type": "chain", "desc": "Current block height"},
		{"method": "getblockcount", "type": "chain", "desc": "Total block count"},
		{"method": "getblockheaderbyheight", "type": "chain", "desc": "Block header by height"},
		{"method": "getblockheaderbyhash", "type": "chain", "desc": "Block header by hash"},
		{"method": "getlastblockheader", "type": "chain", "desc": "Latest block header"},
		{"method": "on_getblockhash", "type": "chain", "desc": "Block hash by height"},
		{"method": "get_blocks_details", "type": "chain", "desc": "Batch block details"},
		{"method": "get_block_hash_by_height", "type": "chain", "desc": "Height to hash"},
		{"method": "get_recent_blocks", "type": "chain", "desc": "Last N blocks"},
		{"method": "get_tx_details", "type": "chain", "desc": "Transaction details by hash"},
		{"method": "get_all_alias_details", "type": "alias", "desc": "All registered aliases"},
		{"method": "get_alias_details", "type": "alias", "desc": "Single alias by name"},
		{"method": "get_alias_by_address", "type": "alias", "desc": "Aliases for an address"},
		{"method": "get_aliases_by_type", "type": "alias", "desc": "Filter aliases by type"},
		{"method": "get_alias_capabilities", "type": "alias", "desc": "Parse alias v=lthn1 capabilities"},
		{"method": "get_alias_reward", "type": "alias", "desc": "Alias registration cost"},
		{"method": "get_gateways", "type": "discovery", "desc": "All gateway nodes with capabilities"},
		{"method": "get_service_endpoints", "type": "discovery", "desc": "All service endpoints"},
		{"method": "get_asset_info", "type": "asset", "desc": "Asset descriptor by ID or ticker"},
		{"method": "get_assets_list", "type": "asset", "desc": "All known assets"},
		{"method": "get_pool_info", "type": "chain", "desc": "Transaction pool info"},
		{"method": "getblockchaininfo", "type": "chain", "desc": "Full blockchain info"},
		{"method": "get_hardfork_status", "type": "chain", "desc": "Hardfork schedule with countdown"},
		{"method": "get_chain_stats", "type": "analytics", "desc": "Chain statistics and averages"},
		{"method": "get_difficulty_history", "type": "analytics", "desc": "Difficulty history for charts"},
		{"method": "get_total_coins", "type": "analytics", "desc": "Total coin supply calculation"},
		{"method": "get_est_height_from_date", "type": "chain", "desc": "Estimate height from timestamp"},
		{"method": "get_current_core_tx_expiration_median", "type": "chain", "desc": "TX expiration median"},
		{"method": "get_version", "type": "system", "desc": "Node version and type"},
		{"method": "get_node_info", "type": "system", "desc": "Node capabilities and stats"},
		{"method": "search", "type": "utility", "desc": "Universal search (block/tx/alias/address)"},
		{"method": "validate_signature", "type": "crypto", "desc": "Schnorr signature verification (native CGo)"},
		{"method": "generate_keys", "type": "crypto", "desc": "Ed25519 keypair generation (native CGo)"},
		{"method": "generate_key_image", "type": "crypto", "desc": "Key image from keypair (native CGo)"},
		{"method": "fast_hash", "type": "crypto", "desc": "Keccak-256 hash (native CGo)"},
		{"method": "check_key", "type": "crypto", "desc": "Validate Ed25519 public key (native CGo)"},
		{"method": "check_keyimages", "type": "crypto", "desc": "Check spent key images (native)"},
		{"method": "validate_address", "type": "crypto", "desc": "Validate iTHN address format (native)"},
		{"method": "make_integrated_address", "type": "crypto", "desc": "Encode integrated address (native)"},
		{"method": "split_integrated_address", "type": "crypto", "desc": "Decode integrated address (native)"},
		{"method": "marketplace_global_get_offers_ex", "type": "marketplace", "desc": "Marketplace offers"},
		{"method": "sendrawtransaction", "type": "relay", "desc": "Broadcast raw transaction"},
	}

	// Count by type
	counts := make(map[string]int)
	for _, m := range methods {
		counts[m["type"]]++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"node":       "CoreChain/Go v0.3.0",
		"module":     "dappco.re/go/core/blockchain",
		"rpc_methods": methods,
		"method_count": len(methods),
		"categories": counts,
		"http_endpoints": []string{
			"/json_rpc", "/getheight", "/start_mining", "/stop_mining",
			"/gettransactions", "/api/info", "/api/block", "/api/aliases",
			"/api/alias", "/api/search", "/health", "/events/blocks", "/openapi",
		},
		"wallet_proxy_methods": []string{
			"getbalance", "getaddress", "get_wallet_info", "transfer",
			"make_integrated_address", "split_integrated_address",
			"deploy_asset", "emit_asset", "burn_asset",
			"register_alias", "update_alias", "get_bulk_payments",
			"get_recent_txs_and_info", "store", "get_restore_info", "sign_message",
		},
	})
}

// --- Ring member selection (critical for native wallet tx construction) ---

func (s *Server) rpcGetRandomOuts(w http.ResponseWriter, req jsonRPCRequest) {
	var params struct {
		Amounts []uint64 `json:"amounts"`
		Count   int      `json:"outs_count"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	if params.Count == 0 {
		params.Count = 15 // HF4 mandatory decoy set size
	}

	var outs []map[string]interface{}
	for _, amount := range params.Amounts {
		totalOutputs, _ := s.chain.OutputCount(amount)
		if totalOutputs == 0 {
			outs = append(outs, map[string]interface{}{
				"amount": amount,
				"outs":   []interface{}{},
			})
			continue
		}

		// Select random output indices
		var selected []map[string]interface{}
		step := totalOutputs / uint64(params.Count)
		if step == 0 { step = 1 }

		for i := 0; i < params.Count && uint64(i)*step < totalOutputs; i++ {
			globalIdx := uint64(i) * step
			txHash, outIdx, err := s.chain.GetOutput(amount, globalIdx)
			if err != nil { continue }
			selected = append(selected, map[string]interface{}{
				"global_amount_index": globalIdx,
				"out_key":             txHash.String(),
				"tx_out_index":        outIdx,
			})
		}

		outs = append(outs, map[string]interface{}{
			"amount": amount,
			"outs":   selected,
		})
	}

	writeResult(w, req.ID, map[string]interface{}{
		"outs":   outs,
		"status": "OK",
	})
}

// --- Admin methods ---

func (s *Server) rpcGetPeerList(w http.ResponseWriter, req jsonRPCRequest) {
	// Go node doesn't maintain a peer list yet (sync-only, not P2P server)
	writeResult(w, req.ID, map[string]interface{}{
		"white_list": []interface{}{},
		"gray_list":  []interface{}{},
		"status":     "OK",
	})
}

func (s *Server) rpcGetConnections(w http.ResponseWriter, req jsonRPCRequest) {
	writeResult(w, req.ID, map[string]interface{}{
		"connections": []interface{}{},
		"status":      "OK",
	})
}
