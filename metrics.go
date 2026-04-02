// Copyright (c) 2017-2026 Lethean (https://lt.hn)
// SPDX-License-Identifier: EUPL-1.2

package blockchain

import (
	"sync/atomic"

	"dappco.re/go/core/blockchain/chain"
)

// Metrics tracks blockchain operational metrics.
//
//	m := blockchain.NewMetrics(chain)
//	m.RecordBlock(height)
type Metrics struct {
	chain          *chain.Chain
	blocksProcessed atomic.Uint64
	aliasesFound    atomic.Uint64
	syncErrors      atomic.Uint64
	lastBlockTime   atomic.Uint64
}

// NewMetrics creates a metrics collector.
//
//	m := blockchain.NewMetrics(ch)
func NewMetrics(ch *chain.Chain) *Metrics {
	return &Metrics{chain: ch}
}

// RecordBlock increments the block counter.
func (m *Metrics) RecordBlock() {
	m.blocksProcessed.Add(1)
}

// RecordAlias increments the alias counter.
func (m *Metrics) RecordAlias() {
	m.aliasesFound.Add(1)
}

// RecordSyncError increments the error counter.
func (m *Metrics) RecordSyncError() {
	m.syncErrors.Add(1)
}

// SetLastBlockTime records the most recent block timestamp.
func (m *Metrics) SetLastBlockTime(ts uint64) {
	m.lastBlockTime.Store(ts)
}

// Snapshot returns current metric values.
//
//	snap := m.Snapshot()
func (m *Metrics) Snapshot() map[string]uint64 {
	h, _ := m.chain.Height()
	return map[string]uint64{
		"height":           h,
		"blocks_processed": m.blocksProcessed.Load(),
		"aliases_found":    m.aliasesFound.Load(),
		"sync_errors":      m.syncErrors.Load(),
		"last_block_time":  m.lastBlockTime.Load(),
	}
}
