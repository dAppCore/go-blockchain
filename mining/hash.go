// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

// Package mining provides a solo PoW miner that talks to a C++ daemon
// via JSON-RPC. It fetches block templates, grinds nonces with RandomX,
// and submits solutions.
package mining

import (
	"encoding/binary"

	"dappco.re/go/core/blockchain/consensus"
	"dappco.re/go/core/blockchain/crypto"
	"dappco.re/go/core/blockchain/types"
	"dappco.re/go/core/blockchain/wire"
)

// RandomXKey is the cache initialisation key for RandomX hashing.
var RandomXKey = []byte("LetheanRandomXv1")

// HeaderMiningHash computes the header hash used as input to RandomX.
// The nonce in the block is set to 0 before computing the hash, matching
// the C++ get_block_header_mining_hash() function.
//
// The result is deterministic for a given block template regardless of
// the block's current nonce value.
func HeaderMiningHash(b *types.Block) [32]byte {
	// Save and zero the nonce.
	savedNonce := b.Nonce
	b.Nonce = 0
	blob := wire.BlockHashingBlob(b)
	b.Nonce = savedNonce

	return wire.Keccak256(blob)
}

// CheckNonce tests whether a specific nonce produces a valid PoW solution
// for the given header mining hash and difficulty.
func CheckNonce(headerHash [32]byte, nonce, difficulty uint64) (bool, error) {
	var input [40]byte
	copy(input[:32], headerHash[:])
	binary.LittleEndian.PutUint64(input[32:], nonce)

	powHash, err := crypto.RandomXHash(RandomXKey, input[:])
	if err != nil {
		return false, err
	}

	return consensus.CheckDifficulty(types.Hash(powHash), difficulty), nil
}
