// SPDX-Licence-Identifier: EUPL-1.2

package crypto

/*
#include "bridge.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// GenerateKeys creates a new random key pair.
func GenerateKeys() (pub [32]byte, sec [32]byte, err error) {
	rc := C.cn_generate_keys(
		(*C.uint8_t)(unsafe.Pointer(&pub[0])),
		(*C.uint8_t)(unsafe.Pointer(&sec[0])),
	)
	if rc != 0 {
		err = fmt.Errorf("crypto: generate_keys failed (rc=%d)", rc)
	}
	return
}

// SecretToPublic derives the public key from a secret key.
func SecretToPublic(sec [32]byte) ([32]byte, error) {
	var pub [32]byte
	rc := C.cn_secret_to_public(
		(*C.uint8_t)(unsafe.Pointer(&sec[0])),
		(*C.uint8_t)(unsafe.Pointer(&pub[0])),
	)
	if rc != 0 {
		return pub, fmt.Errorf("crypto: secret_to_public failed (rc=%d)", rc)
	}
	return pub, nil
}

// CheckKey validates that a public key is a valid curve point.
func CheckKey(pub [32]byte) bool {
	return C.cn_check_key((*C.uint8_t)(unsafe.Pointer(&pub[0]))) == 0
}

// GenerateKeyDerivation computes the ECDH shared secret (key derivation).
func GenerateKeyDerivation(pub [32]byte, sec [32]byte) ([32]byte, error) {
	var d [32]byte
	rc := C.cn_generate_key_derivation(
		(*C.uint8_t)(unsafe.Pointer(&pub[0])),
		(*C.uint8_t)(unsafe.Pointer(&sec[0])),
		(*C.uint8_t)(unsafe.Pointer(&d[0])),
	)
	if rc != 0 {
		return d, fmt.Errorf("crypto: generate_key_derivation failed")
	}
	return d, nil
}

// DerivePublicKey derives an ephemeral public key for a transaction output.
func DerivePublicKey(derivation [32]byte, index uint64, base [32]byte) ([32]byte, error) {
	var derived [32]byte
	rc := C.cn_derive_public_key(
		(*C.uint8_t)(unsafe.Pointer(&derivation[0])),
		C.uint64_t(index),
		(*C.uint8_t)(unsafe.Pointer(&base[0])),
		(*C.uint8_t)(unsafe.Pointer(&derived[0])),
	)
	if rc != 0 {
		return derived, fmt.Errorf("crypto: derive_public_key failed")
	}
	return derived, nil
}

// DeriveSecretKey derives the ephemeral secret key for a received output.
func DeriveSecretKey(derivation [32]byte, index uint64, base [32]byte) ([32]byte, error) {
	var derived [32]byte
	rc := C.cn_derive_secret_key(
		(*C.uint8_t)(unsafe.Pointer(&derivation[0])),
		C.uint64_t(index),
		(*C.uint8_t)(unsafe.Pointer(&base[0])),
		(*C.uint8_t)(unsafe.Pointer(&derived[0])),
	)
	if rc != 0 {
		return derived, fmt.Errorf("crypto: derive_secret_key failed")
	}
	return derived, nil
}
