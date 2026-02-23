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

func TestExplorerModel_Update_Good_EnterBlockDetail(t *testing.T) {
	c := seedChain(t, 5)
	m := NewExplorerModel(c)

	// Cursor starts at 0 which is the newest block (height 4).
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.view != viewBlockDetail {
		t.Errorf("view: got %d, want viewBlockDetail (%d)", m.view, viewBlockDetail)
	}

	out := m.View(80, 20)
	if !strings.Contains(out, "Block 4") {
		t.Errorf("block detail should contain 'Block 4', got:\n%s", out)
	}
}

func TestExplorerModel_Update_Good_EscBackToList(t *testing.T) {
	c := seedChain(t, 5)
	m := NewExplorerModel(c)

	// Enter block detail then press Esc to go back.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.view != viewBlockDetail {
		t.Fatalf("expected viewBlockDetail after Enter, got %d", m.view)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if m.view != viewBlockList {
		t.Errorf("view: got %d, want viewBlockList (%d)", m.view, viewBlockList)
	}
}

func TestExplorerModel_Update_Good_PgDown(t *testing.T) {
	c := seedChain(t, 50)
	m := NewExplorerModel(c)
	m.height = 20

	m.Update(tea.KeyMsg{Type: tea.KeyPgDown})

	if m.cursor <= 10 {
		t.Errorf("cursor after PgDown: got %d, want > 10", m.cursor)
	}
}

func TestExplorerModel_Update_Good_Home(t *testing.T) {
	c := seedChain(t, 50)
	m := NewExplorerModel(c)

	// Move cursor down 10 times.
	for i := 0; i < 10; i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.cursor != 10 {
		t.Fatalf("cursor after 10 downs: got %d, want 10", m.cursor)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyHome})

	if m.cursor != 0 {
		t.Errorf("cursor after Home: got %d, want 0", m.cursor)
	}
}

func TestExplorerModel_ViewBlockDetail_Good_CoinbaseOnly(t *testing.T) {
	c := seedChain(t, 3)
	m := NewExplorerModel(c)

	// Enter block detail — test blocks have no TxHashes.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	out := m.View(80, 20)
	if !strings.Contains(out, "coinbase only") {
		t.Errorf("block detail should contain 'coinbase only' for blocks with no TxHashes, got:\n%s", out)
	}
}
