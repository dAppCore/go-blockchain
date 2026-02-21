// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"context"
	"fmt"
	"log"
)

// P2PConnection abstracts the P2P communication needed for block sync.
type P2PConnection interface {
	// PeerHeight returns the peer's advertised chain height.
	PeerHeight() uint64

	// RequestChain sends NOTIFY_REQUEST_CHAIN and returns the response.
	RequestChain(blockIDs [][]byte) (startHeight uint64, hashes [][]byte, err error)

	// RequestObjects sends NOTIFY_REQUEST_GET_OBJECTS and returns block blobs.
	RequestObjects(blockHashes [][]byte) ([]BlockBlobEntry, error)
}

// BlockBlobEntry holds raw block and transaction blobs from a peer.
type BlockBlobEntry struct {
	Block []byte
	Txs   [][]byte
}

const p2pBatchSize = 200

// P2PSync synchronises the chain from a P2P peer. It runs the
// REQUEST_CHAIN / REQUEST_GET_OBJECTS protocol loop until the local
// chain reaches the peer's height.
func (c *Chain) P2PSync(ctx context.Context, conn P2PConnection, opts SyncOptions) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		localHeight, err := c.Height()
		if err != nil {
			return fmt.Errorf("p2p sync: get height: %w", err)
		}

		peerHeight := conn.PeerHeight()
		if localHeight >= peerHeight {
			return nil // synced
		}

		// Build sparse chain history.
		history, err := c.SparseChainHistory()
		if err != nil {
			return fmt.Errorf("p2p sync: build history: %w", err)
		}

		// Convert Hash to []byte for P2P.
		historyBytes := make([][]byte, len(history))
		for i, h := range history {
			b := make([]byte, 32)
			copy(b, h[:])
			historyBytes[i] = b
		}

		// Request chain entry.
		startHeight, blockIDs, err := conn.RequestChain(historyBytes)
		if err != nil {
			return fmt.Errorf("p2p sync: request chain: %w", err)
		}

		if len(blockIDs) == 0 {
			return nil // nothing to sync
		}

		log.Printf("p2p sync: chain entry from height %d, %d block IDs", startHeight, len(blockIDs))

		// The daemon returns the fork-point block as the first entry.
		// Skip blocks we already have.
		skip := 0
		if startHeight < localHeight {
			skip = int(localHeight - startHeight)
			if skip >= len(blockIDs) {
				continue // all IDs are blocks we already have
			}
		}
		fetchIDs := blockIDs[skip:]
		fetchStart := startHeight + uint64(skip)

		// Fetch blocks in batches.
		for i := 0; i < len(fetchIDs); i += p2pBatchSize {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			end := i + p2pBatchSize
			if end > len(fetchIDs) {
				end = len(fetchIDs)
			}
			batch := fetchIDs[i:end]

			entries, err := conn.RequestObjects(batch)
			if err != nil {
				return fmt.Errorf("p2p sync: request objects: %w", err)
			}

			currentHeight := fetchStart + uint64(i)
			for j, entry := range entries {
				blockHeight := currentHeight + uint64(j)
				if blockHeight > 0 && blockHeight%100 == 0 {
					log.Printf("p2p sync: processing block %d", blockHeight)
				}

				blockDiff, err := c.NextDifficulty(blockHeight)
				if err != nil {
					return fmt.Errorf("p2p sync: compute difficulty for block %d: %w", blockHeight, err)
				}

				if err := c.processBlockBlobs(entry.Block, entry.Txs,
					blockHeight, blockDiff, opts); err != nil {
					return fmt.Errorf("p2p sync: process block %d: %w", blockHeight, err)
				}
			}
		}
	}
}
