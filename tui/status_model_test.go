// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"strings"
	"testing"
	"time"
)

func TestStatusModel_View_Good_Initial(t *testing.T) {
	c := seedChain(t, 0)
	n := NewNode(c)
	m := NewStatusModel(n)

	got := m.View(80, 1)
	if !strings.Contains(got, "syncing") && !strings.Contains(got, "0") {
		t.Errorf("initial View should contain \"syncing\" or \"0\", got %q", got)
	}
}

func TestStatusModel_Update_Good_StatusMsg(t *testing.T) {
	c := seedChain(t, 5)
	n := NewNode(c)
	n.interval = 10 * time.Millisecond // keep test fast
	m := NewStatusModel(n)

	msg := NodeStatusMsg{
		Height:     100,
		Difficulty: 1500000,
		PeerCount:  4,
		SyncPct:    75.5,
		TipTime:    time.Now().Add(-30 * time.Second),
	}

	updated, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("Update(NodeStatusMsg) should return a non-nil cmd")
	}
	if updated == nil {
		t.Fatal("Update(NodeStatusMsg) should return a non-nil model")
	}

	sm, ok := updated.(*StatusModel)
	if !ok {
		t.Fatalf("Update returned %T, want *StatusModel", updated)
	}

	got := sm.View(120, 1)
	if !strings.Contains(got, "100") {
		t.Errorf("View after status update should contain height \"100\", got %q", got)
	}
	if !strings.Contains(got, "75.5") {
		t.Errorf("View after status update should contain sync pct \"75.5\", got %q", got)
	}
}

func TestStatusModel_View_Good_FullSync(t *testing.T) {
	c := seedChain(t, 1)
	n := NewNode(c)
	m := NewStatusModel(n)

	msg := NodeStatusMsg{
		Height:  6312,
		SyncPct: 100.0,
	}
	m.Update(msg)

	got := m.View(120, 1)
	if !strings.Contains(got, "6312") {
		t.Errorf("View should contain height \"6312\", got %q", got)
	}
}

func TestStatusModel_Init_Good(t *testing.T) {
	c := seedChain(t, 3)
	n := NewNode(c)
	m := NewStatusModel(n)

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() should return a non-nil cmd")
	}
}
