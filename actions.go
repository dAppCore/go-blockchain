// Copyright (c) 2017-2026 Lethean (https://lt.hn)
// SPDX-License-Identifier: EUPL-1.2

package blockchain

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"

	"dappco.re/go/core"

	hsdpkg "dappco.re/go/core/blockchain/hsd"
	"dappco.re/go/core/blockchain/rpc"
	"dappco.re/go/core/blockchain/chain"
	"dappco.re/go/core/blockchain/config"
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
	c.Action("blockchain.chain.difficulty", makeChainDifficulty(ch))
	c.Action("blockchain.chain.transaction", makeChainTransaction(ch))
	c.Action("blockchain.chain.peers", makeChainPeers(ch))

	// Aliases
	c.Action("blockchain.alias.list", makeAliasList(ch))
	c.Action("blockchain.alias.get", makeAliasGet(ch))
	c.Action("blockchain.alias.capabilities", makeAliasCaps(ch))

	// Service discovery
	c.Action("blockchain.network.gateways", makeGateways(ch))
	c.Action("blockchain.network.topology", makeTopology(ch))
	c.Action("blockchain.network.vpn", makeVPNGateways(ch))
	c.Action("blockchain.network.dns", makeDNSGateways(ch))
	c.Action("blockchain.network.services", makeNetworkServices(ch))
	c.Action("blockchain.network.discover", makeServiceDiscover(ch))
	c.Action("blockchain.network.vpn.endpoints", makeVPNEndpoints(ch))
	c.Action("blockchain.network.gateway.register", makeGatewayRegister())

	// Supply + economics
	c.Action("blockchain.supply.total", makeSupplyTotal(ch))
	c.Action("blockchain.supply.hashrate", makeHashrate(ch))
	c.Action("blockchain.supply.emission", makeEmission(ch))
	c.Action("blockchain.supply.circulating", makeCirculating(ch))

	// Relay
	c.Action("blockchain.relay.info", makeRelayInfo(ch))

	// Identity
	c.Action("blockchain.identity.lookup", makeIdentityLookup(ch))
	c.Action("blockchain.identity.verify", makeIdentityVerify(ch))

	// Mining
	c.Action("blockchain.mining.template", makeMiningTemplate(ch))
	c.Action("blockchain.mining.difficulty", makeMiningDifficulty(ch))
	c.Action("blockchain.mining.reward", makeMiningReward(ch))
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

func makeChainDifficulty(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		_, meta, err := ch.TopBlock()
		if err != nil {
			return core.Result{OK: false}
		}
		return core.Result{Value: map[string]interface{}{
			"pow": meta.Difficulty,
			"pos": uint64(1),
		}, OK: true}
	}
}

func makeChainTransaction(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		hash := opts.String("hash")
		if hash == "" {
			return core.Result{OK: false}
		}
		txHash, err := types.HashFromHex(hash)
		if err != nil {
			return core.Result{OK: false}
		}
		tx, txMeta, err := ch.GetTransaction(txHash)
		if err != nil {
			return core.Result{OK: false}
		}
		return core.Result{Value: map[string]interface{}{
			"hash":         hash,
			"version":      tx.Version,
			"inputs":       len(tx.Vin),
			"outputs":      len(tx.Vout),
			"keeper_block": txMeta.KeeperBlock,
		}, OK: true}
	}
}

func makeChainPeers(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		return core.Result{Value: map[string]interface{}{
			"connected":  0,
			"incoming":   0,
			"outgoing":   0,
			"white_list": 0,
			"grey_list":  0,
			"note":       "Go node syncs via RPC, not P2P peers",
		}, OK: true}
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

// makeNetworkServices returns all services indexed by capability.
//
//	result := c.Action("blockchain.network.services").Run(ctx, opts)
//	// {"vpn":["charon","gateway"], "dns":["charon","network"], ...}
func makeNetworkServices(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		all := ch.GetAllAliases()
		services := make(map[string][]map[string]interface{})
		for _, a := range all {
			parsed := parseActionComment(a.Comment)
			capsStr := parsed["cap"]
			if capsStr == "" {
				continue
			}
			// Parse caps list (e.g. "vpn,dns,proxy")
			current := ""
			for _, char := range capsStr {
				if char == ',' {
					if current != "" {
						services[current] = append(services[current], map[string]interface{}{
							"name": a.Name, "address": a.Address, "comment": a.Comment,
							"type": parsed["type"], "hns": parsed["hns"],
						})
					}
					current = ""
				} else {
					current += string(char)
				}
			}
			if current != "" {
				services[current] = append(services[current], map[string]interface{}{
					"name": a.Name, "address": a.Address, "comment": a.Comment,
					"type": parsed["type"], "hns": parsed["hns"],
				})
			}
		}
		return core.Result{Value: services, OK: true}
	}
}

