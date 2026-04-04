// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wallet

import (
	"testing"

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"

	"dappco.re/go/core/blockchain/types"
)

// --- Good (happy path) ---

func TestChainScanner_NewChainScanner_Good(t *testing.T) {
	account, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	getter := func(height uint64) (*types.Block, []types.Transaction, error) {
		return nil, nil, nil
	}
	scanner := NewChainScanner(account, getter)
	if scanner == nil {
		t.Fatal("NewChainScanner returned nil")
	}
	if scanner.v1 == nil {
		t.Fatal("scanner.v1 is nil; expected a V1Scanner")
	}
	if scanner.getBlock == nil {
		t.Fatal("scanner.getBlock is nil; expected the getter function")
	}
}

func TestChainScanner_ScanRange_SingleBlock_Good(t *testing.T) {
	account, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	// Build a block whose miner tx is sent to our account.
	tx, _, _ := makeTestTransaction(t, account)

	getter := func(height uint64) (*types.Block, []types.Transaction, error) {
		blk := &types.Block{
			BlockHeader: types.BlockHeader{MajorVersion: 1, Timestamp: 1000},
			MinerTx:     *tx,
		}
		return blk, nil, nil
	}

	scanner := NewChainScanner(account, getter)
	transfers, scanned := scanner.ScanRange(0, 1)

	if scanned != 1 {
		t.Errorf("scanned: got %d, want 1", scanned)
	}
	// The miner tx has a valid output for our account, so we expect a transfer.
	if len(transfers) != 1 {
		t.Errorf("transfers: got %d, want 1", len(transfers))
	}
}

func TestChainScanner_ScanRange_MultipleBlocks_Good(t *testing.T) {
	account, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	tx, _, _ := makeTestTransaction(t, account)

	getter := func(height uint64) (*types.Block, []types.Transaction, error) {
		blk := &types.Block{
			BlockHeader: types.BlockHeader{MajorVersion: 1, Timestamp: 1000 + height*120},
			MinerTx:     *tx,
		}
		return blk, nil, nil
	}

	scanner := NewChainScanner(account, getter)
	transfers, scanned := scanner.ScanRange(0, 5)

	if scanned != 5 {
		t.Errorf("scanned: got %d, want 5", scanned)
	}
	// Each block has a miner tx for our account.
	if len(transfers) != 5 {
		t.Errorf("transfers: got %d, want 5", len(transfers))
	}
}

func TestChainScanner_ScanRange_WithRegularTxs_Good(t *testing.T) {
	account, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	minerTx, _, _ := makeTestTransaction(t, account)
	regularTx, _, _ := makeTestTransaction(t, account)

	getter := func(height uint64) (*types.Block, []types.Transaction, error) {
		blk := &types.Block{
			BlockHeader: types.BlockHeader{MajorVersion: 1},
			MinerTx:     *minerTx,
		}
		return blk, []types.Transaction{*regularTx}, nil
	}

	scanner := NewChainScanner(account, getter)
	transfers, scanned := scanner.ScanRange(0, 1)

	if scanned != 1 {
		t.Errorf("scanned: got %d, want 1", scanned)
	}
	// Both miner tx and regular tx should produce transfers.
	if len(transfers) != 2 {
		t.Errorf("transfers: got %d, want 2", len(transfers))
	}
}

// --- Bad (expected errors / graceful handling) ---

func TestChainScanner_ScanRange_GetterError_Bad(t *testing.T) {
	account, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	getter := func(height uint64) (*types.Block, []types.Transaction, error) {
		return nil, nil, coreerr.E("test", "block not found", nil)
	}

	scanner := NewChainScanner(account, getter)
	transfers, scanned := scanner.ScanRange(0, 10)

	// Getter errors cause the block to be skipped, not counted.
	if scanned != 0 {
		t.Errorf("scanned: got %d, want 0 (all errored)", scanned)
	}
	if len(transfers) != 0 {
		t.Errorf("transfers: got %d, want 0", len(transfers))
	}
}

func TestChainScanner_ScanRange_NonOwnedOutputs_Bad(t *testing.T) {
	account1, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}
	account2, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	// Build a tx for account1 but scan with account2's scanner.
	tx, _, _ := makeTestTransaction(t, account1)

	getter := func(height uint64) (*types.Block, []types.Transaction, error) {
		blk := &types.Block{
			BlockHeader: types.BlockHeader{MajorVersion: 1},
			MinerTx:     *tx,
		}
		return blk, nil, nil
	}

	scanner := NewChainScanner(account2, getter)
	transfers, scanned := scanner.ScanRange(0, 3)

	if scanned != 3 {
		t.Errorf("scanned: got %d, want 3", scanned)
	}
	if len(transfers) != 0 {
		t.Errorf("transfers: got %d, want 0 (non-owned outputs)", len(transfers))
	}
}

