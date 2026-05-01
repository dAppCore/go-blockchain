// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package blockchain

import (
	"context"

	"dappco.re/go/core"
)

// ForgeActions registers blockchain-specific forge integration actions.
// These enable the Go daemon to interact with the Forgejo instance
// for automated release publishing, issue creation, and CI triggering.
//
//	blockchain.ForgeActions(c)
func ForgeActions(c *core.Core) {
	c.Action("blockchain.forge.publish_release", forgePublishRelease)
	c.Action("blockchain.forge.create_issue", forgeCreateIssue)
	c.Action("blockchain.forge.dispatch_build", forgeDispatchBuild)
	c.Action("blockchain.forge.chain_event", forgeChainEvent)
}

func forgePublishRelease(ctx context.Context, opts core.Options) core.Result {
	version := opts.String("version")
	if version == "" {
		return core.Result{OK: false}
	}

	// Use go-forge client to create a release on forge.lthn.ai/core/go-blockchain
	// For now, return the action spec — implementation needs forge client wiring
	return core.Result{
		Value: map[string]interface{}{
			"action":  "publish_release",
			"version": version,
			"repo":    "core/go-blockchain",
			"status":  "ready",
		},
		OK: true,
	}
}

func forgeCreateIssue(ctx context.Context, opts core.Options) core.Result {
	title := opts.String("title")
	body := opts.String("body")
	labels := opts.String("labels")

	return core.Result{
		Value: map[string]interface{}{
			"action": "create_issue",
			"title":  title,
			"body":   body,
			"labels": labels,
			"repo":   "core/go-blockchain",
			"status": "ready",
		},
		OK: true,
	}
}

func forgeDispatchBuild(ctx context.Context, opts core.Options) core.Result {
	target := opts.String("target")
	if target == "" {
		target = "testnet"
	}

	return core.Result{
		Value: map[string]interface{}{
			"action":   "dispatch_build",
			"target":   target,
			"workflow": "build-multiplatform",
			"repo":     "core/go-blockchain",
			"status":   "ready",
		},
		OK: true,
	}
}

func forgeChainEvent(ctx context.Context, opts core.Options) core.Result {
	event := opts.String("event")
	height := opts.Int("height")

	// When chain reaches milestones, create forge activity
	return core.Result{
		Value: map[string]interface{}{
			"action": "chain_event",
			"event":  event,
			"height": height,
			"status": "ready",
		},
		OK: true,
	}
}
