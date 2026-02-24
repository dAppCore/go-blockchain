// SPDX-Licence-Identifier: EUPL-1.2

package crypto

/*
#include "bridge.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

// GenerateSignature creates a standard (non-ring) signature.
func GenerateSignature(hash [32]byte, pub [32]byte, sec [32]byte) ([64]byte, error) {
	var sig [64]byte
	rc := C.cn_generate_signature(
		(*C.uint8_t)(unsafe.Pointer(&hash[0])),
		(*C.uint8_t)(unsafe.Pointer(&pub[0])),
		(*C.uint8_t)(unsafe.Pointer(&sec[0])),
		(*C.uint8_t)(unsafe.Pointer(&sig[0])),
	)
	if rc != 0 {
		return sig, errors.New("crypto: generate_signature failed")
	}
	return sig, nil
}

// CheckSignature verifies a standard signature.
func CheckSignature(hash [32]byte, pub [32]byte, sig [64]byte) bool {
	return C.cn_check_signature(
		(*C.uint8_t)(unsafe.Pointer(&hash[0])),
		(*C.uint8_t)(unsafe.Pointer(&pub[0])),
		(*C.uint8_t)(unsafe.Pointer(&sig[0])),
	) == 0
}

// GenerateRingSignature creates a ring signature using the given key ring.
// pubs contains the public keys of all ring members.
// sec is the secret key of the actual signer at position secIndex.
// Returns one signature pair per ring member.
func GenerateRingSignature(hash [32]byte, image [32]byte, pubs [][32]byte,
	sec [32]byte, secIndex int) ([][64]byte, error) {

	n := len(pubs)
	flatPubs := make([]byte, n*32)
	for i, p := range pubs {
		copy(flatPubs[i*32:], p[:])
	}

	flatSigs := make([]byte, n*64)
	rc := C.cn_generate_ring_signature(
		(*C.uint8_t)(unsafe.Pointer(&hash[0])),
		(*C.uint8_t)(unsafe.Pointer(&image[0])),
		(*C.uint8_t)(unsafe.Pointer(&flatPubs[0])),
		C.size_t(n),
		(*C.uint8_t)(unsafe.Pointer(&sec[0])),
		C.size_t(secIndex),
		(*C.uint8_t)(unsafe.Pointer(&flatSigs[0])),
	)
	if rc != 0 {
		return nil, errors.New("crypto: generate_ring_signature failed")
	}

	sigs := make([][64]byte, n)
	for i := range sigs {
		copy(sigs[i][:], flatSigs[i*64:])
	}
	return sigs, nil
}

// CheckRingSignature verifies a ring signature.
func CheckRingSignature(hash [32]byte, image [32]byte, pubs [][32]byte,
	sigs [][64]byte) bool {

	n := len(pubs)
	if len(sigs) != n {
		return false
	}

	flatPubs := make([]byte, n*32)
	for i, p := range pubs {
		copy(flatPubs[i*32:], p[:])
	}

	flatSigs := make([]byte, n*64)
	for i, s := range sigs {
		copy(flatSigs[i*64:], s[:])
	}

	return C.cn_check_ring_signature(
		(*C.uint8_t)(unsafe.Pointer(&hash[0])),
		(*C.uint8_t)(unsafe.Pointer(&image[0])),
		(*C.uint8_t)(unsafe.Pointer(&flatPubs[0])),
		C.size_t(n),
		(*C.uint8_t)(unsafe.Pointer(&flatSigs[0])),
	) == 0
}
