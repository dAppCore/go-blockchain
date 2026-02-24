# Phase 6: Wallet Core Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Full send+receive wallet for the Lethean blockchain with interface-driven v1/v2+ extensibility.

**Architecture:** Four core interfaces (Scanner, Signer, Builder, RingSelector) with v1 NLSAG implementations. Account keys encrypted via Argon2id+AES-256-GCM in go-store. Wallet orchestrator coordinates sync, balance, and send.

**Tech Stack:** Go, CGo crypto bridge, go-store (SQLite KV), golang.org/x/crypto (Argon2id)

**Design doc:** `docs/plans/2026-02-20-wallet-core-design.md`

---

## Dependencies Between Tasks

```
Task 1 (Mnemonic) ─────────────────────┐
Task 2 (TxExtra) ──────────┐           │
Task 4 (Transfer) ─────────┤           │
                            ├─ Task 5 (Scanner) ──┐
Task 6 (RPC endpoints) ────┤                      │
                            ├─ Task 8 (Ring) ──┐   │
Task 7 (Signer) ───────────┤                  │   │
                            ├─ Task 9 (Builder)│   │
Task 3 (Account) ──────────┤                  │   │
  (needs Task 1)            │                  │   │
                            └──────────────────┴───┴─ Task 10 (Wallet)
                                                       Task 11 (Integration)
```

Tasks 1, 2, 4, 6, 7 can be done in any order (no inter-dependencies).

---

## Task 1: Mnemonic Seed Encode/Decode

**Files:**
- Create: `wallet/wordlist.go`
- Create: `wallet/mnemonic.go`
- Create: `wallet/mnemonic_test.go`

**Context:** The Electrum-style mnemonic encodes a 32-byte secret key as 24 words (4 bytes → 3 words) plus a 25th checksum word. The wordlist has 1626 entries. The encoding algorithm and wordlist are in `~/Code/LetheanNetwork/blockchain/src/common/mnemonic-encoding.cpp`.

**Step 1: Extract wordlist from C++ source**

Run:
```bash
# Extract just the words from the C++ wordsArray
cd ~/Code/LetheanNetwork/blockchain/src/common
grep -oP '"\K[a-z]+(?=")' mnemonic-encoding.cpp | tail -n 1626 > /tmp/wordlist.txt
wc -l /tmp/wordlist.txt
```
Expected: `1626 /tmp/wordlist.txt`

**Step 2: Create `wallet/wordlist.go`**

Generate from extracted words:
```bash
cd ~/Code/core/go-blockchain
mkdir -p wallet
# Script to generate Go source from wordlist
echo 'package wallet' > wallet/wordlist.go
echo '' >> wallet/wordlist.go
echo '// wordlist is the 1626-word Electrum mnemonic dictionary.' >> wallet/wordlist.go
echo '// Extracted from src/common/mnemonic-encoding.cpp in the C++ source.' >> wallet/wordlist.go
echo 'var wordlist = [1626]string{' >> wallet/wordlist.go
awk '{printf "\t\"%s\",\n", $0}' /tmp/wordlist.txt >> wallet/wordlist.go
echo '}' >> wallet/wordlist.go
```

Also add a reverse-lookup map at the bottom of the file:
```go
// wordIndex maps each word to its index in the wordlist.
var wordIndex map[string]int

func init() {
	wordIndex = make(map[string]int, len(wordlist))
	for i, w := range wordlist {
		wordIndex[w] = i
	}
}
```

**Step 3: Write failing tests for mnemonic**

Create `wallet/mnemonic_test.go`:
```go
package wallet

import (
	"strings"
	"testing"
)

func TestWordlistLength(t *testing.T) {
	if len(wordlist) != 1626 {
		t.Fatalf("wordlist length = %d, want 1626", len(wordlist))
	}
}

func TestWordlistFirstLast(t *testing.T) {
	if wordlist[0] != "like" {
		t.Errorf("wordlist[0] = %q, want %q", wordlist[0], "like")
	}
	if wordlist[1625] != "weary" {
		t.Errorf("wordlist[1625] = %q, want %q", wordlist[1625], "weary")
	}
}

func TestWordIndexConsistency(t *testing.T) {
	for i, w := range wordlist {
		idx, ok := wordIndex[w]
		if !ok {
			t.Fatalf("word %q not in index", w)
		}
		if idx != i {
			t.Fatalf("wordIndex[%q] = %d, want %d", w, idx, i)
		}
	}
}

func TestMnemonicRoundTrip(t *testing.T) {
	// Known 32-byte key (all zeros)
	var key [32]byte
	phrase, err := MnemonicEncode(key[:])
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Fields(phrase)
	if len(words) != 25 {
		t.Fatalf("got %d words, want 25", len(words))
	}

	decoded, err := MnemonicDecode(phrase)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != key {
		t.Fatalf("round-trip failed: got %x, want %x", decoded, key)
	}
}

func TestMnemonicRoundTripNonZero(t *testing.T) {
	var key [32]byte
	for i := range key {
		key[i] = byte(i * 7)
	}
	phrase, err := MnemonicEncode(key[:])
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := MnemonicDecode(phrase)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != key {
		t.Fatalf("round-trip failed: got %x, want %x", decoded, key)
	}
}

func TestMnemonicInvalidWordCount(t *testing.T) {
	_, err := MnemonicDecode("like just love")
	if err == nil {
		t.Fatal("expected error for 3 words")
	}
}

func TestMnemonicInvalidWord(t *testing.T) {
	// 25 words but one is invalid
	phrase := "like just love know never want time out there make look eye down only think " +
		"call hand high keep last long make new zzzznotaword"
	_, err := MnemonicDecode(phrase)
	if err == nil {
		t.Fatal("expected error for invalid word")
	}
}

func TestMnemonicBadChecksum(t *testing.T) {
	var key [32]byte
	phrase, _ := MnemonicEncode(key[:])
	words := strings.Fields(phrase)
	// Corrupt the checksum word
	words[24] = "never"
	if words[24] == words[0] {
		words[24] = "want"
	}
	_, err := MnemonicDecode(strings.Join(words, " "))
	if err == nil {
		t.Fatal("expected checksum error")
	}
}

func TestMnemonicInvalidLength(t *testing.T) {
	_, err := MnemonicEncode([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("expected error for non-32-byte input")
	}
}
```

**Step 4: Run tests to verify they fail**

Run: `go test -race -run TestMnemonic -v ./wallet/`
Expected: FAIL (MnemonicEncode/MnemonicDecode not defined)

**Step 5: Implement mnemonic encode/decode**

Create `wallet/mnemonic.go`:
```go
package wallet

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"strings"
)

const numWords = 1626

// MnemonicEncode converts a 32-byte secret key to a 25-word mnemonic phrase.
// The first 24 words encode the key (4 bytes → 3 words × 8 groups).
// The 25th word is a CRC32 checksum.
func MnemonicEncode(key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("wallet: mnemonic encode requires 32 bytes, got %d", len(key))
	}

	words := make([]string, 0, 25)
	n := uint32(numWords)

	for i := 0; i < 32; i += 4 {
		val := binary.LittleEndian.Uint32(key[i : i+4])
		w1 := val % n
		w2 := ((val / n) + w1) % n
		w3 := (((val / n) / n) + w2) % n
		words = append(words, wordlist[w1], wordlist[w2], wordlist[w3])
	}

	// Checksum: CRC32 of first 3 chars of each word, pick word at index.
	checkIdx := checksumIndex(words)
	words = append(words, words[checkIdx])

	return strings.Join(words, " "), nil
}

// MnemonicDecode converts a 25-word mnemonic phrase to a 32-byte secret key.
func MnemonicDecode(phrase string) ([32]byte, error) {
	var key [32]byte
	words := strings.Fields(phrase)
	if len(words) != 25 {
		return key, fmt.Errorf("wallet: mnemonic requires 25 words, got %d", len(words))
	}

	// Verify checksum.
	expected := checksumIndex(words[:24])
	if words[24] != words[expected] {
		return key, fmt.Errorf("wallet: mnemonic checksum failed")
	}

	n := uint32(numWords)
	for i := 0; i < 8; i++ {
		w1, ok1 := wordIndex[words[i*3]]
		w2, ok2 := wordIndex[words[i*3+1]]
		w3, ok3 := wordIndex[words[i*3+2]]
		if !ok1 || !ok2 || !ok3 {
			word := words[i*3]
			if !ok2 {
				word = words[i*3+1]
			}
			if !ok3 {
				word = words[i*3+2]
			}
			return key, fmt.Errorf("wallet: unknown mnemonic word %q", word)
		}

		val := uint32(w1) +
			n*(((n-uint32(w1))+uint32(w2))%n) +
			n*n*(((n-uint32(w2))+uint32(w3))%n)
		binary.LittleEndian.PutUint32(key[i*4:i*4+4], val)
	}

	return key, nil
}

// checksumIndex computes the checksum word index from the first 24 words.
func checksumIndex(words []string) int {
	var prefixes string
	for _, w := range words {
		if len(w) >= 3 {
			prefixes += w[:3]
		} else {
			prefixes += w
		}
	}
	return int(crc32.ChecksumIEEE([]byte(prefixes))) % len(words)
}
```

**Step 6: Run tests to verify they pass**

Run: `go test -race -run "TestWordlist|TestMnemonic" -v ./wallet/`
Expected: PASS (all 8 tests)

**Step 7: Commit**

```bash
git add wallet/wordlist.go wallet/mnemonic.go wallet/mnemonic_test.go
git commit -m "feat(wallet): mnemonic seed encode/decode with Electrum wordlist"
```

---

## Task 2: TX Extra Parsing

**Files:**
- Create: `wallet/extra.go`
- Create: `wallet/extra_test.go`

