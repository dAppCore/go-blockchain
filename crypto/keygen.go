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
