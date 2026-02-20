# Phase 6 Design: Wallet Core

## Summary

Phase 6 adds a `wallet/` package that provides full send+receive wallet
functionality for the Lethean blockchain. It uses interface-driven design
with four core abstractions (Scanner, Signer, Builder, RingSelector) so
v1 (NLSAG) implementations ship now and v2+ (Zarcanum/CLSAG) slot in later
without changing callers.

The wallet scans the local chain (from Phase 5) for owned outputs using
ECDH derivation, tracks transfers and balances, constructs v1 transactions
with NLSAG ring signatures, selects decoy outputs via daemon RPC, and
submits signed transactions for relay.

## Decisions

| Question | Decision |
|----------|----------|
| Scope | Full send+receive wallet |
| Architecture | Interface-heavy (Scanner, Signer, Builder, RingSelector) |
| Extra parsing | Wallet-critical only (tx pub key, unlock_time, derivation_hint) |
| Key storage | go-store with Argon2id + AES-256-GCM encryption |
| Ring selection | RPC random outputs from daemon |
| TX versions | v1 (NLSAG) first, v2+ architecture ready |
| HF progression | Chain will advance through hard forks soon |

## Core Interfaces

The wallet defines four interfaces that decouple scanning, signing,
building, and ring selection.

```go
// Scanner detects outputs belonging to a wallet.
type Scanner interface {
    ScanTransaction(tx *types.Transaction, txHash types.Hash,
                    blockHeight uint64, extra *TxExtra) ([]Transfer, error)
}

// Signer produces signatures for transaction inputs.
type Signer interface {
    SignInput(prefixHash types.Hash, ephemeral KeyPair,
             ring []types.PublicKey, realIndex int) ([]types.Signature, error)
    Version() uint64
}

// RingSelector picks decoy outputs for ring signatures.
type RingSelector interface {
    SelectRing(amount uint64, realGlobalIndex uint64,
               ringSize int) ([]RingMember, error)
}

// Builder constructs unsigned or signed transactions.
type Builder interface {
    Build(req *BuildRequest) (*types.Transaction, error)
}
```

v1 implementations: `NLSAGSigner`, `RPCRingSelector`, `V1Scanner`, `V1Builder`.
Future v2+: `CLSAGSigner`, `ZarcanumScanner`, `ZarcanumBuilder` -- same interfaces.

## Account & Key Management

```go
type Account struct {
    SpendPublicKey  types.PublicKey
    SpendSecretKey  types.SecretKey
    ViewPublicKey   types.PublicKey
    ViewSecretKey   types.SecretKey
    CreatedAt       uint64
    Flags           uint8
}
```

### Creation methods

- `Generate()` -- fresh random keys.
- `RestoreFromSeed(phrase, password)` -- 25-word mnemonic.
- `RestoreViewOnly(viewSecret, spendPublic)` -- watch-only wallet.

### Key derivation

The spend secret key is the master secret. The view secret key is derived
deterministically as `Keccak256(spend_secret_key)`. This matches the C++
`account_base::generate()` pattern.

### Seed phrases

25 words from the CryptoNote wordlist. The 32-byte spend secret key is
encoded as a mnemonic with a checksum word.

### Persistence

The passphrase goes through Argon2id (time=3, mem=64MB, threads=4) to
produce a 32-byte key. The serialised account is encrypted with AES-256-GCM.
The ciphertext + salt + nonce are stored in go-store group `wallet` with
key `account`.

## Extra Field Parsing

A minimal parser for the three wallet-critical variant tags. Everything
else stays as raw bytes.

```go
type TxExtra struct {
    TxPublicKey    types.PublicKey   // tag 22
    UnlockTime     uint64           // tag 14 (0 if absent)
    DerivationHint uint16           // tag 11 (0 if absent)
    Raw            []byte           // original bytes preserved
}
```

