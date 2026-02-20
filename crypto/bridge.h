// SPDX-Licence-Identifier: EUPL-1.2
// Stable C API for go-blockchain CGo bindings.
// Go code calls ONLY these functions — no C++ types cross this boundary.
#ifndef CRYPTONOTE_BRIDGE_H
#define CRYPTONOTE_BRIDGE_H

#include <stdint.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

// ── Hashing ───────────────────────────────────────────────
void bridge_fast_hash(const uint8_t *data, size_t len, uint8_t hash[32]);

// ── Key Operations ────────────────────────────────────────
int cn_generate_keys(uint8_t pub[32], uint8_t sec[32]);
int cn_secret_to_public(const uint8_t sec[32], uint8_t pub[32]);
int cn_check_key(const uint8_t pub[32]);

#ifdef __cplusplus
}
#endif
#endif // CRYPTONOTE_BRIDGE_H
