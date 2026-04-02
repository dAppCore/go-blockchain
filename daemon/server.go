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
