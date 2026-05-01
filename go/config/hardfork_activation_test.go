// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package config

import "testing"

func TestHardforkActivation_HardforkActivationHeight_Good(t *testing.T) {
	tests := []struct {
		name    string
		forks   []HardFork
		version uint8
		want    uint64
		wantOK  bool
	}{
		{"mainnet_hf5", MainnetForks, HF5, 999999999, true},
		{"testnet_hf5", TestnetForks, HF5, 200, true},
		{"testnet_hf4", TestnetForks, HF4Zarcanum, 100, true},
		{"mainnet_hf0", MainnetForks, HF0Initial, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := HardforkActivationHeight(tt.forks, tt.version)
			if ok != tt.wantOK {
				t.Fatalf("HardforkActivationHeight ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("HardforkActivationHeight = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestHardforkActivation_HardforkActivationHeight_Bad(t *testing.T) {
	_, ok := HardforkActivationHeight(MainnetForks, 99)
	if ok {
		t.Error("HardforkActivationHeight with unknown version should return false")
	}
}

func TestHardforkActivation_HardforkActivationHeight_Ugly(t *testing.T) {
	_, ok := HardforkActivationHeight(nil, HF5)
	if ok {
		t.Error("HardforkActivationHeight with nil forks should return false")
	}
}
