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

// GenerateKeyImage computes the key image for a public/secret key pair.
// The key image is used for double-spend detection in ring signatures.
func GenerateKeyImage(pub [32]byte, sec [32]byte) ([32]byte, error) {
	var ki [32]byte
	rc := C.cn_generate_key_image(
		(*C.uint8_t)(unsafe.Pointer(&pub[0])),
		(*C.uint8_t)(unsafe.Pointer(&sec[0])),
		(*C.uint8_t)(unsafe.Pointer(&ki[0])),
	)
	if rc != 0 {
		return ki, coreerr.E("GenerateKeyImage", "generate_key_image failed", nil)
	}
	return ki, nil
}

// ValidateKeyImage checks that a key image is a valid curve point of the correct order.
func ValidateKeyImage(ki [32]byte) bool {
	return C.cn_validate_key_image((*C.uint8_t)(unsafe.Pointer(&ki[0]))) == 0
}
