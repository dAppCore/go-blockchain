// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

//go:build !integration

package consensus

import (
	"testing"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-blockchain/types"
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

func TestValidateTransaction_Good(t *testing.T) {
	tx := validV1Tx()
	blob := make([]byte, 100) // small blob
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	require.NoError(t, err)
}

func TestValidateTransaction_BlobTooLarge(t *testing.T) {
	tx := validV1Tx()
	blob := make([]byte, config.MaxTransactionBlobSize+1)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrTxTooLarge)
}

func TestValidateTransaction_NoInputs(t *testing.T) {
	tx := validV1Tx()
	tx.Vin = nil
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrNoInputs)
}

func TestValidateTransaction_TooManyInputs(t *testing.T) {
	tx := validV1Tx()
	tx.Vin = make([]types.TxInput, config.TxMaxAllowedInputs+1)
	for i := range tx.Vin {
		tx.Vin[i] = types.TxInputToKey{Amount: 1, KeyImage: types.KeyImage{byte(i)}}
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrTooManyInputs)
}

func TestValidateTransaction_InvalidInputType(t *testing.T) {
	tx := validV1Tx()
	tx.Vin = []types.TxInput{types.TxInputGenesis{Height: 1}} // genesis not allowed
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrInvalidInputType)
}

func TestValidateTransaction_NoOutputs(t *testing.T) {
	tx := validV1Tx()
	tx.Vout = nil
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrNoOutputs)
}

func TestValidateTransaction_TooManyOutputs(t *testing.T) {
	tx := validV1Tx()
	tx.Vout = make([]types.TxOutput, config.TxMaxAllowedOutputs+1)
	for i := range tx.Vout {
		tx.Vout[i] = types.TxOutputBare{Amount: 1, Target: types.TxOutToKey{Key: types.PublicKey{byte(i)}}}
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrTooManyOutputs)
}

func TestValidateTransaction_ZeroOutputAmount(t *testing.T) {
	tx := validV1Tx()
	tx.Vout = []types.TxOutput{
		types.TxOutputBare{Amount: 0, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
	}
	blob := make([]byte, 100)
	err := ValidateTransaction(tx, blob, config.MainnetForks, 5000)
	assert.ErrorIs(t, err, ErrInvalidOutput)
}

func TestValidateTransaction_DuplicateKeyImage(t *testing.T) {
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

func TestValidateTransaction_NegativeFee(t *testing.T) {
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
