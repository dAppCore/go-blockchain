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
