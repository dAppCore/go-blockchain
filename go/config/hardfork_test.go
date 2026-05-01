// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package config

import (
	"testing"
)

func TestHardfork_VersionAtHeight_Good(t *testing.T) {
	tests := []struct {
		name   string
		height uint64
		want   uint8
	}{
		// Genesis block should return HF0 (the initial version).
		{"genesis", 0, HF0Initial},

		// Just before HF1/HF2 activation on mainnet (height 10080).
		// HF1 activates at heights > 10080, so height 10080 is still HF0.
		{"before_hf1", 10080, HF0Initial},

		// At height 10081, both HF1 and HF2 activate (both have height 10080).
		// The highest version wins.
		{"at_hf1_hf2", 10081, HF2},

		// Well past HF2 but before any future forks.
		{"mid_chain", 50000, HF2},

		// Far future but still below 999999999 — should still be HF2.
		{"high_but_before_future", 999999999, HF2},

		// At height 1000000000 (> 999999999), all future forks activate.
		{"all_forks_active", 1000000000, HF6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VersionAtHeight(MainnetForks, tt.height)
			if got != tt.want {
				t.Errorf("VersionAtHeight(MainnetForks, %d) = %d, want %d", tt.height, got, tt.want)
			}
		})
	}
}

func TestHardfork_VersionAtHeightTestnet_Good(t *testing.T) {
	tests := []struct {
		name   string
		height uint64
		want   uint8
	}{
		// On testnet, HF0, HF1, and HF3 all have Height=0 so they are
		// active from genesis. The highest version at genesis is HF3.
		{"genesis", 0, HF3},

		// At height 10, HF2 is still not active (activates at > 10).
		{"before_hf2", 10, HF3},

		// At height 11, HF2 activates — but HF3 is already active so
		// the version remains 3.
		{"after_hf2", 11, HF3},

		// At height 101, HF4 Zarcanum activates.
		{"after_hf4", 101, HF4Zarcanum},

		// At height 201, HF5 activates.
		{"after_hf5", 201, HF5},

		// Far future activates HF6.
		{"far_future", 1000000000, HF6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VersionAtHeight(TestnetForks, tt.height)
			if got != tt.want {
				t.Errorf("VersionAtHeight(TestnetForks, %d) = %d, want %d", tt.height, got, tt.want)
			}
		})
	}
}

func TestHardfork_IsHardForkActive_Good(t *testing.T) {
	tests := []struct {
		name    string
		version uint8
		height  uint64
		want    bool
	}{
		// HF0 is always active (Height=0).
		{"hf0_at_genesis", HF0Initial, 0, true},
		{"hf0_at_10000", HF0Initial, 10000, true},

		// HF1 activates at heights > 10080.
		{"hf1_before", HF1, 10080, false},
		{"hf1_at", HF1, 10081, true},

		// HF4 is far future on mainnet.
		{"hf4_now", HF4Zarcanum, 50000, false},
		{"hf4_far_future", HF4Zarcanum, 1000000000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsHardForkActive(MainnetForks, tt.version, tt.height)
			if got != tt.want {
				t.Errorf("IsHardForkActive(MainnetForks, %d, %d) = %v, want %v",
					tt.version, tt.height, got, tt.want)
			}
		})
	}
}

func TestHardfork_IsHardForkActive_Bad(t *testing.T) {
	// Querying a version that does not exist should return false.
	got := IsHardForkActive(MainnetForks, 99, 1000000000)
	if got {
		t.Error("IsHardForkActive with unknown version should return false")
	}
}

func TestHardfork_VersionAtHeight_Ugly(t *testing.T) {
	// Empty fork list should return version 0.
	got := VersionAtHeight(nil, 100)
	if got != 0 {
		t.Errorf("VersionAtHeight(nil, 100) = %d, want 0", got)
	}

	// Single-element fork list.
	single := []HardFork{{Version: 1, Height: 0, Mandatory: true}}
	got = VersionAtHeight(single, 0)
	if got != 1 {
		t.Errorf("VersionAtHeight(single, 0) = %d, want 1", got)
	}
}

func TestHardfork_MainnetForkSchedule_Good(t *testing.T) {
	// Verify the fork schedule matches the C++ ZANO_HARDFORK_* constants.
	expectedMainnet := []struct {
		version uint8
		height  uint64
	}{
		{0, 0},
		{1, 10080},
		{2, 10080},
		{3, 999999999},
		{4, 999999999},
		{5, 999999999},
		{6, 999999999},
	}

	if len(MainnetForks) != len(expectedMainnet) {
		t.Fatalf("MainnetForks length: got %d, want %d", len(MainnetForks), len(expectedMainnet))
	}

	for i, exp := range expectedMainnet {
		hf := MainnetForks[i]
		if hf.Version != exp.version {
			t.Errorf("MainnetForks[%d].Version = %d, want %d", i, hf.Version, exp.version)
		}
		if hf.Height != exp.height {
			t.Errorf("MainnetForks[%d].Height = %d, want %d", i, hf.Height, exp.height)
		}
	}
}

func TestHardfork_TestnetForkSchedule_Good(t *testing.T) {
	expectedTestnet := []struct {
		version uint8
		height  uint64
	}{
		{0, 0},
		{1, 0},
		{2, 10},
		{3, 0},
		{4, 100},
		{5, 200},
		{6, 999999999},
	}

	if len(TestnetForks) != len(expectedTestnet) {
		t.Fatalf("TestnetForks length: got %d, want %d", len(TestnetForks), len(expectedTestnet))
	}

	for i, exp := range expectedTestnet {
		hf := TestnetForks[i]
		if hf.Version != exp.version {
			t.Errorf("TestnetForks[%d].Version = %d, want %d", i, hf.Version, exp.version)
		}
		if hf.Height != exp.height {
			t.Errorf("TestnetForks[%d].Height = %d, want %d", i, hf.Height, exp.height)
		}
	}
}
