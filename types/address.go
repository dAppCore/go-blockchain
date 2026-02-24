// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// You may obtain a copy of the licence at:
//
//     https://joinup.ec.europa.eu/software/page/eupl/licence-eupl
//
// SPDX-License-Identifier: EUPL-1.2

package types

import (
	"errors"
	"fmt"
	"math/big"

	"golang.org/x/crypto/sha3"

	"forge.lthn.ai/core/go-blockchain/config"
)

// FlagAuditable marks an address as auditable. When set, the address was
// generated with a deterministic view key derivation, allowing a third party
// with the view key to audit all incoming transactions.
const FlagAuditable uint8 = 0x01

// Address represents a Lethean account public address consisting of a spend
// public key, a view public key, and optional flags (e.g. auditable).
type Address struct {
	SpendPublicKey PublicKey
	ViewPublicKey  PublicKey
	Flags          uint8
}

// IsAuditable reports whether the address has the auditable flag set.
func (a *Address) IsAuditable() bool {
	return a.Flags&FlagAuditable != 0
}

// IsIntegrated reports whether the given prefix corresponds to an integrated
// address type (standard integrated or auditable integrated).
func (a *Address) IsIntegrated() bool {
	// This method checks whether the address was decoded with an integrated
	// prefix. Since we do not store the prefix in the Address struct, callers
	// should use the prefix returned by DecodeAddress to determine this.
	// This helper exists for convenience when the prefix is not available.
	return false
}

// IsIntegratedPrefix reports whether the given prefix corresponds to an
// integrated address type.
func IsIntegratedPrefix(prefix uint64) bool {
	return prefix == config.IntegratedAddressPrefix ||
		prefix == config.AuditableIntegratedAddressPrefix
}

// Encode serialises the address into a CryptoNote base58 string with the
// given prefix. The encoding format is:
//
//	varint(prefix) || spend_pubkey (32 bytes) || view_pubkey (32 bytes) || flags (1 byte) || checksum (4 bytes)
//
// The checksum is the first 4 bytes of Keccak-256 over the preceding data.
func (a *Address) Encode(prefix uint64) string {
	// Build the raw data: prefix (varint) + keys + flags.
	prefixBytes := encodeVarint(prefix)
	raw := make([]byte, 0, len(prefixBytes)+32+32+1+4)
	raw = append(raw, prefixBytes...)
	raw = append(raw, a.SpendPublicKey[:]...)
	raw = append(raw, a.ViewPublicKey[:]...)
	raw = append(raw, a.Flags)

	// Compute Keccak-256 checksum over the raw data.
	checksum := keccak256Checksum(raw)
	raw = append(raw, checksum[:]...)

	return base58Encode(raw)
}

// DecodeAddress parses a CryptoNote base58-encoded address string. It returns
// the decoded address, the prefix that was used, and any error.
func DecodeAddress(s string) (*Address, uint64, error) {
	raw, err := base58Decode(s)
	if err != nil {
		return nil, 0, fmt.Errorf("types: base58 decode failed: %w", err)
	}

	// The minimum size is: 1 byte prefix varint + 32 + 32 + 1 flags + 4 checksum = 70.
	if len(raw) < 70 {
		return nil, 0, errors.New("types: address data too short")
	}

	// Decode the prefix varint.
	prefix, prefixLen, err := decodeVarint(raw)
	if err != nil {
		return nil, 0, fmt.Errorf("types: invalid address prefix varint: %w", err)
	}

	// After the prefix we need exactly 32+32+1+4 = 69 bytes.
	remaining := raw[prefixLen:]
	if len(remaining) != 69 {
		return nil, 0, fmt.Errorf("types: unexpected address data length: want 69 bytes after prefix, got %d", len(remaining))
	}

	// Validate checksum: Keccak-256 of everything except the last 4 bytes.
	payloadEnd := len(raw) - 4
	expectedChecksum := keccak256Checksum(raw[:payloadEnd])
	actualChecksum := raw[payloadEnd:]
	if expectedChecksum[0] != actualChecksum[0] ||
		expectedChecksum[1] != actualChecksum[1] ||
		expectedChecksum[2] != actualChecksum[2] ||
		expectedChecksum[3] != actualChecksum[3] {
		return nil, 0, errors.New("types: address checksum mismatch")
	}

	addr := &Address{}
	copy(addr.SpendPublicKey[:], remaining[0:32])
	copy(addr.ViewPublicKey[:], remaining[32:64])
	addr.Flags = remaining[64]

	return addr, prefix, nil
}

// keccak256Checksum returns the first 4 bytes of the Keccak-256 hash of data.
// This uses the legacy Keccak-256 (pre-NIST), NOT SHA3-256.
func keccak256Checksum(data []byte) [4]byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	sum := h.Sum(nil)
	var checksum [4]byte
	copy(checksum[:], sum[:4])
	return checksum
}

// ---------------------------------------------------------------------------
// CryptoNote Base58 encoding
// ---------------------------------------------------------------------------

// base58Alphabet is the CryptoNote base58 character set. Note: this omits
// 0, O, I, l to avoid visual ambiguity.
const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// base58BlockSizes maps input byte counts (0-8) to the number of base58
// characters needed to encode that many bytes. CryptoNote encodes data in
// 8-byte blocks, each producing 11 base58 characters, with the final
// partial block producing fewer characters.
var base58BlockSizes = [9]int{0, 2, 3, 5, 6, 7, 9, 10, 11}

