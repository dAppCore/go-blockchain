// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	cli "dappco.re/go/core/cli/pkg/cli"
)

// Compile-time check: StatusModel implements cli.FrameModel.
var _ cli.FrameModel = (*StatusModel)(nil)

// StatusModel displays chain sync status in the header region.
type StatusModel struct {
	node   *Node
	status NodeStatusMsg
}

// NewStatusModel creates a StatusModel backed by the given Node.
func NewStatusModel(n *Node) *StatusModel {
	return &StatusModel{node: n}
}

// Init returns a command that reads the current chain status immediately.
func (m *StatusModel) Init() tea.Cmd {
	return m.node.WaitForStatus()
}

// Update handles incoming messages. On NodeStatusMsg it stores the snapshot
// and schedules the next tick; all other messages are ignored.
func (m *StatusModel) Update(msg tea.Msg) (cli.FrameModel, tea.Cmd) {
	switch msg := msg.(type) {
	case NodeStatusMsg:
		m.status = msg
		return m, m.node.Tick()
	default:
		return m, nil
	}
}

// View renders a single-line status bar. When height is zero the model has
// not yet received a status snapshot, so it shows a placeholder.
func (m *StatusModel) View(width, height int) string {
	var line string
	if height == 0 {
		line = " height 0 | syncing..."
	} else {
		s := m.status
		line = fmt.Sprintf(" height %d | sync %.1f%% | diff %s | %d peers | tip %s",
			s.Height, s.SyncPct, formatDifficulty(s.Difficulty), s.PeerCount, formatAge(s.TipTime))
	}
	if len(line) > width && width > 0 {
		line = line[:width]
	}
	return line
}

// formatAge returns a human-readable duration since t.
func formatAge(t time.Time) string {
	if t.IsZero() {
		return "\u2014"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// formatDifficulty returns a compact difficulty string with SI suffix.
func formatDifficulty(d uint64) string {
	switch {
	case d >= 1_000_000_000:
		return fmt.Sprintf("%.1fG", float64(d)/1_000_000_000)
	case d >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(d)/1_000_000)
	case d >= 1_000:
		return fmt.Sprintf("%.1fK", float64(d)/1_000)
	default:
		return fmt.Sprintf("%d", d)
	}
}