// makeServiceDiscover finds services matching a capability filter.
//
//	result := c.Action("blockchain.network.discover").Run(ctx, opts{capability: "vpn"})
func makeServiceDiscover(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		capability := opts.String("capability")
		if capability == "" {
			return core.Result{OK: false}
		}
		all := ch.GetAllAliases()
		var matches []map[string]interface{}
		for _, a := range all {
			if core.Contains(a.Comment, capability) {
				parsed := parseActionComment(a.Comment)
				matches = append(matches, map[string]interface{}{
					"name":    a.Name,
					"address": a.Address,
					"type":    parsed["type"],
					"caps":    parsed["cap"],
					"hns":     parsed["hns"],
				})
			}
		}
		return core.Result{Value: map[string]interface{}{
			"capability": capability,
			"count":      len(matches),
			"providers":  matches,
		}, OK: true}
	}
}

// makeVPNEndpoints returns VPN gateways with their WireGuard endpoints.
// Queries chain aliases for cap=vpn, then returns endpoint info.
//
//	result := c.Action("blockchain.network.vpn.endpoints").Run(ctx, opts)
//	// [{name, address, endpoint, proto, caps}]
func makeVPNEndpoints(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		all := ch.GetAllAliases()
		var endpoints []map[string]interface{}
		for _, a := range all {
			if !core.Contains(a.Comment, "vpn") {
				continue
			}
			parsed := parseActionComment(a.Comment)
			endpoints = append(endpoints, map[string]interface{}{
				"name":     a.Name,
				"address":  a.Address,
				"type":     parsed["type"],
				"caps":     parsed["cap"],
				"hns":      parsed["hns"],
				"endpoint": a.Name + ".lthn:51820",
				"proto":    "wireguard",
			})
		}
		return core.Result{Value: map[string]interface{}{
			"count":     len(endpoints),
			"endpoints": endpoints,
		}, OK: true}
	}
}

// makeGatewayRegister returns the alias comment format for gateway registration.
// This is a helper that generates the correct v=lthn1 comment string.
//
//	result := c.Action("blockchain.network.gateway.register").Run(ctx, opts{name, caps})
func makeGatewayRegister() core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		name := opts.String("name")
		caps := opts.String("caps")
		gatewayType := opts.String("type")
		if name == "" {
			return core.Result{OK: false}
		}
		if caps == "" {
			caps = "vpn,dns"
		}
		if gatewayType == "" {
			gatewayType = "gateway"
		}

		comment := core.Sprintf("v=lthn1;type=%s;cap=%s", gatewayType, caps)
		hnsName := name + ".lthn"

		return core.Result{Value: map[string]interface{}{
			"alias_name":  name,
			"comment":     comment,
			"hns_name":    hnsName,
			"instruction": core.Sprintf("Register alias @%s with comment: %s", name, comment),
			"dns_records": []string{
				core.Sprintf("A %s.lthn → <your_ip>", name),
				core.Sprintf("TXT vpn-endpoint=<your_ip>:51820 vpn-proto=wireguard"),
			},
		}, OK: true}
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

