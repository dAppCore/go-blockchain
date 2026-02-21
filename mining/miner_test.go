// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package mining

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewMiner_Good(t *testing.T) {
	cfg := Config{
		DaemonURL:    "http://localhost:46941",
		WalletAddr:   "iTHNtestaddr",
		PollInterval: 5 * time.Second,
	}
	m := NewMiner(cfg)

	assert.NotNil(t, m)
	stats := m.Stats()
	assert.Equal(t, float64(0), stats.Hashrate)
	assert.Equal(t, uint64(0), stats.BlocksFound)
	assert.Equal(t, uint64(0), stats.Height)
	assert.Equal(t, uint64(0), stats.Difficulty)
	assert.Equal(t, time.Duration(0), stats.Uptime)
}

func TestNewMiner_Good_DefaultPollInterval(t *testing.T) {
	cfg := Config{
		DaemonURL:  "http://localhost:46941",
		WalletAddr: "iTHNtestaddr",
	}
	m := NewMiner(cfg)

	// PollInterval should default to 3s.
	assert.Equal(t, 3*time.Second, m.cfg.PollInterval)
}
