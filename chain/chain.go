// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

// Package chain stores and indexes the Lethean blockchain by syncing from
// a C++ daemon via RPC.
package chain

import (
	"forge.lthn.ai/core/go-blockchain/types"
	coreerr "forge.lthn.ai/core/go-log"
	store "forge.lthn.ai/core/go-store"
)

// Chain manages blockchain storage and indexing.
type Chain struct {
	store *store.Store
}

// New creates a Chain backed by the given store.
func New(s *store.Store) *Chain {
	return &Chain{store: s}
}

// Height returns the number of stored blocks (0 if empty).
func (c *Chain) Height() (uint64, error) {
	n, err := c.store.Count(groupBlocks)
	if err != nil {
		return 0, coreerr.E("Chain.Height", "chain: height", err)
	}
	return uint64(n), nil
}

// TopBlock returns the highest stored block and its metadata.
// Returns an error if the chain is empty.
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
