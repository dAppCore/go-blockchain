// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"time"

	"dappco.re/go/core/blockchain/types"
)

// NodeStatusMsg carries a periodic status snapshot from the Node goroutine
// into the bubbletea update loop.
// Usage: var value tui.NodeStatusMsg
type NodeStatusMsg struct {
	Height     uint64
	TopHash    types.Hash
	Difficulty uint64
	PeerCount  int
	SyncPct    float64
	TipTime    time.Time
}

// ViewChangedMsg tells the footer to update its key hints.
// Usage: var value tui.ViewChangedMsg
type ViewChangedMsg struct {
	Hints []string
}
