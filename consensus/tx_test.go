// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

//go:build !integration

package consensus

import (
	"testing"

	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validV1Tx returns a minimal valid v1 transaction for testing.
func validV1Tx() *types.Transaction {
	return &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{
				Amount:   100,
				KeyImage: types.KeyImage{1},
			},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 90, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
}

func TestTx_ValidateTransaction_Good(t *testing.T) {
	tx := validV1Tx()
	blob := make([]byte, 100) // small blob
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	require.NoError(t, err)
}

func TestTx_ValidateTransaction_BlobTooLarge_Bad(t *testing.T) {
	tx := validV1Tx()
	blob := make([]byte, config.MaxTransactionBlobSize+1)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrTxTooLarge)
}

func TestTx_ValidateTransaction_NoInputs_Bad(t *testing.T) {
	tx := validV1Tx()
	tx.Vin = nil
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrNoInputs)
}

func TestTx_ValidateTransaction_TooManyInputs_Good(t *testing.T) {
	tx := validV1Tx()
	tx.Vin = make([]types.TxInput, config.TxMaxAllowedInputs+1)
	for i := range tx.Vin {
		tx.Vin[i] = types.TxInputToKey{Amount: 1, KeyImage: types.KeyImage{byte(i)}}
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrTooManyInputs)
}

func TestTx_ValidateTransaction_InvalidInputType_Bad(t *testing.T) {
	tx := validV1Tx()
	tx.Vin = []types.TxInput{types.TxInputGenesis{Height: 1}} // genesis not allowed
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrInvalidInputType)
}

func TestTx_ValidateTransaction_NoOutputs_Bad(t *testing.T) {
	tx := validV1Tx()
	tx.Vout = nil
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrNoOutputs)
}

func TestTx_ValidateTransaction_TooManyOutputs_Good(t *testing.T) {
	tx := validV1Tx()
	tx.Vout = make([]types.TxOutput, config.TxMaxAllowedOutputs+1)
	for i := range tx.Vout {
		tx.Vout[i] = types.TxOutputBare{Amount: 1, Target: types.TxOutToKey{Key: types.PublicKey{byte(i)}}}
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrTooManyOutputs)
}

func TestTx_ValidateTransaction_ZeroOutputAmount_Ugly(t *testing.T) {
	tx := validV1Tx()
	tx.Vout = []types.TxOutput{
		types.TxOutputBare{Amount: 0, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrInvalidOutput)
}

func TestTx_ValidateTransaction_DuplicateKeyImage_Bad(t *testing.T) {
	ki := types.KeyImage{42}
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: 100, KeyImage: ki},
			types.TxInputToKey{Amount: 50, KeyImage: ki}, // duplicate
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 140, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrDuplicateKeyImage)
}

func TestTx_ValidateTransaction_NegativeFee_Bad(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: 10, KeyImage: types.KeyImage{1}},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 100, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrNegativeFee)
}

// --- HF1 gating tests (Task 7) ---

func TestTx_CheckInputTypes_HTLCPreHF1_Bad(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputHTLC{Amount: 100, KeyImage: types.KeyImage{1}},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 90, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000) // pre-HF1 (10080)
	assert.ErrorIs(t, err, ErrInvalidInputType)
}

func TestTx_CheckInputTypes_HTLCPostHF1_Good(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputHTLC{
				Amount:   100,
				KeyImage: types.KeyImage{1},
			},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 90, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 20000) // post-HF1
	require.NoError(t, err)
}

func TestTx_CheckInputTypes_MultisigPreHF1_Bad(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputMultisig{Amount: 100},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 90, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrInvalidInputType)
}

func TestTx_CheckOutputs_HTLCTargetPreHF1_Bad(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: 100, KeyImage: types.KeyImage{1}},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 90,
				Target: types.TxOutHTLC{Expiration: 20000},
			},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrInvalidOutput)
}

func TestTx_CheckOutputs_MultisigTargetPreHF1_Bad(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: 100, KeyImage: types.KeyImage{1}},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 90,
				Target: types.TxOutMultisig{MinimumSigs: 2, Keys: []types.PublicKey{{1}, {2}}},
			},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrInvalidOutput)
}

func TestTx_CheckOutputs_MultisigTargetPostHF1_Good(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: 100, KeyImage: types.KeyImage{1}},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 90,
				Target: types.TxOutMultisig{MinimumSigs: 2, Keys: []types.PublicKey{{1}, {2}}},
			},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 20000) // post-HF1
	require.NoError(t, err)
}

// --- Key image tests for HTLC (Task 8) ---

func TestTx_CheckKeyImages_HTLCDuplicate_Bad(t *testing.T) {
	ki := types.KeyImage{0x42}
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputHTLC{Amount: 100, KeyImage: ki},
			types.TxInputHTLC{Amount: 50, KeyImage: ki}, // duplicate
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 140, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 20000) // post-HF1
	assert.ErrorIs(t, err, ErrDuplicateKeyImage)
}

func TestTx_CheckKeyImages_HTLCAndToKeyDuplicate_Bad(t *testing.T) {
	ki := types.KeyImage{0x42}
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: 100, KeyImage: ki},
			types.TxInputHTLC{Amount: 50, KeyImage: ki}, // duplicate across types
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 140, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 20000)
	assert.ErrorIs(t, err, ErrDuplicateKeyImage)
}

func TestTx_CheckOutputs_HTLCTargetPostHF1_Good(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: 100, KeyImage: types.KeyImage{1}},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{
				Amount: 90,
				Target: types.TxOutHTLC{Expiration: 20000},
			},
		},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 20000) // post-HF1
	require.NoError(t, err)
}
