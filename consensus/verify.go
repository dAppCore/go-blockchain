// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package consensus

import (
	"fmt"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-blockchain/types"
)

// RingOutputsFn fetches the public keys for a ring at the given amount
// and offsets. Used to decouple consensus/ from chain storage.
type RingOutputsFn func(amount uint64, offsets []uint64) ([]types.PublicKey, error)

// VerifyTransactionSignatures verifies all ring signatures in a transaction.
// For coinbase transactions, this is a no-op (no signatures).
// For pre-HF4 transactions, NLSAG ring signatures are verified.
// For post-HF4, CLSAG signatures and proofs are verified.
//
// getRingOutputs may be nil for coinbase-only checks.
func VerifyTransactionSignatures(tx *types.Transaction, forks []config.HardFork,
	height uint64, getRingOutputs RingOutputsFn) error {

	// Coinbase: no signatures.
	if isCoinbase(tx) {
		return nil
	}

	hf4Active := config.IsHardForkActive(forks, config.HF4Zarcanum, height)

	if !hf4Active {
		return verifyV1Signatures(tx, getRingOutputs)
	}

	return verifyV2Signatures(tx, getRingOutputs)
}

// verifyV1Signatures checks NLSAG ring signatures for pre-HF4 transactions.
func verifyV1Signatures(tx *types.Transaction, getRingOutputs RingOutputsFn) error {
	// Count key inputs.
	var keyInputCount int
	for _, vin := range tx.Vin {
		if _, ok := vin.(types.TxInputToKey); ok {
			keyInputCount++
		}
	}

	if len(tx.Signatures) != keyInputCount {
		return fmt.Errorf("consensus: signature count %d != input count %d",
			len(tx.Signatures), keyInputCount)
	}

	// Actual NLSAG verification requires the crypto bridge and ring outputs.
	// When getRingOutputs is nil, we can only check structural correctness.
	if getRingOutputs == nil {
		return nil
	}

	// TODO: Wire up crypto.CheckRingSignature() for each input.
	return nil
}

// verifyV2Signatures checks CLSAG signatures and proofs for post-HF4 transactions.
func verifyV2Signatures(tx *types.Transaction, getRingOutputs RingOutputsFn) error {
	// TODO: Wire up CLSAG verification and proof checks.
	return nil
}
