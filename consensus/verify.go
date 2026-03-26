// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package consensus

import (
	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"

	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/crypto"
	"dappco.re/go/core/blockchain/types"
	"dappco.re/go/core/blockchain/wire"
)

// RingOutputsFn fetches the public keys for a ring at the given amount
// and offsets. Used to decouple consensus/ from chain storage.
type RingOutputsFn func(amount uint64, offsets []uint64) ([]types.PublicKey, error)

// ZCRingMember holds the three public keys per ring entry needed for
// CLSAG GGX verification (HF4+). All fields are premultiplied by 1/8
// as stored on chain.
type ZCRingMember struct {
	StealthAddress   [32]byte
	AmountCommitment [32]byte
	BlindedAssetID   [32]byte
}

// ZCRingOutputsFn fetches ZC ring members for the given global output indices.
// Used for post-HF4 CLSAG GGX signature verification.
type ZCRingOutputsFn func(offsets []uint64) ([]ZCRingMember, error)

// VerifyTransactionSignatures verifies all ring signatures in a transaction.
// For coinbase transactions, this is a no-op (no signatures).
// For pre-HF4 transactions, NLSAG ring signatures are verified.
// For post-HF4, CLSAG signatures and proofs are verified.
//
// getRingOutputs is used for pre-HF4 (V1) signature verification.
// getZCRingOutputs is used for post-HF4 (V2) CLSAG GGX verification.
// Either may be nil for structural-only checks.
func VerifyTransactionSignatures(tx *types.Transaction, forks []config.HardFork,
	height uint64, getRingOutputs RingOutputsFn, getZCRingOutputs ZCRingOutputsFn) error {

	// Coinbase: no signatures.
	if isCoinbase(tx) {
		return nil
	}

	hf4Active := config.IsHardForkActive(forks, config.HF4Zarcanum, height)

	if !hf4Active {
		return verifyV1Signatures(tx, getRingOutputs)
	}

	return verifyV2Signatures(tx, getZCRingOutputs)
}

