// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package mining

import (
	"sync/atomic"
	"time"

	"forge.lthn.ai/core/go-blockchain/rpc"
	"forge.lthn.ai/core/go-blockchain/types"
)

// TemplateProvider abstracts the RPC methods needed by the miner.
// The real rpc.Client satisfies this interface.
type TemplateProvider interface {
	GetBlockTemplate(walletAddr string) (*rpc.BlockTemplateResponse, error)
	SubmitBlock(hexBlob string) error
	GetInfo() (*rpc.DaemonInfo, error)
}

// Config holds the miner configuration.
type Config struct {
	// DaemonURL is the JSON-RPC endpoint of the C++ daemon.
	DaemonURL string

	// WalletAddr is the address that receives mining rewards.
	WalletAddr string

	// PollInterval is how often to check for new blocks. Default: 3s.
	PollInterval time.Duration

	// OnBlockFound is called after a solution is successfully submitted.
	// May be nil.
	OnBlockFound func(height uint64, hash types.Hash)

	// OnNewTemplate is called when a new block template is fetched.
	// May be nil.
	OnNewTemplate func(height uint64, difficulty uint64)

	// Provider is the RPC provider. If nil, a default rpc.Client is
	// created from DaemonURL.
	Provider TemplateProvider
}

// Stats holds read-only mining statistics.
type Stats struct {
	Hashrate    float64
	BlocksFound uint64
	Height      uint64
	Difficulty  uint64
	Uptime      time.Duration
}

// Miner is a solo PoW miner that talks to a C++ daemon via JSON-RPC.
type Miner struct {
	cfg       Config
	provider  TemplateProvider
	startTime time.Time

	// Atomic stats — accessed from Stats() on any goroutine.
	hashCount   atomic.Uint64
	blocksFound atomic.Uint64
	height      atomic.Uint64
	difficulty  atomic.Uint64
}

// NewMiner creates a new miner with the given configuration.
func NewMiner(cfg Config) *Miner {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 3 * time.Second
	}

	var provider TemplateProvider
	if cfg.Provider != nil {
		provider = cfg.Provider
	} else {
		provider = rpc.NewClient(cfg.DaemonURL)
	}

	return &Miner{
		cfg:      cfg,
		provider: provider,
	}
}

// Stats returns a snapshot of the current mining statistics.
// Safe to call from any goroutine.
func (m *Miner) Stats() Stats {
	var uptime time.Duration
	if !m.startTime.IsZero() {
		uptime = time.Since(m.startTime)
	}

	hashes := m.hashCount.Load()
	var hashrate float64
	if uptime > 0 {
		hashrate = float64(hashes) / uptime.Seconds()
	}

	return Stats{
		Hashrate:    hashrate,
		BlocksFound: m.blocksFound.Load(),
		Height:      m.height.Load(),
		Difficulty:  m.difficulty.Load(),
		Uptime:      uptime,
	}
}
