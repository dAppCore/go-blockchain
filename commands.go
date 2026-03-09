// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package blockchain

import (
	"fmt"
	"os"
	"path/filepath"

	"forge.lthn.ai/core/go-blockchain/config"
	"github.com/spf13/cobra"
)

// AddChainCommands registers the "chain" command group with explorer,
// sync, and mine subcommands.
func AddChainCommands(root *cobra.Command) {
	var (
		dataDir string
		seed    string
		testnet bool
	)

	chainCmd := &cobra.Command{
		Use:   "chain",
		Short: "Lethean blockchain node",
		Long:  "Manage the Lethean blockchain — sync, explore, and mine.",
	}

	chainCmd.PersistentFlags().StringVar(&dataDir, "data-dir", defaultDataDir(), "blockchain data directory")
	chainCmd.PersistentFlags().StringVar(&seed, "seed", "seeds.lthn.io:36942", "seed peer address (host:port)")
	chainCmd.PersistentFlags().BoolVar(&testnet, "testnet", false, "use testnet")

	chainCmd.AddCommand(
		newExplorerCmd(&dataDir, &seed, &testnet),
		newSyncCmd(&dataDir, &seed, &testnet),
	)

	root.AddCommand(chainCmd)
}

func resolveConfig(testnet bool, seed *string) (config.ChainConfig, []config.HardFork) {
	if testnet {
		if *seed == "seeds.lthn.io:36942" {
			*seed = "localhost:46942"
		}
		return config.Testnet, config.TestnetForks
	}
	return config.Mainnet, config.MainnetForks
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".lethean"
	}
	return filepath.Join(home, ".lethean", "chain")
}

func ensureDataDir(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	return nil
}
