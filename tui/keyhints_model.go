// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	cli "forge.lthn.ai/core/cli/pkg/cli"
)

// Compile-time check: KeyHintsModel implements cli.FrameModel.
var _ cli.FrameModel = (*KeyHintsModel)(nil)

var defaultHints = []string{"↑/↓ select", "enter view", "q quit"}

// KeyHintsModel displays context-sensitive key hints in the footer region.
type KeyHintsModel struct {
	hints []string
}

// NewKeyHintsModel creates a KeyHintsModel with the default key hints.
func NewKeyHintsModel() *KeyHintsModel {
	return &KeyHintsModel{hints: defaultHints}
}

// Init returns nil; no initialisation command is needed.
func (m *KeyHintsModel) Init() tea.Cmd { return nil }

// Update handles incoming messages. On ViewChangedMsg it replaces the
// displayed hints; all other messages are ignored.
func (m *KeyHintsModel) Update(msg tea.Msg) (cli.FrameModel, tea.Cmd) {
	switch msg := msg.(type) {
	case ViewChangedMsg:
		m.hints = msg.Hints
	}
	return m, nil
}

// View renders a single-line hint bar separated by vertical bars.
// The output is truncated to width if it would overflow.
func (m *KeyHintsModel) View(width, height int) string {
	line := " " + strings.Join(m.hints, "  \u2502  ")
	if len(line) > width && width > 0 {
		line = line[:width]
	}
	return line
}
