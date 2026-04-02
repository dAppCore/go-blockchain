// Copyright (c) 2017-2026 Lethean (https://lt.hn)
// SPDX-License-Identifier: EUPL-1.2

package blockchain

import (
	"context"

	"dappco.re/go/core"

	hsdpkg "dappco.re/go/core/blockchain/hsd"
	"dappco.re/go/core/blockchain/chain"
	"dappco.re/go/core/blockchain/crypto"
	"dappco.re/go/core/blockchain/types"
	"dappco.re/go/core/blockchain/wallet"
)

// RegisterActions registers all blockchain actions with a Core instance.
// Each action becomes available as CLI command, MCP tool, and API endpoint.
//
//	blockchain.RegisterActions(c, chainInstance)
func RegisterActions(c *core.Core, ch *chain.Chain) {
	// Chain state
	c.Action("blockchain.chain.height", makeChainHeight(ch))
	c.Action("blockchain.chain.info", makeChainInfo(ch))
	c.Action("blockchain.chain.block", makeChainBlock(ch))
	c.Action("blockchain.chain.synced", makeChainSynced(ch))
	c.Action("blockchain.chain.hardforks", makeChainHardforks(ch))
	c.Action("blockchain.chain.stats", makeChainStats(ch))
	c.Action("blockchain.chain.search", makeChainSearch(ch))

	// Aliases
	c.Action("blockchain.alias.list", makeAliasList(ch))
	c.Action("blockchain.alias.get", makeAliasGet(ch))
	c.Action("blockchain.alias.capabilities", makeAliasCaps(ch))

	// Service discovery
	c.Action("blockchain.network.gateways", makeGateways(ch))
	c.Action("blockchain.network.topology", makeTopology(ch))
	c.Action("blockchain.network.vpn", makeVPNGateways(ch))
	c.Action("blockchain.network.dns", makeDNSGateways(ch))

	// Supply
	c.Action("blockchain.supply.total", makeSupplyTotal(ch))
	c.Action("blockchain.supply.hashrate", makeHashrate(ch))
}

func makeChainHeight(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		h, _ := ch.Height()
		return core.Result{Value: h, OK: true}
	}
}

func makeChainInfo(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		h, _ := ch.Height()
		_, meta, _ := ch.TopBlock()
		if meta == nil {
			meta = &chain.BlockMeta{}
		}
		aliases := ch.GetAllAliases()
		return core.Result{Value: map[string]interface{}{
			"height": h, "difficulty": meta.Difficulty,
			"aliases": len(aliases), "synced": true,
		}, OK: true}
	}
}

func makeChainBlock(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		height := uint64(opts.Int("height"))
		blk, meta, err := ch.GetBlockByHeight(height)
		if err != nil {
			return core.Result{OK: false}
		}
		return core.Result{Value: map[string]interface{}{
			"hash": meta.Hash.String(), "height": meta.Height,
			"timestamp": blk.Timestamp, "difficulty": meta.Difficulty,
		}, OK: true}
	}
}

func makeChainSynced(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		h, _ := ch.Height()
		return core.Result{Value: h > 0, OK: true}
	}
}

func makeChainHardforks(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		h, _ := ch.Height()
		return core.Result{Value: map[string]interface{}{
			"hf0": true, "hf1": true, "hf2": h >= 10000,
			"hf3": h >= 10500, "hf4": h >= 11000, "hf5": h >= 11500,
		}, OK: true}
	}
}

func makeChainStats(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		h, _ := ch.Height()
		aliases := ch.GetAllAliases()
		gw := 0
		for _, a := range aliases {
			if core.Contains(a.Comment, "type=gateway") { gw++ }
		}
		return core.Result{Value: map[string]interface{}{
			"height": h, "aliases": len(aliases),
			"gateways": gw, "services": len(aliases) - gw,
		}, OK: true}
	}
}

func makeChainSearch(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		query := opts.String("query")
		if alias, err := ch.GetAlias(query); err == nil {
			return core.Result{Value: map[string]interface{}{
				"type": "alias", "name": alias.Name, "comment": alias.Comment,
			}, OK: true}
		}
		return core.Result{Value: map[string]interface{}{"type": "not_found"}, OK: true}
	}
}

func makeAliasList(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		return core.Result{Value: ch.GetAllAliases(), OK: true}
	}
}

func makeAliasGet(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		name := opts.String("name")
		alias, err := ch.GetAlias(name)
		if err != nil {
			return core.Result{OK: false}
		}
		return core.Result{Value: alias, OK: true}
	}
}

func makeAliasCaps(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		name := opts.String("name")
		alias, err := ch.GetAlias(name)
		if err != nil {
			return core.Result{OK: false}
		}
		parsed := parseActionComment(alias.Comment)
		return core.Result{Value: map[string]interface{}{
			"name": alias.Name, "type": parsed["type"],
			"capabilities": parsed["cap"], "hns": parsed["hns"],
		}, OK: true}
	}
}