**Context:** The wallet needs to extract three fields from the raw `tx.Extra` bytes: TX public key (tag 22, 32 bytes), unlock time (tag 14, varint), and derivation hint (tag 11, varint-length-prefixed string of 2 bytes). The raw extra is a variant vector: `[varint count][tag byte + data]...`. The `wire` package already handles raw variant vectors; here we parse the wallet-critical tags.

**Step 1: Write failing tests**

Create `wallet/extra_test.go`:
```go
package wallet

import (
	"testing"

	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

func TestParseTxExtraPublicKey(t *testing.T) {
	// Build extra with just a public key (tag 22 + 32 bytes).
	var key types.PublicKey
	for i := range key {
		key[i] = byte(i + 1)
	}
	raw := buildTestExtra(key, 0, 0)

	extra, err := ParseTxExtra(raw)
	if err != nil {
		t.Fatal(err)
	}
	if extra.TxPublicKey != key {
		t.Fatalf("tx public key mismatch: got %x", extra.TxPublicKey)
	}
}

func TestParseTxExtraUnlockTime(t *testing.T) {
	var key types.PublicKey
	key[0] = 0xAA
	raw := buildTestExtra(key, 500, 0)

	extra, err := ParseTxExtra(raw)
	if err != nil {
		t.Fatal(err)
	}
	if extra.UnlockTime != 500 {
		t.Fatalf("unlock_time = %d, want 500", extra.UnlockTime)
	}
}

func TestParseTxExtraDerivationHint(t *testing.T) {
	var key types.PublicKey
	key[0] = 0xBB
	raw := buildTestExtra(key, 0, 0x1234)

	extra, err := ParseTxExtra(raw)
	if err != nil {
		t.Fatal(err)
	}
	if extra.DerivationHint != 0x1234 {
		t.Fatalf("derivation_hint = %04x, want 1234", extra.DerivationHint)
	}
}

func TestParseTxExtraEmpty(t *testing.T) {
	// Empty extra: just varint(0)
	raw := wire.EncodeVarint(0)
	extra, err := ParseTxExtra(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !extra.TxPublicKey.IsZero() {
		t.Fatal("expected zero tx public key for empty extra")
	}
}

func TestParseTxExtraPreservesRaw(t *testing.T) {
	var key types.PublicKey
	key[0] = 0xCC
	raw := buildTestExtra(key, 100, 0x5678)
	extra, err := ParseTxExtra(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(extra.Raw) != len(raw) {
		t.Fatalf("raw length = %d, want %d", len(extra.Raw), len(raw))
	}
}

func TestBuildTxExtraRoundTrip(t *testing.T) {
	var key types.PublicKey
	for i := range key {
		key[i] = byte(i + 10)
	}
	raw := BuildTxExtra(key)
	extra, err := ParseTxExtra(raw)
	if err != nil {
		t.Fatal(err)
	}
	if extra.TxPublicKey != key {
		t.Fatalf("round-trip key mismatch")
	}
}

// buildTestExtra constructs a raw extra with the given fields.
func buildTestExtra(txPubKey types.PublicKey, unlockTime uint64, hint uint16) []byte {
	var count uint64
	var elements []byte

	// Tag 22: public key (32 bytes)
	if !txPubKey.IsZero() {
		count++
		elements = append(elements, 22)
		elements = append(elements, txPubKey[:]...)
	}

	// Tag 14: unlock time (varint)
	if unlockTime > 0 {
		count++
		elements = append(elements, 14)
		elements = append(elements, wire.EncodeVarint(unlockTime)...)
	}

	// Tag 11: derivation hint (string: varint(2) + 2 bytes)
	if hint > 0 {
		count++
		elements = append(elements, 11)
		elements = append(elements, wire.EncodeVarint(2)...) // length = 2
		elements = append(elements, byte(hint&0xFF), byte(hint>>8))
	}

	raw := wire.EncodeVarint(count)
	raw = append(raw, elements...)
	return raw
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -race -run TestParseTxExtra -v ./wallet/`
Expected: FAIL (ParseTxExtra not defined)

**Step 3: Implement extra parsing**

Create `wallet/extra.go`:
```go
package wallet

import (
	"encoding/binary"
	"fmt"

	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

// Extra variant tags (wallet-critical subset).
const (
	extraTagDerivationHint = 11
	extraTagUnlockTime     = 14
	extraTagPublicKey      = 22
)

// TxExtra holds parsed wallet-critical fields from the tx extra.
type TxExtra struct {
	TxPublicKey    types.PublicKey
	UnlockTime     uint64
	DerivationHint uint16
	Raw            []byte
}

// ParseTxExtra extracts wallet-critical fields from raw extra bytes.
// Unknown tags are skipped. The Raw field preserves the original bytes.
func ParseTxExtra(raw []byte) (*TxExtra, error) {
	extra := &TxExtra{Raw: make([]byte, len(raw))}
	copy(extra.Raw, raw)

	if len(raw) == 0 {
		return extra, nil
	}

	// Read varint count.
	count, n := wire.DecodeVarintBytes(raw)
	if n <= 0 {
		return extra, fmt.Errorf("wallet: extra: invalid varint count")
	}
	pos := n

	for i := uint64(0); i < count && pos < len(raw); i++ {
		tag := raw[pos]
		pos++

		switch tag {
		case extraTagPublicKey:
			if pos+32 > len(raw) {
				return extra, fmt.Errorf("wallet: extra: truncated public key")
			}
			copy(extra.TxPublicKey[:], raw[pos:pos+32])
			pos += 32

		case extraTagUnlockTime:
			val, vn := wire.DecodeVarintBytes(raw[pos:])
			if vn <= 0 {
				return extra, fmt.Errorf("wallet: extra: invalid unlock_time varint")
			}
			extra.UnlockTime = val
			pos += vn

		case extraTagDerivationHint:
			// String-encoded: varint(length) + bytes
			length, vn := wire.DecodeVarintBytes(raw[pos:])
			if vn <= 0 {
				return extra, fmt.Errorf("wallet: extra: invalid hint length")
			}
			pos += vn
			if length == 2 && pos+2 <= len(raw) {
				extra.DerivationHint = binary.LittleEndian.Uint16(raw[pos : pos+2])
			}
			pos += int(length)

		default:
			// Skip unknown tag using wire's element-size knowledge.
			skip, err := skipExtraElement(raw[pos:], tag)
			if err != nil {
				return extra, nil // stop parsing, return what we have
			}
			pos += skip
		}
	}

	return extra, nil
}

// BuildTxExtra builds a raw extra containing just a TX public key.
func BuildTxExtra(txPubKey types.PublicKey) []byte {
	raw := wire.EncodeVarint(1) // count = 1
	raw = append(raw, extraTagPublicKey)
	raw = append(raw, txPubKey[:]...)
	return raw
}

// skipExtraElement returns the number of bytes to skip for a given tag.
func skipExtraElement(data []byte, tag uint8) (int, error) {
	switch tag {
	case 7, 9, 11, 19: // string types
		if len(data) == 0 {
			return 0, fmt.Errorf("no data")
		}
		length, n := wire.DecodeVarintBytes(data)
		if n <= 0 {
			return 0, fmt.Errorf("invalid varint")
		}
		return n + int(length), nil
	case 14, 15, 16, 26, 27: // varint types
		_, n := wire.DecodeVarintBytes(data)
		if n <= 0 {
			return 0, fmt.Errorf("invalid varint")
		}
		return n, nil
	case 10: // two uint32 LE
		return 8, nil
	case 17, 28: // uint32 LE
		return 4, nil
	case 23, 24: // uint16 LE
		return 2, nil
	case 22: // public key
		return 32, nil
	case 8, 29: // two public keys
		return 64, nil
	default:
		return 0, fmt.Errorf("unknown tag %d", tag)
	}
}
```

This requires a `DecodeVarintBytes` helper. Check if `wire` already exports one; if not, add it.

**Step 4: Add DecodeVarintBytes to wire package if needed**

Check `wire/varint.go` for existing export. If not present, add to `wire/varint.go`:
```go
// DecodeVarintBytes decodes a varint from a byte slice, returning the
// value and the number of bytes consumed. Returns (0, 0) on error.
func DecodeVarintBytes(buf []byte) (uint64, int) {
	var val uint64
	for i, b := range buf {
		if i >= 10 {
			return 0, 0
		}
		val |= uint64(b&0x7F) << (7 * uint(i))
		if b&0x80 == 0 {
			return val, i + 1
		}
	}
	return 0, 0
}
```

Also add `IsZero()` to `types.PublicKey` if not already present.

**Step 5: Run tests to verify they pass**

Run: `go test -race -run "TestParseTxExtra|TestBuildTxExtra" -v ./wallet/`
Expected: PASS (all 6 tests)

**Step 6: Commit**

```bash
git add wallet/extra.go wallet/extra_test.go wire/varint.go
git commit -m "feat(wallet): TX extra parsing for wallet-critical tags"
```

---

## Task 3: Account Key Management

**Files:**
- Create: `wallet/account.go`
- Create: `wallet/account_test.go`

**Context:** The Account struct holds spend+view key pairs. The spend secret is the master; the view secret is `Keccak256(spend_secret)`. Persistence uses Argon2id+AES-256-GCM encryption in go-store. Depends on Task 1 (mnemonic).

**Crypto API:**
- `crypto.GenerateKeys() (pub, sec [32]byte, err error)`
- `crypto.SecretToPublic(sec [32]byte) ([32]byte, error)`
- `crypto.FastHash(data []byte) [32]byte` (Keccak-256)

**Step 1: Write failing tests**

