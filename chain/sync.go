// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-blockchain/consensus"
	"forge.lthn.ai/core/go-blockchain/rpc"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

const syncBatchSize = 10

// GenesisHash is the expected genesis block hash.
var GenesisHash = "cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963"

// SyncOptions controls sync behaviour.
type SyncOptions struct {
	// VerifySignatures enables cryptographic signature verification
	// during sync. Default false for fast sync.
	VerifySignatures bool

	// Forks is the hardfork schedule to use for validation.
	Forks []config.HardFork
}

// DefaultSyncOptions returns sync options for fast sync (no signature verification).
func DefaultSyncOptions() SyncOptions {
	return SyncOptions{
		VerifySignatures: false,
		Forks:            config.MainnetForks,
	}
}

// Sync fetches blocks from the daemon and stores them locally.
// It is a blocking function — the caller controls retry and scheduling.
func (c *Chain) Sync(ctx context.Context, client *rpc.Client, opts SyncOptions) error {
	localHeight, err := c.Height()
	if err != nil {
		return fmt.Errorf("sync: get local height: %w", err)
	}

	remoteHeight, err := client.GetHeight()
	if err != nil {
		return fmt.Errorf("sync: get remote height: %w", err)
	}

	for localHeight < remoteHeight {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		remaining := remoteHeight - localHeight
		batch := uint64(syncBatchSize)
		if remaining < batch {
			batch = remaining
		}

		blocks, err := client.GetBlocksDetails(localHeight, batch)
		if err != nil {
			return fmt.Errorf("sync: fetch blocks at %d: %w", localHeight, err)
		}

		if err := resolveBlockBlobs(blocks, client); err != nil {
			return fmt.Errorf("sync: resolve blobs at %d: %w", localHeight, err)
		}

		for _, bd := range blocks {
			if err := c.processBlock(bd, opts); err != nil {
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

func (c *Chain) processBlock(bd rpc.BlockDetails, opts SyncOptions) error {
	if bd.Height > 0 && bd.Height%100 == 0 {
		log.Printf("sync: processing block %d", bd.Height)
	}

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

	// Validate miner transaction structure.
	if err := consensus.ValidateMinerTx(&blk.MinerTx, bd.Height, opts.Forks); err != nil {
		return fmt.Errorf("validate miner tx: %w", err)
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

	// Build a set of the block's regular tx hashes for lookup.
	regularTxs := make(map[string]struct{}, len(blk.TxHashes))
	for _, h := range blk.TxHashes {
		regularTxs[h.String()] = struct{}{}
	}

	// Process regular transactions (skip the miner tx).
	for _, txInfo := range bd.Transactions {
		if _, isRegular := regularTxs[txInfo.ID]; !isRegular {
			continue // skip miner tx entry
		}
		txBlob, err := hex.DecodeString(txInfo.Blob)
		if err != nil {
			return fmt.Errorf("decode tx hex %s: %w", txInfo.ID, err)
		}
		txDec := wire.NewDecoder(bytes.NewReader(txBlob))
		tx := wire.DecodeTransaction(txDec)
		if err := txDec.Err(); err != nil {
			return fmt.Errorf("decode tx wire %s: %w", txInfo.ID, err)
		}

		// Validate transaction semantics.
		if err := consensus.ValidateTransaction(&tx, txBlob, opts.Forks, bd.Height); err != nil {
			return fmt.Errorf("validate tx %s: %w", txInfo.ID, err)
		}

		// Optionally verify signatures using the chain's output index.
		if opts.VerifySignatures {
			if err := consensus.VerifyTransactionSignatures(&tx, opts.Forks, bd.Height, c.GetRingOutputs); err != nil {
				return fmt.Errorf("verify tx signatures %s: %w", txInfo.ID, err)
			}
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
			switch inp := vin.(type) {
			case types.TxInputToKey:
				if err := c.MarkSpent(inp.KeyImage, bd.Height); err != nil {
					return fmt.Errorf("mark spent %s: %w", inp.KeyImage, err)
				}
			case types.TxInputZC:
				if err := c.MarkSpent(inp.KeyImage, bd.Height); err != nil {
					return fmt.Errorf("mark spent %s: %w", inp.KeyImage, err)
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

// ---------------------------------------------------------------------------
// Block blob reconstruction
// ---------------------------------------------------------------------------
// The Zano daemon's get_blocks_details RPC does not populate the "blob" field.
// To process blocks through the normal wire decoder we reconstruct the blob
// from the header fields (parsed from object_in_json) and the miner tx blob
// fetched via /gettransactions.

// resolveBlockBlobs fills in missing Blob fields for BlockDetails and TxInfo
// entries. The Zano daemon's get_blocks_details RPC does not populate blob
// fields, so we batch-fetch all tx blobs via /gettransactions and reconstruct
// each block's wire blob from the parsed header and raw miner tx bytes.
func resolveBlockBlobs(blocks []rpc.BlockDetails, client *rpc.Client) error {
	// Collect all tx hashes that need blobs (miner txs + regular txs).
	var allHashes []string
	hashSet := map[string]struct{}{}
	for i := range blocks {
		if blocks[i].Blob != "" {
			continue // block already has blob
		}
		for _, tx := range blocks[i].Transactions {
			if tx.Blob != "" {
				continue
			}
			if _, seen := hashSet[tx.ID]; !seen {
				allHashes = append(allHashes, tx.ID)
				hashSet[tx.ID] = struct{}{}
			}
		}
	}
	if len(allHashes) == 0 {
		return nil // all blobs present
	}

	// Batch-fetch tx blobs.
	txHexes, missed, err := client.GetTransactions(allHashes)
	if err != nil {
		return fmt.Errorf("fetch tx blobs: %w", err)
	}
	if len(missed) > 0 {
		return fmt.Errorf("daemon missed %d tx(es): %v", len(missed), missed)
	}
	if len(txHexes) != len(allHashes) {
		return fmt.Errorf("expected %d tx blobs, got %d", len(allHashes), len(txHexes))
	}

	// Index fetched blobs by hash.
	blobByHash := make(map[string]string, len(allHashes))
	for j, h := range allHashes {
		blobByHash[h] = txHexes[j]
	}

	// Fill in tx blobs and reconstruct block blobs.
	for i := range blocks {
		if blocks[i].Blob != "" {
			continue
		}
		bd := &blocks[i]

		// Fill in regular tx blobs.
		for j := range bd.Transactions {
			if bd.Transactions[j].Blob == "" {
				if hex, ok := blobByHash[bd.Transactions[j].ID]; ok {
					bd.Transactions[j].Blob = hex
				}
			}
		}

		// Parse header from object_in_json.
		hdr, err := parseBlockHeader(bd.ObjectInJSON)
		if err != nil {
			return fmt.Errorf("block %d: parse header: %w", bd.Height, err)
		}

		// Miner tx blob is transactions_details[0].
		if len(bd.Transactions) == 0 {
			return fmt.Errorf("block %d has no transactions_details", bd.Height)
		}
		minerTxBlob, err := hex.DecodeString(bd.Transactions[0].Blob)
		if err != nil {
			return fmt.Errorf("block %d: decode miner tx hex: %w", bd.Height, err)
		}

		// Collect regular tx hashes.
		var txHashes []types.Hash
		for _, txInfo := range bd.Transactions[1:] {
			h, err := types.HashFromHex(txInfo.ID)
			if err != nil {
				return fmt.Errorf("block %d: parse tx hash %s: %w", bd.Height, txInfo.ID, err)
			}
			txHashes = append(txHashes, h)
		}

		blob := buildBlockBlob(hdr, minerTxBlob, txHashes)
		bd.Blob = hex.EncodeToString(blob)
	}
	return nil
}

// blockHeaderJSON matches the AGGREGATED section of object_in_json.
type blockHeaderJSON struct {
	MajorVersion uint8  `json:"major_version"`
	Nonce        uint64 `json:"nonce"`
	PrevID       string `json:"prev_id"`
	MinorVersion uint64 `json:"minor_version"`
	Timestamp    uint64 `json:"timestamp"`
	Flags        uint8  `json:"flags"`
}

// aggregatedRE extracts the first AGGREGATED JSON object from object_in_json.
var aggregatedRE = regexp.MustCompile(`"AGGREGATED"\s*:\s*\{([^}]+)\}`)

// parseBlockHeader extracts block header fields from the daemon's
// object_in_json string. The Zano daemon serialises blocks using an
// AGGREGATED wrapper that contains the header fields as JSON.
func parseBlockHeader(objectInJSON string) (*types.BlockHeader, error) {
	m := aggregatedRE.FindStringSubmatch(objectInJSON)
	if m == nil {
		return nil, fmt.Errorf("AGGREGATED section not found in object_in_json")
	}

	var hj blockHeaderJSON
	if err := json.Unmarshal([]byte("{"+m[1]+"}"), &hj); err != nil {
		return nil, fmt.Errorf("unmarshal AGGREGATED: %w", err)
	}

	prevID, err := types.HashFromHex(hj.PrevID)
	if err != nil {
		return nil, fmt.Errorf("parse prev_id: %w", err)
	}

	return &types.BlockHeader{
		MajorVersion: hj.MajorVersion,
		Nonce:        hj.Nonce,
		PrevID:       prevID,
		MinorVersion: hj.MinorVersion,
		Timestamp:    hj.Timestamp,
		Flags:        hj.Flags,
	}, nil
}

// buildBlockBlob constructs the consensus wire blob for a block from its
// header, raw miner tx bytes, and regular transaction hashes.
func buildBlockBlob(hdr *types.BlockHeader, minerTxBlob []byte, txHashes []types.Hash) []byte {
	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	wire.EncodeBlockHeader(enc, hdr)
	buf.Write(minerTxBlob)
	enc.WriteVarint(uint64(len(txHashes)))
	for i := range txHashes {
		enc.WriteBlob32((*[32]byte)(&txHashes[i]))
	}
	return buf.Bytes()
}
