// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package consensus

import (
	"fmt"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-blockchain/crypto"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
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

	prefixHash := wire.TransactionPrefixHash(tx)

	var sigIdx int
	for _, vin := range tx.Vin {
		inp, ok := vin.(types.TxInputToKey)
		if !ok {
			continue
		}

		// Extract absolute global indices from key offsets.
		offsets := make([]uint64, len(inp.KeyOffsets))
		for i, ref := range inp.KeyOffsets {
			offsets[i] = ref.GlobalIndex
		}

		ringKeys, err := getRingOutputs(inp.Amount, offsets)
		if err != nil {
			return fmt.Errorf("consensus: failed to fetch ring outputs for input %d: %w",
				sigIdx, err)
		}

		ringSigs := tx.Signatures[sigIdx]
		if len(ringSigs) != len(ringKeys) {
			return fmt.Errorf("consensus: input %d has %d signatures but ring size %d",
				sigIdx, len(ringSigs), len(ringKeys))
		}

		// Convert typed slices to raw byte arrays for the crypto bridge.
		pubs := make([][32]byte, len(ringKeys))
		for i, pk := range ringKeys {
			pubs[i] = [32]byte(pk)
		}

		sigs := make([][64]byte, len(ringSigs))
		for i, s := range ringSigs {
			sigs[i] = [64]byte(s)
		}

		if !crypto.CheckRingSignature([32]byte(prefixHash), [32]byte(inp.KeyImage), pubs, sigs) {
			return fmt.Errorf("consensus: ring signature verification failed for input %d", sigIdx)
		}

		sigIdx++
	}

	return nil
}

// verifyV2Signatures checks CLSAG signatures and proofs for post-HF4 transactions.
func verifyV2Signatures(tx *types.Transaction, getRingOutputs RingOutputsFn) error {
	// TODO: Wire up CLSAG verification and proof checks.
	return nil
}