Create `wallet/account_test.go`:
```go
package wallet

import (
	"strings"
	"testing"

	store "forge.lthn.ai/core/go-store"
)

func TestAccountGenerate(t *testing.T) {
	acc, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	// Spend and view keys should be non-zero.
	var zero [32]byte
	if acc.SpendSecretKey == zero {
		t.Fatal("spend secret is zero")
	}
	if acc.ViewSecretKey == zero {
		t.Fatal("view secret is zero")
	}
	if acc.SpendPublicKey == zero {
		t.Fatal("spend public is zero")
	}
	if acc.ViewPublicKey == zero {
		t.Fatal("view public is zero")
	}
}

func TestAccountSeedRoundTrip(t *testing.T) {
	acc1, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	phrase, err := acc1.ToSeed()
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Fields(phrase)
	if len(words) != 25 {
		t.Fatalf("seed has %d words, want 25", len(words))
	}

	acc2, err := RestoreFromSeed(phrase)
	if err != nil {
		t.Fatal(err)
	}
	if acc1.SpendSecretKey != acc2.SpendSecretKey {
		t.Fatal("spend secret mismatch")
	}
	if acc1.ViewSecretKey != acc2.ViewSecretKey {
		t.Fatal("view secret mismatch")
	}
	if acc1.SpendPublicKey != acc2.SpendPublicKey {
		t.Fatal("spend public mismatch")
	}
	if acc1.ViewPublicKey != acc2.ViewPublicKey {
		t.Fatal("view public mismatch")
	}
}

func TestAccountViewOnly(t *testing.T) {
	acc, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	view, err := RestoreViewOnly(acc.ViewSecretKey, acc.SpendPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if view.ViewPublicKey != acc.ViewPublicKey {
		t.Fatal("view public mismatch")
	}
	if view.SpendPublicKey != acc.SpendPublicKey {
		t.Fatal("spend public mismatch")
	}
	var zero [32]byte
	if view.SpendSecretKey != zero {
		t.Fatal("view-only should have zero spend secret")
	}
}

func TestAccountSaveLoad(t *testing.T) {
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	acc1, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	password := "test-password-123"
	if err := acc1.Save(s, password); err != nil {
		t.Fatal(err)
	}

	acc2, err := LoadAccount(s, password)
	if err != nil {
		t.Fatal(err)
	}
	if acc1.SpendSecretKey != acc2.SpendSecretKey {
		t.Fatal("spend secret mismatch after load")
	}
	if acc1.ViewSecretKey != acc2.ViewSecretKey {
		t.Fatal("view secret mismatch after load")
	}
}

func TestAccountLoadWrongPassword(t *testing.T) {
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	acc, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	if err := acc.Save(s, "correct"); err != nil {
		t.Fatal(err)
	}

	_, err = LoadAccount(s, "wrong")
	if err == nil {
		t.Fatal("expected error with wrong password")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -race -run TestAccount -v ./wallet/`
Expected: FAIL (GenerateAccount not defined)

**Step 3: Implement account management**

Create `wallet/account.go`:
```go
package wallet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/crypto"
	"forge.lthn.ai/core/go-blockchain/types"
	"golang.org/x/crypto/argon2"
)

const (
	walletGroup  = "wallet"
	accountKey   = "account"
	argonTime    = 3
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
	nonceLen     = 12 // AES-GCM nonce
)

// Account holds the wallet key material.
type Account struct {
	SpendPublicKey types.PublicKey  `json:"spend_public_key"`
	SpendSecretKey types.SecretKey `json:"spend_secret_key"`
	ViewPublicKey  types.PublicKey  `json:"view_public_key"`
	ViewSecretKey  types.SecretKey `json:"view_secret_key"`
	CreatedAt      uint64          `json:"created_at"`
	Flags          uint8           `json:"flags"`
}

// GenerateAccount creates a new account with random keys.
// The view secret is derived deterministically from the spend secret.
func GenerateAccount() (*Account, error) {
	spendPub, spendSec, err := crypto.GenerateKeys()
	if err != nil {
		return nil, fmt.Errorf("wallet: generate spend keys: %w", err)
	}
	return accountFromSpendKey(spendSec, spendPub)
}

// RestoreFromSeed restores an account from a 25-word mnemonic.
func RestoreFromSeed(phrase string) (*Account, error) {
	key, err := MnemonicDecode(phrase)
	if err != nil {
		return nil, fmt.Errorf("wallet: restore from seed: %w", err)
	}
	spendPub, err := crypto.SecretToPublic(key)
	if err != nil {
		return nil, fmt.Errorf("wallet: spend pub from secret: %w", err)
	}
	return accountFromSpendKey(key, spendPub)
}

// RestoreViewOnly creates a view-only account (can scan, cannot spend).
func RestoreViewOnly(viewSecret types.SecretKey, spendPublic types.PublicKey) (*Account, error) {
	viewPub, err := crypto.SecretToPublic([32]byte(viewSecret))
	if err != nil {
		return nil, fmt.Errorf("wallet: view pub from secret: %w", err)
	}
	return &Account{
		SpendPublicKey: spendPublic,
		ViewPublicKey:  types.PublicKey(viewPub),
		ViewSecretKey:  viewSecret,
	}, nil
}

// ToSeed exports the spend secret key as a 25-word mnemonic.
func (a *Account) ToSeed() (string, error) {
	return MnemonicEncode(a.SpendSecretKey[:])
}

// Address returns the standard address for this account.
func (a *Account) Address() types.Address {
	return types.Address{
		SpendPublicKey: a.SpendPublicKey,
		ViewPublicKey:  a.ViewPublicKey,
	}
}

// Save encrypts and stores the account in go-store.
func (a *Account) Save(s *store.Store, password string) error {
	plaintext, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("wallet: marshal account: %w", err)
	}

	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("wallet: generate salt: %w", err)
	}

	derived := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	block, err := aes.NewCipher(derived)
	if err != nil {
		return fmt.Errorf("wallet: aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("wallet: gcm: %w", err)
	}

	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("wallet: generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Store format: salt (16) + nonce (12) + ciphertext (variable)
	blob := make([]byte, 0, saltLen+nonceLen+len(ciphertext))
	blob = append(blob, salt...)
	blob = append(blob, nonce...)
	blob = append(blob, ciphertext...)

	encoded := fmt.Sprintf("%x", blob)
	return s.Set(walletGroup, accountKey, encoded)
}

// LoadAccount decrypts and loads an account from go-store.
func LoadAccount(s *store.Store, password string) (*Account, error) {
	encoded, err := s.Get(walletGroup, accountKey)
	if err != nil {
		return nil, fmt.Errorf("wallet: load account: %w", err)
	}

	blob, err := hexDecode(encoded)
	if err != nil {
		return nil, fmt.Errorf("wallet: decode account hex: %w", err)
	}

	if len(blob) < saltLen+nonceLen+1 {
		return nil, fmt.Errorf("wallet: account data too short")
	}

	salt := blob[:saltLen]
	nonce := blob[saltLen : saltLen+nonceLen]
	ciphertext := blob[saltLen+nonceLen:]

	derived := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	block, err := aes.NewCipher(derived)
	if err != nil {
		return nil, fmt.Errorf("wallet: aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("wallet: gcm: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("wallet: decrypt account: %w", err)
	}

	var acc Account
	if err := json.Unmarshal(plaintext, &acc); err != nil {
		return nil, fmt.Errorf("wallet: unmarshal account: %w", err)
	}
	return &acc, nil
}

func accountFromSpendKey(spendSec [32]byte, spendPub [32]byte) (*Account, error) {
	// View secret = Keccak256(spend_secret)
	viewSec := crypto.FastHash(spendSec[:])
	viewPub, err := crypto.SecretToPublic(viewSec)
	if err != nil {
		return nil, fmt.Errorf("wallet: view pub from secret: %w", err)
	}
	return &Account{
		SpendPublicKey: types.PublicKey(spendPub),
		SpendSecretKey: types.SecretKey(spendSec),
		ViewPublicKey:  types.PublicKey(viewPub),
		ViewSecretKey:  types.SecretKey(viewSec),
	}, nil
}

func hexDecode(s string) ([]byte, error) {
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		var v byte
		for j := 0; j < 2; j++ {
			c := s[i+j]
			switch {
			case c >= '0' && c <= '9':
				v = v*16 + c - '0'
			case c >= 'a' && c <= 'f':
				v = v*16 + c - 'a' + 10
			default:
				return nil, fmt.Errorf("invalid hex char %c", c)
			}
		}
		b[i/2] = v
	}
	return b, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -race -run TestAccount -v ./wallet/`
Expected: PASS (all 5 tests)

**Step 5: Commit**

```bash
git add wallet/account.go wallet/account_test.go
git commit -m "feat(wallet): account key management with Argon2id encryption"
```

---

## Task 4: Transfer Type and Storage

**Files:**
- Create: `wallet/transfer.go`
- Create: `wallet/transfer_test.go`

**Context:** The Transfer type represents an owned output. Transfers are stored in go-store group `transfers` keyed by key image hex. Balance is computed from unspent, unlocked transfers.

**Step 1: Write failing tests**

