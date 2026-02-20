// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

// Storage group constants matching the design schema.
const (
	groupBlocks     = "blocks"
	groupBlockIndex = "block_index"
	groupTx         = "transactions"
	groupSpentKeys  = "spent_keys"
	groupOutputsPfx = "outputs:" // suffixed with amount
)

// heightKey returns a zero-padded 10-digit decimal key for the given height.
func heightKey(h uint64) string {
	return fmt.Sprintf("%010d", h)
}

// blockRecord is the JSON value stored in the blocks group.
type blockRecord struct {
	Meta BlockMeta `json:"meta"`
	Blob string    `json:"blob"` // hex-encoded wire format
}

// PutBlock stores a block and updates the block_index.
func (c *Chain) PutBlock(b *types.Block, meta *BlockMeta) error {
	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	wire.EncodeBlock(enc, b)
	if err := enc.Err(); err != nil {
		return fmt.Errorf("chain: encode block %d: %w", meta.Height, err)
	}

	rec := blockRecord{
		Meta: *meta,
		Blob: hex.EncodeToString(buf.Bytes()),
	}
	val, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("chain: marshal block %d: %w", meta.Height, err)
	}

	if err := c.store.Set(groupBlocks, heightKey(meta.Height), string(val)); err != nil {
		return fmt.Errorf("chain: store block %d: %w", meta.Height, err)
	}

	// Update hash -> height index.
	hashHex := meta.Hash.String()
	if err := c.store.Set(groupBlockIndex, hashHex, strconv.FormatUint(meta.Height, 10)); err != nil {
		return fmt.Errorf("chain: index block %d: %w", meta.Height, err)
	}

	return nil
}

// GetBlockByHeight retrieves a block by its height.
func (c *Chain) GetBlockByHeight(height uint64) (*types.Block, *BlockMeta, error) {
	val, err := c.store.Get(groupBlocks, heightKey(height))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil, fmt.Errorf("chain: block %d not found", height)
		}
		return nil, nil, fmt.Errorf("chain: get block %d: %w", height, err)
	}
	return decodeBlockRecord(val)
}

// GetBlockByHash retrieves a block by its hash.
func (c *Chain) GetBlockByHash(hash types.Hash) (*types.Block, *BlockMeta, error) {
	heightStr, err := c.store.Get(groupBlockIndex, hash.String())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil, fmt.Errorf("chain: block %s not found", hash)
		}
		return nil, nil, fmt.Errorf("chain: get block index %s: %w", hash, err)
	}
	height, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("chain: parse height %q: %w", heightStr, err)
	}
	return c.GetBlockByHeight(height)
}

// txRecord is the JSON value stored in the transactions group.
type txRecord struct {
	Meta TxMeta `json:"meta"`
	Blob string `json:"blob"` // hex-encoded wire format
}

// PutTransaction stores a transaction with metadata.
func (c *Chain) PutTransaction(hash types.Hash, tx *types.Transaction, meta *TxMeta) error {
	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	wire.EncodeTransaction(enc, tx)
	if err := enc.Err(); err != nil {
		return fmt.Errorf("chain: encode tx %s: %w", hash, err)
	}

	rec := txRecord{
		Meta: *meta,
		Blob: hex.EncodeToString(buf.Bytes()),
	}
	val, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("chain: marshal tx %s: %w", hash, err)
	}

	if err := c.store.Set(groupTx, hash.String(), string(val)); err != nil {
		return fmt.Errorf("chain: store tx %s: %w", hash, err)
	}
	return nil
}

// GetTransaction retrieves a transaction by hash.
func (c *Chain) GetTransaction(hash types.Hash) (*types.Transaction, *TxMeta, error) {
	val, err := c.store.Get(groupTx, hash.String())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil, fmt.Errorf("chain: tx %s not found", hash)
		}
		return nil, nil, fmt.Errorf("chain: get tx %s: %w", hash, err)
	}

	var rec txRecord
	if err := json.Unmarshal([]byte(val), &rec); err != nil {
		return nil, nil, fmt.Errorf("chain: unmarshal tx: %w", err)
	}
	blob, err := hex.DecodeString(rec.Blob)
	if err != nil {
		return nil, nil, fmt.Errorf("chain: decode tx hex: %w", err)
	}
	dec := wire.NewDecoder(bytes.NewReader(blob))
	tx := wire.DecodeTransaction(dec)
	if err := dec.Err(); err != nil {
		return nil, nil, fmt.Errorf("chain: decode tx wire: %w", err)
	}
	return &tx, &rec.Meta, nil
}

// HasTransaction checks whether a transaction exists in the store.
func (c *Chain) HasTransaction(hash types.Hash) bool {
	_, err := c.store.Get(groupTx, hash.String())
	return err == nil
}

func decodeBlockRecord(val string) (*types.Block, *BlockMeta, error) {
	var rec blockRecord
	if err := json.Unmarshal([]byte(val), &rec); err != nil {
		return nil, nil, fmt.Errorf("chain: unmarshal block: %w", err)
	}
	blob, err := hex.DecodeString(rec.Blob)
	if err != nil {
		return nil, nil, fmt.Errorf("chain: decode block hex: %w", err)
	}
	dec := wire.NewDecoder(bytes.NewReader(blob))
	blk := wire.DecodeBlock(dec)
	if err := dec.Err(); err != nil {
		return nil, nil, fmt.Errorf("chain: decode block wire: %w", err)
	}
	return &blk, &rec.Meta, nil
}
