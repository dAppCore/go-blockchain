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

// ScReduce32 reduces a 32-byte value modulo the Ed25519 group order l.
// This is required when converting a hash output to a valid secret key scalar.
func ScReduce32(key *[32]byte) {
	C.cn_sc_reduce32((*C.uint8_t)(unsafe.Pointer(&key[0])))
}
