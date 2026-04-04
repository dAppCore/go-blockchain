// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package blockchain

import (
	"context"
	"testing"

	"dappco.re/go/core"
)

func TestEntitlement_CheckEntitlement_Good_NilCore(t *testing.T) {
	// nil Core = standalone mode, always allowed
	if !CheckEntitlement(nil, "blockchain.getinfo") {
		t.Error("nil Core should allow all actions")
	}
}

func TestEntitlement_SyncTask_Good(t *testing.T) {
	task := NewSyncTask()

	if task.IsRunning() {
		t.Error("new task should not be running")
	}
	if task.IsDone() {
		t.Error("new task should not be done")
	}
	if task.Progress() != 0 {
		t.Errorf("new task progress: got %f, want 0", task.Progress())
	}

	task.UpdateHeight(5000, 10000)
	p := task.Progress()
	if p != 0.5 {
		t.Errorf("progress at 5000/10000: got %f, want 0.5", p)
	}

	task.UpdateHeight(10000, 10000)
	if task.Progress() != 1.0 {
		t.Errorf("progress at 10000/10000: got %f, want 1.0", task.Progress())
	}
}

func TestEntitlement_Middleware_Good(t *testing.T) {
	called := false
	handler := func(ctx context.Context, opts core.Options) core.Result {
		called = true
		return core.Result{OK: true}
	}

	wrapped := EntitlementMiddleware(nil, handler)
	result := wrapped(context.Background(), core.Options{})

	if !called {
		t.Error("handler was not called")
	}
	if !result.OK {
		t.Error("result should be OK")
	}
}

func TestEntitlement_SyncTask_Ugly_ZeroRemote(t *testing.T) {
	task := NewSyncTask()
	task.UpdateHeight(100, 0)
	if task.Progress() != 0 {
		t.Error("zero remote height should return 0 progress")
	}
}
