// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import "dappco.re/go/core/blockchain/types"

// SparseChainHistory builds the exponentially-spaced block hash list used by
// NOTIFY_REQUEST_CHAIN. Matches the C++ get_short_chain_history() algorithm:
// first 10 block hashes from the tip, then exponentially larger steps back
// to genesis.
// Usage: value.SparseChainHistory(...)
func (c *Chain) SparseChainHistory() ([]types.Hash, error) {
	height, err := c.Height()
	if err != nil {
		return nil, err
	}

	if height == 0 {
		// No blocks stored yet; send the genesis hash so the peer can
		// locate our fork point. A zero hash is meaningless and would
		// cause the peer to ignore the request.
		gh, err := types.HashFromHex(GenesisHash)
		if err != nil {
			return nil, err
		}
		return []types.Hash{gh}, nil
	}

	var hashes []types.Hash
	step := uint64(1)
	current := height - 1 // top block height (Height() returns count, not index)

	for {
		_, meta, err := c.GetBlockByHeight(current)
		if err != nil {
			break
		}
		hashes = append(hashes, meta.Hash)

		if current == 0 {
			break
		}

		// First 10 entries: step=1, then double each time.
		if len(hashes) >= 10 {
			step *= 2
		}

		if current < step {
			if current > 0 {
				current = 0
				continue
			}
			break
		}
		current -= step
	}

	return hashes, nil
}
