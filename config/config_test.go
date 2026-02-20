// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package config

import (
	"testing"
)

func TestMainnetConstants_Good(t *testing.T) {
	// Verify tokenomics match C++ source (default.cmake).
	if Coin != 1_000_000_000_000 {
		t.Errorf("Coin: got %d, want 1000000000000", Coin)
	}
	if DisplayDecimalPoint != 12 {
		t.Errorf("DisplayDecimalPoint: got %d, want 12", DisplayDecimalPoint)
	}
	if BlockReward != 1_000_000_000_000 {
		t.Errorf("BlockReward: got %d, want 1000000000000", BlockReward)
	}
	if DefaultFee != 10_000_000_000 {
		t.Errorf("DefaultFee: got %d, want 10000000000", DefaultFee)
	}
	if MinimumFee != 10_000_000_000 {
		t.Errorf("MinimumFee: got %d, want 10000000000", MinimumFee)
	}
	if BaseRewardDustThreshold != 1_000_000 {
		t.Errorf("BaseRewardDustThreshold: got %d, want 1000000", BaseRewardDustThreshold)
	}
	if DefaultDustThreshold != 0 {
		t.Errorf("DefaultDustThreshold: got %d, want 0", DefaultDustThreshold)
	}
}

func TestMainnetAddressPrefixes_Good(t *testing.T) {
	tests := []struct {
		name string
		got  uint64
		want uint64
	}{
		{"AddressPrefix", AddressPrefix, 0x1eaf7},
		{"IntegratedAddressPrefix", IntegratedAddressPrefix, 0xdeaf7},
		{"AuditableAddressPrefix", AuditableAddressPrefix, 0x3ceff7},
		{"AuditableIntegratedAddressPrefix", AuditableIntegratedAddressPrefix, 0x8b077},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s: got 0x%x, want 0x%x", tt.name, tt.got, tt.want)
		}
	}
}

func TestMainnetPorts_Good(t *testing.T) {
	if Mainnet.P2PPort != 36942 {
		t.Errorf("Mainnet P2P port: got %d, want 36942", Mainnet.P2PPort)
	}
	if Mainnet.RPCPort != 36941 {
		t.Errorf("Mainnet RPC port: got %d, want 36941", Mainnet.RPCPort)
	}
	if Mainnet.StratumPort != 36940 {
		t.Errorf("Mainnet Stratum port: got %d, want 36940", Mainnet.StratumPort)
	}
}

func TestTestnetPortDifferences_Good(t *testing.T) {
	if Testnet.P2PPort != 46942 {
		t.Errorf("Testnet P2P port: got %d, want 46942", Testnet.P2PPort)
	}
	if Testnet.RPCPort != 46941 {
		t.Errorf("Testnet RPC port: got %d, want 46941", Testnet.RPCPort)
	}
	if Testnet.StratumPort != 46940 {
		t.Errorf("Testnet Stratum port: got %d, want 46940", Testnet.StratumPort)
	}
	if Testnet.P2PPort == Mainnet.P2PPort {
		t.Error("Testnet and Mainnet P2P ports must differ")
	}
}

func TestDifficultyConstants_Good(t *testing.T) {
	if DifficultyPowTarget != 120 {
		t.Errorf("DifficultyPowTarget: got %d, want 120", DifficultyPowTarget)
	}
	if DifficultyPosTarget != 120 {
		t.Errorf("DifficultyPosTarget: got %d, want 120", DifficultyPosTarget)
	}
	if DifficultyTotalTarget != 60 {
		t.Errorf("DifficultyTotalTarget: got %d, want 60 ((120+120)/4)", DifficultyTotalTarget)
	}
	if DifficultyWindow != 720 {
		t.Errorf("DifficultyWindow: got %d, want 720", DifficultyWindow)
	}
	if DifficultyLag != 15 {
		t.Errorf("DifficultyLag: got %d, want 15", DifficultyLag)
	}
	if DifficultyCut != 60 {
		t.Errorf("DifficultyCut: got %d, want 60", DifficultyCut)
	}
	if DifficultyBlocksCount != 735 {
		t.Errorf("DifficultyBlocksCount: got %d, want 735 (720+15)", DifficultyBlocksCount)
	}
}

