# Phase 7: Consensus Rules — Design

## Goal

Implement a standalone `consensus/` package with full hardfork-aware validation
for the Lethean blockchain: block reward calculation, fee policy enforcement,
transaction semantic validation, block-level checks, PoW/PoS verification, and
optional signature verification.

## Architecture

Three-layer validation in a standalone `consensus/` package:

1. **Structural** — tx size, input/output counts, key image uniqueness (no crypto)
2. **Economic** — block reward, fees, balance checks, overflow detection
3. **Cryptographic** — signature verification, PoW hash, proof verification (CGo)

`chain/sync.go` calls layers 1+2 always. Layer 3 is optional via `SyncOptions.VerifySignatures`.

All functions take `*config.ChainConfig` + height for hardfork awareness. No global state.

## Scope

Full hardfork coverage (HF0–HF6). Pre-HF4 (v0/v1) and post-HF4 (Zarcanum/CLSAG)
code paths implemented. PoW hash function added to the CGo crypto bridge.

---

## Package Structure

```
consensus/
├── reward.go          Block reward calculation (base + size penalty)
├── reward_test.go
├── fee.go             Fee extraction and validation (pre/post HF4)
├── fee_test.go
├── tx.go              Transaction semantic validation (8 checks)
├── tx_test.go
├── block.go           Block-level validation (timestamp, miner tx, reward)
├── block_test.go
├── pow.go             PoW hash verification via CGo bridge
├── pow_test.go
├── verify.go          Signature verification orchestration
├── verify_test.go
└── doc.go             Package documentation
```

Modified files:

- `crypto/bridge.h` + `crypto/bridge.cpp` — add PoW hash function
- `crypto/pow.go` — Go wrapper for PoW hash
- `chain/sync.go` — call consensus validators during sync
- `chain/validate.go` — delegate to consensus/

---

## Block Reward

### BaseReward

```go
func BaseReward(height uint64) uint64
```

- Height 0: returns `config.Premine` (10,000,000,000,000,000,000 atomic units)
- All other heights: returns `config.BlockReward` (1,000,000,000,000 atomic units)

No halving. No tail emission. Fixed 1 LTHN per block.

### BlockReward

```go
func BlockReward(baseReward uint64, blockSize, medianSize uint64) (uint64, error)
```

Size penalty:

- `blockSize <= GrantedFullRewardZone` (125,000 bytes): full reward
- `blockSize > 2 * medianSize`: error (block rejected)
- Otherwise: `reward = baseReward * (2*median - size) * size / median²`

The intermediate product uses `math/bits.Mul64` to avoid uint64 overflow,
matching the C++ 128-bit arithmetic.

### Fee handling

- Pre-HF4: `reward += totalFees` (fees go to miner)
- Post-HF4: fees burned (miner gets base reward only)

---

## Fee Extraction

```go
func TxFee(tx *types.Transaction, cfg *config.ChainConfig, height uint64) (uint64, error)
```

- Coinbase: fee = 0
- Pre-HF4 (version <= 1): `fee = sum(inputs) - sum(outputs)`
- Post-HF4 (version >= 2): fee read from `zarcanum_tx_data_v1` in tx extra

Overflow check: `sum(inputs) >= sum(outputs)` or error.

---

## Transaction Semantic Validation

```go
func ValidateTransaction(tx *types.Transaction, txBlob []byte, cfg *config.ChainConfig, height uint64) error
```

Eight checks in order, matching `tx_semantic_validation.cpp`:

1. **Blob size** — `len(txBlob) < MaxTransactionBlobSize` (374,600 bytes)
2. **Input count** — at least 1, at most `TxMaxAllowedInputs` (256)
3. **Input types** — `TxInputToKey` pre-HF4, `TxInputZC` post-HF4.
   `TxInputGenesis` only valid in miner tx (checked in block validation).
4. **Output validation** — at least 1 output pre-HF4, at least 2 post-HF4.
   At most `TxMaxAllowedOutputs` (2000). All bare output amounts > 0.
   Valid keys on all output targets.
5. **Money overflow** — `sum(input amounts)` and `sum(output amounts)` each
   checked for uint64 overflow independently.
6. **Key image uniqueness** — no duplicate key images within one transaction.
7. **Extra parsing** — extra field tag boundaries must be valid.
8. **Balance check** — pre-HF4: `sum(inputs) >= sum(outputs)`.
   Post-HF4: Pedersen commitment balance via crypto bridge.

Each check returns a typed error: `ErrTxTooLarge`, `ErrNoInputs`,
`ErrDuplicateKeyImage`, etc.

---

## Block Validation

```go
func ValidateBlock(blk *types.Block, cfg *config.ChainConfig, height uint64,
    prevBlock *types.BlockHeader, timestamps []uint64, medianSize uint64) error
```

### Timestamp validation

- Block timestamp must not exceed `adjustedTime + FutureTimeLimit`
  (7200s PoW, 1200s PoS)
- If 60+ previous timestamps available: block timestamp >= median of last 60
- PoW vs PoS distinguished by `BlockHeader.Flags` (bit 0 = PoS)

### Miner transaction validation

