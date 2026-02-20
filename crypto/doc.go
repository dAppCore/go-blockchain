// SPDX-Licence-Identifier: EUPL-1.2

// Package crypto provides CryptoNote cryptographic operations via CGo
// bridge to the vendored upstream C++ library.
//
// Build the C++ library before running tests:
//
//	cmake -S crypto -B crypto/build -DCMAKE_BUILD_TYPE=Release
//	cmake --build crypto/build --parallel
package crypto