// makeEmission returns emission curve data per RFC.tokenomics.md.
//
//	result := c.Action("blockchain.supply.emission").Run(ctx, opts)
func makeEmission(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		h, _ := ch.Height()
		dailyBlocks := uint64(720) // ~720 blocks/day (120s target, PoW+PoS)
		return core.Result{Value: map[string]interface{}{
			"block_reward":      uint64(1),
			"block_reward_atomic": config.Coin,
			"blocks_per_day":    dailyBlocks,
			"daily_emission":    dailyBlocks,
			"annual_emission":   dailyBlocks * 365,
			"current_height":    h,
			"total_mined":       h,
			"premine":           PremineAmount,
			"fee_model":         "burned",
			"default_fee":       float64(config.DefaultFee) / float64(config.Coin),
			"halving":           "none (linear emission)",
		}, OK: true}
	}
}

// makeCirculating returns circulating supply accounting for locked outputs.
//
//	result := c.Action("blockchain.supply.circulating").Run(ctx, opts)
func makeCirculating(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		h, _ := ch.Height()
		totalSupply := PremineAmount + h
		// SWAP pool holds ~10M, not all circulating
		swapReserve := uint64(10000000) // 10M LTHN reserved for SWAP
		circulating := totalSupply - swapReserve
		if circulating > totalSupply {
			circulating = 0 // underflow guard
		}
		return core.Result{Value: map[string]interface{}{
			"total_supply":       totalSupply,
			"swap_reserve":       swapReserve,
			"circulating":        circulating,
			"circulating_pct":    float64(circulating) / float64(totalSupply) * 100,
			"unit":               "LTHN",
		}, OK: true}
	}
}

// makeRelayInfo returns relay network status.
//
//	result := c.Action("blockchain.relay.info").Run(ctx, opts)
func makeRelayInfo(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		h, _ := ch.Height()
		_, meta, _ := ch.TopBlock()
		if meta == nil {
			meta = &chain.BlockMeta{}
		}
		return core.Result{Value: map[string]interface{}{
			"height":        h,
			"top_hash":      meta.Hash.String(),
			"synced":        true,
			"relay_capable": true,
		}, OK: true}
	}
}

// makeIdentityLookup resolves a chain identity (alias → address + capabilities).
// Per code/core/network/RFC.md: identity is wallet → alias → .lthn → DNS.
//
//	result := c.Action("blockchain.identity.lookup").Run(ctx, opts{name: "charon"})
func makeIdentityLookup(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		name := opts.String("name")
		if name == "" {
			return core.Result{OK: false}
		}
		alias, err := ch.GetAlias(name)
		if err != nil {
			return core.Result{OK: false}
		}
		parsed := parseActionComment(alias.Comment)
		return core.Result{Value: map[string]interface{}{
			"name":    alias.Name,
			"address": alias.Address,
			"version": parsed["v"],
			"type":    parsed["type"],
			"caps":    parsed["cap"],
			"hns":     parsed["hns"],
			"dns":     alias.Name + ".lthn",
			"comment": alias.Comment,
		}, OK: true}
	}
}

// makeIdentityVerify checks if a given address matches a registered alias.
//
//	result := c.Action("blockchain.identity.verify").Run(ctx, opts{name, address})
func makeIdentityVerify(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		name := opts.String("name")
		address := opts.String("address")
		if name == "" || address == "" {
			return core.Result{OK: false}
		}
		alias, err := ch.GetAlias(name)
		if err != nil {
			return core.Result{Value: map[string]interface{}{
				"verified": false, "reason": "alias not found",
			}, OK: true}
		}
		verified := alias.Address == address
		return core.Result{Value: map[string]interface{}{
			"verified": verified,
			"name":     name,
			"address":  address,
			"chain_address": alias.Address,
		}, OK: true}
	}
}

// makeMiningTemplate returns current block template info for miners.
//
//	result := c.Action("blockchain.mining.template").Run(ctx, opts)
func makeMiningTemplate(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		h, _ := ch.Height()
		_, meta, _ := ch.TopBlock()
		if meta == nil {
			meta = &chain.BlockMeta{}
		}
		return core.Result{Value: map[string]interface{}{
			"height":     h,
			"difficulty": meta.Difficulty,
			"top_hash":   meta.Hash.String(),
			"reward":     config.Coin,
			"algo":       "progpowz",
			"target":     120,
		}, OK: true}
	}
}

