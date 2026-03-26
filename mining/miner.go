// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package mining

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"sync/atomic"
	"time"

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"

	"dappco.re/go/core/blockchain/consensus"
	"dappco.re/go/core/blockchain/crypto"
	"dappco.re/go/core/blockchain/rpc"
	"dappco.re/go/core/blockchain/types"
	"dappco.re/go/core/blockchain/wire"
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

// Start runs the mining loop. It blocks until ctx is cancelled.
// Returns the context error (typically context.Canceled or context.DeadlineExceeded).
func (m *Miner) Start(ctx context.Context) error {
	m.startTime = time.Now()

	for {
		// Fetch a block template.
		tmpl, err := m.provider.GetBlockTemplate(m.cfg.WalletAddr)
		if err != nil {
			// Transient RPC error — wait and retry.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(m.cfg.PollInterval):
				continue
			}
		}

		// Parse difficulty.
		diff, err := strconv.ParseUint(tmpl.Difficulty, 10, 64)
		if err != nil {
			return coreerr.E("Miner.Start", core.Sprintf("mining: invalid difficulty %q", tmpl.Difficulty), err)
		}

		// Decode the block template blob.
		blobBytes, err := hex.DecodeString(tmpl.BlockTemplateBlob)
		if err != nil {
			return coreerr.E("Miner.Start", "mining: invalid template blob hex", err)
		}
		dec := wire.NewDecoder(bytes.NewReader(blobBytes))
		block := wire.DecodeBlock(dec)
		if dec.Err() != nil {
			return coreerr.E("Miner.Start", "mining: decode template", dec.Err())
		}

		// Update stats.
		m.height.Store(tmpl.Height)
		m.difficulty.Store(diff)

		if m.cfg.OnNewTemplate != nil {
			m.cfg.OnNewTemplate(tmpl.Height, diff)
		}

		// Compute the header mining hash (once per template).
		headerHash := HeaderMiningHash(&block)

		// Mine until solution found or template becomes stale.
		if err := m.mine(ctx, &block, headerHash, diff); err != nil {
			return err
		}
	}
}

// mine grinds nonces against the given header hash and difficulty.
// Returns nil when a new template should be fetched (new block detected).
// Returns ctx.Err() when shutdown is requested.
func (m *Miner) mine(ctx context.Context, block *types.Block, headerHash [32]byte, difficulty uint64) error {
	pollTicker := time.NewTicker(m.cfg.PollInterval)
	defer pollTicker.Stop()

	currentHeight := m.height.Load()

	for nonce := uint64(0); ; nonce++ {
		// Check for shutdown or poll trigger.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pollTicker.C:
			// Check if chain has advanced.
			info, err := m.provider.GetInfo()
			if err == nil && info.Height > currentHeight {
				return nil // fetch new template
			}
			continue
		default:
		}

		// Compute RandomX hash.
		var input [40]byte
		copy(input[:32], headerHash[:])
		binary.LittleEndian.PutUint64(input[32:], nonce)

		powHash, err := crypto.RandomXHash(RandomXKey, input[:])
		if err != nil {
			return coreerr.E("Miner.mine", "mining: RandomX hash", err)
		}

		m.hashCount.Add(1)

		if consensus.CheckDifficulty(types.Hash(powHash), difficulty) {
			// Solution found!
			block.Nonce = nonce

			var buf bytes.Buffer
			enc := wire.NewEncoder(&buf)
			wire.EncodeBlock(enc, block)
			if enc.Err() != nil {
				return coreerr.E("Miner.mine", "mining: encode solution", enc.Err())
			}

			hexBlob := hex.EncodeToString(buf.Bytes())
			if err := m.provider.SubmitBlock(hexBlob); err != nil {
				return coreerr.E("Miner.mine", "mining: submit block", err)
			}

			m.blocksFound.Add(1)

			if m.cfg.OnBlockFound != nil {
				m.cfg.OnBlockFound(currentHeight, wire.BlockHash(block))
			}

			return nil // fetch new template
		}
	}
}
