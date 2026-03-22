// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// You may obtain a copy of the licence at:
//
//     https://joinup.ec.europa.eu/software/page/eupl/licence-eupl
//
// SPDX-License-Identifier: EUPL-1.2

package wallet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"

	coreerr "dappco.re/go/core/log"

	"golang.org/x/crypto/argon2"

	store "dappco.re/go/core/store"

	"dappco.re/go/core/blockchain/crypto"
	"dappco.re/go/core/blockchain/types"
)

// Store group and key for the encrypted account blob.
const (
	groupAccount = "wallet"
	keyAccount   = "account"
)

// Argon2id parameters for key derivation.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
)

// Encryption envelope sizes.
const (
	saltLen  = 16
	nonceLen = 12
)

// Account holds the spend and view key pairs for a wallet. The spend secret
// key is the master key; the view secret key is deterministically derived as
// Keccak256(spend_secret_key), matching the C++ account_base::generate().
type Account struct {
	SpendPublicKey types.PublicKey `json:"spend_public_key"`
	SpendSecretKey types.SecretKey `json:"spend_secret_key"`
	ViewPublicKey  types.PublicKey `json:"view_public_key"`
	ViewSecretKey  types.SecretKey `json:"view_secret_key"`
	CreatedAt      uint64          `json:"created_at"`
	Flags          uint8           `json:"flags"`
}

// GenerateAccount creates a new account with random spend keys and a
// deterministically derived view key pair.
func GenerateAccount() (*Account, error) {
	spendPub, spendSec, err := crypto.GenerateKeys()
	if err != nil {
		return nil, coreerr.E("GenerateAccount", "wallet: generate spend keys", err)
	}
	return accountFromSpendKey(spendSec, spendPub)
}

// RestoreFromSeed reconstructs an account from a 25-word mnemonic phrase.
// The spend secret is decoded from the phrase; all other keys are derived.
func RestoreFromSeed(phrase string) (*Account, error) {
	key, err := MnemonicDecode(phrase)
	if err != nil {
		return nil, coreerr.E("RestoreFromSeed", "wallet: restore from seed", err)
	}
	spendPub, err := crypto.SecretToPublic(key)
	if err != nil {
		return nil, coreerr.E("RestoreFromSeed", "wallet: spend pub from secret", err)
	}
	return accountFromSpendKey(key, spendPub)
}

// RestoreViewOnly creates a view-only account that can scan incoming
// transactions but cannot spend. The spend secret key is left zeroed.
func RestoreViewOnly(viewSecret types.SecretKey, spendPublic types.PublicKey) (*Account, error) {
	viewPub, err := crypto.SecretToPublic([32]byte(viewSecret))
	if err != nil {
		return nil, coreerr.E("RestoreViewOnly", "wallet: view pub from secret", err)
	}
	return &Account{
		SpendPublicKey: spendPublic,
		ViewPublicKey:  types.PublicKey(viewPub),
		ViewSecretKey:  viewSecret,
	}, nil
}

// ToSeed encodes the spend secret key as a 25-word mnemonic phrase.
func (a *Account) ToSeed() (string, error) {
	return MnemonicEncode(a.SpendSecretKey[:])
}

// Address returns the public address derived from the account's public keys.
func (a *Account) Address() types.Address {
	return types.Address{
		SpendPublicKey: a.SpendPublicKey,
		ViewPublicKey:  a.ViewPublicKey,
	}
}

// Save encrypts the account with Argon2id + AES-256-GCM and persists it to
// the given store. The stored blob layout is: salt (16) | nonce (12) | ciphertext.
func (a *Account) Save(s *store.Store, password string) error {
	plaintext, err := json.Marshal(a)
	if err != nil {
		return coreerr.E("Account.Save", "wallet: marshal account", err)
	}

	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return coreerr.E("Account.Save", "wallet: generate salt", err)
	}

	derived := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	block, err := aes.NewCipher(derived)
	if err != nil {
		return coreerr.E("Account.Save", "wallet: aes cipher", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return coreerr.E("Account.Save", "wallet: gcm", err)
	}

	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return coreerr.E("Account.Save", "wallet: generate nonce", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	blob := make([]byte, 0, saltLen+nonceLen+len(ciphertext))
	blob = append(blob, salt...)
	blob = append(blob, nonce...)
	blob = append(blob, ciphertext...)

	return s.Set(groupAccount, keyAccount, hex.EncodeToString(blob))
}

// LoadAccount decrypts and returns the account stored in the given store.
// Returns an error if the password is incorrect or no account exists.
func LoadAccount(s *store.Store, password string) (*Account, error) {
	encoded, err := s.Get(groupAccount, keyAccount)
	if err != nil {
		return nil, coreerr.E("LoadAccount", "wallet: load account", err)
	}

	blob, err := hex.DecodeString(encoded)
	if err != nil {
		return nil, coreerr.E("LoadAccount", "wallet: decode account hex", err)
	}

	if len(blob) < saltLen+nonceLen+1 {
		return nil, coreerr.E("LoadAccount", "wallet: account data too short", nil)
	}

	salt := blob[:saltLen]
	nonce := blob[saltLen : saltLen+nonceLen]
	ciphertext := blob[saltLen+nonceLen:]

	derived := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	block, err := aes.NewCipher(derived)
	if err != nil {
		return nil, coreerr.E("LoadAccount", "wallet: aes cipher", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, coreerr.E("LoadAccount", "wallet: gcm", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, coreerr.E("LoadAccount", "wallet: decrypt account", err)
	}

	var acc Account
	if err := json.Unmarshal(plaintext, &acc); err != nil {
		return nil, coreerr.E("LoadAccount", "wallet: unmarshal account", err)
	}
	return &acc, nil
}

// accountFromSpendKey derives the full key set from a spend key pair. The
// view secret is computed as sc_reduce32(Keccak256(spendSec)), matching the
// C++ account_base::generate() derivation.
func accountFromSpendKey(spendSec, spendPub [32]byte) (*Account, error) {
	viewSec := crypto.FastHash(spendSec[:])
	crypto.ScReduce32(&viewSec)
	viewPub, err := crypto.SecretToPublic(viewSec)
	if err != nil {
		return nil, coreerr.E("accountFromSpendKey", "wallet: view pub from secret", err)
	}
	return &Account{
		SpendPublicKey: types.PublicKey(spendPub),
		SpendSecretKey: types.SecretKey(spendSec),
		ViewPublicKey:  types.PublicKey(viewPub),
		ViewSecretKey:  types.SecretKey(viewSec),
	}, nil
}