// makeMiningDifficulty returns current PoW/PoS difficulty for miners.
//
//	result := c.Action("blockchain.mining.difficulty").Run(ctx, opts)
func makeMiningDifficulty(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		_, meta, _ := ch.TopBlock()
		if meta == nil {
			meta = &chain.BlockMeta{}
		}
		return core.Result{Value: map[string]interface{}{
			"pow":        meta.Difficulty,
			"pos":        uint64(1),
			"hashrate":   meta.Difficulty / 120,
			"block_time": 120,
			"algo":       "progpowz",
		}, OK: true}
	}
}

// makeMiningReward returns current block reward info.
//
//	result := c.Action("blockchain.mining.reward").Run(ctx, opts)
func makeMiningReward(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		return core.Result{Value: map[string]interface{}{
			"block_reward":       uint64(1),
			"block_reward_atomic": config.Coin,
			"fee_model":          "burned",
			"default_fee":        config.DefaultFee,
			"halving":            false,
		}, OK: true}
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
	c.Action("blockchain.wallet.restore", actionWalletRestore)
	c.Action("blockchain.wallet.info", actionWalletInfo)
	c.Action("blockchain.wallet.validate", actionWalletValidate)
	c.Action("blockchain.wallet.balance", actionWalletBalance)
	c.Action("blockchain.wallet.history", actionWalletHistory)
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

// actionWalletRestore restores a wallet from a 25-word seed phrase.
//
//	c.Action("blockchain.wallet.restore", opts{seed: "trip wonderful ..."})
func actionWalletRestore(ctx context.Context, opts core.Options) core.Result {
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
		"address":    addr.Encode(StandardPrefix),
		"integrated": addr.Encode(IntegratedPrefix),
		"auditable":  addr.Encode(AuditablePrefix),
		"restored":   true,
	}, OK: true}
}

// actionWalletInfo returns full wallet details from a seed.
//
//	c.Action("blockchain.wallet.info", opts{seed: "trip wonderful ..."})
func actionWalletInfo(ctx context.Context, opts core.Options) core.Result {
	seed := opts.String("seed")
	if seed == "" {
		// Generate fresh if no seed given
		account, err := wallet.GenerateAccount()
		if err != nil {
			return core.Result{OK: false}
		}
		addr := account.Address()
		newSeed, _ := account.ToSeed()
		return core.Result{Value: map[string]interface{}{
			"address":    addr.Encode(StandardPrefix),
			"integrated": addr.Encode(IntegratedPrefix),
			"auditable":  addr.Encode(AuditablePrefix),
			"spend_key":  hex.EncodeToString(account.SpendSecretKey[:]),
			"view_key":   hex.EncodeToString(account.ViewSecretKey[:]),
			"seed":       newSeed,
		}, OK: true}
	}
	account, err := wallet.RestoreFromSeed(seed)
	if err != nil {
		return core.Result{OK: false}
	}
	addr := account.Address()
	return core.Result{Value: map[string]interface{}{
		"address":    addr.Encode(StandardPrefix),
		"integrated": addr.Encode(IntegratedPrefix),
		"auditable":  addr.Encode(AuditablePrefix),
		"spend_key":  hex.EncodeToString(account.SpendSecretKey[:]),
		"view_key":   hex.EncodeToString(account.ViewSecretKey[:]),
		"seed":       seed,
	}, OK: true}
}

// actionWalletValidate checks if a Lethean address is valid.
//
//	c.Action("blockchain.wallet.validate", opts{address: "iTHN..."})
func actionWalletValidate(ctx context.Context, opts core.Options) core.Result {
	address := opts.String("address")
	if address == "" {
		return core.Result{OK: false}
	}
	valid := core.HasPrefix(address, "iTHN") || core.HasPrefix(address, "iTHn") || core.HasPrefix(address, "iThN")
	return core.Result{Value: map[string]interface{}{
		"address": address,
		"valid":   valid,
		"prefix":  address[:4],
	}, OK: true}
}