| Tag | Type | Size | Purpose |
|-----|------|------|---------|
| 22 | `crypto::public_key` | 32 bytes fixed | TX public key for ECDH |
| 14 | `etc_tx_details_unlock_time` | varint | Block/timestamp lock |
| 11 | `tx_derivation_hint` | 2 bytes fixed | Fast scanning hint |

Unknown tags are skipped using the same boundary detection the wire package
uses. Raw bytes are preserved so `TransactionHash()` still works.

## Output Scanning & Transfer Tracking

### Transfer type

```go
type Transfer struct {
    TxHash       types.Hash
    OutputIndex  uint32
    Amount       uint64
    GlobalIndex  uint64
    BlockHeight  uint64
    EphemeralKey KeyPair
    KeyImage     types.KeyImage
    Spent        bool
    SpentHeight  uint64
    Flags        uint8
    UnlockTime   uint64
}

type KeyPair struct {
    Public types.PublicKey
    Secret types.SecretKey
}
```

### V1Scanner flow

1. Parse `TxExtra` -- extract TX public key (tag 22).
2. ECDH: `derivation = GenerateKeyDerivation(txPubKey, viewSecretKey)`.
3. For each output `i`:
   - Derive: `expectedPub = DerivePublicKey(derivation, i, spendPublicKey)`.
   - If `expectedPub == output.Target.Key` -- output is ours.
   - Derive secret: `ephemeralSec = DeriveSecretKey(derivation, i, spendSecretKey)`.
   - Key image: `ki = GenerateKeyImage(expectedPub, ephemeralSec)`.
4. Return matching `Transfer` records.

### Balance calculation

A transfer is **spendable** when:
- Not spent (`!Spent`)
- Not locked (coinbase maturity: `blockHeight + MinedMoneyUnlockWindow <= chainHeight`)
- Unlock time satisfied (if non-zero)

### Spend detection

During sync, when a `TxInputToKey` key image matches a stored transfer's
key image, mark that transfer as spent.

## Transaction Construction

### V1Builder flow

1. **Validate** -- sum of source amounts >= sum of destinations + fee.
2. **Generate tx key pair** -- random one-time key for this transaction.
3. **Build inputs** -- for each source:
   - `RingSelector.SelectRing()` for decoys from daemon RPC.
   - Sort ring members by global index (consensus rule).
   - Record real output position after sorting.
   - Create `TxInputToKey` with key offsets and key image.
4. **Build outputs** -- for each destination:
   - Derive ephemeral key via ECDH with destination address.
   - Create `TxOutputBare` with amount and `TxOutToKey`.
5. **Build extra** -- `BuildTxExtra(txPubKey)`.
6. **Compute prefix hash** -- `wire.TransactionPrefixHash(&tx)`.
7. **Sign** -- `Signer.SignInput()` per input with ring and ephemeral keys.
8. **Return** complete signed transaction.

### Ring selection

`RPCRingSelector` calls a new RPC endpoint `GetRandomOutputs(amount, count)`
that wraps the daemon's `getrandom_outs` method.

### Change handling

If source amount exceeds destinations + fee, the builder adds a change
output back to the sender's address. Extra amount is never silently
absorbed as fee.

### Submission

`rpc.Client.SendRawTransaction(txBlob)` -- new RPC endpoint wrapping
the daemon's `/sendrawtransaction` legacy path.

## Wallet Orchestrator

```go
type Wallet struct {
    account      *Account
    store        *store.Store
    chain        *chain.Chain
    scanner      Scanner
    signer       Signer
    ringSelector RingSelector
    builder      Builder
}

func NewWallet(account *Account, s *store.Store, c *chain.Chain,
               client *rpc.Client) *Wallet

func (w *Wallet) Sync() error
func (w *Wallet) Balance() (confirmed, locked uint64, err error)
func (w *Wallet) Send(destinations []Destination, fee uint64) (*types.Transaction, error)
func (w *Wallet) Transfers() ([]Transfer, error)
```

### Sync flow