Create `wallet/transfer_test.go`:
```go
package wallet

import (
	"testing"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/types"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestTransferPutGet(t *testing.T) {
	s := newTestStore(t)
	var ki types.KeyImage
	ki[0] = 0x42

	tr := Transfer{
		TxHash:      types.Hash{1},
		OutputIndex: 0,
		Amount:      1000,
		BlockHeight: 10,
		KeyImage:    ki,
	}

	if err := putTransfer(s, &tr); err != nil {
		t.Fatal(err)
	}

	got, err := getTransfer(s, ki)
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != 1000 {
		t.Fatalf("amount = %d, want 1000", got.Amount)
	}
	if got.KeyImage != ki {
		t.Fatal("key image mismatch")
	}
}

func TestTransferMarkSpent(t *testing.T) {
	s := newTestStore(t)
	var ki types.KeyImage
	ki[0] = 0x43

	tr := Transfer{
		Amount:      500,
		BlockHeight: 5,
		KeyImage:    ki,
	}
	if err := putTransfer(s, &tr); err != nil {
		t.Fatal(err)
	}

	if err := markTransferSpent(s, ki, 20); err != nil {
		t.Fatal(err)
	}

	got, err := getTransfer(s, ki)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Spent {
		t.Fatal("transfer should be marked spent")
	}
	if got.SpentHeight != 20 {
		t.Fatalf("spent height = %d, want 20", got.SpentHeight)
	}
}

func TestTransferListUnspent(t *testing.T) {
	s := newTestStore(t)
	for i := byte(0); i < 3; i++ {
		var ki types.KeyImage
		ki[0] = i + 1
		tr := Transfer{
			Amount:      uint64(i+1) * 100,
			BlockHeight: uint64(i),
			KeyImage:    ki,
		}
		if err := putTransfer(s, &tr); err != nil {
			t.Fatal(err)
		}
	}

	// Mark one as spent
	var ki types.KeyImage
	ki[0] = 2
	if err := markTransferSpent(s, ki, 10); err != nil {
		t.Fatal(err)
	}

	transfers, err := listTransfers(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 3 {
		t.Fatalf("got %d transfers, want 3", len(transfers))
	}

	unspent := 0
	for _, tr := range transfers {
		if !tr.Spent {
			unspent++
		}
	}
	if unspent != 2 {
		t.Fatalf("got %d unspent, want 2", unspent)
	}
}

func TestTransferSpendable(t *testing.T) {
	tr := Transfer{Amount: 1000, BlockHeight: 5, Spent: false}
	// Chain height 20, no unlock, non-coinbase: should be spendable
	if !tr.IsSpendable(20, false) {
		t.Fatal("should be spendable")
	}

	// Spent transfer
	tr.Spent = true
	if tr.IsSpendable(20, false) {
		t.Fatal("spent should not be spendable")
	}

	// Coinbase with insufficient maturity
	tr.Spent = false
	tr.Coinbase = true
	if tr.IsSpendable(10, false) {
		t.Fatal("immature coinbase should not be spendable")
	}
	// Coinbase with sufficient maturity (height 5 + 10 = 15 <= 20)
	if !tr.IsSpendable(20, false) {
		t.Fatal("mature coinbase should be spendable")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -race -run TestTransfer -v ./wallet/`
Expected: FAIL

**Step 3: Implement transfer type and storage**

Create `wallet/transfer.go`:
```go
package wallet

import (
	"encoding/json"
	"fmt"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-blockchain/types"
)

const groupTransfers = "transfers"

// KeyPair holds an ephemeral key pair for a received output.
type KeyPair struct {
	Public types.PublicKey `json:"public"`
	Secret types.SecretKey `json:"secret"`
}

// Transfer represents an owned output detected during scanning.
type Transfer struct {
	TxHash       types.Hash     `json:"tx_hash"`
	OutputIndex  uint32         `json:"output_index"`
	Amount       uint64         `json:"amount"`
	GlobalIndex  uint64         `json:"global_index"`
	BlockHeight  uint64         `json:"block_height"`
	EphemeralKey KeyPair        `json:"ephemeral_key"`
	KeyImage     types.KeyImage `json:"key_image"`
	Spent        bool           `json:"spent"`
	SpentHeight  uint64         `json:"spent_height"`
	Coinbase     bool           `json:"coinbase"`
	UnlockTime   uint64         `json:"unlock_time"`
}

// IsSpendable returns true if this transfer can be used as a tx input.
func (t *Transfer) IsSpendable(chainHeight uint64, _ bool) bool {
	if t.Spent {
		return false
	}
	if t.Coinbase {
		if t.BlockHeight+config.MinedMoneyUnlockWindow > chainHeight {
			return false
		}
	}
	if t.UnlockTime > 0 && t.UnlockTime > chainHeight {
		return false
	}
	return true
}

func putTransfer(s *store.Store, tr *Transfer) error {
	val, err := json.Marshal(tr)
	if err != nil {
		return fmt.Errorf("wallet: marshal transfer: %w", err)
	}
	return s.Set(groupTransfers, tr.KeyImage.String(), string(val))
}

func getTransfer(s *store.Store, ki types.KeyImage) (*Transfer, error) {
	val, err := s.Get(groupTransfers, ki.String())
	if err != nil {
		return nil, fmt.Errorf("wallet: get transfer %s: %w", ki, err)
	}
	var tr Transfer
	if err := json.Unmarshal([]byte(val), &tr); err != nil {
		return nil, fmt.Errorf("wallet: unmarshal transfer: %w", err)
	}
	return &tr, nil
}

func markTransferSpent(s *store.Store, ki types.KeyImage, height uint64) error {
	tr, err := getTransfer(s, ki)
	if err != nil {
		return err
	}
	tr.Spent = true
	tr.SpentHeight = height
	return putTransfer(s, tr)
}

func listTransfers(s *store.Store) ([]Transfer, error) {
	pairs, err := s.List(groupTransfers)
	if err != nil {
		return nil, fmt.Errorf("wallet: list transfers: %w", err)
	}
	transfers := make([]Transfer, 0, len(pairs))
	for _, kv := range pairs {
		var tr Transfer
		if err := json.Unmarshal([]byte(kv.Value), &tr); err != nil {
			return nil, fmt.Errorf("wallet: unmarshal transfer: %w", err)
		}
		transfers = append(transfers, tr)
	}
	return transfers, nil
}
```

**Note:** This uses `s.List(group)` which returns all key-value pairs. Verify go-store supports this; if not, use `s.Keys(group)` + `s.Get()` per key.

**Step 4: Run tests to verify they pass**

Run: `go test -race -run TestTransfer -v ./wallet/`
Expected: PASS (all 4 tests)

**Step 5: Commit**

```bash
git add wallet/transfer.go wallet/transfer_test.go
git commit -m "feat(wallet): transfer type and go-store persistence"
```

---

## Task 5: Scanner Interface and V1Scanner

**Files:**
- Create: `wallet/scanner.go`
- Create: `wallet/scanner_test.go`

**Context:** The scanner detects outputs belonging to a wallet by performing ECDH derivation. For each transaction output, it derives the expected public key and compares it to the output's target key.

**Crypto API:**
- `crypto.GenerateKeyDerivation(pub, sec [32]byte) ([32]byte, error)` — ECDH
- `crypto.DerivePublicKey(derivation [32]byte, index uint64, base [32]byte) ([32]byte, error)`
- `crypto.DeriveSecretKey(derivation [32]byte, index uint64, base [32]byte) ([32]byte, error)`
- `crypto.GenerateKeyImage(pub, sec [32]byte) ([32]byte, error)`

**Step 1: Write failing tests**

Create `wallet/scanner_test.go`:
```go
package wallet

import (
	"testing"

	"forge.lthn.ai/core/go-blockchain/crypto"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

// makeTestTransaction creates a v0 tx with one output sent to the given address.
// Returns the tx, its hash, and the tx secret key used.
func makeTestTransaction(t *testing.T, destAddr *Account) (*types.Transaction, types.Hash, [32]byte) {
	t.Helper()

	// Generate a one-time tx key pair.
	txPub, txSec, err := crypto.GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}

	// Derive output key: derivation = txSec * destViewPub
	derivation, err := crypto.GenerateKeyDerivation(
		[32]byte(destAddr.ViewPublicKey), txSec)
	if err != nil {
		t.Fatal(err)
	}

	// Ephemeral public key for output 0.
	ephPub, err := crypto.DerivePublicKey(
		derivation, 0, [32]byte(destAddr.SpendPublicKey))
	if err != nil {
		t.Fatal(err)
	}

	tx := &types.Transaction{
		Version: 0,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 0}},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 1000,
				Target: types.TxOutToKey{Key: types.PublicKey(ephPub)},
			},
		},
		Extra:      BuildTxExtra(types.PublicKey(txPub)),
		Attachment: wire.EncodeVarint(0),
	}
	tx.Signatures = [][]types.Signature{{}} // one empty ring for genesis

	txHash := wire.TransactionHash(tx)
	return tx, txHash, txSec
}

func TestV1ScannerDetectsOwnedOutput(t *testing.T) {
	acc, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	tx, txHash, _ := makeTestTransaction(t, acc)
	extra, err := ParseTxExtra(tx.Extra)
	if err != nil {
		t.Fatal(err)
	}

	scanner := NewV1Scanner(acc)
	transfers, err := scanner.ScanTransaction(tx, txHash, 1, extra)
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 1 {
		t.Fatalf("got %d transfers, want 1", len(transfers))
	}
	if transfers[0].Amount != 1000 {
		t.Fatalf("amount = %d, want 1000", transfers[0].Amount)
	}
	if transfers[0].OutputIndex != 0 {
		t.Fatalf("output index = %d, want 0", transfers[0].OutputIndex)
	}
	// Key image should be non-zero.
	var zero types.KeyImage
	if transfers[0].KeyImage == zero {
		t.Fatal("key image should be non-zero")
	}
}

func TestV1ScannerRejectsNonOwned(t *testing.T) {
	acc1, _ := GenerateAccount()
	acc2, _ := GenerateAccount()

	// Send to acc1, scan with acc2 — should find nothing.
	tx, txHash, _ := makeTestTransaction(t, acc1)
	extra, _ := ParseTxExtra(tx.Extra)

	scanner := NewV1Scanner(acc2)
	transfers, err := scanner.ScanTransaction(tx, txHash, 1, extra)
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 0 {
		t.Fatalf("got %d transfers, want 0", len(transfers))
	}
}

func TestV1ScannerNoTxPubKey(t *testing.T) {
	acc, _ := GenerateAccount()

	// Transaction with empty extra (no tx pub key).
	tx := &types.Transaction{
		Version:    0,
		Vin:        []types.TxInput{types.TxInputGenesis{Height: 0}},
		Vout:       []types.TxOutput{types.TxOutputBare{Amount: 100}},
		Extra:      wire.EncodeVarint(0),
		Attachment: wire.EncodeVarint(0),
	}
	txHash := wire.TransactionHash(tx)
	extra, _ := ParseTxExtra(tx.Extra)

	scanner := NewV1Scanner(acc)
	transfers, err := scanner.ScanTransaction(tx, txHash, 1, extra)
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 0 {
		t.Fatalf("expected 0 transfers for missing tx pub key, got %d", len(transfers))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -race -run TestV1Scanner -v ./wallet/`