func TestChainScanner_ScanRange_StartEqualsEnd_Bad(t *testing.T) {
	account, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	called := false
	getter := func(height uint64) (*types.Block, []types.Transaction, error) {
		called = true
		return nil, nil, nil
	}

	scanner := NewChainScanner(account, getter)
	transfers, scanned := scanner.ScanRange(5, 5)

	if called {
		t.Error("getter should not be called when start == end")
	}
	if scanned != 0 {
		t.Errorf("scanned: got %d, want 0", scanned)
	}
	if len(transfers) != 0 {
		t.Errorf("transfers: got %d, want 0", len(transfers))
	}
}

// --- Ugly (edge cases) ---

func TestChainScanner_ScanRange_NilBlockReturned_Ugly(t *testing.T) {
	account, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	getter := func(height uint64) (*types.Block, []types.Transaction, error) {
		return nil, nil, nil
	}

	scanner := NewChainScanner(account, getter)
	transfers, scanned := scanner.ScanRange(0, 10)

	// Nil blocks are skipped (not counted as scanned).
	if scanned != 0 {
		t.Errorf("scanned: got %d, want 0 (nil blocks)", scanned)
	}
	if len(transfers) != 0 {
		t.Errorf("transfers: got %d, want 0", len(transfers))
	}
}

func TestChainScanner_ScanRange_ZeroRange_Ugly(t *testing.T) {
	account, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	getter := func(height uint64) (*types.Block, []types.Transaction, error) {
		return nil, nil, nil
	}

	scanner := NewChainScanner(account, getter)
	transfers, scanned := scanner.ScanRange(0, 0)

	if scanned != 0 {
		t.Errorf("scanned: got %d, want 0", scanned)
	}
	if len(transfers) != 0 {
		t.Errorf("transfers: got %d, want 0", len(transfers))
	}
}

func TestChainScanner_ScanRange_MixedNilAndValidBlocks_Ugly(t *testing.T) {
	account, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	tx, _, _ := makeTestTransaction(t, account)

	getter := func(height uint64) (*types.Block, []types.Transaction, error) {
		// Only even heights return a valid block.
		if height%2 == 0 {
			blk := &types.Block{
				BlockHeader: types.BlockHeader{MajorVersion: 1},
				MinerTx:     *tx,
			}
			return blk, nil, nil
		}
		return nil, nil, nil
	}

	scanner := NewChainScanner(account, getter)
	transfers, scanned := scanner.ScanRange(0, 6)

	// Heights 0, 2, 4 are valid (3 blocks); 1, 3, 5 are nil.
	if scanned != 3 {
		t.Errorf("scanned: got %d, want 3", scanned)
	}
	if len(transfers) != 3 {
		t.Errorf("transfers: got %d, want 3", len(transfers))
	}
}

func TestChainScanner_ScanRange_MixedErrorsAndValid_Ugly(t *testing.T) {
	account, err := GenerateAccount()
	if err != nil {
		t.Fatal(err)
	}

	tx, _, _ := makeTestTransaction(t, account)

	getter := func(height uint64) (*types.Block, []types.Transaction, error) {
		if height == 2 {
			return nil, nil, core.E("test", "db read error", nil)
		}
		blk := &types.Block{
			BlockHeader: types.BlockHeader{MajorVersion: 1},
			MinerTx:     *tx,
		}
		return blk, nil, nil
	}

	scanner := NewChainScanner(account, getter)
	transfers, scanned := scanner.ScanRange(0, 5)

	// 5 blocks requested, 1 errored, 4 valid.
	if scanned != 4 {
		t.Errorf("scanned: got %d, want 4", scanned)
	}
	if len(transfers) != 4 {
		t.Errorf("transfers: got %d, want 4", len(transfers))
	}
}

func TestChainScanner_NewChainScanner_NilAccount_Ugly(t *testing.T) {
	// NewChainScanner with nil account should not panic during construction.
	// (It will panic when scanning, but creation itself should be safe.)
	getter := func(height uint64) (*types.Block, []types.Transaction, error) {
		return nil, nil, nil
	}
	scanner := NewChainScanner(nil, getter)
	if scanner == nil {
		t.Fatal("NewChainScanner returned nil even with nil account")
	}
}
