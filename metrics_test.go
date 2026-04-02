package blockchain

import (
	"testing"

	"dappco.re/go/core/blockchain/chain"
	store "dappco.re/go/core/store"
)

func TestMetrics_RecordBlock_Good(t *testing.T) {
	s, _ := store.New(t.TempDir() + "/test.db")
	defer s.Close()
	m := NewMetrics(chain.New(s))

	m.RecordBlock()
	m.RecordBlock()
	m.RecordBlock()

	snap := m.Snapshot()
	if snap["blocks_processed"] != 3 {
		t.Errorf("blocks: got %d, want 3", snap["blocks_processed"])
	}
}

func TestMetrics_Concurrent_Ugly(t *testing.T) {
	s, _ := store.New(t.TempDir() + "/test.db")
	defer s.Close()
	m := NewMetrics(chain.New(s))

	done := make(chan struct{})
	for i := 0; i < 100; i++ {
		go func() {
			m.RecordBlock()
			m.RecordAlias()
			m.RecordSyncError()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 100; i++ {
		<-done
	}

	snap := m.Snapshot()
	if snap["blocks_processed"] != 100 {
		t.Errorf("concurrent blocks: got %d, want 100", snap["blocks_processed"])
	}
}
