package wallet

import (
	"testing"

	"dappco.re/go/core/blockchain/types"
)

func TestChainScanner_New_Good(t *testing.T) {
	account, _ := GenerateAccount()
	getter := func(h uint64) (*types.Block, []types.Transaction, error) {
		return nil, nil, nil
	}
	scanner := NewChainScanner(account, getter)
	if scanner == nil {
		t.Fatal("scanner is nil")
	}
}

func TestChainScanner_ScanRange_Empty_Good(t *testing.T) {
	account, _ := GenerateAccount()
	getter := func(h uint64) (*types.Block, []types.Transaction, error) {
		return nil, nil, nil
	}
	scanner := NewChainScanner(account, getter)
	transfers, scanned := scanner.ScanRange(0, 10)
	if len(transfers) != 0 {
		t.Errorf("expected 0 transfers, got %d", len(transfers))
	}
	if scanned != 0 {
		t.Errorf("expected 0 scanned (nil blocks), got %d", scanned)
	}
}
