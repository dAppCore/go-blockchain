// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wire

import (
	"encoding/hex"
	"testing"
)

func TestKeccak256_Good(t *testing.T) {
	// Empty input: well-known Keccak-256 of "" (pre-NIST).
	got := Keccak256(nil)
	want := "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"
	if hex.EncodeToString(got[:]) != want {
		t.Errorf("Keccak256(nil) = %x, want %s", got, want)
	}
}

func TestTreeHashSingle_Good(t *testing.T) {
	var h [32]byte
	h[0] = 0xAB
	h[31] = 0xCD
	got := TreeHash([][32]byte{h})
	if got != h {
		t.Error("TreeHash of single hash should be identity")
	}
}

func TestTreeHashPair_Good(t *testing.T) {
	var h0, h1 [32]byte
	h0[0] = 0x01
	h1[0] = 0x02
	got := TreeHash([][32]byte{h0, h1})

	// Manual: Keccak256(h0 || h1)
	var buf [64]byte
	copy(buf[:32], h0[:])
	copy(buf[32:], h1[:])
	want := Keccak256(buf[:])
	if got != want {
		t.Errorf("TreeHash pair: got %x, want %x", got, want)
	}
}

func TestTreeHashThree_Good(t *testing.T) {
	var h0, h1, h2 [32]byte
	h0[0] = 0xAA
	h1[0] = 0xBB
	h2[0] = 0xCC

	got := TreeHash([][32]byte{h0, h1, h2})

	// For 3 hashes, cnt=2:
	//   ints[0] = h0 (direct copy)
	//   ints[1] = Keccak256(h1 || h2)
	//   result  = Keccak256(ints[0] || ints[1])
	var buf [64]byte
	copy(buf[:32], h1[:])
	copy(buf[32:], h2[:])
	ints1 := Keccak256(buf[:])

	copy(buf[:32], h0[:])
	copy(buf[32:], ints1[:])
	want := Keccak256(buf[:])

	if got != want {
		t.Errorf("TreeHash(3): got %x, want %x", got, want)
	}
}

func TestTreeHashFour_Good(t *testing.T) {
	hashes := make([][32]byte, 4)
	for i := range hashes {
		hashes[i][0] = byte(i + 1)
	}

	got := TreeHash(hashes)

	// For 4 hashes, cnt=2:
	//   ints[0] = Keccak256(h0 || h1)
	//   ints[1] = Keccak256(h2 || h3)
	//   result  = Keccak256(ints[0] || ints[1])
	var buf [64]byte
	copy(buf[:32], hashes[0][:])
	copy(buf[32:], hashes[1][:])
	ints0 := Keccak256(buf[:])

	copy(buf[:32], hashes[2][:])
	copy(buf[32:], hashes[3][:])
	ints1 := Keccak256(buf[:])

	copy(buf[:32], ints0[:])
	copy(buf[32:], ints1[:])
	want := Keccak256(buf[:])

	if got != want {
		t.Errorf("TreeHash(4): got %x, want %x", got, want)
	}
}

func TestTreeHashFive_Good(t *testing.T) {
	// 5 hashes exercises the iterative cnt > 2 loop.
	hashes := make([][32]byte, 5)
	for i := range hashes {
		hashes[i][0] = byte(i + 1)
	}

	got := TreeHash(hashes)

	// For 5 hashes: cnt=4, direct=3
	// ints[0] = h0, ints[1] = h1, ints[2] = h2
	// ints[3] = Keccak256(h3 || h4)
	// Then cnt=2: ints[0] = Keccak256(ints[0] || ints[1])
	//             ints[1] = Keccak256(ints[2] || ints[3])
	// Final = Keccak256(ints[0] || ints[1])
	var buf [64]byte

	// ints[3]
	copy(buf[:32], hashes[3][:])
	copy(buf[32:], hashes[4][:])
	ints3 := Keccak256(buf[:])

	// Round 1: pair-hash
	copy(buf[:32], hashes[0][:])
	copy(buf[32:], hashes[1][:])
	r1_0 := Keccak256(buf[:])

	copy(buf[:32], hashes[2][:])
	copy(buf[32:], ints3[:])
	r1_1 := Keccak256(buf[:])

	// Final
	copy(buf[:32], r1_0[:])
	copy(buf[32:], r1_1[:])
	want := Keccak256(buf[:])

	if got != want {
		t.Errorf("TreeHash(5): got %x, want %x", got, want)
	}
}

func TestTreeHashEight_Good(t *testing.T) {
	// 8 hashes = perfect power of 2, exercises multiple loop iterations.
	hashes := make([][32]byte, 8)
	for i := range hashes {
		hashes[i][0] = byte(i + 1)
	}

	got := TreeHash(hashes)

	// Verify determinism.
	got2 := TreeHash(hashes)
	if got != got2 {
		t.Error("TreeHash(8) not deterministic")
	}

	// Sanity: result should not be zero.
	if got == ([32]byte{}) {
		t.Error("TreeHash(8) returned zero hash")
	}
}

func TestTreeHashEmpty_Good(t *testing.T) {
	got := TreeHash(nil)
	if got != ([32]byte{}) {
		t.Errorf("TreeHash(nil) should be zero hash, got %x", got)
	}
}
