// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	cli "forge.lthn.ai/core/cli/pkg/cli"
	store "forge.lthn.ai/core/go-store"

	"forge.lthn.ai/core/go-blockchain/chain"
	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-blockchain/p2p"
	"forge.lthn.ai/core/go-blockchain/tui"
	levin "forge.lthn.ai/core/go-p2p/node/levin"
)

func main() {
	dataDir := flag.String("data-dir", defaultDataDir(), "blockchain data directory")
	seed := flag.String("seed", "seeds.lthn.io:36942", "seed peer address (host:port)")
	testnet := flag.Bool("testnet", false, "use testnet")
	flag.Parse()

	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	dbPath := filepath.Join(*dataDir, "chain.db")
	s, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer s.Close()

	c := chain.New(s)
	node := tui.NewNode(c)

	cfg := config.Mainnet
	forks := config.MainnetForks
	if *testnet {
		cfg = config.Testnet
		forks = config.TestnetForks
		if *seed == "seeds.lthn.io:36942" {
			*seed = "localhost:46942"
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Start P2P sync in background.
	go syncLoop(ctx, c, &cfg, forks, *seed)

	status := tui.NewStatusModel(node)
	explorer := tui.NewExplorerModel(c)
	hints := tui.NewKeyHintsModel()

	frame := cli.NewFrame("HCF")
	frame.Header(status)
	frame.Content(explorer)
	frame.Footer(hints)
	frame.Run()
}

func syncLoop(ctx context.Context, c *chain.Chain, cfg *config.ChainConfig, forks []config.HardFork, seed string) {
	opts := chain.SyncOptions{
		VerifySignatures: false,
		Forks:            forks,
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := syncOnce(ctx, c, cfg, opts, seed); err != nil {
			log.Printf("sync: %v (retrying in 10s)", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
			continue
		}

		// Synced -- wait before polling again.
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}
	}
}

func syncOnce(ctx context.Context, c *chain.Chain, cfg *config.ChainConfig, opts chain.SyncOptions, seed string) error {
	conn, err := net.DialTimeout("tcp", seed, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial %s: %w", seed, err)
	}
	defer conn.Close()

	lc := levin.NewConnection(conn)

	var peerIDBuf [8]byte
	rand.Read(peerIDBuf[:])
	peerID := binary.LittleEndian.Uint64(peerIDBuf[:])

	localHeight, _ := c.Height()

	req := p2p.HandshakeRequest{
		NodeData: p2p.NodeData{
			NetworkID: cfg.NetworkID,
			PeerID:    peerID,
			LocalTime: time.Now().Unix(),
			MyPort:    0,
		},
		PayloadData: p2p.CoreSyncData{
			CurrentHeight:  localHeight,
			ClientVersion:  config.ClientVersion,
			NonPruningMode: true,
		},
	}
	payload, err := p2p.EncodeHandshakeRequest(&req)
	if err != nil {
		return fmt.Errorf("encode handshake: %w", err)
	}
	if err := lc.WritePacket(p2p.CommandHandshake, payload, true); err != nil {
		return fmt.Errorf("write handshake: %w", err)
	}

	hdr, data, err := lc.ReadPacket()
	if err != nil {
		return fmt.Errorf("read handshake: %w", err)
	}
	if hdr.Command != uint32(p2p.CommandHandshake) {
		return fmt.Errorf("unexpected command %d", hdr.Command)
	}

	var resp p2p.HandshakeResponse
	if err := resp.Decode(data); err != nil {
		return fmt.Errorf("decode handshake: %w", err)
	}

	localSync := p2p.CoreSyncData{
		CurrentHeight:  localHeight,
		ClientVersion:  config.ClientVersion,
		NonPruningMode: true,
	}
	p2pConn := chain.NewLevinP2PConn(lc, resp.PayloadData.CurrentHeight, localSync)

	return c.P2PSync(ctx, p2pConn, opts)
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".lethean"
	}
	return filepath.Join(home, ".lethean", "chain")
}
