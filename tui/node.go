// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"dappco.re/go/core/blockchain/chain"
)

// Node wraps a [chain.Chain] and provides bubbletea commands for periodic
// status polling. It does NOT own the sync goroutine — that is managed by
// cmd/chain/main.go.
type Node struct {
	chain    *chain.Chain
	interval time.Duration
}

// NewNode creates a Node backed by the given chain with a default polling
// interval of 2 seconds.
func NewNode(c *chain.Chain) *Node {
	return &Node{chain: c, interval: 2 * time.Second}
}

// Chain returns the underlying chain instance.
func (n *Node) Chain() *chain.Chain { return n.chain }

// Status reads the current chain state and returns a snapshot.
func (n *Node) Status() (NodeStatusMsg, error) {
	height, err := n.chain.Height()
	if err != nil {
		return NodeStatusMsg{}, err
	}
	if height == 0 {
		return NodeStatusMsg{Height: 0}, nil
	}
	_, meta, err := n.chain.TopBlock()
	if err != nil {
		return NodeStatusMsg{}, err
	}
	return NodeStatusMsg{
		Height:     height,
		TopHash:    meta.Hash,
		Difficulty: meta.Difficulty,
		TipTime:    time.Unix(int64(meta.Timestamp), 0),
	}, nil
}

// WaitForStatus returns a tea.Cmd that reads the current chain status
// immediately (no sleep). Use this for Init().
func (n *Node) WaitForStatus() tea.Cmd {
	return func() tea.Msg {
		status, _ := n.Status()
		return status
	}
}

// Tick returns a tea.Cmd that sleeps for the polling interval, then
// reads the current chain status.
func (n *Node) Tick() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(n.interval)
		status, _ := n.Status()
		return status
	}
}
