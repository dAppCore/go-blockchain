// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package mining

import (
	"bytes"
	"context"
	"encoding/hex"
	"sync/atomic"
	"testing"
	"time"

	"forge.lthn.ai/core/go-blockchain/rpc"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
	"github.com/stretchr/testify/assert"
)

type mockProvider struct {
	templates     []*rpc.BlockTemplateResponse
	infos         []*rpc.DaemonInfo
	templateCalls atomic.Int64
	infoCalls     atomic.Int64
	submitCalls   atomic.Int64
	submitted     []string
}

func (m *mockProvider) GetBlockTemplate(walletAddr string) (*rpc.BlockTemplateResponse, error) {
	idx := int(m.templateCalls.Add(1) - 1)
	if idx >= len(m.templates) {
		idx = len(m.templates) - 1
	}
	return m.templates[idx], nil
}

func (m *mockProvider) SubmitBlock(hexBlob string) error {
	m.submitCalls.Add(1)
	m.submitted = append(m.submitted, hexBlob)
	return nil
}

func (m *mockProvider) GetInfo() (*rpc.DaemonInfo, error) {
	idx := int(m.infoCalls.Add(1) - 1)
	if idx >= len(m.infos) {
		idx = len(m.infos) - 1
	}
	return m.infos[idx], nil
}

// minimalBlockBlob returns a serialised block that can be decoded by wire.DecodeBlock.
func minimalBlockBlob(t *testing.T) []byte {
	t.Helper()
	block := types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Nonce:        0,
			Timestamp:    1770897600,
		},
		MinerTx: types.Transaction{
			Version:    1,
			Vin:        []types.TxInput{types.TxInputGenesis{Height: 100}},
			Vout: []types.TxOutput{types.TxOutputBare{
				Amount: 1000000000000,
				Target: types.TxOutToKey{},
			}},
			Extra:      []byte{0x00}, // varint 0 = empty variant vector
			Attachment: []byte{0x00}, // varint 0 = empty variant vector
		},
	}
	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	wire.EncodeBlock(enc, &block)
	if enc.Err() != nil {
		t.Fatalf("encode block: %v", enc.Err())
	}
	return buf.Bytes()
}

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

func TestMiner_Start_Good_ShutdownOnCancel(t *testing.T) {
	mock := &mockProvider{
		templates: []*rpc.BlockTemplateResponse{
			{
				Difficulty:        "1",
				Height:            100,
				BlockTemplateBlob: hex.EncodeToString(minimalBlockBlob(t)),
				Status:            "OK",
			},
		},
		infos: []*rpc.DaemonInfo{{Height: 100}},
	}

	cfg := Config{
		DaemonURL:    "http://localhost:46941",
		WalletAddr:   "iTHNtestaddr",
		PollInterval: 100 * time.Millisecond,
		Provider:     mock,
	}
	m := NewMiner(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := m.Start(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	stats := m.Stats()
	assert.Equal(t, uint64(100), stats.Height)
	assert.Equal(t, uint64(1), stats.Difficulty)
}

func TestMiner_Start_Good_TemplateRefresh(t *testing.T) {
	// First call returns height 100, second returns 101 — triggers refresh.
	mock := &mockProvider{
		templates: []*rpc.BlockTemplateResponse{
			{Difficulty: "1", Height: 100, BlockTemplateBlob: hex.EncodeToString(minimalBlockBlob(t)), Status: "OK"},
			{Difficulty: "2", Height: 101, BlockTemplateBlob: hex.EncodeToString(minimalBlockBlob(t)), Status: "OK"},
		},
		infos: []*rpc.DaemonInfo{
			{Height: 100},
			{Height: 101}, // triggers refresh
			{Height: 101},
		},
	}

	cfg := Config{
		DaemonURL:    "http://localhost:46941",
		WalletAddr:   "iTHNtestaddr",
		PollInterval: 50 * time.Millisecond,
		Provider:     mock,
	}
	m := NewMiner(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	_ = m.Start(ctx)

	assert.GreaterOrEqual(t, mock.templateCalls.Load(), int64(2))
}