- First input must be `TxInputGenesis` with height == current block height
- PoW: exactly 1 input. PoS: exactly 2 inputs (genesis + stake)
- PoS second input: `TxInputToKey` pre-HF4, `TxInputZC` post-HF4
- Unlock time: all outputs >= `height + MinedMoneyUnlockWindow` (10)
- Pre-HF1: unlock must be exactly `height + 10`
- Post-HF1: unlock >= `height + 10`
- Post-HF4: no attachments, exactly 3 proofs (surjection, range, balance)
- Pre-HF4: no attachments, no proofs

### Reward validation

- Calculate expected reward via `BlockReward()` with size penalty
- Pre-HF4: miner tx output sum == `baseReward + totalFees`
- Post-HF4: validated via balance proof (fees burned)
- Miner tx output sum must not exceed expected reward

---

## PoW and PoS Verification

### PoW

```go
func CheckPoW(blockBlob []byte, difficulty uint64) bool
```

- PoW hash function added to `crypto/bridge.h` (same CGo pattern as ring sigs)
- Hash the block blob, compare result against difficulty target
- PoS blocks (Flags bit 0 set) skip PoW check

### PoS

- Coinstake kernel: hash of previous block + stake input + timestamp
- Compared against weighted difficulty (stake amount * target)
- Uses crypto bridge for the hash function

---

## Signature Verification

```go
func VerifyTransactionSignatures(tx *types.Transaction, cfg *config.ChainConfig,
    height uint64, getRingOutputs func(amount uint64, offsets []uint64) ([]types.PublicKey, error)) error
```

- Pre-HF4: NLSAG ring signatures via `crypto.CheckRingSignature()`
- Post-HF4: CLSAG via `crypto.VerifyCLSAGGG()` + proof verification via
  `crypto.VerifyBPPE()` / `crypto.VerifyBGE()`
- `getRingOutputs` callback fetches decoy public keys from chain storage,
  keeping consensus/ independent of chain/

### Integration with chain/sync.go

```go
type SyncOptions struct {
    VerifySignatures bool
}
```

- Sync always runs structural + economic validation (layers 1+2)
- When `VerifySignatures == true`, also runs cryptographic layer

---

## Testing

### Unit tests (no CGo, no daemon)

- `reward_test.go` — genesis premine, standard reward, size penalty, oversized
  rejection, fee handling pre/post HF4, overflow
- `fee_test.go` — fee extraction v0/v1, zero-fee coinbase, overflow
- `tx_test.go` — all 8 semantic checks individually, table-driven with
  `_Good`/`_Bad`/`_Ugly` naming
- `block_test.go` — timestamp future/median, miner tx inputs (PoW/PoS),
  genesis height, unlock times (pre/post HF1), reward mismatch

### Crypto layer tests (CGo)

- `pow_test.go` — known PoW hash vector, difficulty comparison
- `verify_test.go` — ring signature verification with synthetic rings,
  CLSAG verification, invalid signature rejection

### Integration test (`//go:build integration`)

- Sync first N blocks from testnet with full validation
- Verify every block passes all three layers
- Confirms Go consensus matches C++ daemon's accepted chain

### Coverage target

>85% across consensus/ files.

---

## C++ Reference Files

| Function | File | Lines |
|----------|------|-------|
| `get_base_block_reward` | `currency_format_utils.cpp` | 4188–4194 |
| `get_block_reward` | `currency_format_utils.cpp` | 4205–4245 |
| `get_tx_fee` | `currency_format_utils.cpp` | 903–949 |
| `validate_tx_semantic` | `tx_semantic_validation.cpp` | 50–107 |
| `check_inputs_types_supported` | `currency_format_utils.cpp` | 3060–3072 |
| `check_outs_valid` | `currency_format_utils.cpp` | 3130–3173 |
| `check_bare_money_overflow` | `currency_format_utils.cpp` | 3175–3232 |
| `check_tx_inputs_keyimages_diff` | `tx_semantic_validation.cpp` | 35–48 |
| `prevalidate_miner_transaction` | `blockchain_storage.cpp` | 1561–1622 |
| `validate_miner_transaction` | `blockchain_storage.cpp` | 1637–1663 |
| `check_block_timestamp_main` | `blockchain_storage.cpp` | 6032–6048 |
| `check_block_timestamp` | `blockchain_storage.cpp` | 6050–6070 |
| `handle_block_to_main_chain` | `blockchain_storage.cpp` | 6957+ |

## Design Decisions

1. **Standalone package** — `consensus/` has no dependency on `chain/`.
   Validation functions are pure: take types + config, return errors.
2. **Three layers** — structural, economic, cryptographic. Each independently
   testable. Crypto layer optional for fast sync.
3. **CGo for PoW** — same pattern as Phase 2 crypto bridge. Guarantees
   bit-identical hash results.
4. **Callback for ring outputs** — `getRingOutputs` function parameter keeps
   consensus/ decoupled from chain storage.
5. **Full hardfork coverage** — pre-HF4 and post-HF4 code paths both
   implemented, with hardfork-conditional logic throughout.
6. **`math/bits.Mul64`** — for 128-bit intermediate products in size penalty
   calculation, avoiding `math/big` overhead.
