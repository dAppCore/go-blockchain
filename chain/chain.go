// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

// Package chain stores and indexes the Lethean blockchain by syncing from
// a C++ daemon via RPC.
package chain

import (
	"dappco.re/go/core/blockchain/types"
	coreerr "dappco.re/go/core/log"
	store "dappco.re/go/core/store"
)

// Chain manages blockchain storage and indexing.
// Usage: var value chain.Chain
type Chain struct {
	store         *store.Store
	blockCallback BlockCallback
	syncCallback  SyncCallback
}

// New creates a Chain backed by the given store.
// Usage: chain.New(...)
func New(s *store.Store) *Chain {
	return &Chain{store: s}
}

// Height returns the number of stored blocks (0 if empty).
// Usage: value.Height(...)
func (c *Chain) Height() (uint64, error) {
	n, err := c.store.Count(groupBlocks)
	if err != nil {
		return 0, coreerr.E("Chain.Height", "chain: height", err)
	}
	return uint64(n), nil
}

// TopBlock returns the highest stored block and its metadata.
// Returns an error if the chain is empty.
// Usage: value.TopBlock(...)
func (c *Chain) TopBlock() (*types.Block, *BlockMeta, error) {
	h, err := c.Height()
	if err != nil {
		return nil, nil, err
	}
	if h == 0 {
		return nil, nil, coreerr.E("Chain.TopBlock", "chain: no blocks stored", nil)
	}
	return c.GetBlockByHeight(h - 1)
}

// Snapshot returns a consistent view of chain height and top block.
// This avoids TOCTOU between Height() and TopBlock() calls.
//
//	height, blk, meta := c.Snapshot()
func (c *Chain) Snapshot() (uint64, *types.Block, *BlockMeta) {
	height, err := c.Height()
	if err != nil || height == 0 {
		return 0, nil, &BlockMeta{}
	}
	blk, meta, err := c.TopBlock()
	if err != nil {
		return height, nil, &BlockMeta{Height: height}
	}
	return height, blk, meta
}

// BlockCallback is called after a block is successfully stored.
type BlockCallback func(height uint64, hash string, aliasName string)

// SyncCallback is called periodically during chain sync with progress info.
type SyncCallback func(localHeight, remoteHeight uint64, blocksPerSecond float64)

// SetBlockCallback sets a function called after each block is stored.
//
//	c.SetBlockCallback(func(height uint64, hash string, alias string) { ... })
func (c *Chain) SetBlockCallback(cb BlockCallback) {
	c.blockCallback = cb
}

// SetSyncCallback sets a function called during sync progress.
//
//	c.SetSyncCallback(func(local, remote uint64, bps float64) { ... })
func (c *Chain) SetSyncCallback(cb SyncCallback) {
	c.syncCallback = cb
}
