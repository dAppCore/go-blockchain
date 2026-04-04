// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-Licence-Identifier: EUPL-1.2

package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPow_RandomXHash_Good(t *testing.T) {
	key := []byte("LetheanRandomXv1")
	input := make([]byte, 40) // 32-byte hash + 8-byte nonce

	hash, err := RandomXHash(key, input)
	require.NoError(t, err)
	assert.NotEqual(t, [32]byte{}, hash, "hash should be non-zero")

	// Determinism: same input must produce the same output.
	hash2, err := RandomXHash(key, input)
	require.NoError(t, err)
	assert.Equal(t, hash, hash2, "hash must be deterministic")
}

func TestPow_RandomXHash_Bad(t *testing.T) {
	key := []byte("LetheanRandomXv1")
	input1 := make([]byte, 40)
	input2 := make([]byte, 40)
	input2[0] = 1

	hash1, err := RandomXHash(key, input1)
	require.NoError(t, err)
	hash2, err := RandomXHash(key, input2)
	require.NoError(t, err)
	assert.NotEqual(t, hash1, hash2, "different inputs must produce different hashes")
}
