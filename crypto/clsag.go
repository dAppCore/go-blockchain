// SPDX-Licence-Identifier: EUPL-1.2

package crypto

/*
#include "bridge.h"
*/
import "C"

import (
	"unsafe"

	coreerr "dappco.re/go/core/log"
)

// PointMul8 multiplies a curve point by the cofactor 8.
func PointMul8(pk [32]byte) ([32]byte, error) {
	var result [32]byte
	rc := C.cn_point_mul8(
		(*C.uint8_t)(unsafe.Pointer(&pk[0])),
		(*C.uint8_t)(unsafe.Pointer(&result[0])),
	)
	if rc != 0 {
		return result, coreerr.E("PointMul8", "point_mul8 failed", nil)
	}
	return result, nil
}

// PointDiv8 premultiplies a curve point by 1/8 (cofactor inverse).
// This is the on-chain storage form for commitments and key images.
func PointDiv8(pk [32]byte) ([32]byte, error) {
	var result [32]byte
	rc := C.cn_point_div8(
		(*C.uint8_t)(unsafe.Pointer(&pk[0])),
		(*C.uint8_t)(unsafe.Pointer(&result[0])),
	)
	if rc != 0 {
		return result, coreerr.E("PointDiv8", "point_div8 failed", nil)
	}
	return result, nil
}

// PointSub computes a - b on the Ed25519 curve.
func PointSub(a, b [32]byte) ([32]byte, error) {
	var result [32]byte
	rc := C.cn_point_sub(
		(*C.uint8_t)(unsafe.Pointer(&a[0])),
		(*C.uint8_t)(unsafe.Pointer(&b[0])),
		(*C.uint8_t)(unsafe.Pointer(&result[0])),
	)
	if rc != 0 {
		return result, coreerr.E("PointSub", "point_sub failed", nil)
	}
	return result, nil
}

// CLSAGGGSigSize returns the byte size of a CLSAG_GG signature for a given ring size.
func CLSAGGGSigSize(ringSize int) int {
	return int(C.cn_clsag_gg_sig_size(C.size_t(ringSize)))
}

// GenerateCLSAGGG creates a CLSAG_GG ring signature.
// ring is a flat slice of [stealth_addr(32) | amount_commitment(32)] per entry.
// pseudoOut is the pseudo output commitment (not premultiplied by 1/8).
// secretX and secretF are the secret scalars for the signer.
func GenerateCLSAGGG(hash [32]byte, ring []byte, ringSize int,
	pseudoOut [32]byte, ki [32]byte,
	secretX [32]byte, secretF [32]byte, secretIndex int) ([]byte, error) {

	sigLen := CLSAGGGSigSize(ringSize)
	sig := make([]byte, sigLen)

	rc := C.cn_clsag_gg_generate(
		(*C.uint8_t)(unsafe.Pointer(&hash[0])),
		(*C.uint8_t)(unsafe.Pointer(&ring[0])),
		C.size_t(ringSize),
		(*C.uint8_t)(unsafe.Pointer(&pseudoOut[0])),
		(*C.uint8_t)(unsafe.Pointer(&ki[0])),
		(*C.uint8_t)(unsafe.Pointer(&secretX[0])),
		(*C.uint8_t)(unsafe.Pointer(&secretF[0])),
		C.size_t(secretIndex),
		(*C.uint8_t)(unsafe.Pointer(&sig[0])),
	)
	if rc != 0 {
		return nil, coreerr.E("GenerateCLSAGGG", "generate_CLSAG_GG failed", nil)
	}
	return sig, nil
}

// VerifyCLSAGGG verifies a CLSAG_GG ring signature.
// ring is a flat slice of [stealth_addr(32) | amount_commitment(32)] per entry.
// pseudoOut is the pseudo output commitment (premultiplied by 1/8).
func VerifyCLSAGGG(hash [32]byte, ring []byte, ringSize int,
	pseudoOut [32]byte, ki [32]byte, sig []byte) bool {

	return C.cn_clsag_gg_verify(
		(*C.uint8_t)(unsafe.Pointer(&hash[0])),
		(*C.uint8_t)(unsafe.Pointer(&ring[0])),
		C.size_t(ringSize),
		(*C.uint8_t)(unsafe.Pointer(&pseudoOut[0])),
		(*C.uint8_t)(unsafe.Pointer(&ki[0])),
		(*C.uint8_t)(unsafe.Pointer(&sig[0])),
	) == 0
}

// CLSAGGGXSigSize returns the byte size of a CLSAG_GGX signature for a given ring size.
func CLSAGGGXSigSize(ringSize int) int {
	return int(C.cn_clsag_ggx_sig_size(C.size_t(ringSize)))
}

// VerifyCLSAGGGX verifies a CLSAG_GGX ring signature.
// ring is a flat slice of [stealth(32) | commitment(32) | blinded_asset_id(32)] per entry.
func VerifyCLSAGGGX(hash [32]byte, ring []byte, ringSize int,
	pseudoOutCommitment [32]byte, pseudoOutAssetID [32]byte,
	ki [32]byte, sig []byte) bool {

	return C.cn_clsag_ggx_verify(
		(*C.uint8_t)(unsafe.Pointer(&hash[0])),
		(*C.uint8_t)(unsafe.Pointer(&ring[0])),
		C.size_t(ringSize),
		(*C.uint8_t)(unsafe.Pointer(&pseudoOutCommitment[0])),
		(*C.uint8_t)(unsafe.Pointer(&pseudoOutAssetID[0])),
		(*C.uint8_t)(unsafe.Pointer(&ki[0])),
		(*C.uint8_t)(unsafe.Pointer(&sig[0])),
	) == 0
}

// CLSAGGGXXGSigSize returns the byte size of a CLSAG_GGXXG signature for a given ring size.
func CLSAGGGXXGSigSize(ringSize int) int {
	return int(C.cn_clsag_ggxxg_sig_size(C.size_t(ringSize)))
}

// VerifyCLSAGGGXXG verifies a CLSAG_GGXXG ring signature.
// ring is a flat slice of [stealth(32) | commitment(32) | blinded_asset_id(32) | concealing(32)] per entry.
func VerifyCLSAGGGXXG(hash [32]byte, ring []byte, ringSize int,
	pseudoOutCommitment [32]byte, pseudoOutAssetID [32]byte,
	extendedCommitment [32]byte, ki [32]byte, sig []byte) bool {

	return C.cn_clsag_ggxxg_verify(
		(*C.uint8_t)(unsafe.Pointer(&hash[0])),
		(*C.uint8_t)(unsafe.Pointer(&ring[0])),
		C.size_t(ringSize),
		(*C.uint8_t)(unsafe.Pointer(&pseudoOutCommitment[0])),
		(*C.uint8_t)(unsafe.Pointer(&pseudoOutAssetID[0])),
		(*C.uint8_t)(unsafe.Pointer(&extendedCommitment[0])),
		(*C.uint8_t)(unsafe.Pointer(&ki[0])),
		(*C.uint8_t)(unsafe.Pointer(&sig[0])),
	) == 0
}