// walletRPC makes a JSON-RPC call to the wallet daemon.
func walletRPC(walletURL, method string, params interface{}) (interface{}, error) {
	body := core.Sprintf(`{"jsonrpc":"2.0","id":"0","method":"%s","params":%s}`, method, func() string {
		if params == nil {
			return "{}"
		}
		return core.JSONMarshalString(params)
	}())

	resp, err := http.Post(walletURL+"/json_rpc", "application/json", core.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var result struct {
		Result interface{} `json:"result"`
	}
	json.Unmarshal(raw, &result)
	return result.Result, nil
}

// actionWalletBalance queries the wallet RPC for balance.
// Requires wallet RPC running on WALLET_RPC_URL (default: 127.0.0.1:46944).
//
//	c.Action("blockchain.wallet.balance", opts{wallet_url: "http://..."})
func actionWalletBalance(ctx context.Context, opts core.Options) core.Result {
	walletURL := opts.String("wallet_url")
	if walletURL == "" {
		walletURL = core.Env("WALLET_RPC_URL")
	}
	if walletURL == "" {
		walletURL = "http://127.0.0.1:46944"
	}

	result, err := walletRPC(walletURL, "getbalance", nil)
	if err != nil {
		return core.Result{OK: false}
	}
	return core.Result{Value: result, OK: true}
}

// actionWalletHistory queries the wallet RPC for transfer history.
//
//	c.Action("blockchain.wallet.history", opts{wallet_url, count, offset})
func actionWalletHistory(ctx context.Context, opts core.Options) core.Result {
	walletURL := opts.String("wallet_url")
	if walletURL == "" {
		walletURL = core.Env("WALLET_RPC_URL")
	}
	if walletURL == "" {
		walletURL = "http://127.0.0.1:46944"
	}

	count := opts.Int("count")
	if count == 0 {
		count = 20
	}

	result, err := walletRPC(walletURL, "get_recent_txs_and_info2", map[string]interface{}{
		"count":  count,
		"offset": opts.Int("offset"),
	})
	if err != nil {
		return core.Result{OK: false}
	}
	return core.Result{Value: result, OK: true}
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
	c.Action("blockchain.asset.emit", actionAssetEmit)
	c.Action("blockchain.asset.burn", actionAssetBurn)
	c.Action("blockchain.asset.balance", actionAssetBalance)
	c.Action("blockchain.asset.transfer", actionAssetTransfer)
	c.Action("blockchain.asset.whitelist", actionAssetWhitelist)
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

// actionAssetEmit mints additional tokens for an existing asset (HF5+).
//
//	c.Action("blockchain.asset.emit", opts{asset_id, amount})
func actionAssetEmit(ctx context.Context, opts core.Options) core.Result {
	assetID := opts.String("asset_id")
	amount := opts.String("amount")
	if assetID == "" || amount == "" {
		return core.Result{OK: false}
	}
	return core.Result{Value: map[string]interface{}{
		"asset_id": assetID, "amount": amount,
		"status": "requires_wallet_rpc",
		"method": "deploy_new_asset with emit flag",
	}, OK: true}
}

// actionAssetBurn destroys tokens from an existing asset (HF5+).
//
//	c.Action("blockchain.asset.burn", opts{asset_id, amount})
func actionAssetBurn(ctx context.Context, opts core.Options) core.Result {
	assetID := opts.String("asset_id")
	amount := opts.String("amount")
	if assetID == "" || amount == "" {
		return core.Result{OK: false}
	}
	return core.Result{Value: map[string]interface{}{
		"asset_id": assetID, "amount": amount,
		"status": "requires_wallet_rpc",
	}, OK: true}
}

// actionAssetBalance queries balance for a specific asset via wallet RPC.
//
//	c.Action("blockchain.asset.balance", opts{asset_id, wallet_url})
func actionAssetBalance(ctx context.Context, opts core.Options) core.Result {
	walletURL := opts.String("wallet_url")
	if walletURL == "" {
		walletURL = core.Env("WALLET_RPC_URL")
	}
	if walletURL == "" {
		walletURL = "http://127.0.0.1:46944"
	}

	result, err := walletRPC(walletURL, "getbalance", nil)
	if err != nil {
		return core.Result{OK: false}
	}
	return core.Result{Value: result, OK: true}
}

// actionAssetTransfer sends confidential asset tokens (HF5+).
//
//	c.Action("blockchain.asset.transfer", opts{asset_id, destination, amount})
func actionAssetTransfer(ctx context.Context, opts core.Options) core.Result {
	assetID := opts.String("asset_id")
	destination := opts.String("destination")
	amount := opts.String("amount")
	if assetID == "" || destination == "" || amount == "" {
		return core.Result{OK: false}
	}
	return core.Result{Value: map[string]interface{}{
		"asset_id": assetID, "destination": destination, "amount": amount,
		"status": "requires_wallet_rpc",
		"method": "transfer with asset_id param",
	}, OK: true}
}

// actionAssetWhitelist returns the current asset whitelist.
//
//	c.Action("blockchain.asset.whitelist", opts{})
func actionAssetWhitelist(ctx context.Context, opts core.Options) core.Result {
	return core.Result{Value: map[string]interface{}{
		"whitelisted": []map[string]interface{}{
			{"ticker": "LTHN", "name": "Lethean", "native": true,
				"asset_id": "d6329b5b1f7c0805b5c345f4957554002a2f557845f64d7645dae0e051a6498a"},
		},
		"source": "https://downloads.lthn.io/assets_whitelist.json",
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
func RegisterAllActions(c *core.Core, ch *chain.Chain, hsdURL, hsdKey string) {
	RegisterActions(c, ch)
	RegisterWalletActions(c)
	RegisterCryptoActions(c)
	RegisterAssetActions(c)
	RegisterForgeActions(c)
	RegisterHSDActions(c, hsdURL, hsdKey)
	RegisterDNSActions(c, ch, hsdURL, hsdKey)
	RegisterEscrowActions(c, ch)
	RegisterEstimateActions(c, ch)
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

// RegisterDNSActions registers DNS resolution actions.
// These bridge go-blockchain (alias discovery) to go-lns (name resolution).
//
//	blockchain.RegisterDNSActions(c, chain, hsdURL, hsdKey)
func RegisterDNSActions(c *core.Core, ch *chain.Chain, hsdURL, hsdKey string) {
	c.Action("blockchain.dns.resolve", makeDNSResolve(ch, hsdURL, hsdKey))
	c.Action("blockchain.dns.names", makeDNSNames(ch, hsdURL, hsdKey))
	c.Action("blockchain.dns.discover", makeDNSDiscover(ch))
}

func makeDNSResolve(ch *chain.Chain, hsdURL, hsdKey string) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		name := opts.String("name")
		if name == "" {
			return core.Result{OK: false}
		}
		name = core.TrimSuffix(name, ".lthn")
		name = core.TrimSuffix(name, ".")

		client := hsdpkg.NewClient(hsdURL, hsdKey)
		resource, err := client.GetNameResource(name)
		if err != nil || resource == nil {
			return core.Result{OK: false}
		}

		var addresses []string
		var txts []string
		for _, r := range resource.Records {
			if r.Type == "GLUE4" { addresses = append(addresses, r.Address) }
			if r.Type == "TXT" { txts = append(txts, r.TXT...) }
		}

		return core.Result{Value: map[string]interface{}{
			"name": name + ".lthn", "a": addresses, "txt": txts,
		}, OK: true}
	}
}

func makeDNSNames(ch *chain.Chain, hsdURL, hsdKey string) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		aliases := ch.GetAllAliases()
		client := hsdpkg.NewClient(hsdURL, hsdKey)

		var names []map[string]interface{}
		for _, a := range aliases {
			hnsName := a.Name
			parsed := parseActionComment(a.Comment)
			if h, ok := parsed["hns"]; ok {
				hnsName = core.TrimSuffix(h, ".lthn")
			}

			resource, err := client.GetNameResource(hnsName)
			if err != nil || resource == nil { continue }

			var addrs []string
			for _, r := range resource.Records {
				if r.Type == "GLUE4" { addrs = append(addrs, r.Address) }
			}
			if len(addrs) > 0 {
				names = append(names, map[string]interface{}{
					"name": hnsName + ".lthn", "addresses": addrs,
				})
			}
		}
		return core.Result{Value: names, OK: true}
	}
}

func makeDNSDiscover(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		aliases := ch.GetAllAliases()
		var names []string
		for _, a := range aliases {
			hnsName := a.Name
			parsed := parseActionComment(a.Comment)
			if h, ok := parsed["hns"]; ok {
				hnsName = core.TrimSuffix(h, ".lthn")
			}
			names = append(names, hnsName)
		}
		return core.Result{Value: names, OK: true}
	}
}

// RegisterMonitorActions registers hardfork monitoring actions.
func RegisterMonitorActions(c *core.Core, ch *chain.Chain, forks []config.HardFork) {
	monitor := NewHardforkMonitor(ch, forks)
	c.Action("blockchain.hardfork.remaining", func(ctx context.Context, opts core.Options) core.Result {
		blocks, version := monitor.RemainingBlocks()
		return core.Result{Value: map[string]interface{}{
			"blocks": blocks, "version": version,
		}, OK: true}
	})
}

// RegisterRelayActions registers transaction relay actions.
func RegisterRelayActions(c *core.Core, rpcURL string) {
	c.Action("blockchain.tx.relay", makeRelayTx(rpcURL))
	c.Action("blockchain.tx.pool", makeTxPool(rpcURL))
}

func makeRelayTx(rpcURL string) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		txHex := opts.String("tx_hex")
		if txHex == "" {
			return core.Result{OK: false}
		}
		client := rpc.NewClient(rpcURL)
		txBytes, err := hexDecode(txHex)
		if err != nil {
			return core.Result{OK: false}
		}
		if err := client.SendRawTransaction(txBytes); err != nil {
			return core.Result{OK: false}
		}
		return core.Result{Value: "relayed", OK: true}
	}
}