// base58ReverseBlockSizes maps encoded character counts back to byte counts.
var base58ReverseBlockSizes [12]int

func init() {
	for i := range base58ReverseBlockSizes {
		base58ReverseBlockSizes[i] = -1
	}
	for byteCount, charCount := range base58BlockSizes {
		if charCount < len(base58ReverseBlockSizes) {
			base58ReverseBlockSizes[charCount] = byteCount
		}
	}
}

// base58Encode encodes raw bytes using the CryptoNote base58 scheme.
// Data is split into 8-byte blocks; each block is encoded independently.
func base58Encode(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	result := make([]byte, 0, len(data)*2)
	fullBlocks := len(data) / 8
	lastBlockSize := len(data) % 8

	for i := range fullBlocks {
		block := data[i*8 : (i+1)*8]
		encoded := encodeBlock(block, 11)
		result = append(result, encoded...)
	}

	if lastBlockSize > 0 {
		block := data[fullBlocks*8:]
		encodedSize := base58BlockSizes[lastBlockSize]
		encoded := encodeBlock(block, encodedSize)
		result = append(result, encoded...)
	}

	return string(result)
}

// encodeBlock encodes a single block (up to 8 bytes) into the specified
// number of base58 characters.
func encodeBlock(block []byte, encodedSize int) []byte {
	// Convert the block to a big integer.
	num := new(big.Int).SetBytes(block)
	base := big.NewInt(58)

	result := make([]byte, encodedSize)
	for i := range result {
		result[i] = base58Alphabet[0] // fill with '1' (zero digit)
	}

	// Encode from least significant digit to most significant.
	idx := encodedSize - 1
	zero := new(big.Int)
	mod := new(big.Int)
	for num.Cmp(zero) > 0 && idx >= 0 {
		num.DivMod(num, base, mod)
		result[idx] = base58Alphabet[mod.Int64()]
		idx--
	}

	return result
}

// base58Decode decodes a CryptoNote base58 string back into raw bytes.
func base58Decode(s string) ([]byte, error) {
	if len(s) == 0 {
		return nil, errors.New("types: empty base58 string")
	}

	fullBlocks := len(s) / 11
	lastBlockChars := len(s) % 11

	// Validate that the last block size maps to a valid byte count.
	if lastBlockChars > 0 && base58ReverseBlockSizes[lastBlockChars] < 0 {
		return nil, fmt.Errorf("types: invalid base58 string length %d", len(s))
	}

	var result []byte

	for i := range fullBlocks {
		blockStr := s[i*11 : (i+1)*11]
		decoded, err := decodeBlock(blockStr, 8)
		if err != nil {
			return nil, err
		}
		result = append(result, decoded...)
	}

	if lastBlockChars > 0 {
		blockStr := s[fullBlocks*11:]
		byteCount := base58ReverseBlockSizes[lastBlockChars]
		decoded, err := decodeBlock(blockStr, byteCount)
		if err != nil {
			return nil, err
		}
		result = append(result, decoded...)
	}

	return result, nil
}

// decodeBlock decodes a base58 string block into the specified number of bytes.
func decodeBlock(s string, byteCount int) ([]byte, error) {
	num := new(big.Int)
	base := big.NewInt(58)

	for _, c := range []byte(s) {
		idx := base58CharIndex(c)
		if idx < 0 {
			return nil, fmt.Errorf("types: invalid base58 character %q", c)
		}
		num.Mul(num, base)
		num.Add(num, big.NewInt(int64(idx)))
	}

	// Convert to fixed-size byte array, big-endian.
	raw := num.Bytes()
	if len(raw) > byteCount {
		return nil, fmt.Errorf("types: base58 block overflow: decoded %d bytes, expected %d", len(raw), byteCount)
	}

	// Pad with leading zeroes if necessary.
	result := make([]byte, byteCount)
	copy(result[byteCount-len(raw):], raw)

	return result, nil
}

// base58CharIndex returns the index of character c in the base58 alphabet,
// or -1 if the character is not in the alphabet.
func base58CharIndex(c byte) int {
	for i, ch := range []byte(base58Alphabet) {
		if ch == c {
			return i
		}
	}
	return -1
}

// ---------------------------------------------------------------------------
// Varint helpers (inlined from wire package to avoid import cycle)
// ---------------------------------------------------------------------------

func encodeVarint(v uint64) []byte {
	if v == 0 {
		return []byte{0x00}
	}
	var buf [10]byte
	n := 0
	for v > 0 {
		buf[n] = byte(v & 0x7f)
		v >>= 7
		if v > 0 {
			buf[n] |= 0x80
		}
		n++
	}
	return append([]byte(nil), buf[:n]...)
}

func decodeVarint(data []byte) (uint64, int, error) {
	if len(data) == 0 {
		return 0, 0, errors.New("types: cannot decode varint from empty data")
	}
	var v uint64
	for i := 0; i < len(data) && i < 10; i++ {
		v |= uint64(data[i]&0x7f) << (7 * uint(i))
		if data[i]&0x80 == 0 {
			return v, i + 1, nil
		}
	}
	return 0, 0, errors.New("types: varint overflow")
}
