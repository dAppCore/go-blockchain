// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	coreerr "dappco.re/go/core/log"

	"dappco.re/go/core/blockchain/types"
	store "dappco.re/go/core/store"
)

// MarkSpent records a key image as spent at the given block height.
func (c *Chain) MarkSpent(ki types.KeyImage, height uint64) error {
	if err := c.store.Set(groupSpentKeys, ki.String(), strconv.FormatUint(height, 10)); err != nil {
		return coreerr.E("Chain.MarkSpent", fmt.Sprintf("chain: mark spent %s", ki), err)
	}
	return nil
}

// IsSpent checks whether a key image has been spent.
func (c *Chain) IsSpent(ki types.KeyImage) (bool, error) {
	_, err := c.store.Get(groupSpentKeys, ki.String())
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, coreerr.E("Chain.IsSpent", fmt.Sprintf("chain: check spent %s", ki), err)
	}
	return true, nil
}

// outputGroup returns the go-store group for outputs of the given amount.
func outputGroup(amount uint64) string {
	return groupOutputsPfx + strconv.FormatUint(amount, 10)
}

// PutOutput appends an output to the global index for the given amount.
// Returns the assigned global index.
func (c *Chain) PutOutput(amount uint64, txID types.Hash, outNo uint32) (uint64, error) {
	grp := outputGroup(amount)
	count, err := c.store.Count(grp)
	if err != nil {
		return 0, coreerr.E("Chain.PutOutput", "chain: output count", err)
	}
	gindex := uint64(count)

	entry := outputEntry{
		TxID:  txID.String(),
		OutNo: outNo,
	}
	val, err := json.Marshal(entry)
	if err != nil {
		return 0, coreerr.E("Chain.PutOutput", "chain: marshal output", err)
	}

	key := strconv.FormatUint(gindex, 10)
	if err := c.store.Set(grp, key, string(val)); err != nil {
		return 0, coreerr.E("Chain.PutOutput", "chain: store output", err)
	}
	return gindex, nil
}

// GetOutput retrieves an output by amount and global index.
func (c *Chain) GetOutput(amount uint64, gindex uint64) (types.Hash, uint32, error) {
	grp := outputGroup(amount)
	key := strconv.FormatUint(gindex, 10)
	val, err := c.store.Get(grp, key)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return types.Hash{}, 0, coreerr.E("Chain.GetOutput", fmt.Sprintf("chain: output %d:%d not found", amount, gindex), nil)
		}
		return types.Hash{}, 0, coreerr.E("Chain.GetOutput", "chain: get output", err)
	}

	var entry outputEntry
	if err := json.Unmarshal([]byte(val), &entry); err != nil {
		return types.Hash{}, 0, coreerr.E("Chain.GetOutput", "chain: unmarshal output", err)
	}
	hash, err := types.HashFromHex(entry.TxID)
	if err != nil {
		return types.Hash{}, 0, coreerr.E("Chain.GetOutput", "chain: parse output tx_id", err)
	}
	return hash, entry.OutNo, nil
}

// OutputCount returns the number of outputs indexed for the given amount.
func (c *Chain) OutputCount(amount uint64) (uint64, error) {
	n, err := c.store.Count(outputGroup(amount))
	if err != nil {
		return 0, coreerr.E("Chain.OutputCount", "chain: output count", err)
	}
	return uint64(n), nil
}
