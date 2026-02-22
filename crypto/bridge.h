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

// ── Scalar Operations ────────────────────────────────────
// Reduce a 32-byte scalar modulo the Ed25519 group order l.
void cn_sc_reduce32(uint8_t key[32]);

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

// Subtract two curve points: result = a - b.
int cn_point_sub(const uint8_t a[32], const uint8_t b[32], uint8_t result[32]);

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

// ── Range Proofs (BPP — Bulletproofs++) ──────────────────
// Verifies a BPP range proof (1 delta). Used for zc_outs_range_proof in
// post-HF4 transactions. proof is the wire-serialised bpp_signature:
//   varint(len(L)) + L[]*32 + varint(len(R)) + R[]*32
//   + A0(32) + A(32) + B(32) + r(32) + s(32) + delta(32)
// commitments is a flat array of 32-byte public keys — the
// amount_commitments_for_rp_aggregation (E'_j, premultiplied by 1/8).
// Uses bpp_crypto_trait_ZC_out (generators UGX, N=64, values_max=32).
// Returns 0 on success, 1 on verification failure or deserialisation error.
int cn_bpp_verify(const uint8_t *proof, size_t proof_len,
                  const uint8_t *commitments, size_t num_commitments);

// ── Range Proofs (BPPE — Bulletproofs++ Enhanced) ────────
// Verifies a BPPE range proof (2 deltas). Used for Zarcanum PoS E_range_proof.
// proof is the wire-serialised bppe_signature:
//   varint(len(L)) + L[]*32 + varint(len(R)) + R[]*32
//   + A0(32) + A(32) + B(32) + r(32) + s(32) + delta_1(32) + delta_2(32)
// commitments is a flat array of 32-byte public keys (premultiplied by 1/8).
// Uses bpp_crypto_trait_Zarcanum (N=128, values_max=16).
// Returns 0 on success, 1 on verification failure or deserialisation error.
int cn_bppe_verify(const uint8_t *proof, size_t proof_len,
                   const uint8_t *commitments, size_t num_commitments);

// ── BGE One-out-of-Many ───────────────────────────────────
// Verifies a BGE one-out-of-many proof. proof is wire-serialised BGE_proof:
//   A(32) + B(32) + varint(len(Pk)) + Pk[]*32
//   + varint(len(f)) + f[]*32 + y(32) + z(32)
// ring is a flat array of 32-byte public keys. context is a 32-byte hash.
// Returns 0 on success, 1 on verification failure or deserialisation error.
int cn_bge_verify(const uint8_t context[32], const uint8_t *ring,
                  size_t ring_size, const uint8_t *proof, size_t proof_len);

// ── Zarcanum PoS ──────────────────────────────────────────
// TODO: extend API to accept kernel_hash, ring, last_pow_block_id,
// stake_ki, pos_difficulty. Currently returns -1 (not implemented).
int cn_zarcanum_verify(const uint8_t hash[32], const uint8_t *proof,
                       size_t proof_len);

// ── RandomX PoW Hashing ──────────────────────────────────
// key/key_size: RandomX cache key (e.g. "LetheanRandomXv1")
// input/input_size: block header hash (32 bytes) + nonce (8 bytes LE)
// output: 32-byte hash result
// Returns 0 on success.
int bridge_randomx_hash(const uint8_t* key, size_t key_size,
                        const uint8_t* input, size_t input_size,
                        uint8_t* output);

#ifdef __cplusplus
}
#endif
#endif // CRYPTONOTE_BRIDGE_H
