// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// You may obtain a copy of the licence at:
//
//     https://joinup.ec.europa.eu/software/page/eupl/licence-eupl
//
// SPDX-License-Identifier: EUPL-1.2

package wallet

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"forge.lthn.ai/core/go-blockchain/chain"
	"forge.lthn.ai/core/go-blockchain/rpc"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
	store "forge.lthn.ai/core/go-store"
)

const (
	scanHeightKey = "scan_height"
)

// Wallet ties together scanning, building, and sending.
type Wallet struct {
	account      *Account
	store        *store.Store
	chain        *chain.Chain
	client       *rpc.Client
	scanner      Scanner
	signer       Signer
	ringSelector RingSelector
	builder      Builder
}

// NewWallet creates a wallet with v1 defaults.
func NewWallet(account *Account, s *store.Store, c *chain.Chain,
	client *rpc.Client) *Wallet {

	scanner := NewV1Scanner(account)
	signer := &NLSAGSigner{}
	var ringSelector RingSelector
	var builder Builder
	if client != nil {
		ringSelector = NewRPCRingSelector(client)
		builder = NewV1Builder(signer, ringSelector)
	}

	return &Wallet{
		account:      account,
		store:        s,
		chain:        c,
		client:       client,
		scanner:      scanner,
		signer:       signer,
		ringSelector: ringSelector,
		builder:      builder,
	}
}

// Sync scans blocks from the last checkpoint to the chain tip.
func (w *Wallet) Sync() error {
	lastScanned := w.loadScanHeight()

	chainHeight, err := w.chain.Height()
	if err != nil {
		return fmt.Errorf("wallet: chain height: %w", err)
	}

	for h := lastScanned; h < chainHeight; h++ {
		blk, _, err := w.chain.GetBlockByHeight(h)
		if err != nil {
			return fmt.Errorf("wallet: get block %d: %w", h, err)
		}

		// Scan miner tx.
		if err := w.scanTx(&blk.MinerTx, h); err != nil {
			return err
		}

		// Scan regular transactions.
		for _, txHash := range blk.TxHashes {
			tx, _, err := w.chain.GetTransaction(txHash)
			if err != nil {
				continue // skip missing txs
			}
			if err := w.scanTx(tx, h); err != nil {
				return err
			}
		}

		w.saveScanHeight(h + 1)
	}

	return nil
}

func (w *Wallet) scanTx(tx *types.Transaction, blockHeight uint64) error {
	txHash := wire.TransactionHash(tx)
	extra, err := ParseTxExtra(tx.Extra)
	if err != nil {
		return nil // skip unparseable extras
	}

	// Detect owned outputs.
	transfers, err := w.scanner.ScanTransaction(tx, txHash, blockHeight, extra)
	if err != nil {
		return nil
	}
	for i := range transfers {
		if err := putTransfer(w.store, &transfers[i]); err != nil {
			return fmt.Errorf("wallet: store transfer: %w", err)
		}
	}

	// Check key images for spend detection.
	for _, vin := range tx.Vin {
		toKey, ok := vin.(types.TxInputToKey)
		if !ok {
			continue
		}
		// Try to mark any matching transfer as spent.
		tr, err := getTransfer(w.store, toKey.KeyImage)
		if err != nil {
			continue // not our transfer
		}
		if !tr.Spent {
			markTransferSpent(w.store, toKey.KeyImage, blockHeight)
		}
	}

	return nil
}

// Balance returns confirmed (spendable) and locked amounts.
func (w *Wallet) Balance() (confirmed, locked uint64, err error) {
	chainHeight, err := w.chain.Height()
	if err != nil {
		return 0, 0, err
	}

	transfers, err := listTransfers(w.store)
	if err != nil {
		return 0, 0, err
	}

	for _, tr := range transfers {
		if tr.Spent {
			continue
		}
		if tr.IsSpendable(chainHeight, false) {
			confirmed += tr.Amount
		} else {
			locked += tr.Amount
		}
	}

	return confirmed, locked, nil
}

// Send constructs and submits a transaction.
func (w *Wallet) Send(destinations []Destination, fee uint64) (*types.Transaction, error) {
	if w.builder == nil || w.client == nil {
		return nil, errors.New("wallet: no RPC client configured")
	}

	chainHeight, err := w.chain.Height()
	if err != nil {
		return nil, err
	}

	var destSum uint64
	for _, d := range destinations {
		destSum += d.Amount
	}
	needed := destSum + fee

	// Coin selection: largest-first greedy.
	transfers, err := listTransfers(w.store)
	if err != nil {
		return nil, err
	}

	// Filter spendable and sort by amount descending.
	var spendable []Transfer
	for _, tr := range transfers {
		if tr.IsSpendable(chainHeight, false) {
			spendable = append(spendable, tr)
		}
	}
	slices.SortFunc(spendable, func(a, b Transfer) int {
		return cmp.Compare(b.Amount, a.Amount) // descending
	})

	var selected []Transfer
	var selectedSum uint64
	for _, tr := range spendable {
		selected = append(selected, tr)
		selectedSum += tr.Amount
		if selectedSum >= needed {
			break
		}
	}
	if selectedSum < needed {
		return nil, fmt.Errorf("wallet: insufficient balance: have %d, need %d",
			selectedSum, needed)
	}

	req := &BuildRequest{
		Sources:       selected,
		Destinations:  destinations,
		Fee:           fee,
		SenderAddress: w.account.Address(),
	}

	tx, err := w.builder.Build(req)
	if err != nil {
		return nil, err
	}

	blob, err := SerializeTransaction(tx)
	if err != nil {
		return nil, err
	}

	if err := w.client.SendRawTransaction(blob); err != nil {
		return nil, fmt.Errorf("wallet: submit tx: %w", err)
	}

	// Optimistically mark sources as spent.
	for _, src := range selected {
		markTransferSpent(w.store, src.KeyImage, chainHeight)
	}

	return tx, nil
}

// Transfers returns all tracked transfers.
func (w *Wallet) Transfers() ([]Transfer, error) {
	return listTransfers(w.store)
}

func (w *Wallet) loadScanHeight() uint64 {
	val, err := w.store.Get(groupAccount, scanHeightKey)
	if err != nil {
		return 0
	}
	h, _ := strconv.ParseUint(val, 10, 64)
	return h
}

func (w *Wallet) saveScanHeight(h uint64) {
	w.store.Set(groupAccount, scanHeightKey, strconv.FormatUint(h, 10))
}
