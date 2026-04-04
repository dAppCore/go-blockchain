// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

// Package crypto provides the CGo bridge to the C++ cryptographic library.
//
// CGo Patterns (will move to go-cgo when available):
//
// Buffer allocation:
//
//	var buf [32]byte
//	C.my_function((*C.uint8_t)(unsafe.Pointer(&buf[0])), C.size_t(32))
//	result := buf[:] // Go slice from C buffer
//
// Error handling:
//
//	ret := C.crypto_function(args...)
//	if ret != 0 {
//	    return coreerr.E("crypto.Function", "operation failed", nil)
//	}
//
// String conversion:
//
//	// Go → C: pass as pointer + length (no null terminator needed)
//	C.func((*C.uint8_t)(unsafe.Pointer(&data[0])), C.size_t(len(data)))
//
//	// C → Go: copy into Go slice immediately
//	copy(result[:], C.GoBytes(unsafe.Pointer(cPtr), C.int(32)))
//
// All 29 functions follow these patterns. See bridge.h for the C API.
// When go-cgo ships, these patterns become helper functions:
//
//	buf := cgo.NewBuffer(32)
//	defer buf.Free()
//	err := cgo.Call(C.my_function, buf.Ptr(), buf.Size())
package crypto
