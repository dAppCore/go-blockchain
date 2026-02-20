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

	store "forge.lthn.ai/core/go-store"
	"forge.lthn.ai/core/go-blockchain/types"
)

// MarkSpent records a key image as spent at the given block height.
func (c *Chain) MarkSpent(ki types.KeyImage, height uint64) error {
	if err := c.store.Set(groupSpentKeys, ki.String(), strconv.FormatUint(height, 10)); err != nil {
		return fmt.Errorf("chain: mark spent %s: %w", ki, err)
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
		return false, fmt.Errorf("chain: check spent %s: %w", ki, err)
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
		return 0, fmt.Errorf("chain: output count: %w", err)
	}
	gindex := uint64(count)

	entry := outputEntry{
		TxID:  txID.String(),
		OutNo: outNo,
	}
	val, err := json.Marshal(entry)
	if err != nil {
		return 0, fmt.Errorf("chain: marshal output: %w", err)
	}

	key := strconv.FormatUint(gindex, 10)
	if err := c.store.Set(grp, key, string(val)); err != nil {
		return 0, fmt.Errorf("chain: store output: %w", err)
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
			return types.Hash{}, 0, fmt.Errorf("chain: output %d:%d not found", amount, gindex)
		}
		return types.Hash{}, 0, fmt.Errorf("chain: get output: %w", err)
	}

	var entry outputEntry
	if err := json.Unmarshal([]byte(val), &entry); err != nil {
		return types.Hash{}, 0, fmt.Errorf("chain: unmarshal output: %w", err)
	}
	hash, err := types.HashFromHex(entry.TxID)
	if err != nil {
		return types.Hash{}, 0, fmt.Errorf("chain: parse output tx_id: %w", err)
	}
	return hash, entry.OutNo, nil
}

// OutputCount returns the number of outputs indexed for the given amount.
func (c *Chain) OutputCount(amount uint64) (uint64, error) {
	n, err := c.store.Count(outputGroup(amount))
	if err != nil {
		return 0, fmt.Errorf("chain: output count: %w", err)
	}
	return uint64(n), nil
}
