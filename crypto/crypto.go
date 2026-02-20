// SPDX-Licence-Identifier: EUPL-1.2

package crypto

/*
#cgo CPPFLAGS: -I${SRCDIR}/upstream -I${SRCDIR}/compat
#cgo LDFLAGS: -L${SRCDIR}/build -lcryptonote -lstdc++ -lssl -lcrypto
#include "bridge.h"
*/
import "C"

import "unsafe"

// FastHash computes the CryptoNote fast hash (Keccak-256) of the input.
func FastHash(data []byte) [32]byte {
	var hash [32]byte
	if len(data) == 0 {
		C.bridge_fast_hash(nil, 0, (*C.uint8_t)(unsafe.Pointer(&hash[0])))
	} else {
		C.bridge_fast_hash((*C.uint8_t)(unsafe.Pointer(&data[0])),
			C.size_t(len(data)),
			(*C.uint8_t)(unsafe.Pointer(&hash[0])))
	}
	return hash
}
