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
)

// validV2Tx returns a minimal valid v2 (Zarcanum) transaction for testing.
func validV2Tx() *types.Transaction {
	return &types.Transaction{
		Version: types.VersionPostHF4,
		Vin: []types.TxInput{
			types.TxInputZC{
				KeyImage: types.KeyImage{1},
			},
		},
		Vout: []types.TxOutput{
			types.TxOutputZarcanum{StealthAddress: types.PublicKey{1}},
			types.TxOutputZarcanum{StealthAddress: types.PublicKey{2}},
		},
	}
}

// validV3Tx returns a minimal valid v3 (HF5) transaction for testing.
func validV3Tx() *types.Transaction {
	return &types.Transaction{
		Version:    types.VersionPostHF5,
		HardforkID: 5,
		Vin: []types.TxInput{
			types.TxInputZC{
				KeyImage: types.KeyImage{1},
			},
		},
		Vout: []types.TxOutput{
			types.TxOutputZarcanum{StealthAddress: types.PublicKey{1}},
			types.TxOutputZarcanum{StealthAddress: types.PublicKey{2}},
		},
	}
}

func TestTxVersion_CheckTxVersion_Good(t *testing.T) {
	tests := []struct {
		name   string
		tx     *types.Transaction
		forks  []config.HardFork
		height uint64
	}{
		// v1 transaction before HF4 — valid.
		{"v1_before_hf4", validV1Tx(), config.MainnetForks, 5000},
		// v2 transaction after HF4, before HF5 — valid.
		{"v2_after_hf4_before_hf5", validV2Tx(), config.TestnetForks, 150},
		// v3 transaction after HF5 — valid.
		{"v3_after_hf5", validV3Tx(), config.TestnetForks, 250},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkTxVersion(tt.tx, tt.forks, tt.height)
			if err != nil {
				t.Errorf("checkTxVersion returned unexpected error: %v", err)
			}
		})
	}
}

func TestTxVersion_CheckTxVersion_Bad(t *testing.T) {
	tests := []struct {
		name   string
		tx     *types.Transaction
		forks  []config.HardFork
		height uint64
	}{
		// v2 transaction after HF5 — must be v3.
		{"v2_after_hf5", validV2Tx(), config.TestnetForks, 250},
		// v3 transaction before HF5 — too early.
		{"v3_before_hf5", validV3Tx(), config.TestnetForks, 150},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkTxVersion(tt.tx, tt.forks, tt.height)
			if err == nil {
				t.Error("expected ErrTxVersionInvalid, got nil")
			}
		})
	}
}

func TestTxVersion_CheckTxVersion_Ugly(t *testing.T) {
	// v3 at exact HF5 activation boundary (height 201 on testnet, HF5.Height=200).
	tx := validV3Tx()
	err := checkTxVersion(tx, config.TestnetForks, 201)
	if err != nil {
		t.Errorf("v3 at HF5 activation boundary should be valid: %v", err)
	}

	// v2 at exact HF5 activation boundary — should be rejected.
	tx2 := validV2Tx()
	err = checkTxVersion(tx2, config.TestnetForks, 201)
	if err == nil {
		t.Error("v2 at HF5 activation boundary should be rejected")
	}
}