Expected: FAIL

**Step 3: Implement scanner**

Create `wallet/scanner.go`:
```go
package wallet

import (
	"forge.lthn.ai/core/go-blockchain/crypto"
	"forge.lthn.ai/core/go-blockchain/types"
)

// Scanner detects outputs belonging to a wallet.
type Scanner interface {
	ScanTransaction(tx *types.Transaction, txHash types.Hash,
		blockHeight uint64, extra *TxExtra) ([]Transfer, error)
}

// V1Scanner implements Scanner for v0/v1 transactions using ECDH.
type V1Scanner struct {
	account *Account
}

// NewV1Scanner creates a scanner for the given account.
func NewV1Scanner(acc *Account) *V1Scanner {
	return &V1Scanner{account: acc}
}

// ScanTransaction checks each output for ownership via ECDH derivation.
func (s *V1Scanner) ScanTransaction(tx *types.Transaction, txHash types.Hash,
	blockHeight uint64, extra *TxExtra) ([]Transfer, error) {

	// Need a TX public key to compute derivation.
	if extra.TxPublicKey.IsZero() {
		return nil, nil
	}

	derivation, err := crypto.GenerateKeyDerivation(
		[32]byte(extra.TxPublicKey),
		[32]byte(s.account.ViewSecretKey))
	if err != nil {
		return nil, nil // skip tx if derivation fails
	}

	isCoinbase := len(tx.Vin) > 0 && tx.Vin[0].InputType() == types.InputTypeGenesis

	var transfers []Transfer
	for i, out := range tx.Vout {
		bare, ok := out.(types.TxOutputBare)
		if !ok {
			continue
		}

		expectedPub, err := crypto.DerivePublicKey(
			derivation, uint64(i), [32]byte(s.account.SpendPublicKey))
		if err != nil {
			continue
		}

		if types.PublicKey(expectedPub) != bare.Target.Key {
			continue
		}

		// Output is ours. Derive ephemeral secret key.
		ephSec, err := crypto.DeriveSecretKey(
			derivation, uint64(i), [32]byte(s.account.SpendSecretKey))
		if err != nil {
			continue
		}

		// Generate key image.
		ki, err := crypto.GenerateKeyImage(expectedPub, ephSec)
		if err != nil {
			continue
		}

		transfers = append(transfers, Transfer{
			TxHash:      txHash,
			OutputIndex: uint32(i),
			Amount:      bare.Amount,
			BlockHeight: blockHeight,
			EphemeralKey: KeyPair{
				Public: types.PublicKey(expectedPub),
				Secret: types.SecretKey(ephSec),
			},
			KeyImage:   types.KeyImage(ki),
			Coinbase:   isCoinbase,
			UnlockTime: extra.UnlockTime,
		})
	}

	return transfers, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -race -run TestV1Scanner -v ./wallet/`
Expected: PASS (all 3 tests)

**Step 5: Commit**

```bash
git add wallet/scanner.go wallet/scanner_test.go
git commit -m "feat(wallet): V1Scanner with ECDH output detection"
```

---

## Task 6: New RPC Endpoints

**Files:**
- Create: `rpc/wallet.go`
- Create: `rpc/wallet_test.go`

**Context:** The wallet needs two new RPC endpoints: `GetRandomOutputs` (for ring selection decoys) wrapping the daemon's `getrandom_outs` legacy endpoint, and `SendRawTransaction` wrapping `/sendrawtransaction`.

**Step 1: Write failing tests**

Create `rpc/wallet_test.go`:
```go
package rpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetRandomOutputs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getrandom_outs1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		resp := struct {
			Outs   []RandomOutputEntry `json:"outs"`
			Status string              `json:"status"`
		}{
			Outs: []RandomOutputEntry{
				{GlobalIndex: 10, PublicKey: "aa" + repeatHex("00", 31)},
				{GlobalIndex: 20, PublicKey: "bb" + repeatHex("00", 31)},
			},
			Status: "OK",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	outs, err := client.GetRandomOutputs(1000, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(outs) != 2 {
		t.Fatalf("got %d outputs, want 2", len(outs))
	}
	if outs[0].GlobalIndex != 10 {
		t.Fatalf("first output global index = %d, want 10", outs[0].GlobalIndex)
	}
}

func TestSendRawTransaction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sendrawtransaction" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		resp := struct {
			Status string `json:"status"`
		}{Status: "OK"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	err := client.SendRawTransaction([]byte{0x01, 0x02})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSendRawTransactionRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			Status string `json:"status"`
		}{Status: "Failed"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	err := client.SendRawTransaction([]byte{0x01})
	if err == nil {
		t.Fatal("expected error for rejected tx")
	}
}

func repeatHex(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -race -run "TestGetRandom|TestSendRaw" -v ./rpc/`
Expected: FAIL

**Step 3: Implement RPC endpoints**

Create `rpc/wallet.go`:
```go
package rpc

import (
	"encoding/hex"
	"fmt"
)

// RandomOutputEntry is a decoy output returned by getrandom_outs.
type RandomOutputEntry struct {
	GlobalIndex uint64 `json:"global_index"`
	PublicKey   string `json:"public_key"`
}

// GetRandomOutputs fetches random decoy outputs for ring construction.
// Uses the legacy /getrandom_outs1 endpoint.
func (c *Client) GetRandomOutputs(amount uint64, count int) ([]RandomOutputEntry, error) {
	params := struct {
		Amount uint64 `json:"amount"`
		Count  int    `json:"outs_count"`
	}{Amount: amount, Count: count}

	var resp struct {
		Outs   []RandomOutputEntry `json:"outs"`
		Status string              `json:"status"`
	}

	if err := c.legacyCall("/getrandom_outs1", params, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "OK" {
		return nil, fmt.Errorf("getrandom_outs: status %q", resp.Status)
	}
	return resp.Outs, nil
}

// SendRawTransaction submits a serialised transaction for relay.
// Uses the legacy /sendrawtransaction endpoint.
func (c *Client) SendRawTransaction(txBlob []byte) error {
	params := struct {
		TxAsHex string `json:"tx_as_hex"`
	}{TxAsHex: hex.EncodeToString(txBlob)}

	var resp struct {
		Status string `json:"status"`
	}

	if err := c.legacyCall("/sendrawtransaction", params, &resp); err != nil {
		return err
	}
	if resp.Status != "OK" {
		return fmt.Errorf("sendrawtransaction: status %q", resp.Status)
	}
	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -race -run "TestGetRandom|TestSendRaw" -v ./rpc/`
Expected: PASS (all 3 tests)

**Step 5: Commit**

```bash
git add rpc/wallet.go rpc/wallet_test.go
git commit -m "feat(rpc): GetRandomOutputs and SendRawTransaction endpoints"
```

---

## Task 7: Signer Interface and NLSAGSigner

**Files:**
- Create: `wallet/signer.go`
- Create: `wallet/signer_test.go`

**Context:** Wraps the CGo ring signature functions behind a Signer interface.

**Step 1: Write failing tests**

Create `wallet/signer_test.go`:
```go
package wallet

import (
	"testing"

	"forge.lthn.ai/core/go-blockchain/crypto"
	"forge.lthn.ai/core/go-blockchain/types"
)

func TestNLSAGSignerRoundTrip(t *testing.T) {
	// Generate ephemeral key pair (simulating an owned output).
	pub, sec, err := crypto.GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}

	// Generate key image.
	ki, err := crypto.GenerateKeyImage(pub, sec)
	if err != nil {
		t.Fatal(err)
	}

	// Build a ring of 3 keys with our key at index 1.
	ring := make([]types.PublicKey, 3)
	for i := range ring {
		p, _, err := crypto.GenerateKeys()
		if err != nil {
			t.Fatal(err)
		}
		ring[i] = types.PublicKey(p)
	}
	ring[1] = types.PublicKey(pub)

	// Sign.
	var prefixHash types.Hash
	prefixHash[0] = 0xFF

	signer := &NLSAGSigner{}
	sigs, err := signer.SignInput(prefixHash, KeyPair{
		Public: types.PublicKey(pub),
		Secret: types.SecretKey(sec),
	}, ring, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(sigs) != 3 {
		t.Fatalf("got %d sigs, want 3", len(sigs))
	}

	// Verify.
	pubs := make([][32]byte, len(ring))
	for i, k := range ring {
		pubs[i] = [32]byte(k)
	}
	rawSigs := make([][64]byte, len(sigs))
	for i, s := range sigs {
		rawSigs[i] = [64]byte(s)
	}
	if !crypto.CheckRingSignature([32]byte(prefixHash), ki, pubs, rawSigs) {
		t.Fatal("ring signature verification failed")
	}
}

func TestNLSAGSignerVersion(t *testing.T) {
	signer := &NLSAGSigner{}
	if signer.Version() != 1 {
		t.Fatalf("version = %d, want 1", signer.Version())
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -race -run TestNLSAG -v ./wallet/`
Expected: FAIL

**Step 3: Implement signer**

