// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package config

import "testing"

func TestFileConfig_DefaultFileConfig_Good(t *testing.T) {
	cfg := DefaultFileConfig()
	if cfg.Network != "testnet" {
		t.Errorf("default network: got %s, want testnet", cfg.Network)
	}
	if cfg.RPCPort != "46941" {
		t.Errorf("default rpc port: got %s, want 46941", cfg.RPCPort)
	}
	if cfg.DNSPort != "53" {
		t.Errorf("default dns port: got %s, want 53", cfg.DNSPort)
	}
}

func TestFileConfig_IsTestnet_Good(t *testing.T) {
	cfg := FileConfig{Network: "testnet"}
	if !cfg.IsTestnet() {
		t.Error("expected IsTestnet() true for testnet")
	}

	cfg.Network = "mainnet"
	if cfg.IsTestnet() {
		t.Error("expected IsTestnet() false for mainnet")
	}
}

func TestFileConfig_ToChainConfig_Good(t *testing.T) {
	cfg := FileConfig{Network: "testnet"}
	chainCfg, forks := cfg.ToChainConfig()
	if !chainCfg.IsTestnet {
		t.Error("expected testnet chain config")
	}
	if len(forks) == 0 {
		t.Error("expected hardfork schedule")
	}

	cfg.Network = "mainnet"
	chainCfg, _ = cfg.ToChainConfig()
	if chainCfg.IsTestnet {
		t.Error("expected mainnet chain config")
	}
}

func TestFileConfig_LoadFromEnv_Good(t *testing.T) {
	t.Setenv("RPC_PORT", "12345")
	t.Setenv("HSD_URL", "http://custom:14037")

	cfg := DefaultFileConfig()
	cfg.LoadFromEnv()

	if cfg.RPCPort != "12345" {
		t.Errorf("env override: got %s, want 12345", cfg.RPCPort)
	}
	if cfg.HSDUrl != "http://custom:14037" {
		t.Errorf("env override: got %s, want http://custom:14037", cfg.HSDUrl)
	}
}
