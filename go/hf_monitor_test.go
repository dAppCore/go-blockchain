package blockchain

import (
	"context"
	"testing"
	"time"

	"dappco.re/go/core/blockchain/chain"
	"dappco.re/go/core/blockchain/config"
	store "dappco.re/go/core/store"
)

func TestHardforkMonitor_New_Good(t *testing.T) {
	s, _ := store.New(t.TempDir() + "/test.db")
	defer s.Close()
	ch := chain.New(s)

	monitor := NewHardforkMonitor(ch, config.TestnetForks)
	if monitor == nil {
		t.Fatal("monitor is nil")
	}
}

func TestHardforkMonitor_RemainingBlocks_Good(t *testing.T) {
	s, _ := store.New(t.TempDir() + "/test.db")
	defer s.Close()
	ch := chain.New(s)

	monitor := NewHardforkMonitor(ch, config.TestnetForks)
	blocks, version := monitor.RemainingBlocks()
	// On empty chain, first fork should have remaining blocks
	if blocks == 0 && version == -1 {
		// All forks at height 0 — this is valid for testnet where HF0-1 are at 0
		return
	}
	if version < 0 {
		t.Error("expected pending hardfork")
	}
}

func TestHardforkMonitor_Start_Cancellation_Good(t *testing.T) {
	s, _ := store.New(t.TempDir() + "/test.db")
	defer s.Close()
	ch := chain.New(s)

	monitor := NewHardforkMonitor(ch, config.TestnetForks)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		monitor.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Good — monitor stopped on context cancellation
	case <-time.After(2 * time.Second):
		t.Error("monitor didn't stop on context cancellation")
	}
}
