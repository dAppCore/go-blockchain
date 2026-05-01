// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package consensus

import (
	"bytes"
	"encoding/hex"
	"testing"

	"dappco.re/go/core"
	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/types"
	"dappco.re/go/core/blockchain/wire"
	coreio "dappco.re/go/core/io"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadTestTx loads and decodes a hex-encoded transaction from testdata.
func loadTestTx(t *testing.T, filename string) *types.Transaction {
	t.Helper()
	hexData, err := coreio.Local.Read(filename)
	require.NoError(t, err, "read %s", filename)

	blob, err := hex.DecodeString(core.Trim(hexData))
	require.NoError(t, err, "decode hex")

	dec := wire.NewDecoder(bytes.NewReader(blob))
	tx := wire.DecodeTransaction(dec)
	require.NoError(t, dec.Err(), "decode transaction")
	return &tx
}

func TestV2sig_ParseV2Signatures_Mixin0_Good(t *testing.T) {
	tx := loadTestTx(t, "../testdata/v2_spending_tx_mixin0.hex")
	require.Equal(t, uint64(3), tx.Version, "expected v3 transaction")

	// Should have 1 ZC input.
	require.Len(t, tx.Vin, 1)
	_, ok := tx.Vin[0].(types.TxInputZC)
	require.True(t, ok, "expected TxInputZC")

	// Parse signatures.
	entries, err := parseV2Signatures(tx.SignaturesRaw)
	require.NoError(t, err)
	require.Len(t, entries, 1, "should have 1 signature entry")

	// Should be a ZC_sig.
	assert.Equal(t, types.SigTypeZC, entries[0].tag)
	require.NotNil(t, entries[0].zcSig)

	zc := entries[0].zcSig

	// Ring size should be 16 (mixin 0 + ZC default = 16).
	assert.Equal(t, 16, zc.ringSize, "expected ring size 16")

	// Flat sig size: c(32) + r_g[16*32] + r_x[16*32] + K1(32) + K2(32) = 1120.
	expectedSigSize := 32 + 16*32 + 16*32 + 64
	assert.Equal(t, expectedSigSize, len(zc.clsagFlatSig))

	// Pseudo-out commitment and asset ID should be non-zero.
	assert.NotEqual(t, [32]byte{}, zc.pseudoOutCommitment)
	assert.NotEqual(t, [32]byte{}, zc.pseudoOutAssetID)
}

func TestV2sig_ParseV2Signatures_Mixin10_Good(t *testing.T) {
	tx := loadTestTx(t, "../testdata/v2_spending_tx_mixin10.hex")
	require.Equal(t, uint64(3), tx.Version)

	// Mixin10 tx has 2 ZC inputs.
	require.Len(t, tx.Vin, 2)

	entries, err := parseV2Signatures(tx.SignaturesRaw)
	require.NoError(t, err)
	require.Len(t, entries, 2, "should have 2 signature entries (one per input)")

	for i, entry := range entries {
		assert.Equal(t, types.SigTypeZC, entry.tag, "entry %d", i)
		require.NotNil(t, entry.zcSig, "entry %d", i)

		zc := entry.zcSig

		// Both inputs use ring size 16 (ZC default).
		assert.Equal(t, 16, zc.ringSize, "entry %d: expected ring size 16", i)

		// Flat sig size: c(32) + r_g[16*32] + r_x[16*32] + K1(32) + K2(32) = 1120.
		expectedSigSize := 32 + 16*32 + 16*32 + 64
		assert.Equal(t, expectedSigSize, len(zc.clsagFlatSig), "entry %d", i)
	}
}

func TestV2sig_ParseV2Proofs_Mixin0_Good(t *testing.T) {
	tx := loadTestTx(t, "../testdata/v2_spending_tx_mixin0.hex")

	proofs, err := parseV2Proofs(tx.Proofs)
	require.NoError(t, err)

	// Should have BGE proofs (one per output).
	assert.Len(t, proofs.bgeProofs, 2, "expected 2 BGE proofs (one per output)")
	for i, p := range proofs.bgeProofs {
		assert.True(t, len(p) > 0, "BGE proof %d should be non-empty", i)
	}

	// Should have BPP range proof.
	assert.True(t, len(proofs.bppProofBytes) > 0, "BPP proof should be non-empty")

	// Should have BPP commitments (one per output).
	assert.Len(t, proofs.bppCommitments, 2, "expected 2 BPP commitments")
	for i, c := range proofs.bppCommitments {
		assert.NotEqual(t, [32]byte{}, c, "BPP commitment %d should be non-zero", i)
	}

	// Should have balance proof (96 bytes).
	assert.Len(t, proofs.balanceProof, 96, "balance proof should be 96 bytes")
}

func TestV2sig_ParseV2Proofs_Mixin10_Good(t *testing.T) {
	tx := loadTestTx(t, "../testdata/v2_spending_tx_mixin10.hex")

	proofs, err := parseV2Proofs(tx.Proofs)
	require.NoError(t, err)

	assert.Len(t, proofs.bgeProofs, 2)
	assert.True(t, len(proofs.bppProofBytes) > 0)
	assert.Len(t, proofs.bppCommitments, 2)
	assert.Len(t, proofs.balanceProof, 96)
}

func TestV2sig_VerifyV2Signatures_StructuralOnly_Good(t *testing.T) {
	// Test structural validation (no ring output function).
	tx := loadTestTx(t, "../testdata/v2_spending_tx_mixin0.hex")

	// With nil getZCRingOutputs, should pass structural checks.
	err := VerifyTransactionSignatures(tx, config.TestnetForks, 2823, nil, nil)
	require.NoError(t, err)
}

func TestV2sig_VerifyV2Signatures_BadSigCount_Bad(t *testing.T) {
	tx := loadTestTx(t, "../testdata/v2_spending_tx_mixin0.hex")

	// Corrupt SignaturesRaw to have wrong count.
	tx.SignaturesRaw = wire.EncodeVarint(5) // 5 sigs but only 1 input

	err := verifyV2Signatures(tx, nil)
	assert.Error(t, err, "should fail with mismatched sig count")
}

func TestV2sig_VerifyV2Signatures_TxHash_Good(t *testing.T) {
	// Verify the known tx hash matches.
	tx := loadTestTx(t, "../testdata/v2_spending_tx_mixin0.hex")
	txHash := wire.TransactionHash(tx)

	expectedHash := "89c8839e3c6be3bb3616a5c2e7028fd8f33992e4f9ff218f8224825702865b8b"
	assert.Equal(t, expectedHash, hex.EncodeToString(txHash[:]))
}

func TestV2sig_VerifyV2Signatures_TxHashMixin10_Good(t *testing.T) {
	tx := loadTestTx(t, "../testdata/v2_spending_tx_mixin10.hex")
	txHash := wire.TransactionHash(tx)

	expectedHash := "87fbc60cde013579e1ad6ab403dee81c4da7a6b4621bea44f6973568c37b0af6"
	assert.Equal(t, expectedHash, hex.EncodeToString(txHash[:]))
}
