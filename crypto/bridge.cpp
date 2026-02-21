// SPDX-Licence-Identifier: EUPL-1.2
// Thin C wrappers around CryptoNote C++ crypto library.
// This is the implementation of bridge.h.

#include "bridge.h"

#include <cstring>
#include <vector>
#include "crypto.h"
#include "crypto-sugar.h"
#include "crypto-ops.h"
#include "clsag.h"
#include "hash-ops.h"
#include "randomx.h"

extern "C" {

void bridge_fast_hash(const uint8_t *data, size_t len, uint8_t hash[32]) {
    crypto::cn_fast_hash(data, len, reinterpret_cast<char*>(hash));
}

// ── Scalar Operations ────────────────────────────────────

void cn_sc_reduce32(uint8_t key[32]) {
    crypto::sc_reduce32(key);
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

// ── Standard Signatures ──────────────────────────────────

int cn_generate_signature(const uint8_t hash[32], const uint8_t pub[32],
                          const uint8_t sec[32], uint8_t sig[64]) {
    crypto::hash h;
    crypto::public_key pk;
    crypto::secret_key sk;
    crypto::signature s;
    memcpy(&h, hash, 32);
    memcpy(&pk, pub, 32);
    memcpy(&sk, sec, 32);
    crypto::generate_signature(h, pk, sk, s);
    memcpy(sig, &s, 64);
    return 0;
}

int cn_check_signature(const uint8_t hash[32], const uint8_t pub[32],
                       const uint8_t sig[64]) {
    crypto::hash h;
    crypto::public_key pk;
    crypto::signature s;
    memcpy(&h, hash, 32);
    memcpy(&pk, pub, 32);
    memcpy(&s, sig, 64);
    return crypto::check_signature(h, pk, s) ? 0 : 1;
}

// ── Ring Signatures (NLSAG) ─────────────────────────────

int cn_generate_ring_signature(const uint8_t hash[32], const uint8_t image[32],
                               const uint8_t *pubs, size_t pubs_count,
                               const uint8_t sec[32], size_t sec_index,
                               uint8_t *sigs) {
    crypto::hash h;
    crypto::key_image ki;
    crypto::secret_key sk;
    memcpy(&h, hash, 32);
    memcpy(&ki, image, 32);
    memcpy(&sk, sec, 32);

    // Reconstruct pointer array from flat buffer.
    std::vector<const crypto::public_key*> pk_ptrs(pubs_count);
    std::vector<crypto::public_key> pk_storage(pubs_count);
    for (size_t i = 0; i < pubs_count; i++) {
        memcpy(&pk_storage[i], pubs + i * 32, 32);
        pk_ptrs[i] = &pk_storage[i];
    }

    std::vector<crypto::signature> sig_vec(pubs_count);
    crypto::generate_ring_signature(h, ki, pk_ptrs.data(), pubs_count,
                                    sk, sec_index, sig_vec.data());
    memcpy(sigs, sig_vec.data(), pubs_count * 64);
    return 0;
}

int cn_check_ring_signature(const uint8_t hash[32], const uint8_t image[32],
                            const uint8_t *pubs, size_t pubs_count,
                            const uint8_t *sigs) {
    crypto::hash h;
    crypto::key_image ki;
    memcpy(&h, hash, 32);
    memcpy(&ki, image, 32);

    std::vector<const crypto::public_key*> pk_ptrs(pubs_count);
    std::vector<crypto::public_key> pk_storage(pubs_count);
    for (size_t i = 0; i < pubs_count; i++) {
        memcpy(&pk_storage[i], pubs + i * 32, 32);
        pk_ptrs[i] = &pk_storage[i];
    }

    auto* sig_ptr = reinterpret_cast<const crypto::signature*>(sigs);
    return crypto::check_ring_signature(h, ki, pk_ptrs.data(), pubs_count, sig_ptr) ? 0 : 1;
}

// ── Point Helpers ────────────────────────────────────────

int cn_point_mul8(const uint8_t pk[32], uint8_t result[32]) {
    crypto::public_key src;
    memcpy(&src, pk, 32);
    crypto::point_t pt(src);
    pt.modify_mul8();
    crypto::public_key dst;
    pt.to_public_key(dst);
    memcpy(result, &dst, 32);
    return 0;
}

int cn_point_div8(const uint8_t pk[32], uint8_t result[32]) {
    crypto::public_key src;
    memcpy(&src, pk, 32);
    crypto::point_t pt(src);
    crypto::point_t div8 = crypto::c_scalar_1div8 * pt;
    crypto::public_key dst;
    div8.to_public_key(dst);
    memcpy(result, &dst, 32);
    return 0;
}

// ── CLSAG (HF4+) ────────────────────────────────────────

// Signature layout for GG: c(32) | r[N*32] | K1(32)
size_t cn_clsag_gg_sig_size(size_t ring_size) {
    return 32 + ring_size * 32 + 32;
}

int cn_clsag_gg_generate(const uint8_t hash[32], const uint8_t *ring,
                          size_t ring_size, const uint8_t pseudo_out[32],
                          const uint8_t ki[32], const uint8_t secret_x[32],
                          const uint8_t secret_f[32], size_t secret_index,
                          uint8_t *sig) {
    crypto::hash h;
    memcpy(&h, hash, 32);

    // Build ring from flat buffer: [stealth(32) | commitment(32)] per entry.
    std::vector<crypto::public_key> stealth_keys(ring_size);
    std::vector<crypto::public_key> commitments(ring_size);
    std::vector<crypto::CLSAG_GG_input_ref_t> ring_refs;
    ring_refs.reserve(ring_size);
    for (size_t i = 0; i < ring_size; i++) {
        memcpy(&stealth_keys[i], ring + i * 64, 32);
        memcpy(&commitments[i], ring + i * 64 + 32, 32);
        ring_refs.emplace_back(stealth_keys[i], commitments[i]);
    }

    // pseudo_out for generation is point_t (not premultiplied by 1/8).
    crypto::public_key po_pk;
    memcpy(&po_pk, pseudo_out, 32);
    crypto::point_t po_pt(po_pk);

    crypto::key_image key_img;
    memcpy(&key_img, ki, 32);

    crypto::scalar_t sx, sf;
    memcpy(sx.m_s, secret_x, 32);
    memcpy(sf.m_s, secret_f, 32);

    crypto::CLSAG_GG_signature clsag_sig;
    bool ok = crypto::generate_CLSAG_GG(h, ring_refs, po_pt, key_img,
                                         sx, sf, secret_index, clsag_sig);
    if (!ok) return 1;

    // Serialise: c(32) | r[N*32] | K1(32)
    uint8_t *p = sig;
    memcpy(p, clsag_sig.c.m_s, 32); p += 32;
    for (size_t i = 0; i < ring_size; i++) {
        memcpy(p, clsag_sig.r[i].m_s, 32); p += 32;
    }
    memcpy(p, &clsag_sig.K1, 32);
    return 0;
}

int cn_clsag_gg_verify(const uint8_t hash[32], const uint8_t *ring,
                        size_t ring_size, const uint8_t pseudo_out[32],
                        const uint8_t ki[32], const uint8_t *sig) {
    crypto::hash h;
    memcpy(&h, hash, 32);

    std::vector<crypto::public_key> stealth_keys(ring_size);
    std::vector<crypto::public_key> commitments(ring_size);
    std::vector<crypto::CLSAG_GG_input_ref_t> ring_refs;
    ring_refs.reserve(ring_size);
    for (size_t i = 0; i < ring_size; i++) {
        memcpy(&stealth_keys[i], ring + i * 64, 32);
        memcpy(&commitments[i], ring + i * 64 + 32, 32);
        ring_refs.emplace_back(stealth_keys[i], commitments[i]);
    }

    // pseudo_out for verification is public_key (premultiplied by 1/8).
    crypto::public_key po_pk;
    memcpy(&po_pk, pseudo_out, 32);

    crypto::key_image key_img;
    memcpy(&key_img, ki, 32);

    // Deserialise: c(32) | r[N*32] | K1(32)
    crypto::CLSAG_GG_signature clsag_sig;
    const uint8_t *p = sig;
    memcpy(clsag_sig.c.m_s, p, 32); p += 32;
    clsag_sig.r.resize(ring_size);
    for (size_t i = 0; i < ring_size; i++) {
        memcpy(clsag_sig.r[i].m_s, p, 32); p += 32;
    }
    memcpy(&clsag_sig.K1, p, 32);

    return crypto::verify_CLSAG_GG(h, ring_refs, po_pk, key_img, clsag_sig) ? 0 : 1;
}

// Signature layout for GGX: c(32) | r_g[N*32] | r_x[N*32] | K1(32) | K2(32)
size_t cn_clsag_ggx_sig_size(size_t ring_size) {
    return 32 + ring_size * 64 + 64;
}

int cn_clsag_ggx_verify(const uint8_t hash[32], const uint8_t *ring,
                         size_t ring_size, const uint8_t pseudo_out_commitment[32],
                         const uint8_t pseudo_out_asset_id[32],
                         const uint8_t ki[32], const uint8_t *sig) {
    crypto::hash h;
    memcpy(&h, hash, 32);

    // Ring entries: [stealth(32) | commitment(32) | blinded_asset_id(32)] per entry.
    std::vector<crypto::public_key> stealth_keys(ring_size);
    std::vector<crypto::public_key> commitments(ring_size);
    std::vector<crypto::public_key> asset_ids(ring_size);
    std::vector<crypto::CLSAG_GGX_input_ref_t> ring_refs;
    ring_refs.reserve(ring_size);
    for (size_t i = 0; i < ring_size; i++) {
        memcpy(&stealth_keys[i], ring + i * 96, 32);
        memcpy(&commitments[i], ring + i * 96 + 32, 32);
        memcpy(&asset_ids[i], ring + i * 96 + 64, 32);
        ring_refs.emplace_back(stealth_keys[i], commitments[i], asset_ids[i]);
    }

    crypto::public_key po_commitment, po_asset_id;
    memcpy(&po_commitment, pseudo_out_commitment, 32);
    memcpy(&po_asset_id, pseudo_out_asset_id, 32);

    crypto::key_image key_img;
    memcpy(&key_img, ki, 32);

    // Deserialise: c(32) | r_g[N*32] | r_x[N*32] | K1(32) | K2(32)
    crypto::CLSAG_GGX_signature clsag_sig;
    const uint8_t *p = sig;
    memcpy(clsag_sig.c.m_s, p, 32); p += 32;
    clsag_sig.r_g.resize(ring_size);
    for (size_t i = 0; i < ring_size; i++) {
        memcpy(clsag_sig.r_g[i].m_s, p, 32); p += 32;
    }
    clsag_sig.r_x.resize(ring_size);
    for (size_t i = 0; i < ring_size; i++) {
        memcpy(clsag_sig.r_x[i].m_s, p, 32); p += 32;
    }
    memcpy(&clsag_sig.K1, p, 32); p += 32;
    memcpy(&clsag_sig.K2, p, 32);

    return crypto::verify_CLSAG_GGX(h, ring_refs, po_commitment, po_asset_id, key_img, clsag_sig) ? 0 : 1;
}

// Signature layout for GGXXG: c(32) | r_g[N*32] | r_x[N*32] | K1(32) | K2(32) | K3(32) | K4(32)
size_t cn_clsag_ggxxg_sig_size(size_t ring_size) {
    return 32 + ring_size * 64 + 128;
}

int cn_clsag_ggxxg_verify(const uint8_t hash[32], const uint8_t *ring,
                           size_t ring_size, const uint8_t pseudo_out_commitment[32],
                           const uint8_t pseudo_out_asset_id[32],
                           const uint8_t extended_commitment[32],
                           const uint8_t ki[32], const uint8_t *sig) {
    crypto::hash h;
    memcpy(&h, hash, 32);

    // Ring entries: [stealth(32) | commitment(32) | blinded_asset_id(32) | concealing(32)] per entry.
    std::vector<crypto::public_key> stealth_keys(ring_size);
    std::vector<crypto::public_key> commitments(ring_size);
    std::vector<crypto::public_key> asset_ids(ring_size);
    std::vector<crypto::public_key> concealing_pts(ring_size);
    std::vector<crypto::CLSAG_GGXXG_input_ref_t> ring_refs;
    ring_refs.reserve(ring_size);
    for (size_t i = 0; i < ring_size; i++) {
        memcpy(&stealth_keys[i], ring + i * 128, 32);
        memcpy(&commitments[i], ring + i * 128 + 32, 32);
        memcpy(&asset_ids[i], ring + i * 128 + 64, 32);
        memcpy(&concealing_pts[i], ring + i * 128 + 96, 32);
        ring_refs.emplace_back(stealth_keys[i], commitments[i], asset_ids[i], concealing_pts[i]);
    }

    crypto::public_key po_commitment, po_asset_id, ext_commitment;
    memcpy(&po_commitment, pseudo_out_commitment, 32);
    memcpy(&po_asset_id, pseudo_out_asset_id, 32);
    memcpy(&ext_commitment, extended_commitment, 32);

    crypto::key_image key_img;
    memcpy(&key_img, ki, 32);

    // Deserialise: c(32) | r_g[N*32] | r_x[N*32] | K1(32) | K2(32) | K3(32) | K4(32)
    crypto::CLSAG_GGXXG_signature clsag_sig;
    const uint8_t *p = sig;
    memcpy(clsag_sig.c.m_s, p, 32); p += 32;
    clsag_sig.r_g.resize(ring_size);
    for (size_t i = 0; i < ring_size; i++) {
        memcpy(clsag_sig.r_g[i].m_s, p, 32); p += 32;
    }
    clsag_sig.r_x.resize(ring_size);
    for (size_t i = 0; i < ring_size; i++) {
        memcpy(clsag_sig.r_x[i].m_s, p, 32); p += 32;
    }
    memcpy(&clsag_sig.K1, p, 32); p += 32;
    memcpy(&clsag_sig.K2, p, 32); p += 32;
    memcpy(&clsag_sig.K3, p, 32); p += 32;
    memcpy(&clsag_sig.K4, p, 32);

    return crypto::verify_CLSAG_GGXXG(h, ring_refs, po_commitment, po_asset_id, ext_commitment, key_img, clsag_sig) ? 0 : 1;
}

// ── Range Proofs (stubs — need on-chain binary format deserialiser) ──

int cn_bppe_verify(const uint8_t * /*proof*/, size_t /*proof_len*/,
                   const uint8_t * /*commitments*/, size_t /*num_commitments*/) {
    return -1; // not implemented
}

int cn_bge_verify(const uint8_t /*context*/[32], const uint8_t * /*ring*/,
                  size_t /*ring_size*/, const uint8_t * /*proof*/, size_t /*proof_len*/) {
    return -1; // not implemented
}

int cn_zarcanum_verify(const uint8_t /*hash*/[32], const uint8_t * /*proof*/,
                       size_t /*proof_len*/) {
    return -1; // not implemented
}

// ── RandomX PoW Hashing ──────────────────────────────────

int bridge_randomx_hash(const uint8_t* key, size_t key_size,
                        const uint8_t* input, size_t input_size,
                        uint8_t* output) {
    // Static RandomX state — initialised on first call.
    // Thread safety: not thread-safe; Go wrapper must serialise calls.
    static randomx_cache* rx_cache = nullptr;
    static randomx_vm* rx_vm = nullptr;

    if (rx_cache == nullptr) {
        randomx_flags flags = randomx_get_flags();
        // Use light mode (no dataset) for verification.
        flags = (randomx_flags)(flags | RANDOMX_FLAG_DEFAULT);
        rx_cache = randomx_alloc_cache(flags);
        if (rx_cache == nullptr) return -1;
        randomx_init_cache(rx_cache, key, key_size);
        rx_vm = randomx_create_vm(flags, rx_cache, nullptr);
        if (rx_vm == nullptr) {
            randomx_release_cache(rx_cache);
            rx_cache = nullptr;
            return -1;
        }
    }

    randomx_calculate_hash(rx_vm, input, input_size, output);
    return 0;
}

} // extern "C"
