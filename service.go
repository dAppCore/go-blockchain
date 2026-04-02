// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2
package blockchain

import (
	"context"
	"time"
	"net/http"

	"dappco.re/go/core"

	"dappco.re/go/core/blockchain/chain"
	"dappco.re/go/core/blockchain/daemon"
	"dappco.re/go/core/blockchain/rpc"
	store "dappco.re/go/core/store"
)

// BlockchainOptions configures the blockchain service.
type BlockchainOptions struct {
	DataDir string
	Seed    string
	Testnet bool
	RPCPort string
	RPCBind string
}

// BlockchainService is a Core-managed blockchain node.
//
//	svc := blockchain.NewBlockchainService(c, opts)
//	c.RegisterService("blockchain", svc)
type BlockchainService struct {
	core    *core.Core
	opts    BlockchainOptions
	chain   *chain.Chain
	store   *store.Store
	daemon  *daemon.Server
	cancel  context.CancelFunc
}

// NewBlockchainService creates and registers the blockchain as a Core service.
//
//	svc := blockchain.NewBlockchainService(c, opts)
func NewBlockchainService(c *core.Core, opts BlockchainOptions) *BlockchainService {
	svc := &BlockchainService{core: c, opts: opts}

	// Register as Core service with lifecycle.
	c.Service("blockchain", core.Service{
		Name:     "blockchain",
		Instance: svc,
		OnStart:  svc.start,
		OnStop:   svc.stop,
	})

	// Register blockchain actions.
	c.Action("blockchain.height", svc.actionHeight)
	c.Action("blockchain.info", svc.actionInfo)
	c.Action("blockchain.block", svc.actionBlock)
	c.Action("blockchain.aliases", svc.actionAliases)
	c.Action("blockchain.alias", svc.actionAlias)
	c.Action("blockchain.wallet.create", svc.actionWalletCreate)

	return svc
}

func (s *BlockchainService) start() core.Result {
	if s.opts.Testnet {
		chain.GenesisHash = chain.TestnetGenesisHash
	}

	dbPath := core.JoinPath(s.opts.DataDir, "chain.db")
	st, err := store.New(dbPath)
	if err != nil {
		return core.Result{OK: false}
	}
	s.store = st
	s.chain = chain.New(st)

	cfg, forks := resolveConfig(s.opts.Testnet, &s.opts.Seed)

	// Start background sync.
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	go func() {
		client := rpc.NewClient(seedToRPC(s.opts.Seed, s.opts.Testnet))
		opts := chain.SyncOptions{Forks: forks}
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err := s.chain.Sync(ctx, client, opts); err != nil {
				if ctx.Err() != nil {
					return
				}
				core.Print(nil, "blockchain sync: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
		}
	}()

	// Start RPC daemon.
	s.daemon = daemon.NewServer(s.chain, &cfg)
	addr := s.opts.RPCBind + ":" + s.opts.RPCPort
	go func() {
		core.Print(nil, "blockchain RPC on %s", addr)
		(&http.Server{Addr: addr, Handler: s.daemon, ReadTimeout: 30 * time.Second, WriteTimeout: 120 * time.Second}).ListenAndServe()
	}()

	core.Print(nil, "blockchain service started (testnet=%v, seed=%s)", s.opts.Testnet, s.opts.Seed)
	return core.Result{OK: true}
}

func (s *BlockchainService) stop() core.Result {
	if s.cancel != nil {
		s.cancel()
	}
	if s.store != nil {
		s.store.Close()
	}
	core.Print(nil, "blockchain service stopped")
	return core.Result{OK: true}
}

// --- Actions ---

func (s *BlockchainService) actionHeight(ctx context.Context, opts core.Options) core.Result {
	h, _ := s.chain.Height()
	return core.Result{Value: h, OK: true}
}

func (s *BlockchainService) actionInfo(ctx context.Context, opts core.Options) core.Result {
	h, _ := s.chain.Height()
	_, meta, _ := s.chain.TopBlock()
	aliases := s.chain.GetAllAliases()
	return core.Result{Value: map[string]interface{}{
		"height":      h,
		"difficulty":  meta.Difficulty,
		"alias_count": len(aliases),
		"synced":      true,
	}, OK: true}
}

func (s *BlockchainService) actionBlock(ctx context.Context, opts core.Options) core.Result {
	height := uint64(opts.Int("height"))
	blk, meta, err := s.chain.GetBlockByHeight(height)
	if err != nil {
		return core.Result{OK: false}
	}
	return core.Result{Value: map[string]interface{}{
		"hash":      meta.Hash.String(),
		"height":    meta.Height,
		"timestamp": blk.Timestamp,
	}, OK: true}
}

func (s *BlockchainService) actionAliases(ctx context.Context, opts core.Options) core.Result {
	return core.Result{Value: s.chain.GetAllAliases(), OK: true}
}

func (s *BlockchainService) actionAlias(ctx context.Context, opts core.Options) core.Result {
	name := opts.String("name")
	alias, err := s.chain.GetAlias(name)
	if err != nil {
		return core.Result{OK: false}
	}
	return core.Result{Value: alias, OK: true}
}

func (s *BlockchainService) actionWalletCreate(ctx context.Context, opts core.Options) core.Result {
	// Delegate to wallet package
	return core.Result{OK: false} // TODO: wire up wallet.GenerateAccount
}

// --- Helpers ---

func seedToRPC(seed string, testnet bool) string {
	if core.Contains(seed, ":46942") {
		return "http://127.0.0.1:46941"
	}
	if core.Contains(seed, ":36942") {
		return "http://127.0.0.1:36941"
	}
	return core.Sprintf("http://%s", seed)
}

// SyncStatus returns the current sync state.
func (s *BlockchainService) SyncStatus() map[string]interface{} {
	if s.chain == nil {
		return map[string]interface{}{"synced": false, "height": 0}
	}
	h, _ := s.chain.Height()
	return map[string]interface{}{
		"synced":  true,
		"height":  h,
		"aliases": len(s.chain.GetAllAliases()),
	}
}
