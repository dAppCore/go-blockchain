// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import "testing"

func TestGenesis_DetectNetwork_Good(t *testing.T) {
	tests := []struct{
		hash string
		want string
	}{
		{MainnetGenesisHash, "mainnet"},
		{TestnetGenesisHash, "testnet"},
		{"0000000000000000000000000000000000000000000000000000000000000000", "unknown"},
	}

	for _, tt := range tests {
		got := DetectNetwork(tt.hash)
		if got != tt.want {
			t.Errorf("DetectNetwork(%s...): got %s, want %s", tt.hash[:16], got, tt.want)
		}
	}
}