// verifyV1Signatures checks NLSAG ring signatures for pre-HF4 transactions.
func verifyV1Signatures(tx *types.Transaction, getRingOutputs RingOutputsFn) error {
	// Count ring-signing inputs (TxInputToKey and TxInputHTLC contribute
	// ring signatures; TxInputMultisig does not).
	var ringInputCount int
	for _, vin := range tx.Vin {
		switch vin.(type) {
		case types.TxInputToKey, types.TxInputHTLC:
			ringInputCount++
		}
	}

	if len(tx.Signatures) != ringInputCount {
		return coreerr.E("verifyV1Signatures", core.Sprintf("consensus: signature count %d != input count %d", len(tx.Signatures), ringInputCount), nil)
	}

	// Actual NLSAG verification requires the crypto bridge and ring outputs.
	// When getRingOutputs is nil, we can only check structural correctness.
	if getRingOutputs == nil {
		return nil
	}

	prefixHash := wire.TransactionPrefixHash(tx)

	var sigIdx int
	for _, vin := range tx.Vin {
		// Extract amount and key offsets from ring-signing input types.
		var amount uint64
		var keyOffsets []types.TxOutRef
		var keyImage types.KeyImage

		switch v := vin.(type) {
		case types.TxInputToKey:
			amount = v.Amount
			keyOffsets = v.KeyOffsets
			keyImage = v.KeyImage
		case types.TxInputHTLC:
			amount = v.Amount
			keyOffsets = v.KeyOffsets
			keyImage = v.KeyImage
		default:
			continue // TxInputMultisig and others do not use NLSAG
		}

		// Extract absolute global indices from key offsets.
		offsets := make([]uint64, len(keyOffsets))
		for i, ref := range keyOffsets {
			offsets[i] = ref.GlobalIndex
		}

		ringKeys, err := getRingOutputs(amount, offsets)
		if err != nil {
			return coreerr.E("verifyV1Signatures", core.Sprintf("consensus: failed to fetch ring outputs for input %d", sigIdx), err)
		}

		ringSigs := tx.Signatures[sigIdx]
		if len(ringSigs) != len(ringKeys) {
			return coreerr.E("verifyV1Signatures", core.Sprintf("consensus: input %d has %d signatures but ring size %d", sigIdx, len(ringSigs), len(ringKeys)), nil)
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

		if !crypto.CheckRingSignature([32]byte(prefixHash), [32]byte(keyImage), pubs, sigs) {
			return coreerr.E("verifyV1Signatures", core.Sprintf("consensus: ring signature verification failed for input %d", sigIdx), nil)
		}

		sigIdx++
	}

	return nil
}

// verifyV2Signatures checks CLSAG signatures and proofs for post-HF4 transactions.
func verifyV2Signatures(tx *types.Transaction, getZCRingOutputs ZCRingOutputsFn) error {
	// Parse the signature variant vector.
	sigEntries, err := parseV2Signatures(tx.SignaturesRaw)
	if err != nil {
		return coreerr.E("verifyV2Signatures", "consensus", err)
	}

	// Match signatures to inputs: each input must have a corresponding signature.
	if len(sigEntries) != len(tx.Vin) {
		return coreerr.E("verifyV2Signatures", core.Sprintf("consensus: V2 signature count %d != input count %d", len(sigEntries), len(tx.Vin)), nil)
	}

	// Validate that ZC inputs have ZC_sig and vice versa.
	for i, vin := range tx.Vin {
		switch vin.(type) {
		case types.TxInputZC:
			if sigEntries[i].tag != types.SigTypeZC {
				return coreerr.E("verifyV2Signatures", core.Sprintf("consensus: input %d is ZC but signature tag is 0x%02x", i, sigEntries[i].tag), nil)
			}
		case types.TxInputToKey:
			if sigEntries[i].tag != types.SigTypeNLSAG && sigEntries[i].tag != types.SigTypeVoid {
				return coreerr.E("verifyV2Signatures", core.Sprintf("consensus: input %d is to_key but signature tag is 0x%02x", i, sigEntries[i].tag), nil)
			}
		}
	}

	// Without ring output data, we can only check structural correctness.
	if getZCRingOutputs == nil {
		return nil
	}

	// Compute tx prefix hash for CLSAG verification.
	// For normal transactions (not TX_FLAG_SIGNATURE_MODE_SEPARATE),
	// the hash is simply the transaction prefix hash.
	prefixHash := wire.TransactionPrefixHash(tx)

	// Verify CLSAG GGX signature for each ZC input.
	for i, vin := range tx.Vin {
		zcIn, ok := vin.(types.TxInputZC)
		if !ok {
			continue
		}

		zc := sigEntries[i].zcSig
		if zc == nil {
			return coreerr.E("verifyV2Signatures", core.Sprintf("consensus: input %d: missing ZC_sig data", i), nil)
		}

		// Extract absolute global indices from key offsets.
		offsets := make([]uint64, len(zcIn.KeyOffsets))
		for j, ref := range zcIn.KeyOffsets {
			offsets[j] = ref.GlobalIndex
		}

		ringMembers, err := getZCRingOutputs(offsets)
		if err != nil {
			return coreerr.E("verifyV2Signatures", core.Sprintf("consensus: failed to fetch ZC ring outputs for input %d", i), err)
		}

		if len(ringMembers) != zc.ringSize {
			return coreerr.E("verifyV2Signatures", core.Sprintf("consensus: input %d: ring size %d from chain != %d from sig", i, len(ringMembers), zc.ringSize), nil)
		}

		// Build flat ring: [stealth(32) | commitment(32) | blinded_asset_id(32)] per entry.
		ring := make([]byte, 0, len(ringMembers)*96)
		for _, m := range ringMembers {
			ring = append(ring, m.StealthAddress[:]...)
			ring = append(ring, m.AmountCommitment[:]...)
			ring = append(ring, m.BlindedAssetID[:]...)
		}

		if !crypto.VerifyCLSAGGGX(
			[32]byte(prefixHash),
			ring, zc.ringSize,
			zc.pseudoOutCommitment,
			zc.pseudoOutAssetID,
			[32]byte(zcIn.KeyImage),
			zc.clsagFlatSig,
		) {
			return coreerr.E("verifyV2Signatures", core.Sprintf("consensus: CLSAG GGX verification failed for input %d", i), nil)
		}
	}

	// Parse and verify proofs.
	proofs, err := parseV2Proofs(tx.Proofs)
	if err != nil {
		return coreerr.E("verifyV2Signatures", "consensus", err)
	}

	// Verify BPP range proof if present.
	if len(proofs.bppProofBytes) > 0 && len(proofs.bppCommitments) > 0 {
		if !crypto.VerifyBPP(proofs.bppProofBytes, proofs.bppCommitments) {
			return coreerr.E("verifyV2Signatures", "consensus: BPP range proof verification failed", nil)
		}
	}

	// Verify BGE asset surjection proofs.
	// One proof per output, with a ring of (pseudo_out_asset_id - output_asset_id)
	// per ZC input.
	if len(proofs.bgeProofs) > 0 {
		if err := verifyBGEProofs(tx, sigEntries, proofs, prefixHash); err != nil {
			return err
		}
	}

	// TODO: Verify balance proof (generic_double_schnorr_sig).
	// Requires computing commitment_to_zero and a new bridge function.

	return nil
}

// verifyBGEProofs verifies the BGE asset surjection proofs.
// There is one BGE proof per Zarcanum output. For each output j, the proof
// demonstrates that the output's blinded asset ID matches one of the
// pseudo-out asset IDs from the ZC inputs.
//
// The BGE ring for output j has one entry per ZC input i:
//
//	ring[i] = mul8(pseudo_out_blinded_asset_id_i) - mul8(output_blinded_asset_id_j)
//
// The context hash is the transaction prefix hash (tx_id).
func verifyBGEProofs(tx *types.Transaction, sigEntries []v2SigEntry,
	proofs *v2ProofData, prefixHash types.Hash) error {

	// Collect Zarcanum output blinded asset IDs.
	var outputAssetIDs [][32]byte
	for _, out := range tx.Vout {
		if zOut, ok := out.(types.TxOutputZarcanum); ok {
			outputAssetIDs = append(outputAssetIDs, [32]byte(zOut.BlindedAssetID))
		}
	}

	if len(proofs.bgeProofs) != len(outputAssetIDs) {
		return coreerr.E("verifyBGEProofs", core.Sprintf("consensus: BGE proof count %d != Zarcanum output count %d", len(proofs.bgeProofs), len(outputAssetIDs)), nil)
	}

	// Collect pseudo-out asset IDs from ZC signatures and expand to full points.
	var pseudoOutAssetIDs [][32]byte
	for _, entry := range sigEntries {
		if entry.tag == types.SigTypeZC && entry.zcSig != nil {
			pseudoOutAssetIDs = append(pseudoOutAssetIDs, entry.zcSig.pseudoOutAssetID)
		}
	}

	// mul8 all pseudo-out asset IDs (stored premultiplied by 1/8 on chain).
	mul8PseudoOuts := make([][32]byte, len(pseudoOutAssetIDs))
	for i, p := range pseudoOutAssetIDs {
		full, err := crypto.PointMul8(p)
		if err != nil {
			return coreerr.E("verifyBGEProofs", core.Sprintf("consensus: mul8 pseudo-out asset ID %d", i), err)
		}
		mul8PseudoOuts[i] = full
	}

	// For each output, build the BGE ring and verify.
	context := [32]byte(prefixHash)
	for j, outAssetID := range outputAssetIDs {
		// mul8 the output's blinded asset ID.
		mul8Out, err := crypto.PointMul8(outAssetID)
		if err != nil {
			return coreerr.E("verifyBGEProofs", core.Sprintf("consensus: mul8 output asset ID %d", j), err)
		}

		// ring[i] = mul8(pseudo_out_i) - mul8(output_j)
		ring := make([][32]byte, len(mul8PseudoOuts))
		for i, mul8Pseudo := range mul8PseudoOuts {
			diff, err := crypto.PointSub(mul8Pseudo, mul8Out)
			if err != nil {
				return coreerr.E("verifyBGEProofs", core.Sprintf("consensus: BGE ring[%d][%d] sub", j, i), err)
			}
			ring[i] = diff
		}

		if !crypto.VerifyBGE(context, ring, proofs.bgeProofs[j]) {
			return coreerr.E("verifyBGEProofs", core.Sprintf("consensus: BGE proof verification failed for output %d", j), nil)
		}
	}

	return nil
}
