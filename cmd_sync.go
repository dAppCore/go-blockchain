// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package blockchain

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"forge.lthn.ai/core/go-blockchain/chain"
	"forge.lthn.ai/core/go-process"
	store "forge.lthn.ai/core/go-store"
	"github.com/spf13/cobra"
)

func newSyncCmd(dataDir, seed *string, testnet *bool) *cobra.Command {
	var (
		daemon bool
		stop   bool
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Headless P2P chain sync",
		Long:  "Sync the blockchain from P2P peers without the TUI explorer.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if stop {
				return stopSyncDaemon(*dataDir)
			}
			if daemon {
				return runSyncDaemon(*dataDir, *seed, *testnet)
			}
			return runSyncForeground(*dataDir, *seed, *testnet)
		},
	}

	cmd.Flags().BoolVar(&daemon, "daemon", false, "run as background daemon")
	cmd.Flags().BoolVar(&stop, "stop", false, "stop a running sync daemon")

	return cmd
}

func runSyncForeground(dataDir, seed string, testnet bool) error {
	if err := ensureDataDir(dataDir); err != nil {
		return err
	}

	dbPath := filepath.Join(dataDir, "chain.db")
	s, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer s.Close()

	c := chain.New(s)
	cfg, forks := resolveConfig(testnet, &seed)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Println("Starting headless P2P sync...")
	syncLoop(ctx, c, &cfg, forks, seed)
	log.Println("Sync stopped.")
	return nil
}

func runSyncDaemon(dataDir, seed string, testnet bool) error {
	if err := ensureDataDir(dataDir); err != nil {
		return err
	}

	pidFile := filepath.Join(dataDir, "sync.pid")

	d := process.NewDaemon(process.DaemonOptions{
		PIDFile:  pidFile,
		Registry: process.DefaultRegistry(),
		RegistryEntry: process.DaemonEntry{
			Code:   "forge.lthn.ai/core/go-blockchain",
			Daemon: "sync",
		},
	})

	if err := d.Start(); err != nil {
		return fmt.Errorf("daemon start: %w", err)
	}

	dbPath := filepath.Join(dataDir, "chain.db")
	s, err := store.New(dbPath)
	if err != nil {
		_ = d.Stop()
		return fmt.Errorf("open store: %w", err)
	}
	defer s.Close()

	c := chain.New(s)
	cfg, forks := resolveConfig(testnet, &seed)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	d.SetReady(true)
	log.Println("Sync daemon started.")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		syncLoop(ctx, c, &cfg, forks, seed)
	}()

	err = d.Run(ctx)
	wg.Wait() // Wait for syncLoop to finish before closing store.
	return err
}

func stopSyncDaemon(dataDir string) error {
	pidFile := filepath.Join(dataDir, "sync.pid")
	pid, running := process.ReadPID(pidFile)
	if pid == 0 || !running {
		return fmt.Errorf("no running sync daemon found")
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("signal process %d: %w", pid, err)
	}

	log.Printf("Sent SIGTERM to sync daemon (PID %d)", pid)
	return nil
}