func TestNetworkIdentity_Good(t *testing.T) {
	if CurrencyFormationVersion != 84 {
		t.Errorf("CurrencyFormationVersion: got %d, want 84", CurrencyFormationVersion)
	}
	if CurrencyFormationVersionTestnet != 100 {
		t.Errorf("CurrencyFormationVersionTestnet: got %d, want 100", CurrencyFormationVersionTestnet)
	}
	if P2PNetworkIDVer != 84 {
		t.Errorf("P2PNetworkIDVer: got %d, want 84 (84+0)", P2PNetworkIDVer)
	}
}

func TestChainConfigStruct_Good(t *testing.T) {
	// Verify Mainnet struct fields are populated correctly.
	if Mainnet.Name != "Lethean" {
		t.Errorf("Mainnet.Name: got %q, want %q", Mainnet.Name, "Lethean")
	}
	if Mainnet.Abbreviation != "LTHN" {
		t.Errorf("Mainnet.Abbreviation: got %q, want %q", Mainnet.Abbreviation, "LTHN")
	}
	if Mainnet.IsTestnet {
		t.Error("Mainnet.IsTestnet should be false")
	}
	if !Testnet.IsTestnet {
		t.Error("Testnet.IsTestnet should be true")
	}
	if Testnet.Name != "Lethean_testnet" {
		t.Errorf("Testnet.Name: got %q, want %q", Testnet.Name, "Lethean_testnet")
	}
}

func TestTransactionLimits_Good(t *testing.T) {
	if TxMaxAllowedInputs != 256 {
		t.Errorf("TxMaxAllowedInputs: got %d, want 256", TxMaxAllowedInputs)
	}
	if TxMaxAllowedOutputs != 2000 {
		t.Errorf("TxMaxAllowedOutputs: got %d, want 2000", TxMaxAllowedOutputs)
	}
	if DefaultDecoySetSize != 10 {
		t.Errorf("DefaultDecoySetSize: got %d, want 10", DefaultDecoySetSize)
	}
	if HF4MandatoryDecoySetSize != 15 {
		t.Errorf("HF4MandatoryDecoySetSize: got %d, want 15", HF4MandatoryDecoySetSize)
	}
}

func TestTransactionVersionConstants_Good(t *testing.T) {
	if TransactionVersionInitial != 0 {
		t.Errorf("TransactionVersionInitial: got %d, want 0", TransactionVersionInitial)
	}
	if TransactionVersionPreHF4 != 1 {
		t.Errorf("TransactionVersionPreHF4: got %d, want 1", TransactionVersionPreHF4)
	}
	if TransactionVersionPostHF4 != 2 {
		t.Errorf("TransactionVersionPostHF4: got %d, want 2", TransactionVersionPostHF4)
	}
	if TransactionVersionPostHF5 != 3 {
		t.Errorf("TransactionVersionPostHF5: got %d, want 3", TransactionVersionPostHF5)
	}
	if CurrentTransactionVersion != 3 {
		t.Errorf("CurrentTransactionVersion: got %d, want 3", CurrentTransactionVersion)
	}
}

func TestNetworkID_Good(t *testing.T) {
	// Mainnet: byte 10 = 0 (not testnet), byte 15 = 84 (0x54)
	if NetworkIDMainnet[10] != 0x00 {
		t.Errorf("mainnet testnet flag: got %x, want 0x00", NetworkIDMainnet[10])
	}
	if NetworkIDMainnet[15] != 0x54 {
		t.Errorf("mainnet version: got %x, want 0x54", NetworkIDMainnet[15])
	}
	// Testnet: byte 10 = 1, byte 15 = 100 (0x64)
	if NetworkIDTestnet[10] != 0x01 {
		t.Errorf("testnet testnet flag: got %x, want 0x01", NetworkIDTestnet[10])
	}
	if NetworkIDTestnet[15] != 0x64 {
		t.Errorf("testnet version: got %x, want 0x64", NetworkIDTestnet[15])
	}
	// ChainConfig should have them
	if Mainnet.NetworkID != NetworkIDMainnet {
		t.Error("Mainnet.NetworkID mismatch")
	}
	if Testnet.NetworkID != NetworkIDTestnet {
		t.Error("Testnet.NetworkID mismatch")
	}
}