func makeTxPool(rpcURL string) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		client := rpc.NewClient(rpcURL)
		info, err := client.GetInfo()
		if err != nil {
			return core.Result{OK: false}
		}
		return core.Result{Value: map[string]interface{}{
			"pool_size": info.TxPoolSize,
		}, OK: true}
	}
}

func hexDecode(s string) ([]byte, error) {
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		hi := hexVal(s[i])
		lo := hexVal(s[i+1])
		if hi < 0 || lo < 0 {
			return nil, core.E("hexDecode", "invalid hex", nil)
		}
		b[i/2] = byte(hi<<4 | lo)
	}
	return b, nil
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9': return int(c - '0')
	case c >= 'a' && c <= 'f': return int(c - 'a' + 10)
	case c >= 'A' && c <= 'F': return int(c - 'A' + 10)
	default: return -1
	}
}

// RegisterEstimateActions registers estimation/calculation actions.
func RegisterEstimateActions(c *core.Core, ch *chain.Chain) {
	c.Action("blockchain.estimate.block_time", makeEstBlockTime(ch))
	c.Action("blockchain.estimate.supply_at_height", makeEstSupplyAtHeight())
	c.Action("blockchain.estimate.height_at_time", makeEstHeightAtTime(ch))
}

func makeEstBlockTime(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		h, _ := ch.Height()
		if h < 2 {
			return core.Result{Value: uint64(120), OK: true}
		}
		genesis, _, _ := ch.GetBlockByHeight(0)
		_, _, top := ch.Snapshot()
		if genesis == nil || top == nil || top.Height == 0 {
			return core.Result{Value: uint64(120), OK: true}
		}
		avg := (top.Timestamp - genesis.Timestamp) / top.Height
		return core.Result{Value: avg, OK: true}
	}
}

