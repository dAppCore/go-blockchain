# Provenance

Vendored from `~/Code/LetheanNetwork/blockchain/` at commit `fa1608cf`.

## Upstream Files (unmodified copies)

| Local Path | Upstream Path | Modified |
|-----------|---------------|----------|
| upstream/crypto-ops.c | src/crypto/crypto-ops.c | no |
| upstream/crypto-ops-data.c | src/crypto/crypto-ops-data.c | no |
| upstream/crypto-ops.h | src/crypto/crypto-ops.h | no |
| upstream/crypto.cpp | src/crypto/crypto.cpp | no |
| upstream/crypto.h | src/crypto/crypto.h | no |
| upstream/crypto-sugar.h | src/crypto/crypto-sugar.h | no |
| upstream/crypto-sugar.cpp | src/crypto/crypto-sugar.cpp | no |
| upstream/clsag.h | src/crypto/clsag.h | no |
| upstream/clsag.cpp | src/crypto/clsag.cpp | no |
| upstream/zarcanum.h | src/crypto/zarcanum.h | no |
| upstream/zarcanum.cpp | src/crypto/zarcanum.cpp | no |
| upstream/range_proofs.h | src/crypto/range_proofs.h | no |
| upstream/range_proofs.cpp | src/crypto/range_proofs.cpp | no |
| upstream/range_proof_bppe.h | src/crypto/range_proof_bppe.h | no |
| upstream/range_proof_bpp.h | src/crypto/range_proof_bpp.h | no |
| upstream/one_out_of_many_proofs.h | src/crypto/one_out_of_many_proofs.h | no |
| upstream/one_out_of_many_proofs.cpp | src/crypto/one_out_of_many_proofs.cpp | no |
| upstream/msm.h | src/crypto/msm.h | no |
| upstream/msm.cpp | src/crypto/msm.cpp | no |
| upstream/keccak.c | src/crypto/keccak.c | no |
| upstream/keccak.h | src/crypto/keccak.h | no |
| upstream/hash.c | src/crypto/hash.c | no |
| upstream/hash.h | src/crypto/hash.h | no |
| upstream/hash-ops.h | src/crypto/hash-ops.h | no |
| upstream/random.c | src/crypto/random.c | no |
| upstream/random.h | src/crypto/random.h | no |
| upstream/blake2.h | src/crypto/blake2.h | no |
| upstream/blake2-impl.h | src/crypto/blake2-impl.h | no |
| upstream/blake2b-ref.c | src/crypto/blake2b-ref.c | no |
| upstream/generic-ops.h | src/crypto/generic-ops.h | no |
| upstream/eth_signature.h | src/crypto/eth_signature.h | no |
| upstream/eth_signature.cpp | src/crypto/eth_signature.cpp | no |
| upstream/RIPEMD160.c | src/crypto/RIPEMD160.c | no |
| upstream/RIPEMD160.h | src/crypto/RIPEMD160.h | no |
| upstream/RIPEMD160_helper.h | src/crypto/RIPEMD160_helper.h | no |
| upstream/RIPEMD160_helper.cpp | src/crypto/RIPEMD160_helper.cpp | no |
| upstream/initializer.h | src/crypto/initializer.h | no |

## Compat Files (extracted subsets or stubs)

| Local Path | Origin | Notes |
|-----------|--------|-------|
| compat/currency_core/crypto_config.h | src/currency_core/crypto_config.h | unmodified copy |
| compat/common/pod-class.h | src/common/pod-class.h | unmodified copy |
| compat/common/varint.h | src/common/varint.h | unmodified copy |
| compat/warnings.h | N/A | stub — no-op macros |
| compat/epee/include/misc_log_ex.h | N/A | stub — no-op logging macros |
| compat/common/crypto_stream_operators.h | N/A | stub — empty |
| compat/include_base_utils.h | N/A | stub — empty |

## Update Workflow

1. Note the current provenance commit (`fa1608cf`)
2. In the upstream repo, `git diff fa1608cf..HEAD -- src/crypto/`
3. Copy changed files into `upstream/`
4. Rebuild: `cmake --build crypto/build --parallel`
5. Run tests: `go test -race ./crypto/...`
6. Update this file with new commit hash
