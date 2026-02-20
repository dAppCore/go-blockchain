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

int cn_generate_keys(uint8_t pub[32], uint8_t sec[32]) {
    crypto::public_key pk;
    crypto::secret_key sk;
    crypto::generate_keys(pk, sk);
    memcpy(pub, &pk, 32);
    memcpy(sec, &sk, 32);
    return 0;
}

int cn_secret_to_public(const uint8_t sec[32], uint8_t pub[32]) {
    crypto::secret_key sk;
    crypto::public_key pk;
    memcpy(&sk, sec, 32);
    bool ok = crypto::secret_key_to_public_key(sk, pk);
    if (!ok) return 1;
    memcpy(pub, &pk, 32);
    return 0;
}

int cn_check_key(const uint8_t pub[32]) {
    crypto::public_key pk;
    memcpy(&pk, pub, 32);
    return crypto::check_key(pk) ? 0 : 1;
}

} // extern "C"
