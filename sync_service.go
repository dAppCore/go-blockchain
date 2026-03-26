// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package blockchain

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"log"
	"net"
	"time"

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"

	"dappco.re/go/core/blockchain/chain"
	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/p2p"
	levin "dappco.re/go/core/p2p/node/levin"
)

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
		return coreerr.E("syncOnce", core.Sprintf("dial %s", seed), err)
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
		return coreerr.E("syncOnce", "encode handshake", err)
	}
	if err := lc.WritePacket(p2p.CommandHandshake, payload, true); err != nil {
		return coreerr.E("syncOnce", "write handshake", err)
	}

	hdr, data, err := lc.ReadPacket()
	if err != nil {
		return coreerr.E("syncOnce", "read handshake", err)
	}
	if hdr.Command != uint32(p2p.CommandHandshake) {
		return coreerr.E("syncOnce", core.Sprintf("unexpected command %d", hdr.Command), nil)
	}

	var resp p2p.HandshakeResponse
	if err := resp.Decode(data); err != nil {
		return coreerr.E("syncOnce", "decode handshake", err)
	}

	localSync := p2p.CoreSyncData{
		CurrentHeight:  localHeight,
		ClientVersion:  config.ClientVersion,
		NonPruningMode: true,
	}
	p2pConn := chain.NewLevinP2PConn(lc, resp.PayloadData.CurrentHeight, localSync)

	return c.P2PSync(ctx, p2pConn, opts)
}