Create `wallet/signer.go`:
```go
package wallet

import (
	"fmt"

	"forge.lthn.ai/core/go-blockchain/crypto"
	"forge.lthn.ai/core/go-blockchain/types"
)

// Signer produces signatures for transaction inputs.
type Signer interface {
	SignInput(prefixHash types.Hash, ephemeral KeyPair,
		ring []types.PublicKey, realIndex int) ([]types.Signature, error)
	Version() uint64
}

// NLSAGSigner signs using NLSAG ring signatures (v0/v1 transactions).
type NLSAGSigner struct{}

// SignInput generates an NLSAG ring signature.
func (s *NLSAGSigner) SignInput(prefixHash types.Hash, ephemeral KeyPair,
	ring []types.PublicKey, realIndex int) ([]types.Signature, error) {

	ki, err := crypto.GenerateKeyImage(
		[32]byte(ephemeral.Public), [32]byte(ephemeral.Secret))
	if err != nil {
		return nil, fmt.Errorf("wallet: key image: %w", err)
	}

	pubs := make([][32]byte, len(ring))
	for i, k := range ring {
		pubs[i] = [32]byte(k)
	}

	rawSigs, err := crypto.GenerateRingSignature(
		[32]byte(prefixHash), ki, pubs,
		[32]byte(ephemeral.Secret), realIndex)
	if err != nil {
		return nil, fmt.Errorf("wallet: ring signature: %w", err)
	}

	sigs := make([]types.Signature, len(rawSigs))
	for i, rs := range rawSigs {
		sigs[i] = types.Signature(rs)
	}
	return sigs, nil
}

// Version returns the transaction version this signer supports.
func (s *NLSAGSigner) Version() uint64 { return 1 }
```

**Step 4: Run tests to verify they pass**

Run: `go test -race -run TestNLSAG -v ./wallet/`
Expected: PASS (both tests)

**Step 5: Commit**

```bash
git add wallet/signer.go wallet/signer_test.go
git commit -m "feat(wallet): NLSAGSigner with ring signature interface"
```

---

## Task 8: RingSelector Interface and RPCRingSelector

**Files:**
- Create: `wallet/ring.go`
- Create: `wallet/ring_test.go`

**Context:** RPCRingSelector fetches decoy outputs from the daemon and builds rings.

**Step 1: Write failing tests**

Create `wallet/ring_test.go`:
```go
package wallet

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"forge.lthn.ai/core/go-blockchain/rpc"
	"forge.lthn.ai/core/go-blockchain/types"
)

func TestRPCRingSelector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			Outs   []rpc.RandomOutputEntry `json:"outs"`
			Status string                  `json:"status"`
		}{
			Status: "OK",
		}
		// Return 12 decoys (more than ring size, selector picks subset).
		for i := 0; i < 12; i++ {
			var key types.PublicKey
			key[0] = byte(i + 1)
			resp.Outs = append(resp.Outs, rpc.RandomOutputEntry{
				GlobalIndex: uint64((i + 1) * 100),
				PublicKey:   key.String(),
			})
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := rpc.NewClient(srv.URL)
	selector := NewRPCRingSelector(client)

	members, err := selector.SelectRing(1000, 500, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 10 {
		t.Fatalf("got %d ring members, want 10", len(members))
	}

	// Verify no duplicates.
	seen := make(map[uint64]bool)
	for _, m := range members {
		if seen[m.GlobalIndex] {
			t.Fatalf("duplicate global index %d", m.GlobalIndex)
		}
		seen[m.GlobalIndex] = true
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -race -run TestRPCRing -v ./wallet/`
Expected: FAIL

**Step 3: Implement ring selector**

Create `wallet/ring.go`:
```go
package wallet

import (
	"fmt"

	"forge.lthn.ai/core/go-blockchain/rpc"
	"forge.lthn.ai/core/go-blockchain/types"
)

// RingMember is a public key + global index used in ring construction.
type RingMember struct {
	PublicKey   types.PublicKey
	GlobalIndex uint64
}

// RingSelector picks decoy outputs for ring signatures.
type RingSelector interface {
	SelectRing(amount uint64, realGlobalIndex uint64,
		ringSize int) ([]RingMember, error)
}

// RPCRingSelector fetches decoys from the daemon via RPC.
type RPCRingSelector struct {
	client *rpc.Client
}

// NewRPCRingSelector creates a ring selector using the given RPC client.
func NewRPCRingSelector(client *rpc.Client) *RPCRingSelector {
	return &RPCRingSelector{client: client}
}

// SelectRing fetches random outputs and returns ringSize decoy members,
// excluding the real output's global index.
func (s *RPCRingSelector) SelectRing(amount uint64, realGlobalIndex uint64,
	ringSize int) ([]RingMember, error) {

	// Request extra outputs to account for filtering.
	outs, err := s.client.GetRandomOutputs(amount, ringSize+5)
	if err != nil {
		return nil, fmt.Errorf("wallet: get random outputs: %w", err)
	}

	var members []RingMember
	seen := map[uint64]bool{realGlobalIndex: true}
	for _, out := range outs {
		if seen[out.GlobalIndex] {
			continue
		}
		seen[out.GlobalIndex] = true

		pk, err := types.PublicKeyFromHex(out.PublicKey)
		if err != nil {
			continue
		}
		members = append(members, RingMember{
			PublicKey:   pk,
			GlobalIndex: out.GlobalIndex,
		})
		if len(members) >= ringSize {
			break
		}
	}

	if len(members) < ringSize {
		return nil, fmt.Errorf("wallet: insufficient decoys: got %d, need %d",
			len(members), ringSize)
	}

	return members, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -race -run TestRPCRing -v ./wallet/`
Expected: PASS

**Step 5: Commit**

```bash
git add wallet/ring.go wallet/ring_test.go
git commit -m "feat(wallet): RPCRingSelector for decoy output selection"
```

---

## Task 9: Builder Interface and V1Builder

**Files:**
- Create: `wallet/builder.go`
- Create: `wallet/builder_test.go`

**Context:** Constructs signed v1 transactions. Takes source transfers, destinations, and fee. Handles change output, ring construction, and NLSAG signing.

**Step 1: Write failing tests**

Create `wallet/builder_test.go`:
```go
package wallet

import (
	"testing"

	"forge.lthn.ai/core/go-blockchain/crypto"
	"forge.lthn.ai/core/go-blockchain/types"
)

// mockRingSelector returns fixed ring members for testing.
type mockRingSelector struct{}

func (m *mockRingSelector) SelectRing(amount uint64, realGlobalIndex uint64,
	ringSize int) ([]RingMember, error) {
	members := make([]RingMember, ringSize)
	for i := range members {
		pub, _, _ := crypto.GenerateKeys()
		members[i] = RingMember{
			PublicKey:   types.PublicKey(pub),
			GlobalIndex: uint64(i * 100),
		}
	}
	return members, nil
}

func TestV1BuilderBasic(t *testing.T) {
	sender, _ := GenerateAccount()
	receiver, _ := GenerateAccount()

	// Create a fake source transfer.
	pub, sec, _ := crypto.GenerateKeys()
	ki, _ := crypto.GenerateKeyImage(pub, sec)
	source := Transfer{
		Amount:      5000,
		GlobalIndex: 42,
		EphemeralKey: KeyPair{
			Public: types.PublicKey(pub),
			Secret: types.SecretKey(sec),
		},
		KeyImage: types.KeyImage(ki),
	}

	builder := NewV1Builder(&NLSAGSigner{}, &mockRingSelector{})
	req := &BuildRequest{
		Sources: []Transfer{source},
		Destinations: []Destination{
			{Address: receiver.Address(), Amount: 3000},
		},
		Fee:           1000,
		SenderAddress: sender.Address(),
	}

	tx, err := builder.Build(req)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Version != 1 {
		t.Fatalf("version = %d, want 1", tx.Version)
	}
	if len(tx.Vin) != 1 {
		t.Fatalf("inputs = %d, want 1", len(tx.Vin))
	}
	// 1 destination + 1 change = 2 outputs
	if len(tx.Vout) != 2 {
		t.Fatalf("outputs = %d, want 2", len(tx.Vout))
	}

	// Verify output amounts sum correctly.
	var outSum uint64
	for _, out := range tx.Vout {
		bare := out.(types.TxOutputBare)
		outSum += bare.Amount
	}
	if outSum+1000 != 5000 {
		t.Fatalf("output sum + fee = %d, want 5000", outSum+1000)
	}
}

func TestV1BuilderInsufficientFunds(t *testing.T) {
	sender, _ := GenerateAccount()
	receiver, _ := GenerateAccount()

	pub, sec, _ := crypto.GenerateKeys()
	ki, _ := crypto.GenerateKeyImage(pub, sec)
	source := Transfer{
		Amount:       1000,
		GlobalIndex:  1,
		EphemeralKey: KeyPair{Public: types.PublicKey(pub), Secret: types.SecretKey(sec)},
		KeyImage:     types.KeyImage(ki),
	}

	builder := NewV1Builder(&NLSAGSigner{}, &mockRingSelector{})
	req := &BuildRequest{
		Sources:      []Transfer{source},
		Destinations: []Destination{{Address: receiver.Address(), Amount: 2000}},
		Fee:          1000,
		SenderAddress: sender.Address(),
	}

	_, err := builder.Build(req)
	if err == nil {
		t.Fatal("expected insufficient funds error")
	}
}

func TestV1BuilderExactAmount(t *testing.T) {
	sender, _ := GenerateAccount()
	receiver, _ := GenerateAccount()

	pub, sec, _ := crypto.GenerateKeys()
	ki, _ := crypto.GenerateKeyImage(pub, sec)
	source := Transfer{
		Amount:       2000,
		GlobalIndex:  5,
		EphemeralKey: KeyPair{Public: types.PublicKey(pub), Secret: types.SecretKey(sec)},
		KeyImage:     types.KeyImage(ki),
	}

	builder := NewV1Builder(&NLSAGSigner{}, &mockRingSelector{})
	req := &BuildRequest{
		Sources:       []Transfer{source},
		Destinations:  []Destination{{Address: receiver.Address(), Amount: 1000}},
		Fee:           1000,
		SenderAddress: sender.Address(),
	}

	tx, err := builder.Build(req)
	if err != nil {
		t.Fatal(err)
	}
	// Exact amount: no change output needed.
	if len(tx.Vout) != 1 {
		t.Fatalf("outputs = %d, want 1 (no change)", len(tx.Vout))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -race -run TestV1Builder -v ./wallet/`
