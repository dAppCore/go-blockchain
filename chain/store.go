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

	coreerr "forge.lthn.ai/core/go-log"

	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
	store "forge.lthn.ai/core/go-store"
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
		return coreerr.E("Chain.PutBlock", fmt.Sprintf("chain: encode block %d", meta.Height), err)
	}

	rec := blockRecord{
		Meta: *meta,
		Blob: hex.EncodeToString(buf.Bytes()),
	}
	val, err := json.Marshal(rec)
	if err != nil {
		return coreerr.E("Chain.PutBlock", fmt.Sprintf("chain: marshal block %d", meta.Height), err)
	}

	if err := c.store.Set(groupBlocks, heightKey(meta.Height), string(val)); err != nil {
		return coreerr.E("Chain.PutBlock", fmt.Sprintf("chain: store block %d", meta.Height), err)
	}

	// Update hash -> height index.
	hashHex := meta.Hash.String()
	if err := c.store.Set(groupBlockIndex, hashHex, strconv.FormatUint(meta.Height, 10)); err != nil {
		return coreerr.E("Chain.PutBlock", fmt.Sprintf("chain: index block %d", meta.Height), err)
	}

	return nil
}

// GetBlockByHeight retrieves a block by its height.
func (c *Chain) GetBlockByHeight(height uint64) (*types.Block, *BlockMeta, error) {
	val, err := c.store.Get(groupBlocks, heightKey(height))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil, coreerr.E("Chain.GetBlockByHeight", fmt.Sprintf("chain: block %d not found", height), nil)
		}
		return nil, nil, coreerr.E("Chain.GetBlockByHeight", fmt.Sprintf("chain: get block %d", height), err)
	}
	return decodeBlockRecord(val)
}

// GetBlockByHash retrieves a block by its hash.
func (c *Chain) GetBlockByHash(hash types.Hash) (*types.Block, *BlockMeta, error) {
	heightStr, err := c.store.Get(groupBlockIndex, hash.String())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil, coreerr.E("Chain.GetBlockByHash", fmt.Sprintf("chain: block %s not found", hash), nil)
		}
		return nil, nil, coreerr.E("Chain.GetBlockByHash", fmt.Sprintf("chain: get block index %s", hash), err)
	}
	height, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		return nil, nil, coreerr.E("Chain.GetBlockByHash", fmt.Sprintf("chain: parse height %q", heightStr), err)
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
		return coreerr.E("Chain.PutTransaction", fmt.Sprintf("chain: encode tx %s", hash), err)
	}

	rec := txRecord{
		Meta: *meta,
		Blob: hex.EncodeToString(buf.Bytes()),
	}
	val, err := json.Marshal(rec)
	if err != nil {
		return coreerr.E("Chain.PutTransaction", fmt.Sprintf("chain: marshal tx %s", hash), err)
	}

	if err := c.store.Set(groupTx, hash.String(), string(val)); err != nil {
		return coreerr.E("Chain.PutTransaction", fmt.Sprintf("chain: store tx %s", hash), err)
	}
	return nil
}

// GetTransaction retrieves a transaction by hash.
func (c *Chain) GetTransaction(hash types.Hash) (*types.Transaction, *TxMeta, error) {
	val, err := c.store.Get(groupTx, hash.String())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil, coreerr.E("Chain.GetTransaction", fmt.Sprintf("chain: tx %s not found", hash), nil)
		}
		return nil, nil, coreerr.E("Chain.GetTransaction", fmt.Sprintf("chain: get tx %s", hash), err)
	}

	var rec txRecord
	if err := json.Unmarshal([]byte(val), &rec); err != nil {
		return nil, nil, coreerr.E("Chain.GetTransaction", "chain: unmarshal tx", err)
	}
	blob, err := hex.DecodeString(rec.Blob)
	if err != nil {
		return nil, nil, coreerr.E("Chain.GetTransaction", "chain: decode tx hex", err)
	}
	dec := wire.NewDecoder(bytes.NewReader(blob))
	tx := wire.DecodeTransaction(dec)
	if err := dec.Err(); err != nil {
		return nil, nil, coreerr.E("Chain.GetTransaction", "chain: decode tx wire", err)
	}
	return &tx, &rec.Meta, nil
}

// HasTransaction checks whether a transaction exists in the store.
func (c *Chain) HasTransaction(hash types.Hash) bool {
	_, err := c.store.Get(groupTx, hash.String())
	return err == nil
}

// getBlockMeta retrieves only the metadata for a block at the given height,
// without decoding the wire blob. Useful for lightweight lookups.
func (c *Chain) getBlockMeta(height uint64) (*BlockMeta, error) {
	val, err := c.store.Get(groupBlocks, heightKey(height))
	if err != nil {
		return nil, coreerr.E("Chain.getBlockMeta", fmt.Sprintf("chain: block meta %d", height), err)
	}
	var rec blockRecord
	if err := json.Unmarshal([]byte(val), &rec); err != nil {
		return nil, coreerr.E("Chain.getBlockMeta", fmt.Sprintf("chain: unmarshal block meta %d", height), err)
	}
	return &rec.Meta, nil
}

func decodeBlockRecord(val string) (*types.Block, *BlockMeta, error) {
	var rec blockRecord
	if err := json.Unmarshal([]byte(val), &rec); err != nil {
		return nil, nil, coreerr.E("decodeBlockRecord", "chain: unmarshal block", err)
	}
	blob, err := hex.DecodeString(rec.Blob)
	if err != nil {
		return nil, nil, coreerr.E("decodeBlockRecord", "chain: decode block hex", err)
	}
	dec := wire.NewDecoder(bytes.NewReader(blob))
	blk := wire.DecodeBlock(dec)
	if err := dec.Err(); err != nil {
		return nil, nil, coreerr.E("decodeBlockRecord", "chain: decode block wire", err)
	}
	return &blk, &rec.Meta, nil
}
