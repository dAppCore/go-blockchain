// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

//go:build !integration

package consensus

import (
	"testing"

	"dappco.re/go/core/blockchain/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTxFee_Good(t *testing.T) {
	// Coinbase tx: fee is 0.
	coinbase := &types.Transaction{
		Version: types.VersionInitial,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 1}},
	}
	fee, err := TxFee(coinbase)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), fee)

	// Normal v1 tx: fee = inputs - outputs.
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: 100},
			types.TxInputToKey{Amount: 50},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 120},
		},
	}
	fee, err = TxFee(tx)
	require.NoError(t, err)
	assert.Equal(t, uint64(30), fee)
}

func TestTxFee_Bad(t *testing.T) {
	// Outputs exceed inputs.
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: 50},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 100},
		},
	}
	_, err := TxFee(tx)
	assert.ErrorIs(t, err, ErrNegativeFee)
}

func TestTxFee_Ugly(t *testing.T) {
	// Input amounts that would overflow uint64.
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: ^uint64(0)},
			types.TxInputToKey{Amount: 1},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 1},
		},
	}
	_, err := TxFee(tx)
	assert.ErrorIs(t, err, ErrInputOverflow)
}

// --- HTLC and multisig fee tests (Task 8) ---

func TestTxFee_HTLCInput_Good(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputHTLC{Amount: 100, KeyImage: types.KeyImage{1}},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 90, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	fee, err := TxFee(tx)
	require.NoError(t, err)
	assert.Equal(t, uint64(10), fee)
}

func TestTxFee_MultisigInput_Good(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputMultisig{Amount: 200},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 150, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	fee, err := TxFee(tx)
	require.NoError(t, err)
	assert.Equal(t, uint64(50), fee)
}

func TestTxFee_MixedInputs_Good(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: 100, KeyImage: types.KeyImage{1}},
			types.TxInputHTLC{Amount: 50, KeyImage: types.KeyImage{2}},
			types.TxInputMultisig{Amount: 30},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 170, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	fee, err := TxFee(tx)
	require.NoError(t, err)
	assert.Equal(t, uint64(10), fee) // 180 - 170
}
