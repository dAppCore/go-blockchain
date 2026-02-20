// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import "forge.lthn.ai/core/go-blockchain/types"

// BlockMeta holds metadata stored alongside each block.
type BlockMeta struct {
	Hash           types.Hash `json:"hash"`
	Height         uint64     `json:"height"`
	Timestamp      uint64     `json:"timestamp"`
	Difficulty     uint64     `json:"difficulty"`
	CumulativeDiff uint64     `json:"cumulative_diff"`
	GeneratedCoins uint64     `json:"generated_coins"`
}

// TxMeta holds metadata stored alongside each transaction.
type TxMeta struct {
	KeeperBlock         uint64   `json:"keeper_block"`
	GlobalOutputIndexes []uint64 `json:"global_output_indexes"`
}

// outputEntry is the value stored in the outputs index.
type outputEntry struct {
	TxID  string `json:"tx_id"`
	OutNo uint32 `json:"out_no"`
}
