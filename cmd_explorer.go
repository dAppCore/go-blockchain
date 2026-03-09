// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package blockchain

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	cli "forge.lthn.ai/core/cli/pkg/cli"
	store "forge.lthn.ai/core/go-store"

	"forge.lthn.ai/core/go-blockchain/chain"
	"forge.lthn.ai/core/go-blockchain/tui"
	"github.com/spf13/cobra"
)

func newExplorerCmd(dataDir, seed *string, testnet *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "explorer",
		Short: "TUI block explorer",
		Long:  "Interactive terminal block explorer with live sync status.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExplorer(*dataDir, *seed, *testnet)
		},
	}
}

func runExplorer(dataDir, seed string, testnet bool) error {
	if err := ensureDataDir(dataDir); err != nil {
		return err
	}

	dbPath := filepath.Join(dataDir, "chain.db")
	s, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer s.Close()

	c := chain.New(s)
	cfg, forks := resolveConfig(testnet, &seed)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	go syncLoop(ctx, c, &cfg, forks, seed)

	node := tui.NewNode(c)
	status := tui.NewStatusModel(node)
	explorer := tui.NewExplorerModel(c)
	hints := tui.NewKeyHintsModel()

	frame := cli.NewFrame("HCF")
	frame.Header(status)
	frame.Content(explorer)
	frame.Footer(hints)
	frame.Run()

	return nil
}
