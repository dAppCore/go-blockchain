// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// You may obtain a copy of the licence at:
//
//     https://joinup.ec.europa.eu/software/page/eupl/licence-eupl
//
// SPDX-License-Identifier: EUPL-1.2

// Package types defines the core cryptographic and blockchain data types for
// the Lethean protocol. All types are fixed-size byte arrays matching the
// CryptoNote specification.
package types

import (
	"encoding/hex"
	"fmt"

	coreerr "dappco.re/go/core/log"
)

// Hash is a 256-bit (32-byte) hash value, typically produced by Keccak-256.
type Hash [32]byte

// String returns the hexadecimal representation of the hash.
func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

// IsZero reports whether the hash is all zeroes.
func (h Hash) IsZero() bool {
	for _, b := range h {
		if b != 0 {
			return false
		}
	}
	return true
}

// HashFromHex parses a 64-character hexadecimal string into a Hash.
func HashFromHex(s string) (Hash, error) {
	var h Hash
	b, err := hex.DecodeString(s)
	if err != nil {
		return h, coreerr.E("HashFromHex", "types: invalid hex for hash", err)
	}
	if len(b) != 32 {
		return h, coreerr.E("HashFromHex", fmt.Sprintf("types: hash hex must be 64 characters, got %d", len(s)), nil)
	}
	copy(h[:], b)
	return h, nil
}

// PublicKey is a 256-bit Ed25519 public key.
type PublicKey [32]byte

// String returns the hexadecimal representation of the public key.
func (pk PublicKey) String() string {
	return hex.EncodeToString(pk[:])
}

// IsZero reports whether the public key is all zeroes.
func (pk PublicKey) IsZero() bool { return pk == PublicKey{} }

// PublicKeyFromHex parses a 64-character hexadecimal string into a PublicKey.
func PublicKeyFromHex(s string) (PublicKey, error) {
	var pk PublicKey
	b, err := hex.DecodeString(s)
	if err != nil {
		return pk, coreerr.E("PublicKeyFromHex", "types: invalid hex for public key", err)
	}
	if len(b) != 32 {
		return pk, coreerr.E("PublicKeyFromHex", fmt.Sprintf("types: public key hex must be 64 characters, got %d", len(s)), nil)
	}
	copy(pk[:], b)
	return pk, nil
}

// SecretKey is a 256-bit Ed25519 secret (private) key.
type SecretKey [32]byte

// String returns the hexadecimal representation of the secret key.
// Note: take care when logging or displaying secret keys.
func (sk SecretKey) String() string {
	return hex.EncodeToString(sk[:])
}

// KeyImage is a 256-bit key image used for double-spend detection.
type KeyImage [32]byte

// String returns the hexadecimal representation of the key image.
func (ki KeyImage) String() string {
	return hex.EncodeToString(ki[:])
}

// Signature is a 512-bit (64-byte) cryptographic signature.
type Signature [64]byte
