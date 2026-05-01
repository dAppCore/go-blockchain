//go:build integration

// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package mining

import (
	"bytes"
	"encoding/hex"
	"testing"

	"dappco.re/go/core/blockchain/rpc"
	"dappco.re/go/core/blockchain/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_GetBlockTemplate_Good(t *testing.T) {
	client := rpc.NewClient("http://localhost:46941")

	// Get daemon info to check it's running.
	info, err := client.GetInfo()
	require.NoError(t, err, "daemon must be running on localhost:46941")
	t.Logf("daemon height: %d, pow_difficulty: %d", info.Height, info.PowDifficulty)

	// Get a block template.
	// Use the testnet genesis coinbase address (all zeros — won't receive real
	// coins but the daemon accepts it for template generation).
	tmpl, err := client.GetBlockTemplate("iTHNtestaddr")
	if err != nil {
		t.Skipf("getblocktemplate failed (may need valid address): %v", err)
	}

	assert.Greater(t, tmpl.Height, uint64(0))
	assert.NotEmpty(t, tmpl.Difficulty)
	assert.NotEmpty(t, tmpl.BlockTemplateBlob)

	// Decode the template blob.
	blobBytes, err := hex.DecodeString(tmpl.BlockTemplateBlob)
	require.NoError(t, err)

	dec := wire.NewDecoder(bytes.NewReader(blobBytes))
	block := wire.DecodeBlock(dec)
	require.NoError(t, dec.Err())

	t.Logf("template: height=%d, major=%d, timestamp=%d, txs=%d",
		tmpl.Height, block.MajorVersion, block.Timestamp, len(block.TxHashes))

	// Compute header mining hash.
	headerHash := HeaderMiningHash(&block)
	t.Logf("header mining hash: %s", hex.EncodeToString(headerHash[:]))

	// Verify the header hash is non-zero.
	assert.False(t, headerHash == [32]byte{})
}
