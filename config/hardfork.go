// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// You may obtain a copy of the licence at:
//
//     https://joinup.ec.europa.eu/software/page/eupl/licence-eupl
//
// SPDX-License-Identifier: EUPL-1.2

package config

// HardFork describes a single hard fork activation point on the chain.
type HardFork struct {
	// Version is the hardfork version number (0-6).
	Version uint8

	// Height is the block height AFTER which this fork activates.
	// The fork is active at heights strictly greater than this value.
	// A value of 0 means the fork is active from genesis.
	Height uint64

	// Mandatory indicates whether nodes must support this fork version
	// to remain on the network.
	Mandatory bool

	// Description is a short human-readable summary of what this fork changes.
	Description string
}

// Hardfork version constants, matching the C++ ZANO_HARDFORK_* identifiers.
const (
	HF0Initial   uint8 = 0
	HF1          uint8 = 1
	HF2          uint8 = 2
	HF3          uint8 = 3
	HF4Zarcanum  uint8 = 4
	HF5          uint8 = 5
	HF6          uint8 = 6
	HFTotal      uint8 = 7
)

// MainnetForks lists all hardfork activations for the Lethean mainnet.
// Heights correspond to ZANO_HARDFORK_*_AFTER_HEIGHT in the C++ source.
// The fork activates at heights strictly greater than the listed height,
// so Height=0 means active from genesis, and Height=10080 means active
// from block 10081 onwards.
var MainnetForks = []HardFork{
	{Version: HF0Initial, Height: 0, Mandatory: true, Description: "CryptoNote base, PoW+PoS hybrid"},
	{Version: HF1, Height: 10080, Mandatory: true, Description: "New transaction types"},
	{Version: HF2, Height: 10080, Mandatory: true, Description: "Block time adjustment (720 blocks/day)"},
	{Version: HF3, Height: 999999999, Mandatory: false, Description: "Block version 2"},
	{Version: HF4Zarcanum, Height: 999999999, Mandatory: false, Description: "Zarcanum privacy (confidential txs, CLSAG)"},
	{Version: HF5, Height: 999999999, Mandatory: false, Description: "Confidential assets, surjection proofs"},
	{Version: HF6, Height: 999999999, Mandatory: false, Description: "Block time halving (120s to 240s)"},
}

// TestnetForks lists all hardfork activations for the Lethean testnet.
var TestnetForks = []HardFork{
	{Version: HF0Initial, Height: 0, Mandatory: true, Description: "CryptoNote base, PoW+PoS hybrid"},
	{Version: HF1, Height: 0, Mandatory: true, Description: "New transaction types"},
	{Version: HF2, Height: 10, Mandatory: true, Description: "Block time adjustment"},
	{Version: HF3, Height: 0, Mandatory: true, Description: "Block version 2"},
	{Version: HF4Zarcanum, Height: 100, Mandatory: true, Description: "Zarcanum privacy"},
	{Version: HF5, Height: 200, Mandatory: true, Description: "Confidential assets"},
	{Version: HF6, Height: 999999999, Mandatory: false, Description: "Block time halving"},
}

// VersionAtHeight returns the highest hardfork version that is active at the
// given block height. It performs a reverse scan of the fork list to find the
// latest applicable version.
//
// A fork with Height=0 is active from genesis (height 0).
// A fork with Height=N is active at heights > N.
func VersionAtHeight(forks []HardFork, height uint64) uint8 {
	var version uint8
	for _, hf := range forks {
		if hf.Height == 0 || height > hf.Height {
			if hf.Version > version {
				version = hf.Version
			}
		}
	}
	return version
}

// IsHardForkActive reports whether the specified hardfork version is active
// at the given block height.
func IsHardForkActive(forks []HardFork, version uint8, height uint64) bool {
	for _, hf := range forks {
		if hf.Version == version {
			return hf.Height == 0 || height > hf.Height
		}
	}
	return false
}
