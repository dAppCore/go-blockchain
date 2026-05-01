// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

// Known genesis hashes for network detection.
const (
	// MainnetGenesisHash is the expected genesis block hash for mainnet.
	//
	//	if hash == chain.MainnetGenesisHash { /* mainnet */ }
	MainnetGenesisHash = "cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963"

	// TestnetGenesisHash is the expected genesis block hash for testnet.
	//
	//	if hash == chain.TestnetGenesisHash { /* testnet */ }
	TestnetGenesisHash = "7cf844dc3e7d8dd6af65642c68164ebe18109aa5167b5f76043f310dd6e142d0"
)

// DetectNetwork identifies the network from a genesis block hash.
// Returns "mainnet", "testnet", or "unknown".
//
//	network := chain.DetectNetwork(genesisHash)
func DetectNetwork(genesisHash string) string {
	switch genesisHash {
	case MainnetGenesisHash:
		return "mainnet"
	case TestnetGenesisHash:
		return "testnet"
	default:
		return "unknown"
	}
}
