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

// ── Key Derivation ────────────────────────────────────────

int cn_generate_key_derivation(const uint8_t pub[32], const uint8_t sec[32],
                               uint8_t derivation[32]) {
    crypto::public_key pk;
    crypto::secret_key sk;
    crypto::key_derivation kd;
    memcpy(&pk, pub, 32);
    memcpy(&sk, sec, 32);
    bool ok = crypto::generate_key_derivation(pk, sk, kd);
    if (!ok) return 1;
    memcpy(derivation, &kd, 32);
    return 0;
}

int cn_derive_public_key(const uint8_t derivation[32], uint64_t index,
                         const uint8_t base[32], uint8_t derived[32]) {
    crypto::key_derivation kd;
    crypto::public_key base_pk, derived_pk;
    memcpy(&kd, derivation, 32);
    memcpy(&base_pk, base, 32);
    bool ok = crypto::derive_public_key(kd, index, base_pk, derived_pk);
    if (!ok) return 1;
    memcpy(derived, &derived_pk, 32);
    return 0;
}

int cn_derive_secret_key(const uint8_t derivation[32], uint64_t index,
                         const uint8_t base[32], uint8_t derived[32]) {
    crypto::key_derivation kd;
    crypto::secret_key base_sk, derived_sk;
    memcpy(&kd, derivation, 32);
    memcpy(&base_sk, base, 32);
    crypto::derive_secret_key(kd, index, base_sk, derived_sk);
    memcpy(derived, &derived_sk, 32);
    return 0;
}

// ── Key Images ────────────────────────────────────────────

int cn_generate_key_image(const uint8_t pub[32], const uint8_t sec[32],
                          uint8_t image[32]) {
    crypto::public_key pk;
    crypto::secret_key sk;
    crypto::key_image ki;
    memcpy(&pk, pub, 32);
    memcpy(&sk, sec, 32);
    crypto::generate_key_image(pk, sk, ki);
    memcpy(image, &ki, 32);
    return 0;
}

int cn_validate_key_image(const uint8_t image[32]) {
    crypto::key_image ki;
    memcpy(&ki, image, 32);
    return crypto::validate_key_image(ki) ? 0 : 1;
}

} // extern "C"