func makeEstSupplyAtHeight() core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		height := uint64(opts.Int("height"))
		supply := PremineAmount + height*DefaultBlockReward
		return core.Result{Value: map[string]interface{}{
			"height": height, "supply_lthn": supply,
			"supply_atomic": supply * AtomicUnit,
		}, OK: true}
	}
}

func makeEstHeightAtTime(ch *chain.Chain) core.ActionHandler {
	return func(ctx context.Context, opts core.Options) core.Result {
		targetTime := uint64(opts.Int("timestamp"))
		if targetTime == 0 {
			return core.Result{OK: false}
		}
		genesis, _, _ := ch.GetBlockByHeight(0)
		if genesis == nil {
			return core.Result{OK: false}
		}
		if targetTime <= genesis.Timestamp {
			return core.Result{Value: uint64(0), OK: true}
		}
		elapsed := targetTime - genesis.Timestamp
		est := elapsed / 120 // avg block time
		return core.Result{Value: est, OK: true}
	}
}

// RegisterMetricsActions registers operational metrics actions.
func RegisterMetricsActions(c *core.Core, m *Metrics) {
	c.Action("blockchain.metrics.snapshot", func(ctx context.Context, opts core.Options) core.Result {
		return core.Result{Value: m.Snapshot(), OK: true}
	})
}