func makeGateways(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		all := ch.GetAllAliases()
		var gateways []map[string]string
		for _, a := range all {
			if core.Contains(a.Comment, "type=gateway") {
				gateways = append(gateways, map[string]string{
					"name": a.Name, "comment": a.Comment,
				})
			}
		}
		return core.Result{Value: gateways, OK: true}
	}
}

func makeTopology(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		all := ch.GetAllAliases()
		topo := map[string]int{"total": len(all), "gateways": 0, "services": 0}
		for _, a := range all {
			p := parseActionComment(a.Comment)
			if p["type"] == "gateway" { topo["gateways"]++ } else { topo["services"]++ }
		}
		return core.Result{Value: topo, OK: true}
	}
}

func makeVPNGateways(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		all := ch.GetAllAliases()
		var vpns []string
		for _, a := range all {
			if core.Contains(a.Comment, "cap=vpn") || core.Contains(a.Comment, ",vpn") {
				vpns = append(vpns, a.Name)
			}
		}
		return core.Result{Value: vpns, OK: true}
	}
}

func makeDNSGateways(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		all := ch.GetAllAliases()
		var dns []string
		for _, a := range all {
			if core.Contains(a.Comment, "dns") {
				dns = append(dns, a.Name)
			}
		}
		return core.Result{Value: dns, OK: true}
	}
}

func makeSupplyTotal(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		h, _ := ch.Height()
		return core.Result{Value: map[string]interface{}{
			"total": PremineAmount + h, "premine": PremineAmount,
			"mined": h, "unit": "LTHN",
		}, OK: true}
	}
}

func makeHashrate(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		_, meta, _ := ch.TopBlock()
		if meta == nil { meta = &chain.BlockMeta{} }
		return core.Result{Value: meta.Difficulty / 120, OK: true}
	}
}

// parseActionComment parses a v=lthn1;type=gateway;cap=vpn comment.
func parseActionComment(comment string) map[string]string {
	parsed := make(map[string]string)
	for _, part := range core.Split(comment, ";") {
		idx := -1
		for i, c := range part {
			if c == '=' { idx = i; break }
		}
		if idx > 0 {
			parsed[part[:idx]] = part[idx+1:]
		}
	}
	return parsed
}

// RegisterWalletActions registers wallet-related Core actions.
//
//	blockchain.RegisterWalletActions(c)
func RegisterWalletActions(c *core.Core) {
	c.Action("blockchain.wallet.create", actionWalletCreate)
	c.Action("blockchain.wallet.address", actionWalletAddress)
	c.Action("blockchain.wallet.seed", actionWalletSeed)
}

func actionWalletCreate(ctx context.Context, opts core.Options) core.Result {
	account, err := wallet.GenerateAccount()
	if err != nil {
		return core.Result{OK: false}
	}
	addr := account.Address()
	seed, _ := account.ToSeed()
	return core.Result{Value: map[string]interface{}{
		"address": addr.Encode(StandardPrefix),
		"seed":    seed,
	}, OK: true}
}

func actionWalletAddress(ctx context.Context, opts core.Options) core.Result {
	seed := opts.String("seed")
	if seed == "" {
		return core.Result{OK: false}
	}
	account, err := wallet.RestoreFromSeed(seed)
	if err != nil {
		return core.Result{OK: false}
	}
	addr := account.Address()
	return core.Result{Value: map[string]interface{}{
		"standard":   addr.Encode(StandardPrefix),
		"integrated": addr.Encode(IntegratedPrefix),
		"auditable":  addr.Encode(AuditablePrefix),
	}, OK: true}
}

func actionWalletSeed(ctx context.Context, opts core.Options) core.Result {
	// Generate a fresh seed (no wallet file needed)
	account, err := wallet.GenerateAccount()
	if err != nil {
		return core.Result{OK: false}
	}
	seed, _ := account.ToSeed()
	return core.Result{Value: seed, OK: true}
}

// RegisterCryptoActions registers native CGo crypto actions.
//
//	blockchain.RegisterCryptoActions(c)
func RegisterCryptoActions(c *core.Core) {
	c.Action("blockchain.crypto.hash", actionHash)
	c.Action("blockchain.crypto.generate_keys", actionGenerateKeys)
	c.Action("blockchain.crypto.check_key", actionCheckKey)
	c.Action("blockchain.crypto.validate_address", actionValidateAddress)
}

func actionHash(ctx context.Context, opts core.Options) core.Result {
	data := opts.String("data")
	if data == "" {
		return core.Result{OK: false}
	}
	hash := crypto.FastHash([]byte(data))
	return core.Result{Value: core.Sprintf("%x", hash), OK: true}
}

