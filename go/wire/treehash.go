// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wire

import "golang.org/x/crypto/sha3"

// Keccak256 computes the Keccak-256 hash (pre-NIST, no domain separation)
// used as cn_fast_hash throughout the CryptoNote protocol.
// Usage: wire.Keccak256(...)
func Keccak256(data []byte) [32]byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	var out [32]byte
	h.Sum(out[:0])
	return out
}

// TreeHash computes the CryptoNote Merkle tree hash over a set of 32-byte
// hashes. This is a direct port of crypto/tree-hash.c from the C++ daemon.
//
// Algorithm:
//   - 0 hashes: returns zero hash
//   - 1 hash:   returns the hash itself (identity)
//   - 2 hashes: returns Keccak256(h0 || h1)
//   - N hashes: pad to power-of-2 leaves, pairwise Keccak up the tree
//
// Usage: wire.TreeHash(...)
func TreeHash(hashes [][32]byte) [32]byte {
	count := len(hashes)
	if count == 0 {
		return [32]byte{}
	}
	if count == 1 {
		return hashes[0]
	}
	if count == 2 {
		var buf [64]byte
		copy(buf[:32], hashes[0][:])
		copy(buf[32:], hashes[1][:])
		return Keccak256(buf[:])
	}

	// Find largest power of 2 that is <= count. This mirrors the C++ bit trick:
	//   cnt = count - 1; for (i = 1; i < bits; i <<= 1) cnt |= cnt >> i;
	//   cnt &= ~(cnt >> 1);
	cnt := count - 1
	for i := 1; i < 64; i <<= 1 {
		cnt |= cnt >> uint(i)
	}
	cnt &= ^(cnt >> 1)

	// Allocate intermediate hash buffer.
	ints := make([][32]byte, cnt)

	// Copy the first (2*cnt - count) hashes directly into ints.
	direct := 2*cnt - count
	copy(ints[:direct], hashes[:direct])

	// Pair-hash the remaining hashes into ints.
	i := direct
	for j := direct; j < cnt; j++ {
		var buf [64]byte
		copy(buf[:32], hashes[i][:])
		copy(buf[32:], hashes[i+1][:])
		ints[j] = Keccak256(buf[:])
		i += 2
	}

	// Iteratively pair-hash until we have 2 hashes left.
	for cnt > 2 {
		cnt >>= 1
		for i, j := 0, 0; j < cnt; j++ {
			var buf [64]byte
			copy(buf[:32], ints[i][:])
			copy(buf[32:], ints[i+1][:])
			ints[j] = Keccak256(buf[:])
			i += 2
		}
	}

	// Final hash of the remaining pair.
	var buf [64]byte
	copy(buf[:32], ints[0][:])
	copy(buf[32:], ints[1][:])
	return Keccak256(buf[:])
}
