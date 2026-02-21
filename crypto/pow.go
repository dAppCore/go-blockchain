// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-Licence-Identifier: EUPL-1.2

package crypto

// #include "bridge.h"
import "C"
import (
	"fmt"
	"unsafe"
)

// RandomXHash computes the RandomX PoW hash. The key is the cache
// initialisation key (e.g. "LetheanRandomXv1"). Input is typically
// the block header hash (32 bytes) concatenated with the nonce (8 bytes LE).
func RandomXHash(key, input []byte) ([32]byte, error) {
	var output [32]byte
	ret := C.bridge_randomx_hash(
		(*C.uint8_t)(unsafe.Pointer(&key[0])), C.size_t(len(key)),
		(*C.uint8_t)(unsafe.Pointer(&input[0])), C.size_t(len(input)),
		(*C.uint8_t)(unsafe.Pointer(&output[0])),
	)
	if ret != 0 {
		return output, fmt.Errorf("crypto: RandomX hash failed with code %d", ret)
	}
	return output, nil
}
