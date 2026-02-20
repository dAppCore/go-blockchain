// SPDX-Licence-Identifier: EUPL-1.2
// Thin C wrappers around CryptoNote C++ crypto library.
// This is the implementation of bridge.h.

#include "bridge.h"

#include <cstring>
#include "crypto.h"
#include "hash-ops.h"

extern "C" {

void bridge_fast_hash(const uint8_t *data, size_t len, uint8_t hash[32]) {
    crypto::cn_fast_hash(data, len, reinterpret_cast<char*>(hash));
}

} // extern "C"
