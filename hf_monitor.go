// Copyright (c) 2017-2026 Lethean (https://lt.hn)
// SPDX-License-Identifier: EUPL-1.2

package blockchain

import (
	"context"
	"time"

	"dappco.re/go/core"

	"dappco.re/go/core/blockchain/chain"
	"dappco.re/go/core/blockchain/config"
)

// HardforkMonitor watches for hardfork activations and fires callbacks.
//
//	monitor := blockchain.NewHardforkMonitor(chain, config.TestnetForks)
//	monitor.OnActivation = func(version int, height uint64) { ... }
//	monitor.Start(ctx)
type HardforkMonitor struct {
	chain        *chain.Chain
	forks        []config.HardFork
	OnActivation func(version int, height uint64)
	activated    map[int]bool
}

// NewHardforkMonitor creates a monitor that watches for HF activations.
//
//	monitor := blockchain.NewHardforkMonitor(ch, config.TestnetForks)
func NewHardforkMonitor(ch *chain.Chain, forks []config.HardFork) *HardforkMonitor {
	return &HardforkMonitor{
		chain:     ch,
		forks:     forks,
		activated: make(map[int]bool),
	}
}

// Start begins monitoring in the background. Checks every 30 seconds.
//
//	go monitor.Start(ctx)
func (m *HardforkMonitor) Start(ctx context.Context) {
	// Mark already-active forks
	height, _ := m.chain.Height()
	for _, f := range m.forks {
		if height >= f.Height {
			m.activated[int(f.Version)] = true
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}

		height, _ := m.chain.Height()
		for _, f := range m.forks {
			if height >= f.Height && !m.activated[int(f.Version)] {
				m.activated[int(f.Version)] = true
				core.Print(nil, "HARDFORK %d ACTIVATED at height %d", f.Version, height)
				if m.OnActivation != nil {
					m.OnActivation(int(f.Version), height)
				}
			}
		}
	}
}

// RemainingBlocks returns blocks until next inactive hardfork.
//
//	blocks, version := monitor.RemainingBlocks()
func (m *HardforkMonitor) RemainingBlocks() (uint64, int) {
	height, _ := m.chain.Height()
	for _, f := range m.forks {
		if height < f.Height {
			return f.Height - height, int(f.Version)
		}
	}
	return 0, -1
}
