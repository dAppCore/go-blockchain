// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2
package blockchain

import (
	"context"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"

	"dappco.re/go/core/blockchain/chain"
	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/daemon"
	"dappco.re/go/core/blockchain/rpc"
	store "dappco.re/go/core/store"
	"github.com/spf13/cobra"
)

func newServeCmd(dataDir, seed *string, testnet *bool) *cobra.Command {
	var (
		rpcPort string
		rpcBind string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Sync chain and serve JSON-RPC",
		Long:  "Sync the blockchain from a seed node via RPC and serve a JSON-RPC API.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(*dataDir, *seed, *testnet, rpcBind, rpcPort)
		},
	}

	cmd.Flags().StringVar(&rpcPort, "rpc-port", "47941", "JSON-RPC port")
	cmd.Flags().StringVar(&rpcBind, "rpc-bind", "127.0.0.1", "JSON-RPC bind address")

	return cmd
}

func runServe(dataDir, seed string, testnet bool, rpcBind, rpcPort string) error {
	if err := ensureDataDir(dataDir); err != nil {
		return err
	}

	dbPath := core.JoinPath(dataDir, "chain.db")
	s, err := store.New(dbPath)
	if err != nil {
		return coreerr.E("runServe", "open store", err)
	}
	defer s.Close()

	c := chain.New(s)
	cfg, forks := resolveConfig(testnet, &seed)

	// Set genesis hash for testnet.
	if testnet {
		chain.GenesisHash = "7cf844dc3e7d8dd6af65642c68164ebe18109aa5167b5f76043f310dd6e142d0"
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Start RPC sync in background.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		rpcSyncLoop(ctx, c, &cfg, forks, seed)
	}()

	// Start JSON-RPC server.
	srv := daemon.NewServer(c, &cfg)
	addr := rpcBind + ":" + rpcPort
	core.Print(nil, "Go daemon RPC on %s (syncing from %s)", addr, seed)

	httpSrv := &http.Server{Addr: addr, Handler: srv}
	go func() {
		<-ctx.Done()
		httpSrv.Close()
	}()

	err = httpSrv.ListenAndServe()
	if err == http.ErrServerClosed {
		err = nil
	}
	cancel()
	wg.Wait()
	return err
}

// rpcSyncLoop syncs from a remote daemon via JSON-RPC (not P2P).
func rpcSyncLoop(ctx context.Context, c *chain.Chain, cfg *config.ChainConfig, forks []config.HardFork, seed string) {
	opts := chain.SyncOptions{
		VerifySignatures: false,
		Forks:            forks,
	}

	// Derive RPC URL from seed address (replace P2P port with RPC port).
	rpcURL := core.Sprintf("http://%s", seed)
	// If seed has P2P port, swap to RPC.
	if core.Contains(seed, ":46942") {
		rpcURL = "http://127.0.0.1:46941"
	} else if core.Contains(seed, ":36942") {
		rpcURL = "http://127.0.0.1:36941"
	}

	client := rpc.NewClient(rpcURL)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := c.Sync(ctx, client, opts); err != nil {
			if ctx.Err() != nil {
				return
			}
			core.Print(nil, "rpc sync: %v (retrying in 10s)", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
}
