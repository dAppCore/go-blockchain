// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"fmt"

	"forge.lthn.ai/core/go-blockchain/types"
)

// GetRingOutputs fetches the public keys for the given global output indices
// at the specified amount. This implements the consensus.RingOutputsFn
// signature for use during signature verification.
func (c *Chain) GetRingOutputs(amount uint64, offsets []uint64) ([]types.PublicKey, error) {
	pubs := make([]types.PublicKey, len(offsets))
	for i, gidx := range offsets {
		txHash, outNo, err := c.GetOutput(amount, gidx)
		if err != nil {
			return nil, fmt.Errorf("ring output %d (amount=%d, gidx=%d): %w", i, amount, gidx, err)
		}

		tx, _, err := c.GetTransaction(txHash)
		if err != nil {
			return nil, fmt.Errorf("ring output %d: tx %s: %w", i, txHash, err)
		}

		if int(outNo) >= len(tx.Vout) {
			return nil, fmt.Errorf("ring output %d: tx %s has %d outputs, want index %d",
				i, txHash, len(tx.Vout), outNo)
		}

		switch out := tx.Vout[outNo].(type) {
		case types.TxOutputBare:
			pubs[i] = out.Target.Key
		default:
			return nil, fmt.Errorf("ring output %d: unsupported output type %T", i, out)
		}
	}
	return pubs, nil
}
