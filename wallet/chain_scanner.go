// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wallet

import (
	"dappco.re/go/core/blockchain/types"
)

// ChainScanner scans a local chain for owned outputs.
// Much faster than RPC scanning — reads directly from go-store.
//
//	scanner := wallet.NewChainScanner(account, chainGetter)
type ChainScanner struct {
	v1       *V1Scanner
	getBlock func(height uint64) (*types.Block, []types.Transaction, error)
}

// BlockGetter retrieves a block and its transactions by height.
type BlockGetter func(height uint64) (*types.Block, []types.Transaction, error)

// NewChainScanner creates a scanner that reads from local chain storage.
//
//	scanner := wallet.NewChainScanner(account, chain.GetBlockWithTxs)
func NewChainScanner(account *Account, getter BlockGetter) *ChainScanner {
	return &ChainScanner{
		v1:       NewV1Scanner(account),
		getBlock: getter,
	}
}

// ScanRange scans blocks from startHeight to endHeight for owned outputs.
// Returns all found transfers and the number of blocks scanned.
//
//	transfers, scanned := scanner.ScanRange(0, 11000)
func (s *ChainScanner) ScanRange(startHeight, endHeight uint64) ([]Transfer, uint64) {
	var allTransfers []Transfer
	scanned := uint64(0)

	for h := startHeight; h < endHeight; h++ {
		blk, txs, err := s.getBlock(h)
		if err != nil || blk == nil {
			continue
		}

		// Scan miner tx
		extra, err := ParseTxExtra(blk.MinerTx.Extra)
		if err == nil && extra != nil {
			minerHash := types.Hash{} // TODO: compute from wire
			transfers, _ := s.v1.ScanTransaction(&blk.MinerTx, minerHash, h, extra)
			allTransfers = append(allTransfers, transfers...)
		}

		// Scan regular txs
		for _, tx := range txs {
			extra, err := ParseTxExtra(tx.Extra)
			if err != nil || extra == nil {
				continue
			}
			txHash := types.Hash{} // TODO: compute from wire
			transfers, _ := s.v1.ScanTransaction(&tx, txHash, h, extra)
			allTransfers = append(allTransfers, transfers...)
		}

		scanned++
	}

	return allTransfers, scanned
}
