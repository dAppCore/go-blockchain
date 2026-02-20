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

// ── Key Derivation ────────────────────────────────────────
int cn_generate_key_derivation(const uint8_t pub[32], const uint8_t sec[32],
                               uint8_t derivation[32]);
int cn_derive_public_key(const uint8_t derivation[32], uint64_t index,
                         const uint8_t base[32], uint8_t derived[32]);
int cn_derive_secret_key(const uint8_t derivation[32], uint64_t index,
                         const uint8_t base[32], uint8_t derived[32]);

// ── Key Images ────────────────────────────────────────────
int cn_generate_key_image(const uint8_t pub[32], const uint8_t sec[32],
                          uint8_t image[32]);
int cn_validate_key_image(const uint8_t image[32]);

// ── Standard Signatures ───────────────────────────────────
int cn_generate_signature(const uint8_t hash[32], const uint8_t pub[32],
                          const uint8_t sec[32], uint8_t sig[64]);
int cn_check_signature(const uint8_t hash[32], const uint8_t pub[32],
                       const uint8_t sig[64]);

// ── Ring Signatures (NLSAG) ──────────────────────────────
int cn_generate_ring_signature(const uint8_t hash[32], const uint8_t image[32],
                               const uint8_t *pubs, size_t pubs_count,
                               const uint8_t sec[32], size_t sec_index,
                               uint8_t *sigs);
int cn_check_ring_signature(const uint8_t hash[32], const uint8_t image[32],
                            const uint8_t *pubs, size_t pubs_count,
                            const uint8_t *sigs);

// ── Point Helpers ─────────────────────────────────────────
// Multiply a curve point by the cofactor 8 (for clearing small subgroup component).
int cn_point_mul8(const uint8_t pk[32], uint8_t result[32]);
// Premultiply by 1/8 (cofactor inverse). Stored form on-chain.
int cn_point_div8(const uint8_t pk[32], uint8_t result[32]);

// ── CLSAG Verification (HF4+) ────────────────────────────
// Ring entries are flat arrays of 32-byte public keys per entry:
//   GG:    [stealth_addr(32) | amount_commitment(32)] per entry = 64 bytes
//   GGX:   [stealth(32) | commitment(32) | blinded_asset_id(32)] = 96 bytes
//   GGXXG: [stealth(32) | commitment(32) | blinded_asset_id(32) | concealing(32)] = 128 bytes
// Signature layout (flat):
//   GG:    c(32) | r[ring_size*32] | K1(32) = 64 + ring_size*32
//   GGX:   c(32) | r_g[ring_size*32] | r_x[ring_size*32] | K1(32) | K2(32) = 96 + ring_size*64
//   GGXXG: c(32) | r_g[ring_size*32] | r_x[ring_size*32] | K1(32) | K2(32) | K3(32) | K4(32) = 160 + ring_size*64

size_t cn_clsag_gg_sig_size(size_t ring_size);
int cn_clsag_gg_generate(const uint8_t hash[32], const uint8_t *ring,
                          size_t ring_size, const uint8_t pseudo_out[32],
                          const uint8_t ki[32], const uint8_t secret_x[32],
                          const uint8_t secret_f[32], size_t secret_index,
                          uint8_t *sig);
int cn_clsag_gg_verify(const uint8_t hash[32], const uint8_t *ring,
                        size_t ring_size, const uint8_t pseudo_out[32],
                        const uint8_t ki[32], const uint8_t *sig);

size_t cn_clsag_ggx_sig_size(size_t ring_size);
int cn_clsag_ggx_verify(const uint8_t hash[32], const uint8_t *ring,
                         size_t ring_size, const uint8_t pseudo_out_commitment[32],
                         const uint8_t pseudo_out_asset_id[32],
                         const uint8_t ki[32], const uint8_t *sig);

size_t cn_clsag_ggxxg_sig_size(size_t ring_size);
int cn_clsag_ggxxg_verify(const uint8_t hash[32], const uint8_t *ring,
                           size_t ring_size, const uint8_t pseudo_out_commitment[32],
                           const uint8_t pseudo_out_asset_id[32],
                           const uint8_t extended_commitment[32],
                           const uint8_t ki[32], const uint8_t *sig);

// ── Range Proofs (Bulletproofs+ Enhanced) ─────────────────
// Proof verification requires deserialising variable-length BPPE structs from
// on-chain binary format. Implementation deferred to Phase 4 (needs RPC + chain data).
// Returns 0 on success, 1 on verification failure, -1 if not implemented.
int cn_bppe_verify(const uint8_t *proof, size_t proof_len,
                   const uint8_t *commitments, size_t num_commitments);

// ── BGE One-out-of-Many ───────────────────────────────────
int cn_bge_verify(const uint8_t context[32], const uint8_t *ring,
                  size_t ring_size, const uint8_t *proof, size_t proof_len);

// ── Zarcanum PoS ──────────────────────────────────────────
int cn_zarcanum_verify(const uint8_t hash[32], const uint8_t *proof,
                       size_t proof_len);

#ifdef __cplusplus
}
#endif
#endif // CRYPTONOTE_BRIDGE_H