1. Read last scanned height from go-store (group `wallet`, key `scan_height`).
2. Get chain height from `chain.Height()`.
3. For each block from `lastScanned+1` to `chainHeight-1`:
   - Fetch block, scan miner tx + regular txs.
   - Store new transfers, check key images for spend detection.
4. Update `scan_height`.

### Transfer storage

go-store group `transfers`, key = `keyImage.String()`, value = JSON
`Transfer`. Spend detection is O(1) key image lookup.

### Send flow

1. Verify sufficient balance.
2. Coin selection: largest-first, greedy.
3. Build + sign via `builder.Build()`.
4. Submit via `rpc.Client.SendRawTransaction()`.
5. Mark source transfers as spent (optimistic, confirmed on next sync).

## Package Structure

```
wallet/
    wallet.go           -- Wallet struct, NewWallet, Sync, Send, Balance
    account.go          -- Account, Generate, RestoreFromSeed, Save/Load
    scanner.go          -- Scanner interface + V1Scanner
    signer.go           -- Signer interface + NLSAGSigner
    builder.go          -- Builder interface + V1Builder
    ring.go             -- RingSelector interface + RPCRingSelector
    transfer.go         -- Transfer type, transfer storage helpers
    extra.go            -- TxExtra parsing and construction
    mnemonic.go         -- 25-word seed phrase encode/decode
    wallet_test.go      -- Wallet sync + send integration tests
    account_test.go     -- Key generation + seed round-trip tests
    scanner_test.go     -- Output detection tests
    builder_test.go     -- Transaction construction tests
    extra_test.go       -- Extra parsing tests
    mnemonic_test.go    -- Mnemonic encode/decode tests
```

### New RPC endpoints (in `rpc/`)

- `GetRandomOutputs(amount, count)` -- for ring selection.
- `SendRawTransaction(txBlob)` -- for transaction submission.

## Testing Strategy

### Unit tests (go-store `:memory:`)

- Account: generate, seed round-trip, save/load with encryption, watch-only.
- Extra: parse known extras, extract tx pub key, skip unknown tags, build extra.
- Scanner: detect owned output, reject non-owned, handle missing tx pub key.
- Builder: valid tx construction, insufficient funds error, change output.
- Signer: NLSAG ring signature round-trip (sign + verify).
- Mnemonic: encode/decode round-trip, invalid word rejection, checksum.

### Mock RPC tests

- RPCRingSelector with httptest server returning canned random outputs.
- SendRawTransaction with mock acceptance/rejection.

### Integration test (build-tagged)

- Generate wallet, sync from testnet, verify balance matches expected.
- Construct and submit a transaction (if testnet has spendable outputs).

### Coverage target

Greater than 80% across `wallet/` files.

## Dependencies

- `forge.lthn.ai/core/go-store` -- persistence (already in go.mod)
- `forge.lthn.ai/core/go-blockchain/crypto` -- key operations, signatures
- `forge.lthn.ai/core/go-blockchain/chain` -- block/tx retrieval
- `forge.lthn.ai/core/go-blockchain/rpc` -- daemon communication
- `forge.lthn.ai/core/go-blockchain/wire` -- serialisation, hashing
- `forge.lthn.ai/core/go-blockchain/types` -- core types
- `forge.lthn.ai/core/go-blockchain/config` -- constants
- `golang.org/x/crypto` -- Argon2id (already a dependency)

No new external dependencies.

## C++ Reference Files

- `src/currency_core/account.h` -- account key structure, seed phrases
- `src/currency_core/currency_format_utils.h` -- `is_out_to_acc()`, output detection
- `src/currency_core/currency_format_utils.cpp` -- `construct_tx()`, key derivation
- `src/currency_core/currency_basic.h` -- extra variant tags, type definitions
- `src/wallet/wallet2.h` -- full wallet lifecycle, transfer tracking
- `src/wallet/wallet2_base.h` -- transfer details, wallet state containers
