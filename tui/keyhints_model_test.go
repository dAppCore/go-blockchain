// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"testing"

	"dappco.re/go/core"
)

func TestKeyhintsModel_KeyHintsModel_View_Default_Good(t *testing.T) {
	m := NewKeyHintsModel()

	got := m.View(80, 1)
	if !core.Contains(got, "quit") {
		t.Errorf("default View should contain \"quit\", got %q", got)
	}
}

func TestKeyhintsModel_KeyHintsModel_Update_ViewChanged_Good(t *testing.T) {
	m := NewKeyHintsModel()

	updated, cmd := m.Update(ViewChangedMsg{Hints: []string{"esc back", "enter view"}})
	if cmd != nil {
		t.Errorf("Update(ViewChangedMsg) should return nil cmd, got %v", cmd)
	}

	km, ok := updated.(*KeyHintsModel)
	if !ok {
		t.Fatalf("Update returned %T, want *KeyHintsModel", updated)
	}

	got := km.View(80, 1)
	if !core.Contains(got, "esc back") {
		t.Errorf("View after ViewChangedMsg should contain \"esc back\", got %q", got)
	}
	if !core.Contains(got, "enter view") {
		t.Errorf("View after ViewChangedMsg should contain \"enter view\", got %q", got)
	}
}

func TestKeyhintsModel_KeyHintsModel_Init_Good(t *testing.T) {
	m := NewKeyHintsModel()

	cmd := m.Init()
	if cmd != nil {
		t.Errorf("Init() should return nil, got %v", cmd)
	}
}