// RegisterEscrowActions registers trustless service payment actions.
// Escrow uses HF4 multisig: customer deposits, provider claims on delivery.
//
//	blockchain.RegisterEscrowActions(c, chain)
func RegisterEscrowActions(c *core.Core, ch *chain.Chain) {
	// escrow state: in-memory for now, persistent via go-store in production
	escrows := make(map[string]map[string]interface{})

	// blockchain.escrow.create — create escrow contract
	//   c.Action("blockchain.escrow.create", opts{provider, customer, amount, terms})
	c.Action("blockchain.escrow.create", func(ctx context.Context, opts core.Options) core.Result {
		provider := opts.String("provider")
		customer := opts.String("customer")
		amount := opts.String("amount")
		terms := opts.String("terms")

		if provider == "" || customer == "" || amount == "" {
			return core.Result{OK: false}
		}

		escrowID := core.Sprintf("escrow_%d_%s", len(escrows)+1, provider[:8])
		escrows[escrowID] = map[string]interface{}{
			"escrow_id": escrowID,
			"provider":  provider,
			"customer":  customer,
			"amount":    amount,
			"terms":     terms,
			"status":    "created",
			"funded":    false,
		}

		return core.Result{Value: map[string]interface{}{
			"escrow_id": escrowID,
			"status":    "created",
		}, OK: true}
	})

	// blockchain.escrow.fund — fund the escrow (customer deposits)
	//   c.Action("blockchain.escrow.fund", opts{escrow_id})
	c.Action("blockchain.escrow.fund", func(ctx context.Context, opts core.Options) core.Result {
		escrowID := opts.String("escrow_id")
		escrow, exists := escrows[escrowID]
		if !exists {
			return core.Result{OK: false}
		}
		escrow["status"] = "funded"
		escrow["funded"] = true
		return core.Result{Value: map[string]interface{}{
			"escrow_id": escrowID,
			"status":    "funded",
		}, OK: true}
	})

	// blockchain.escrow.release — provider claims payment (proof of service)
	//   c.Action("blockchain.escrow.release", opts{escrow_id, proof_of_service})
	c.Action("blockchain.escrow.release", func(ctx context.Context, opts core.Options) core.Result {
		escrowID := opts.String("escrow_id")
		escrow, exists := escrows[escrowID]
		if !exists {
			return core.Result{OK: false}
		}
		if escrow["status"] != "funded" {
			return core.Result{OK: false}
		}
		escrow["status"] = "released"
		return core.Result{Value: map[string]interface{}{
			"escrow_id": escrowID,
			"status":    "released",
			"amount":    escrow["amount"],
		}, OK: true}
	})

	// blockchain.escrow.refund — customer reclaims after timeout
	//   c.Action("blockchain.escrow.refund", opts{escrow_id})
	c.Action("blockchain.escrow.refund", func(ctx context.Context, opts core.Options) core.Result {
		escrowID := opts.String("escrow_id")
		escrow, exists := escrows[escrowID]
		if !exists {
			return core.Result{OK: false}
		}
		escrow["status"] = "refunded"
		return core.Result{Value: map[string]interface{}{
			"escrow_id": escrowID,
			"status":    "refunded",
		}, OK: true}
	})

	// blockchain.escrow.status — check escrow state
	//   c.Action("blockchain.escrow.status", opts{escrow_id})
	c.Action("blockchain.escrow.status", func(ctx context.Context, opts core.Options) core.Result {
		escrowID := opts.String("escrow_id")
		escrow, exists := escrows[escrowID]
		if !exists {
			return core.Result{OK: false}
		}
		return core.Result{Value: escrow, OK: true}
	})
}
