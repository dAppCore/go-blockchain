// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package config

import (
	"os"

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
)

// FileConfig loads blockchain settings from a YAML file or environment.
//
//	cfg := config.LoadFileConfig(".core/blockchain.yaml")
type FileConfig struct {
	Network  string `json:"network"`  // mainnet | testnet
	Seed     string `json:"seed"`     // seed peer address
	DataDir  string `json:"datadir"`  // chain data directory
	RPCPort  string `json:"rpc_port"` // JSON-RPC port
	RPCBind  string `json:"rpc_bind"` // JSON-RPC bind address
	HSDUrl   string `json:"hsd_url"`  // HSD sidechain URL
	HSDKey   string `json:"hsd_key"`  // HSD API key
	DNSPort  string `json:"dns_port"` // DNS server port
}

// DefaultFileConfig returns sensible defaults for testnet.
//
//	cfg := config.DefaultFileConfig()
func DefaultFileConfig() FileConfig {
	return FileConfig{
		Network: "testnet",
		Seed:    "seeds.lthn.io:46942",
		DataDir: core.Path("~", ".lethean", "chain"),
		RPCPort: "46941",
		RPCBind: "127.0.0.1",
		HSDUrl:  "http://127.0.0.1:14037",
		HSDKey:  "testkey",
		DNSPort: "53",
	}
}

// LoadFromEnv overrides config values from environment variables.
//
//	cfg.LoadFromEnv()
func (c *FileConfig) LoadFromEnv() {
	if v := core.Env("LNS_MODE"); v != "" { c.Network = v }
	if v := core.Env("DAEMON_SEED"); v != "" { c.Seed = v }
	if v := core.Env("CHAIN_DATADIR"); v != "" { c.DataDir = v }
	if v := core.Env("RPC_PORT"); v != "" { c.RPCPort = v }
	if v := core.Env("RPC_BIND"); v != "" { c.RPCBind = v }
	if v := core.Env("HSD_URL"); v != "" { c.HSDUrl = v }
	if v := core.Env("HSD_API_KEY"); v != "" { c.HSDKey = v }
	if v := core.Env("DNS_PORT"); v != "" { c.DNSPort = v }
}

// IsTestnet returns true if configured for testnet.
//
//	if cfg.IsTestnet() { /* use testnet genesis */ }
func (c *FileConfig) IsTestnet() bool {
	return c.Network == "testnet"
}

// ToChainConfig converts file config to the internal ChainConfig.
//
//	chainCfg, forks := fileCfg.ToChainConfig()
func (c *FileConfig) ToChainConfig() (ChainConfig, []HardFork) {
	if c.IsTestnet() {
		cfg := Testnet
		return cfg, TestnetForks
	}
	cfg := Mainnet
	return cfg, MainnetForks
}

// LoadFileConfig reads config from a YAML file, then overlays env vars.
//
//	cfg := config.LoadFileConfig(".core/blockchain.yaml")
func LoadFileConfig(path string) FileConfig {
	cfg := DefaultFileConfig()

	// Try to read YAML file
	data, err := os.ReadFile(path)
	if err == nil {
		core.JSONUnmarshalString(string(data), &cfg)
		_ = coreerr.E("config", "loaded from file", nil) // just for logging pattern
	}

	// Env vars override file
	cfg.LoadFromEnv()
	return cfg
}