Expected: FAIL

**Step 3: Implement builder**

Create `wallet/builder.go`:
```go
package wallet

import (
	"bytes"
	"fmt"
	"sort"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-blockchain/crypto"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

// Destination is a recipient address and amount.
type Destination struct {
	Address types.Address
	Amount  uint64
}

// BuildRequest holds the parameters for building a transaction.
type BuildRequest struct {
	Sources       []Transfer
	Destinations  []Destination
	Fee           uint64
	SenderAddress types.Address
}

// Builder constructs unsigned or signed transactions.
type Builder interface {
	Build(req *BuildRequest) (*types.Transaction, error)
}

// V1Builder constructs v1 transactions with NLSAG ring signatures.
type V1Builder struct {
	signer       Signer
	ringSelector RingSelector
}

// NewV1Builder creates a builder with the given signer and ring selector.
func NewV1Builder(signer Signer, ringSelector RingSelector) *V1Builder {
	return &V1Builder{signer: signer, ringSelector: ringSelector}
}

// Build constructs a signed v1 transaction.
func (b *V1Builder) Build(req *BuildRequest) (*types.Transaction, error) {
	// Validate amounts.
	var sourceSum uint64
	for _, s := range req.Sources {
		sourceSum += s.Amount
	}
	var destSum uint64
	for _, d := range req.Destinations {
		destSum += d.Amount
	}
	if sourceSum < destSum+req.Fee {
		return nil, fmt.Errorf("wallet: insufficient funds: have %d, need %d",
			sourceSum, destSum+req.Fee)
	}

	// Generate one-time TX key pair.
	txPub, txSec, err := crypto.GenerateKeys()
	if err != nil {
		return nil, fmt.Errorf("wallet: generate tx keys: %w", err)
	}

	tx := &types.Transaction{Version: 1}

	// Build inputs.
	type inputMeta struct {
		ring      []types.PublicKey
		realIndex int
		ephemeral KeyPair
	}
	var metas []inputMeta

	for _, src := range req.Sources {
		ringMembers, err := b.ringSelector.SelectRing(
			src.Amount, src.GlobalIndex, int(config.DefaultDecoySetSize))
		if err != nil {
			return nil, fmt.Errorf("wallet: select ring: %w", err)
		}

		// Insert real output into the ring.
		all := append(ringMembers, RingMember{
			PublicKey:   src.EphemeralKey.Public,
			GlobalIndex: src.GlobalIndex,
		})

		// Sort by global index (consensus rule).
		sort.Slice(all, func(i, j int) bool {
			return all[i].GlobalIndex < all[j].GlobalIndex
		})

		// Find real index after sorting.
		realIdx := -1
		ring := make([]types.PublicKey, len(all))
		offsets := make([]types.TxOutRef, len(all))
		for i, m := range all {
			ring[i] = m.PublicKey
			offsets[i] = types.TxOutRef{
				Tag:         types.RefTypeGlobalIndex,
				GlobalIndex: m.GlobalIndex,
			}
			if m.GlobalIndex == src.GlobalIndex {
				realIdx = i
			}
		}
		if realIdx < 0 {
			return nil, fmt.Errorf("wallet: real output not found in ring")
		}

		tx.Vin = append(tx.Vin, types.TxInputToKey{
			Amount:     src.Amount,
			KeyOffsets: offsets,
			KeyImage:   src.KeyImage,
			EtcDetails: wire.EncodeVarint(0),
		})
		metas = append(metas, inputMeta{
			ring:      ring,
			realIndex: realIdx,
			ephemeral: src.EphemeralKey,
		})
	}

	// Build outputs.
	outputIdx := 0
	for _, dest := range req.Destinations {
		outKey, err := deriveOutputKey(txSec, dest.Address, uint64(outputIdx))
		if err != nil {
			return nil, err
		}
		tx.Vout = append(tx.Vout, types.TxOutputBare{
			Amount: dest.Amount,
			Target: types.TxOutToKey{Key: outKey},
		})
		outputIdx++
	}

	// Change output.
	change := sourceSum - destSum - req.Fee
	if change > 0 {
		outKey, err := deriveOutputKey(txSec, req.SenderAddress, uint64(outputIdx))
		if err != nil {
			return nil, err
		}
		tx.Vout = append(tx.Vout, types.TxOutputBare{
			Amount: change,
			Target: types.TxOutToKey{Key: outKey},
		})
	}

	// Build extra.
	tx.Extra = BuildTxExtra(types.PublicKey(txPub))
	tx.Attachment = wire.EncodeVarint(0)

	// Compute prefix hash.
	prefixHash := wire.TransactionPrefixHash(tx)

	// Sign each input.
	tx.Signatures = make([][]types.Signature, len(tx.Vin))
	for i, meta := range metas {
		sigs, err := b.signer.SignInput(prefixHash, meta.ephemeral, meta.ring, meta.realIndex)
		if err != nil {
			return nil, fmt.Errorf("wallet: sign input %d: %w", i, err)
		}
		tx.Signatures[i] = sigs
	}

	return tx, nil
}

// deriveOutputKey derives the one-time output key for a destination.
func deriveOutputKey(txSec [32]byte, addr types.Address, outputIdx uint64) (types.PublicKey, error) {
	derivation, err := crypto.GenerateKeyDerivation(
		[32]byte(addr.ViewPublicKey), txSec)
	if err != nil {
		return types.PublicKey{}, fmt.Errorf("wallet: output derivation: %w", err)
	}
	ephPub, err := crypto.DerivePublicKey(
		derivation, outputIdx, [32]byte(addr.SpendPublicKey))
	if err != nil {
		return types.PublicKey{}, fmt.Errorf("wallet: derive output key: %w", err)
	}
	return types.PublicKey(ephPub), nil
}

// SerializeTransaction serialises a transaction to wire format bytes.
func SerializeTransaction(tx *types.Transaction) ([]byte, error) {
	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	wire.EncodeTransaction(enc, tx)
	if err := enc.Err(); err != nil {
		return nil, fmt.Errorf("wallet: encode tx: %w", err)
	}
	return buf.Bytes(), nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -race -run TestV1Builder -v ./wallet/`
Expected: PASS (all 3 tests)

**Step 5: Commit**

```bash
git add wallet/builder.go wallet/builder_test.go
git commit -m "feat(wallet): V1Builder for transaction construction with ring signatures"
```

---

## Task 10: Wallet Orchestrator

**Files:**
- Create: `wallet/wallet.go`
- Create: `wallet/wallet_test.go`

**Context:** The Wallet struct ties everything together: sync from chain, compute balance, construct and send transactions.

**Step 1: Write failing tests**

Create `wallet/wallet_test.go`:
```go
package wallet

import (
	"testing"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/chain"
	"forge.lthn.ai/core/go-blockchain/crypto"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

func makeTestBlock(t *testing.T, height uint64, prevHash types.Hash,
	destAccount *Account) (*types.Block, types.Hash) {
	t.Helper()

	txPub, txSec, _ := crypto.GenerateKeys()
	derivation, _ := crypto.GenerateKeyDerivation(
		[32]byte(destAccount.ViewPublicKey), txSec)
	ephPub, _ := crypto.DerivePublicKey(
		derivation, 0, [32]byte(destAccount.SpendPublicKey))

	minerTx := types.Transaction{
		Version: 0,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: height}},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 1_000_000_000_000, // 1 LTHN
				Target: types.TxOutToKey{Key: types.PublicKey(ephPub)},
			},
		},
		Extra:      BuildTxExtra(types.PublicKey(txPub)),
		Attachment: wire.EncodeVarint(0),
	}
	minerTx.Signatures = [][]types.Signature{{}}

	blk := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Timestamp:    1770897600 + height*120,
			PrevID:       prevHash,
		},
		MinerTx: minerTx,
	}

	hash := wire.BlockHash(blk)
	return blk, hash
}

func TestWalletSyncAndBalance(t *testing.T) {
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	acc, _ := GenerateAccount()
	c := chain.New(s)

	// Store 3 blocks, each paying 1 LTHN to our account.
	var prevHash types.Hash
	for h := uint64(0); h < 3; h++ {
		blk, hash := makeTestBlock(t, h, prevHash, acc)
		meta := &chain.BlockMeta{Hash: hash, Height: h}
		if err := c.PutBlock(blk, meta); err != nil {
			t.Fatal(err)
		}
		// Index the output.
		txHash := wire.TransactionHash(&blk.MinerTx)
		c.PutOutput(1_000_000_000_000, txHash, 0)
		prevHash = hash
	}

	w := NewWallet(acc, s, c, nil)
	if err := w.Sync(); err != nil {
		t.Fatal(err)
	}

	confirmed, locked, err := w.Balance()
	if err != nil {
		t.Fatal(err)
	}

	// All 3 blocks are coinbase with MinedMoneyUnlockWindow=10.
	// Chain height = 3, so all 3 are locked (height + 10 > 3).
	if locked != 3_000_000_000_000 {
		t.Fatalf("locked = %d, want 3_000_000_000_000", locked)
	}
	if confirmed != 0 {
		t.Fatalf("confirmed = %d, want 0 (all locked)", confirmed)
	}
}

func TestWalletTransfers(t *testing.T) {
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	acc, _ := GenerateAccount()
	c := chain.New(s)

	blk, hash := makeTestBlock(t, 0, types.Hash{}, acc)
	meta := &chain.BlockMeta{Hash: hash, Height: 0}
	c.PutBlock(blk, meta)
	txHash := wire.TransactionHash(&blk.MinerTx)
	c.PutOutput(1_000_000_000_000, txHash, 0)

	w := NewWallet(acc, s, c, nil)
	w.Sync()

	transfers, err := w.Transfers()
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 1 {
		t.Fatalf("got %d transfers, want 1", len(transfers))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -race -run "TestWallet" -v ./wallet/`
