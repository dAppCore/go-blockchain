// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"

	"dappco.re/go/core/blockchain/consensus"
	"dappco.re/go/core/blockchain/types"
)

// GetRingOutputs fetches the public keys for the given global output indices
// at the specified amount. This implements the consensus.RingOutputsFn
// signature for use during signature verification.
func (c *Chain) GetRingOutputs(amount uint64, offsets []uint64) ([]types.PublicKey, error) {
	pubs := make([]types.PublicKey, len(offsets))
	for i, gidx := range offsets {
		txHash, outNo, err := c.GetOutput(amount, gidx)
		if err != nil {
			return nil, coreerr.E("Chain.GetRingOutputs", core.Sprintf("ring output %d (amount=%d, gidx=%d)", i, amount, gidx), err)
		}

		tx, _, err := c.GetTransaction(txHash)
		if err != nil {
			return nil, coreerr.E("Chain.GetRingOutputs", core.Sprintf("ring output %d: tx %s", i, txHash), err)
		}

		if int(outNo) >= len(tx.Vout) {
			return nil, coreerr.E("Chain.GetRingOutputs", core.Sprintf("ring output %d: tx %s has %d outputs, want index %d", i, txHash, len(tx.Vout), outNo), nil)
		}

		switch out := tx.Vout[outNo].(type) {
		case types.TxOutputBare:
			toKey, ok := out.Target.(types.TxOutToKey)
			if !ok {
				return nil, coreerr.E("Chain.GetRingOutputs", core.Sprintf("ring output %d: unsupported target type %T", i, out.Target), nil)
			}
			pubs[i] = toKey.Key
		default:
			return nil, coreerr.E("Chain.GetRingOutputs", core.Sprintf("ring output %d: unsupported output type %T", i, out), nil)
		}
	}
	return pubs, nil
}

// GetZCRingOutputs fetches ZC ring members (stealth address, amount commitment,
// blinded asset ID) for the given global output indices. This implements the
// consensus.ZCRingOutputsFn signature for post-HF4 CLSAG GGX verification.
//
// ZC outputs are indexed at amount=0 (confidential amounts).
func (c *Chain) GetZCRingOutputs(offsets []uint64) ([]consensus.ZCRingMember, error) {
	members := make([]consensus.ZCRingMember, len(offsets))
	for i, gidx := range offsets {
		txHash, outNo, err := c.GetOutput(0, gidx)
		if err != nil {
			return nil, coreerr.E("Chain.GetZCRingOutputs", core.Sprintf("ZC ring output %d (gidx=%d)", i, gidx), err)
		}

		tx, _, err := c.GetTransaction(txHash)
		if err != nil {
			return nil, coreerr.E("Chain.GetZCRingOutputs", core.Sprintf("ZC ring output %d: tx %s", i, txHash), err)
		}

		if int(outNo) >= len(tx.Vout) {
			return nil, coreerr.E("Chain.GetZCRingOutputs", core.Sprintf("ZC ring output %d: tx %s has %d outputs, want index %d", i, txHash, len(tx.Vout), outNo), nil)
		}

		switch out := tx.Vout[outNo].(type) {
		case types.TxOutputZarcanum:
			members[i] = consensus.ZCRingMember{
				StealthAddress:   [32]byte(out.StealthAddress),
				AmountCommitment: [32]byte(out.AmountCommitment),
				BlindedAssetID:   [32]byte(out.BlindedAssetID),
			}
		default:
			return nil, coreerr.E("Chain.GetZCRingOutputs", core.Sprintf("ZC ring output %d: expected TxOutputZarcanum, got %T", i, out), nil)
		}
	}
	return members, nil
}
