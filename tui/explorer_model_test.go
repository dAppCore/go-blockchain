// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestExplorerModel_View_Good_BlockList(t *testing.T) {
	c := seedChain(t, 5)
	m := NewExplorerModel(c)

	out := m.View(80, 20)
	// Should show block heights.
	if !strings.Contains(out, "4") {
		t.Errorf("view should contain top block height 4, got:\n%s", out)
	}
	if !strings.Contains(out, "0") {
		t.Errorf("view should contain genesis height 0, got:\n%s", out)
	}
}

func TestExplorerModel_View_Good_Empty(t *testing.T) {
	c := seedChain(t, 0)
	m := NewExplorerModel(c)

	out := m.View(80, 20)
	if !strings.Contains(out, "no blocks") && !strings.Contains(out, "empty") {
		t.Errorf("empty chain should show empty message, got:\n%s", out)
	}
}

func TestExplorerModel_Update_Good_CursorDown(t *testing.T) {
	c := seedChain(t, 10)
	m := NewExplorerModel(c)

	// Move cursor down.
	m.Update(tea.KeyMsg{Type: tea.KeyDown})

	if m.cursor != 1 {
		t.Errorf("cursor: got %d, want 1", m.cursor)
	}
}

func TestExplorerModel_Update_Good_CursorUp(t *testing.T) {
	c := seedChain(t, 10)
	m := NewExplorerModel(c)

	// Move down then up.
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.KeyMsg{Type: tea.KeyUp})

	if m.cursor != 1 {
		t.Errorf("cursor: got %d, want 1", m.cursor)
	}
}

func TestExplorerModel_Update_Good_CursorBoundsTop(t *testing.T) {
	c := seedChain(t, 5)
	m := NewExplorerModel(c)

	// Try to go above 0.
	m.Update(tea.KeyMsg{Type: tea.KeyUp})

	if m.cursor != 0 {
		t.Errorf("cursor should not go below 0, got %d", m.cursor)
	}
}

func TestExplorerModel_Init_Good(t *testing.T) {
	c := seedChain(t, 5)
	m := NewExplorerModel(c)

	cmd := m.Init()
	if cmd != nil {
		t.Error("Init should return nil (block list loads synchronously)")
	}
}

func TestExplorerModel_Update_Good_Refresh(t *testing.T) {
	c := seedChain(t, 5)
	m := NewExplorerModel(c)

	// A NodeStatusMsg should trigger a refresh of the block list.
	m.Update(NodeStatusMsg{Height: 5})

	out := m.View(80, 20)
	if !strings.Contains(out, "4") {
		t.Errorf("view should show height 4 after refresh, got:\n%s", out)
	}
}