Expected: FAIL

**Step 3: Implement wallet orchestrator**

Create `wallet/wallet.go`:
```go
package wallet

import (
	"fmt"
	"sort"
	"strconv"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/chain"
	"forge.lthn.ai/core/go-blockchain/rpc"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

const (
	scanHeightKey = "scan_height"
)

// Wallet ties together scanning, building, and sending.
type Wallet struct {
	account      *Account
	store        *store.Store
	chain        *chain.Chain
	client       *rpc.Client
	scanner      Scanner
	signer       Signer
	ringSelector RingSelector
	builder      Builder
}

// NewWallet creates a wallet with v1 defaults.
func NewWallet(account *Account, s *store.Store, c *chain.Chain,
	client *rpc.Client) *Wallet {

	scanner := NewV1Scanner(account)
	signer := &NLSAGSigner{}
	var ringSelector RingSelector
	var builder Builder
	if client != nil {
		ringSelector = NewRPCRingSelector(client)
		builder = NewV1Builder(signer, ringSelector)
	}

	return &Wallet{
		account:      account,
		store:        s,
		chain:        c,
		client:       client,
		scanner:      scanner,
		signer:       signer,
		ringSelector: ringSelector,
		builder:      builder,
	}
}

// Sync scans blocks from the last checkpoint to the chain tip.
func (w *Wallet) Sync() error {
	lastScanned := w.loadScanHeight()

	chainHeight, err := w.chain.Height()
	if err != nil {
		return fmt.Errorf("wallet: chain height: %w", err)
	}

	for h := lastScanned; h < chainHeight; h++ {
		blk, _, err := w.chain.GetBlockByHeight(h)
		if err != nil {
			return fmt.Errorf("wallet: get block %d: %w", h, err)
		}

		// Scan miner tx.
		if err := w.scanTx(&blk.MinerTx, h); err != nil {
			return err
		}

		// Scan regular transactions.
		for _, txHash := range blk.TxHashes {
			tx, _, err := w.chain.GetTransaction(txHash)
			if err != nil {
				continue // skip missing txs
			}
			if err := w.scanTx(tx, h); err != nil {
				return err
			}
		}

		w.saveScanHeight(h + 1)
	}

	return nil
}

func (w *Wallet) scanTx(tx *types.Transaction, blockHeight uint64) error {
	txHash := wire.TransactionHash(tx)
	extra, err := ParseTxExtra(tx.Extra)
	if err != nil {
		return nil // skip unparseable extras
	}

	// Detect owned outputs.
	transfers, err := w.scanner.ScanTransaction(tx, txHash, blockHeight, extra)
	if err != nil {
		return nil
	}
	for i := range transfers {
		if err := putTransfer(w.store, &transfers[i]); err != nil {
			return fmt.Errorf("wallet: store transfer: %w", err)
		}
	}

	// Check key images for spend detection.
	for _, vin := range tx.Vin {
		toKey, ok := vin.(types.TxInputToKey)
		if !ok {
			continue
		}
		// Try to mark any matching transfer as spent.
		tr, err := getTransfer(w.store, toKey.KeyImage)
		if err != nil {
			continue // not our transfer
		}
		if !tr.Spent {
			markTransferSpent(w.store, toKey.KeyImage, blockHeight)
		}
	}

	return nil
}

// Balance returns confirmed (spendable) and locked amounts.
func (w *Wallet) Balance() (confirmed, locked uint64, err error) {
	chainHeight, err := w.chain.Height()
	if err != nil {
		return 0, 0, err
	}

	transfers, err := listTransfers(w.store)
	if err != nil {
		return 0, 0, err
	}

	for _, tr := range transfers {
		if tr.Spent {
			continue
		}
		if tr.IsSpendable(chainHeight, false) {
			confirmed += tr.Amount
		} else {
			locked += tr.Amount
		}
	}

	return confirmed, locked, nil
}

// Send constructs and submits a transaction.
func (w *Wallet) Send(destinations []Destination, fee uint64) (*types.Transaction, error) {
	if w.builder == nil || w.client == nil {
		return nil, fmt.Errorf("wallet: no RPC client configured")
	}

	chainHeight, err := w.chain.Height()
	if err != nil {
		return nil, err
	}

	var destSum uint64
	for _, d := range destinations {
		destSum += d.Amount
	}
	needed := destSum + fee

	// Coin selection: largest-first greedy.
	transfers, err := listTransfers(w.store)
	if err != nil {
		return nil, err
	}

	// Filter spendable and sort by amount descending.
	var spendable []Transfer
	for _, tr := range transfers {
		if tr.IsSpendable(chainHeight, false) {
			spendable = append(spendable, tr)
		}
	}
	sort.Slice(spendable, func(i, j int) bool {
		return spendable[i].Amount > spendable[j].Amount
	})

	var selected []Transfer
	var selectedSum uint64
	for _, tr := range spendable {
		selected = append(selected, tr)
		selectedSum += tr.Amount
		if selectedSum >= needed {
			break
		}
	}
	if selectedSum < needed {
		return nil, fmt.Errorf("wallet: insufficient balance: have %d, need %d",
			selectedSum, needed)
	}

	req := &BuildRequest{
		Sources:       selected,
		Destinations:  destinations,
		Fee:           fee,
		SenderAddress: w.account.Address(),
	}

	tx, err := w.builder.Build(req)
	if err != nil {
		return nil, err
	}

	blob, err := SerializeTransaction(tx)
	if err != nil {
		return nil, err
	}

	if err := w.client.SendRawTransaction(blob); err != nil {
		return nil, fmt.Errorf("wallet: submit tx: %w", err)
	}

	// Optimistically mark sources as spent.
	for _, src := range selected {
		markTransferSpent(w.store, src.KeyImage, chainHeight)
	}

	return tx, nil
}

// Transfers returns all tracked transfers.
func (w *Wallet) Transfers() ([]Transfer, error) {
	return listTransfers(w.store)
}

func (w *Wallet) loadScanHeight() uint64 {
	val, err := w.store.Get(walletGroup, scanHeightKey)
	if err != nil {
		return 0
	}
	h, _ := strconv.ParseUint(val, 10, 64)
	return h
}

func (w *Wallet) saveScanHeight(h uint64) {
	w.store.Set(walletGroup, scanHeightKey, strconv.FormatUint(h, 10))
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -race -run "TestWallet" -v ./wallet/`
Expected: PASS (both tests)

**Step 5: Commit**

```bash
git add wallet/wallet.go wallet/wallet_test.go
git commit -m "feat(wallet): orchestrator with sync, balance, and send"
```

---

## Task 11: Integration Test and Documentation

**Files:**
- Create: `wallet/integration_test.go`
- Modify: `docs/architecture.md`
- Modify: `docs/history.md`

**Step 1: Write integration test**

Create `wallet/integration_test.go`:
```go
//go:build integration

package wallet

import (
	"testing"

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/chain"
	"forge.lthn.ai/core/go-blockchain/rpc"
)

func TestWalletIntegration(t *testing.T) {
	client := rpc.NewClient("http://localhost:46941")

	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	c := chain.New(s)

	// Sync chain first.
	if err := c.Sync(client); err != nil {
		t.Fatalf("chain sync: %v", err)
	}

	acc, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	w := NewWallet(acc, s, c, client)
	if err := w.Sync(); err != nil {
		t.Fatal(err)
	}

	confirmed, locked, err := w.Balance()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Balance: confirmed=%d, locked=%d", confirmed, locked)

	transfers, err := w.Transfers()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Transfers: %d", len(transfers))
}
```

**Step 2: Update docs/architecture.md**

Add a `### wallet` section describing the package, its interfaces, and its role.

**Step 3: Update docs/history.md**

Add Phase 6 completion entry with commit hashes and summary.

**Step 4: Run full test suite**

Run: `go test -race ./...`
Expected: PASS

Run: `go vet ./...`
Expected: clean

**Step 5: Commit**

```bash
git add wallet/integration_test.go docs/architecture.md docs/history.md
git commit -m "docs: Phase 6 wallet core documentation and integration test"
```

---

## Verification Checklist

1. `go test -race ./...` — all tests pass
2. `go vet ./...` — no warnings
3. Coverage: `go test -coverprofile=cover.out ./wallet/ && go tool cover -func=cover.out` — target >80%
4. Mnemonic round-trip works for random keys
5. Scanner detects owned outputs and rejects non-owned
6. Builder produces valid signed transactions
7. Wallet sync tracks transfers and computes balance correctly

## Notes for Implementer

- **go-store `List()` method**: Verify this exists. If not, use `Keys()` + individual `Get()` calls, or iterate with `Count()` + sequential numeric keys.
- **`wire.DecodeVarintBytes`**: If this doesn't exist, add it — the TxExtra parser needs to decode varints from byte slices without an `io.Reader`.
- **`types.PublicKey.IsZero()`**: Verify this method exists. If not, compare against `types.PublicKey{}`.
- **`types.PublicKeyFromHex()`**: Verify this exists in types package. If not, add a simple hex-decode + copy into `[32]byte`.
- **C++ mnemonic wordlist**: Extract from `~/Code/LetheanNetwork/blockchain/src/common/mnemonic-encoding.cpp` using the wordsArray (lines 1665-3292). The map (lines 36-1663) has the same words in different order — use the **array** for index-based lookup.
