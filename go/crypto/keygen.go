// SPDX-Licence-Identifier: EUPL-1.2

package crypto

/*
#include "bridge.h"
*/
import "C"

import (
	"unsafe"

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
)

// GenerateKeys creates a new random key pair.
// Usage: crypto.GenerateKeys(...)
func GenerateKeys() (pub [32]byte, sec [32]byte, err error) {
	rc := C.cn_generate_keys(
		(*C.uint8_t)(unsafe.Pointer(&pub[0])),
		(*C.uint8_t)(unsafe.Pointer(&sec[0])),
	)
	if rc != 0 {
		err = coreerr.E("GenerateKeys", core.Sprintf("generate_keys failed (rc=%d)", rc), nil)
	}
	return
}

// SecretToPublic derives the public key from a secret key.
// Usage: crypto.SecretToPublic(...)
func SecretToPublic(sec [32]byte) ([32]byte, error) {
	var pub [32]byte
	rc := C.cn_secret_to_public(
		(*C.uint8_t)(unsafe.Pointer(&sec[0])),
		(*C.uint8_t)(unsafe.Pointer(&pub[0])),
	)
	if rc != 0 {
		return pub, coreerr.E("SecretToPublic", core.Sprintf("secret_to_public failed (rc=%d)", rc), nil)
	}
	return pub, nil
}

// CheckKey validates that a public key is a valid curve point.
// Usage: crypto.CheckKey(...)
func CheckKey(pub [32]byte) bool {
	return C.cn_check_key((*C.uint8_t)(unsafe.Pointer(&pub[0]))) == 0
}

// GenerateKeyDerivation computes the ECDH shared secret (key derivation).
// Usage: crypto.GenerateKeyDerivation(...)
func GenerateKeyDerivation(pub [32]byte, sec [32]byte) ([32]byte, error) {
	var d [32]byte
	rc := C.cn_generate_key_derivation(
		(*C.uint8_t)(unsafe.Pointer(&pub[0])),
		(*C.uint8_t)(unsafe.Pointer(&sec[0])),
		(*C.uint8_t)(unsafe.Pointer(&d[0])),
	)
	if rc != 0 {
		return d, coreerr.E("GenerateKeyDerivation", "generate_key_derivation failed", nil)
	}
	return d, nil
}

// DerivePublicKey derives an ephemeral public key for a transaction output.
// Usage: crypto.DerivePublicKey(...)
func DerivePublicKey(derivation [32]byte, index uint64, base [32]byte) ([32]byte, error) {
	var derived [32]byte
	rc := C.cn_derive_public_key(
		(*C.uint8_t)(unsafe.Pointer(&derivation[0])),
		C.uint64_t(index),
		(*C.uint8_t)(unsafe.Pointer(&base[0])),
		(*C.uint8_t)(unsafe.Pointer(&derived[0])),
	)
	if rc != 0 {
		return derived, coreerr.E("DerivePublicKey", "derive_public_key failed", nil)
	}
	return derived, nil
}

// DeriveSecretKey derives the ephemeral secret key for a received output.
// Usage: crypto.DeriveSecretKey(...)
func DeriveSecretKey(derivation [32]byte, index uint64, base [32]byte) ([32]byte, error) {
	var derived [32]byte
	rc := C.cn_derive_secret_key(
		(*C.uint8_t)(unsafe.Pointer(&derivation[0])),
		C.uint64_t(index),
		(*C.uint8_t)(unsafe.Pointer(&base[0])),
		(*C.uint8_t)(unsafe.Pointer(&derived[0])),
	)
	if rc != 0 {
		return derived, coreerr.E("DeriveSecretKey", "derive_secret_key failed", nil)
	}
	return derived, nil
}
