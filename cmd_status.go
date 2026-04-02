// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package blockchain

import (
	"dappco.re/go/core"

	"dappco.re/go/core/blockchain/rpc"
	"github.com/spf13/cobra"
)

func newStatusCmd(dataDir, seed *string, testnet *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show chain and network status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(*seed, *testnet)
		},
	}
}

func runStatus(seed string, testnet bool) error {
	rpcURL := seedToRPC(seed, testnet)
	client := rpc.NewClient(rpcURL)

	info, err := client.GetInfo()
	if err != nil {
		core.Print(nil, "Daemon unreachable at %s", rpcURL)
		return err
	}

	core.Print(nil, "Lethean Chain Status")
	core.Print(nil, "  Height:     %d", info.Height)
	core.Print(nil, "  Difficulty: %d", info.PowDifficulty)
	core.Print(nil, "  Aliases:    %d", info.AliasCount)
	core.Print(nil, "  TX Pool:    %d", info.TxPoolSize)
	core.Print(nil, "  PoS:        %v", info.PosAllowed)
	core.Print(nil, "  Synced:     %v", info.DaemonNetworkState == 2)
	core.Print(nil, "")

	// Hardfork status

	hf5Remaining := int64(11500) - int64(info.Height)
	if hf5Remaining > 0 {
		core.Print(nil, "")
		core.Print(nil, "  HF5 in %d blocks (~%.1f hours)", hf5Remaining, float64(hf5Remaining)*2/60)
	}

	return nil
}
