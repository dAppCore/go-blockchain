// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package blockchain

import (
	"context"
	"sync/atomic"

	"dappco.re/go/core"
)

// SyncTask tracks background sync progress for PerformAsync pattern.
//
//	task := NewSyncTask()
//	go task.Run(ctx, chain, client)
//	progress := task.Progress() // 0.0 — 1.0
type SyncTask struct {
	localHeight  atomic.Uint64
	remoteHeight atomic.Uint64
	running      atomic.Bool
	done         atomic.Bool
}

// NewSyncTask creates a sync progress tracker.
//
//	task := blockchain.NewSyncTask()
func NewSyncTask() *SyncTask {
	return &SyncTask{}
}

// Progress returns sync progress as a float64 (0.0 to 1.0).
//
//	p := task.Progress() // 0.85 = 85% synced
func (t *SyncTask) Progress() float64 {
	remote := t.remoteHeight.Load()
	if remote == 0 {
		return 0
	}
	local := t.localHeight.Load()
	return float64(local) / float64(remote)
}

// UpdateHeight sets the current sync heights for progress tracking.
//
//	task.UpdateHeight(localHeight, remoteHeight)
func (t *SyncTask) UpdateHeight(local, remote uint64) {
	t.localHeight.Store(local)
	t.remoteHeight.Store(remote)
}

// IsRunning returns whether the sync is active.
//
//	if task.IsRunning() { /* syncing */ }
func (t *SyncTask) IsRunning() bool { return t.running.Load() }

// IsDone returns whether the sync completed successfully.
//
//	if task.IsDone() { /* fully synced */ }
func (t *SyncTask) IsDone() bool { return t.done.Load() }

// --- Entitlement stubs ---

// CheckEntitlement verifies an action is allowed.
// Default: everything allowed (beta.1). Gating comes with c.Entitlement.
//
//	if !blockchain.CheckEntitlement(c, "blockchain.getinfo") { return }
func CheckEntitlement(c *core.Core, action string) bool {
	if c == nil {
		return true // no Core instance = standalone mode, allow all
	}
	e := c.Entitled(action)
	return e.Allowed
}

// EntitlementMiddleware wraps an action handler with entitlement check.
//
//	c.Action("blockchain.getinfo", blockchain.EntitlementMiddleware(c, handler))
func EntitlementMiddleware(c *core.Core, handler core.ActionHandler) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		// For beta.1: always allowed. When c.Entitlement is wired:
		// action := opts.String("_action")
		// if !CheckEntitlement(c, action) {
		//     return core.Result{OK: false}
		// }
		return handler(ctx, opts)
	}
}
