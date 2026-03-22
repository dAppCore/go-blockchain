// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package consensus

import (
	"encoding/binary"
	"math/big"

	"dappco.re/go/core/blockchain/crypto"
	"dappco.re/go/core/blockchain/types"
)

// maxTarget is 2^256, used for difficulty comparison.
var maxTarget = new(big.Int).Lsh(big.NewInt(1), 256)

// CheckDifficulty returns true if hash meets the given difficulty target.
// The hash (interpreted as a 256-bit little-endian number) must be less
// than maxTarget / difficulty.
func CheckDifficulty(hash types.Hash, difficulty uint64) bool {
	if difficulty == 0 {
		return true
	}

	// Convert hash to big.Int (little-endian as per CryptoNote convention).
	// Reverse to big-endian for big.Int.
	var be [32]byte
	for i := range 32 {
		be[i] = hash[31-i]
	}
	hashInt := new(big.Int).SetBytes(be[:])

	target := new(big.Int).Div(maxTarget, new(big.Int).SetUint64(difficulty))

	return hashInt.Cmp(target) < 0
}

// CheckPoWHash computes the RandomX hash of a block header hash + nonce
// and checks it against the difficulty target.
func CheckPoWHash(headerHash types.Hash, nonce, difficulty uint64) (bool, error) {
	// Build input: header_hash (32 bytes) || nonce (8 bytes LE).
	var input [40]byte
	copy(input[:32], headerHash[:])
	binary.LittleEndian.PutUint64(input[32:], nonce)

	key := []byte("LetheanRandomXv1")
	powHash, err := crypto.RandomXHash(key, input[:])
	if err != nil {
		return false, err
	}

	return CheckDifficulty(types.Hash(powHash), difficulty), nil
}
