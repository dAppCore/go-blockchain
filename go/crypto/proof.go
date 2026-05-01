// SPDX-Licence-Identifier: EUPL-1.2

package crypto

/*
#include "bridge.h"
*/
import "C"

import "unsafe"

// VerifyBPP verifies a Bulletproofs++ range proof (1 delta).
// Used for zc_outs_range_proof in post-HF4 transactions.
// proof is the wire-serialised bpp_signature blob.
// commitments are the amount_commitments_for_rp_aggregation (E'_j, premultiplied by 1/8).
// Uses bpp_crypto_trait_ZC_out (generators UGX, N=64, values_max=32).
// Usage: crypto.VerifyBPP(...)
func VerifyBPP(proof []byte, commitments [][32]byte) bool {
	if len(proof) == 0 || len(commitments) == 0 {
		return false
	}
	n := len(commitments)
	flat := make([]byte, n*32)
	for i, c := range commitments {
		copy(flat[i*32:], c[:])
	}
	return C.cn_bpp_verify(
		(*C.uint8_t)(unsafe.Pointer(&proof[0])),
		C.size_t(len(proof)),
		(*C.uint8_t)(unsafe.Pointer(&flat[0])),
		C.size_t(n),
	) == 0
}

// VerifyBPPE verifies a Bulletproofs++ Enhanced range proof (2 deltas).
// Used for Zarcanum PoS E_range_proof.
// proof is the wire-serialised bppe_signature blob.
// commitments are the output amount commitments (premultiplied by 1/8).
// Uses bpp_crypto_trait_Zarcanum (N=128, values_max=16).
// Usage: crypto.VerifyBPPE(...)
func VerifyBPPE(proof []byte, commitments [][32]byte) bool {
	if len(proof) == 0 || len(commitments) == 0 {
		return false
	}
	n := len(commitments)
	flat := make([]byte, n*32)
	for i, c := range commitments {
		copy(flat[i*32:], c[:])
	}
	return C.cn_bppe_verify(
		(*C.uint8_t)(unsafe.Pointer(&proof[0])),
		C.size_t(len(proof)),
		(*C.uint8_t)(unsafe.Pointer(&flat[0])),
		C.size_t(n),
	) == 0
}

// VerifyBGE verifies a BGE one-out-of-many proof.
// context is a 32-byte hash. ring is the set of public keys.
// proof is the wire-serialised BGE_proof blob.
// Usage: crypto.VerifyBGE(...)
func VerifyBGE(context [32]byte, ring [][32]byte, proof []byte) bool {
	if len(ring) == 0 || len(proof) == 0 {
		return false
	}
	n := len(ring)
	flat := make([]byte, n*32)
	for i, r := range ring {
		copy(flat[i*32:], r[:])
	}
	return C.cn_bge_verify(
		(*C.uint8_t)(unsafe.Pointer(&context[0])),
		(*C.uint8_t)(unsafe.Pointer(&flat[0])),
		C.size_t(n),
		(*C.uint8_t)(unsafe.Pointer(&proof[0])),
		C.size_t(len(proof)),
	) == 0
}

// VerifyZarcanum verifies a Zarcanum PoS proof.
// Currently returns false — bridge API needs extending to pass kernel_hash,
// ring, last_pow_block_id, stake_ki, and pos_difficulty.
// Usage: crypto.VerifyZarcanum(...)
func VerifyZarcanum(hash [32]byte, proof []byte) bool {
	if len(proof) == 0 {
		return false
	}
	return C.cn_zarcanum_verify(
		(*C.uint8_t)(unsafe.Pointer(&hash[0])),
		(*C.uint8_t)(unsafe.Pointer(&proof[0])),
		C.size_t(len(proof)),
	) == 0
}
