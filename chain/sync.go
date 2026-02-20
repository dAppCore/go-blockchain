// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"

	"forge.lthn.ai/core/go-blockchain/rpc"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

const syncBatchSize = 10

// GenesisHash is the expected genesis block hash.
var GenesisHash = "cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963"

// Sync fetches blocks from the daemon and stores them locally.
// It is a blocking function — the caller controls retry and scheduling.
func (c *Chain) Sync(client *rpc.Client) error {
	localHeight, err := c.Height()
	if err != nil {
		return fmt.Errorf("sync: get local height: %w", err)
	}

	remoteHeight, err := client.GetHeight()
	if err != nil {
		return fmt.Errorf("sync: get remote height: %w", err)
	}

	for localHeight < remoteHeight {
		remaining := remoteHeight - localHeight
		batch := uint64(syncBatchSize)
		if remaining < batch {
			batch = remaining
		}

		blocks, err := client.GetBlocksDetails(localHeight, batch)
		if err != nil {
			return fmt.Errorf("sync: fetch blocks at %d: %w", localHeight, err)
		}

		for _, bd := range blocks {
			if err := c.processBlock(bd); err != nil {
				return fmt.Errorf("sync: process block %d: %w", bd.Height, err)
			}
		}

		localHeight, err = c.Height()
		if err != nil {
			return fmt.Errorf("sync: get height after batch: %w", err)
		}
	}

	return nil
}

func (c *Chain) processBlock(bd rpc.BlockDetails) error {
	// Decode block blob.
	blockBlob, err := hex.DecodeString(bd.Blob)
	if err != nil {
		return fmt.Errorf("decode block hex: %w", err)
	}
	dec := wire.NewDecoder(bytes.NewReader(blockBlob))
	blk := wire.DecodeBlock(dec)
	if err := dec.Err(); err != nil {
		return fmt.Errorf("decode block wire: %w", err)
	}

	// Compute and verify block hash.
	computedHash := wire.BlockHash(&blk)
	blockHash, err := types.HashFromHex(bd.ID)
	if err != nil {
		return fmt.Errorf("parse block hash: %w", err)
	}
	if computedHash != blockHash {
		return fmt.Errorf("block hash mismatch: computed %s, daemon says %s",
			computedHash, blockHash)
	}

	// Genesis chain identity check.
	if bd.Height == 0 {
		if bd.ID != GenesisHash {
			return fmt.Errorf("genesis hash %s does not match expected %s",
				bd.ID, GenesisHash)
		}
	}

	// Validate header.
	if err := c.ValidateHeader(&blk, bd.Height); err != nil {
		return err
	}

	// Parse difficulty from string.
	diff, _ := strconv.ParseUint(bd.Difficulty, 10, 64)

	// Calculate cumulative difficulty.
	var cumulDiff uint64
	if bd.Height > 0 {
		_, prevMeta, err := c.TopBlock()
		if err != nil {
			return fmt.Errorf("get prev block meta: %w", err)
		}
		cumulDiff = prevMeta.CumulativeDiff + diff
	} else {
		cumulDiff = diff
	}

	// Store miner transaction.
	minerTxHash := wire.TransactionHash(&blk.MinerTx)
	minerGindexes, err := c.indexOutputs(minerTxHash, &blk.MinerTx)
	if err != nil {
		return fmt.Errorf("index miner tx outputs: %w", err)
	}
	if err := c.PutTransaction(minerTxHash, &blk.MinerTx, &TxMeta{
		KeeperBlock:         bd.Height,
		GlobalOutputIndexes: minerGindexes,
	}); err != nil {
		return fmt.Errorf("store miner tx: %w", err)
	}

	// Process regular transactions.
	for _, txInfo := range bd.Transactions {
		txBlob, err := hex.DecodeString(txInfo.Blob)
		if err != nil {
			return fmt.Errorf("decode tx hex %s: %w", txInfo.ID, err)
		}
		txDec := wire.NewDecoder(bytes.NewReader(txBlob))
		tx := wire.DecodeTransaction(txDec)
		if err := txDec.Err(); err != nil {
			return fmt.Errorf("decode tx wire %s: %w", txInfo.ID, err)
		}

		txHash, err := types.HashFromHex(txInfo.ID)
		if err != nil {
			return fmt.Errorf("parse tx hash: %w", err)
		}

		// Index outputs.
		gindexes, err := c.indexOutputs(txHash, &tx)
		if err != nil {
			return fmt.Errorf("index tx outputs %s: %w", txInfo.ID, err)
		}

		// Mark key images as spent.
		for _, vin := range tx.Vin {
			if toKey, ok := vin.(types.TxInputToKey); ok {
				if err := c.MarkSpent(toKey.KeyImage, bd.Height); err != nil {
					return fmt.Errorf("mark spent %s: %w", toKey.KeyImage, err)
				}
			}
		}

		// Store transaction.
		if err := c.PutTransaction(txHash, &tx, &TxMeta{
			KeeperBlock:         bd.Height,
			GlobalOutputIndexes: gindexes,
		}); err != nil {
			return fmt.Errorf("store tx %s: %w", txInfo.ID, err)
		}
	}

	// Store block.
	meta := &BlockMeta{
		Hash:           blockHash,
		Height:         bd.Height,
		Timestamp:      bd.Timestamp,
		Difficulty:     diff,
		CumulativeDiff: cumulDiff,
		GeneratedCoins: bd.BaseReward,
	}
	return c.PutBlock(&blk, meta)
}

// indexOutputs adds each output of a transaction to the global output index.
func (c *Chain) indexOutputs(txHash types.Hash, tx *types.Transaction) ([]uint64, error) {
	gindexes := make([]uint64, len(tx.Vout))
	for i, out := range tx.Vout {
		var amount uint64
		switch o := out.(type) {
		case types.TxOutputBare:
			amount = o.Amount
		case types.TxOutputZarcanum:
			amount = 0 // hidden amount
		default:
			continue
		}
		gidx, err := c.PutOutput(amount, txHash, uint32(i))
		if err != nil {
			return nil, err
		}
		gindexes[i] = gidx
	}
	return gindexes, nil
}
