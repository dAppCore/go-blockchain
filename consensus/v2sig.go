// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package consensus

import (
	"bytes"
	"fmt"

	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

// zcSigData holds the parsed components of a ZC_sig variant element
// needed for CLSAG GGX verification.
type zcSigData struct {
	pseudoOutCommitment [32]byte // premultiplied by 1/8 on chain
	pseudoOutAssetID    [32]byte // premultiplied by 1/8 on chain
	clsagFlatSig        []byte   // flat: c(32) | r_g[N*32] | r_x[N*32] | K1(32) | K2(32)
	ringSize            int
}

// v2SigEntry is one parsed entry from the V2 signature variant vector.
type v2SigEntry struct {
	tag   uint8
	zcSig *zcSigData // non-nil when tag == SigTypeZC
}

// parseV2Signatures parses the SignaturesRaw variant vector into a slice
// of v2SigEntry. The order matches the transaction inputs.
func parseV2Signatures(raw []byte) ([]v2SigEntry, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	dec := wire.NewDecoder(bytes.NewReader(raw))
	count := dec.ReadVarint()
	if dec.Err() != nil {
		return nil, fmt.Errorf("read sig count: %w", dec.Err())
	}

	entries := make([]v2SigEntry, 0, count)
	for i := uint64(0); i < count; i++ {
		tag := dec.ReadUint8()
		if dec.Err() != nil {
			return nil, fmt.Errorf("read sig tag %d: %w", i, dec.Err())
		}

		entry := v2SigEntry{tag: tag}

		switch tag {
		case types.SigTypeZC:
			zc, err := parseZCSig(dec)
			if err != nil {
				return nil, fmt.Errorf("parse ZC_sig %d: %w", i, err)
			}
			entry.zcSig = zc

		case types.SigTypeVoid:
			// Empty struct — nothing to read.

		case types.SigTypeNLSAG:
			// Skip: varint(count) + count * 64-byte signatures.
			n := dec.ReadVarint()
			if n > 0 && dec.Err() == nil {
				_ = dec.ReadBytes(int(n) * 64)
			}

		case types.SigTypeZarcanum:
			// Skip: 10 scalars + bppe + public_key + CLSAG_GGXXG.
			// Use skipZarcanumSig to advance past the data.
			skipZarcanumSig(dec)

		default:
			return nil, fmt.Errorf("unsupported sig tag 0x%02x", tag)
		}

		if dec.Err() != nil {
			return nil, fmt.Errorf("parse sig %d (tag 0x%02x): %w", i, tag, dec.Err())
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// parseZCSig parses a ZC_sig element (after the tag byte) from the decoder.
// Wire: pseudo_out_amount_commitment(32) + pseudo_out_blinded_asset_id(32) + CLSAG_GGX_serialized.
func parseZCSig(dec *wire.Decoder) (*zcSigData, error) {
	var zc zcSigData

	dec.ReadBlob32(&zc.pseudoOutCommitment)
	dec.ReadBlob32(&zc.pseudoOutAssetID)
	if dec.Err() != nil {
		return nil, dec.Err()
	}

	// CLSAG_GGX_serialized wire format:
	//   c(32) + varint(N) + r_g[N*32] + varint(N) + r_x[N*32] + K1(32) + K2(32)
	//
	// C bridge expects flat:
	//   c(32) | r_g[N*32] | r_x[N*32] | K1(32) | K2(32)

	var c [32]byte
	dec.ReadBlob32(&c)

	rgCount := dec.ReadVarint()
	rgBytes := dec.ReadBytes(int(rgCount) * 32)

	rxCount := dec.ReadVarint()
	rxBytes := dec.ReadBytes(int(rxCount) * 32)

	if dec.Err() != nil {
		return nil, dec.Err()
	}

	if rgCount != rxCount {
		return nil, fmt.Errorf("CLSAG r_g count %d != r_x count %d", rgCount, rxCount)
	}
	zc.ringSize = int(rgCount)

	var k1, k2 [32]byte
	dec.ReadBlob32(&k1)
	dec.ReadBlob32(&k2)
	if dec.Err() != nil {
		return nil, dec.Err()
	}

	// Build flat sig for C bridge.
	flat := make([]byte, 0, 32+len(rgBytes)+len(rxBytes)+64)
	flat = append(flat, c[:]...)
	flat = append(flat, rgBytes...)
	flat = append(flat, rxBytes...)
	flat = append(flat, k1[:]...)
	flat = append(flat, k2[:]...)
	zc.clsagFlatSig = flat

	return &zc, nil
}

// skipZarcanumSig advances the decoder past a zarcanum_sig element.
// Wire: 10 scalars + bppe_serialized + public_key(32) + CLSAG_GGXXG.
func skipZarcanumSig(dec *wire.Decoder) {
	// 10 fixed scalars/points (320 bytes).
	_ = dec.ReadBytes(10 * 32)

	// bppe_serialized: vec(L) + vec(R) + 7 scalars (224 bytes).
	skipVecOfPoints(dec) // L
	skipVecOfPoints(dec) // R
	_ = dec.ReadBytes(7 * 32)

	// pseudo_out_amount_commitment (32 bytes).
	_ = dec.ReadBytes(32)

	// CLSAG_GGXXG: c(32) + vec(r_g) + vec(r_x) + K1(32) + K2(32) + K3(32) + K4(32).
	_ = dec.ReadBytes(32) // c
	skipVecOfPoints(dec)  // r_g
	skipVecOfPoints(dec)  // r_x
	_ = dec.ReadBytes(128) // K1+K2+K3+K4
}

// skipVecOfPoints advances the decoder past a varint(count) + count*32 vector.
func skipVecOfPoints(dec *wire.Decoder) {
	n := dec.ReadVarint()
	if n > 0 && dec.Err() == nil {
		_ = dec.ReadBytes(int(n) * 32)
	}
}

// v2ProofData holds parsed proof components from the Proofs raw bytes.
type v2ProofData struct {
	// bgeProofs contains one BGE proof blob per output (wire-serialised).
	// Each blob can be passed directly to crypto.VerifyBGE.
	bgeProofs [][]byte

	// bppProofBytes is the bpp_signature blob (wire-serialised).
	// Can be passed directly to crypto.VerifyBPP.
	bppProofBytes []byte

	// bppCommitments are the amount_commitments_for_rp_aggregation (E'_j).
	// Premultiplied by 1/8 as stored on chain.
	bppCommitments [][32]byte

	// balanceProof is the generic_double_schnorr_sig (96 bytes: c, y0, y1).
	balanceProof []byte
}

// parseV2Proofs parses the Proofs raw variant vector.
func parseV2Proofs(raw []byte) (*v2ProofData, error) {
	if len(raw) == 0 {
		return &v2ProofData{}, nil
	}

	dec := wire.NewDecoder(bytes.NewReader(raw))
	count := dec.ReadVarint()
	if dec.Err() != nil {
		return nil, fmt.Errorf("read proof count: %w", dec.Err())
	}

	var data v2ProofData
	for i := uint64(0); i < count; i++ {
		tag := dec.ReadUint8()
		if dec.Err() != nil {
			return nil, fmt.Errorf("read proof tag %d: %w", i, dec.Err())
		}

		switch tag {
		case 46: // zc_asset_surjection_proof: varint(nBGE) + nBGE * BGE_proof
			nBGE := dec.ReadVarint()
			if dec.Err() != nil {
				return nil, fmt.Errorf("parse BGE count: %w", dec.Err())
			}
			data.bgeProofs = make([][]byte, nBGE)
			for j := uint64(0); j < nBGE; j++ {
				data.bgeProofs[j] = readBGEProofBytes(dec)
				if dec.Err() != nil {
					return nil, fmt.Errorf("parse BGE proof %d: %w", j, dec.Err())
				}
			}

		case 47: // zc_outs_range_proof: bpp_serialized + aggregation_proof
			data.bppProofBytes = readBPPBytes(dec)
			if dec.Err() != nil {
				return nil, fmt.Errorf("parse BPP proof: %w", dec.Err())
			}
			data.bppCommitments = readAggregationCommitments(dec)
			if dec.Err() != nil {
				return nil, fmt.Errorf("parse aggregation proof: %w", dec.Err())
			}

		case 48: // zc_balance_proof: 96 bytes (c, y0, y1)
			data.balanceProof = dec.ReadBytes(96)
			if dec.Err() != nil {
				return nil, fmt.Errorf("parse balance proof: %w", dec.Err())
			}

		default:
			return nil, fmt.Errorf("unsupported proof tag 0x%02x", tag)
		}
	}

	return &data, nil
}

// readBGEProofBytes reads a BGE_proof_s and returns the raw wire bytes.
// Wire: A(32) + B(32) + vec(Pk) + vec(f) + y(32) + z(32).
func readBGEProofBytes(dec *wire.Decoder) []byte {
	var raw []byte

	// A + B
	ab := dec.ReadBytes(64)
	raw = append(raw, ab...)

	// Pk vector
	pkCount := dec.ReadVarint()
	if dec.Err() != nil {
		return nil
	}
	raw = append(raw, wire.EncodeVarint(pkCount)...)
	if pkCount > 0 {
		pkData := dec.ReadBytes(int(pkCount) * 32)
		raw = append(raw, pkData...)
	}

	// f vector
	fCount := dec.ReadVarint()
	if dec.Err() != nil {
		return nil
	}
	raw = append(raw, wire.EncodeVarint(fCount)...)
	if fCount > 0 {
		fData := dec.ReadBytes(int(fCount) * 32)
		raw = append(raw, fData...)
	}

	// y + z
	yz := dec.ReadBytes(64)
	raw = append(raw, yz...)

	return raw
}

// readBPPBytes reads a bpp_signature_serialized and returns the raw wire bytes.
// Wire: vec(L) + vec(R) + A0(32) + A(32) + B(32) + r(32) + s(32) + delta(32).
func readBPPBytes(dec *wire.Decoder) []byte {
	var raw []byte

	// L vector
	lCount := dec.ReadVarint()
	if dec.Err() != nil {
		return nil
	}
	raw = append(raw, wire.EncodeVarint(lCount)...)
	if lCount > 0 {
		raw = append(raw, dec.ReadBytes(int(lCount)*32)...)
	}

	// R vector
	rCount := dec.ReadVarint()
	if dec.Err() != nil {
		return nil
	}
	raw = append(raw, wire.EncodeVarint(rCount)...)
	if rCount > 0 {
		raw = append(raw, dec.ReadBytes(int(rCount)*32)...)
	}

	// 6 fixed scalars
	raw = append(raw, dec.ReadBytes(6*32)...)

	return raw
}

// readAggregationCommitments reads the aggregation proof and extracts
// the amount_commitments_for_rp_aggregation (the first vector).
// Wire: vec(commitments) + vec(y0s) + vec(y1s) + c(32).
func readAggregationCommitments(dec *wire.Decoder) [][32]byte {
	// Read commitments vector.
	nCommit := dec.ReadVarint()
	if dec.Err() != nil {
		return nil
	}
	commits := make([][32]byte, nCommit)
	for i := uint64(0); i < nCommit; i++ {
		dec.ReadBlob32(&commits[i])
	}

	// Skip y0s vector.
	skipVecOfPoints(dec)
	// Skip y1s vector.
	skipVecOfPoints(dec)
	// Skip c scalar.
	_ = dec.ReadBytes(32)

	return commits
}
