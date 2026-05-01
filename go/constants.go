// Copyright (c) 2017-2026 Lethean (https://lt.hn)
// SPDX-License-Identifier: EUPL-1.2

package blockchain

// Chain constants used across commands and services.
const (
	// AtomicUnit is the number of atomic units per 1 LTHN.
	AtomicUnit uint64 = 1000000000000

	// StandardPrefix is the base58 address prefix for standard addresses.
	StandardPrefix uint64 = 0x1eaf7

	// IntegratedPrefix is the base58 prefix for integrated addresses.
	IntegratedPrefix uint64 = 0xdeaf7

	// AuditablePrefix is the base58 prefix for auditable addresses.
	AuditablePrefix uint64 = 0x3ceff7

	// AuditableIntegratedPrefix is the base58 prefix for auditable integrated.
	AuditableIntegratedPrefix uint64 = 0x8b077

	// HF5Height is the activation height for confidential assets.
	HF5Height uint64 = 11500

	// HF4Height is the activation height for Zarcanum.
	HF4Height uint64 = 11000

	// DefaultBlockReward is the fixed block reward in LTHN.
	DefaultBlockReward uint64 = 1

	// DefaultFee is the default transaction fee in atomic units.
	DefaultFee uint64 = 10000000000 // 0.01 LTHN

	// PremineAmount is the genesis premine in LTHN.
	PremineAmount uint64 = 10000000

	// TestnetDaemonRPC is the default testnet daemon RPC port.
	TestnetDaemonRPC = "46941"

	// TestnetWalletRPC is the default testnet wallet RPC port.
	TestnetWalletRPC = "46944"

	// TestnetP2P is the default testnet P2P port.
	TestnetP2P = "46942"

	// MainnetDaemonRPC is the default mainnet daemon RPC port.
	MainnetDaemonRPC = "36941"

	// TestnetHSDPort is the default HSD sidechain RPC port.
	TestnetHSDPort = "14037"
)