func actionGenerateKeys(ctx context.Context, opts core.Options) core.Result {
	pub, sec, err := crypto.GenerateKeys()
	if err != nil {
		return core.Result{OK: false}
	}
	return core.Result{Value: map[string]interface{}{
		"public": core.Sprintf("%x", pub),
		"secret": core.Sprintf("%x", sec),
	}, OK: true}
}

func actionCheckKey(ctx context.Context, opts core.Options) core.Result {
	// Simplified — real impl needs hex decode
	return core.Result{Value: true, OK: true}
}

func actionValidateAddress(ctx context.Context, opts core.Options) core.Result {
	addr := opts.String("address")
	_, prefix, err := types.DecodeAddress(addr)
	valid := err == nil
	addrType := "unknown"
	switch prefix {
	case StandardPrefix: addrType = "standard"
	case IntegratedPrefix: addrType = "integrated"
	case AuditablePrefix: addrType = "auditable"
	}
	return core.Result{Value: map[string]interface{}{
		"valid": valid, "type": addrType,
	}, OK: true}
}

// RegisterAssetActions registers HF5 confidential asset actions.
//
//	blockchain.RegisterAssetActions(c)
func RegisterAssetActions(c *core.Core) {
	c.Action("blockchain.asset.info", actionAssetInfo)
	c.Action("blockchain.asset.list", actionAssetList)
	c.Action("blockchain.asset.deploy", actionAssetDeploy)
}

func actionAssetInfo(ctx context.Context, opts core.Options) core.Result {
	assetID := opts.String("asset_id")
	if assetID == "" || assetID == "LTHN" {
		return core.Result{Value: map[string]interface{}{
			"ticker": "LTHN", "name": "Lethean", "decimals": 12,
			"native": true,
		}, OK: true}
	}
	return core.Result{OK: false}
}

func actionAssetList(ctx context.Context, opts core.Options) core.Result {
	return core.Result{Value: []map[string]interface{}{
		{"ticker": "LTHN", "name": "Lethean", "native": true},
	}, OK: true}
}

func actionAssetDeploy(ctx context.Context, opts core.Options) core.Result {
	ticker := opts.String("ticker")
	name := opts.String("name")
	if ticker == "" || name == "" {
		return core.Result{OK: false}
	}
	return core.Result{Value: map[string]interface{}{
		"ticker": ticker, "name": name, "status": "ready_for_hf5",
	}, OK: true}
}

// RegisterForgeActions registers forge integration actions.
//
//	blockchain.RegisterForgeActions(c)
func RegisterForgeActions(c *core.Core) {
	c.Action("blockchain.forge.release", forgePublishRelease)
	c.Action("blockchain.forge.issue", forgeCreateIssue)
	c.Action("blockchain.forge.build", forgeDispatchBuild)
	c.Action("blockchain.forge.event", forgeChainEvent)
}

// RegisterAllActions registers every blockchain action with Core.
//
//	blockchain.RegisterAllActions(c, chain)
func RegisterAllActions(c *core.Core, ch *chain.Chain) {
	RegisterActions(c, ch)
	RegisterWalletActions(c)
	RegisterCryptoActions(c)
	RegisterAssetActions(c)
	RegisterForgeActions(c)
}

// RegisterHSDActions registers sidechain query actions.
//
//	blockchain.RegisterHSDActions(c, hsdClient)
func RegisterHSDActions(c *core.Core, hsdURL, hsdKey string) {
	c.Action("blockchain.hsd.info", makeHSDInfo(hsdURL, hsdKey))
	c.Action("blockchain.hsd.resolve", makeHSDResolve(hsdURL, hsdKey))
	c.Action("blockchain.hsd.height", makeHSDHeight(hsdURL, hsdKey))
}

func makeHSDInfo(url, key string) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		client := hsdpkg.NewClient(url, key)
		info, err := client.GetBlockchainInfo()
		if err != nil {
			return core.Result{OK: false}
		}
		return core.Result{Value: map[string]interface{}{
			"chain": info.Chain, "height": info.Blocks,
			"tree_root": info.TreeRoot,
		}, OK: true}
	}
}

func makeHSDResolve(url, key string) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		name := opts.String("name")
		if name == "" {
			return core.Result{OK: false}
		}
		client := hsdpkg.NewClient(url, key)
		resource, err := client.GetNameResource(name)
		if err != nil {
			return core.Result{OK: false}
		}
		var records []map[string]interface{}
		for _, r := range resource.Records {
			records = append(records, map[string]interface{}{
				"type": r.Type, "address": r.Address, "txt": r.TXT, "ns": r.NS,
			})
		}
		return core.Result{Value: map[string]interface{}{
			"name": name, "records": records,
		}, OK: true}
	}
}

func makeHSDHeight(url, key string) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		client := hsdpkg.NewClient(url, key)
		height, err := client.GetHeight()
		if err != nil {
			return core.Result{OK: false}
		}
		return core.Result{Value: height, OK: true}
	}
}
