// SPDX-Licence-Identifier: EUPL-1.2

package crypto

/*
#include "bridge.h"
*/
import "C"

import "unsafe"

// VerifyBPPE verifies a Bulletproofs+ Enhanced range proof.
// Returns true if the proof is valid, false otherwise.
// Currently returns false (not implemented) — needs on-chain binary format
// deserialiser. Full implementation arrives in Phase 4 with RPC + chain data.
func VerifyBPPE(proof []byte, commitments [][32]byte) bool {
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
// Currently returns false (not implemented).
func VerifyBGE(context [32]byte, ring [][32]byte, proof []byte) bool {
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
// Currently returns false (not implemented).
func VerifyZarcanum(hash [32]byte, proof []byte) bool {
	return C.cn_zarcanum_verify(
		(*C.uint8_t)(unsafe.Pointer(&hash[0])),
		(*C.uint8_t)(unsafe.Pointer(&proof[0])),
		C.size_t(len(proof)),
	) == 0
}
