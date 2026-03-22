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
	"encoding/json"
	"fmt"

	coreerr "dappco.re/go/core/log"

	store "dappco.re/go/core/store"

	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/types"
)

// groupTransfers is the go-store group name for wallet transfer records.
const groupTransfers = "transfers"

// KeyPair holds an ephemeral public/secret key pair for an owned output.
type KeyPair struct {
	Public types.PublicKey `json:"public"`
	Secret types.SecretKey `json:"secret"`
}

// Transfer represents an owned transaction output tracked by the wallet.
// Each transfer is keyed by its unique key image for double-spend detection.
type Transfer struct {
	TxHash       types.Hash     `json:"tx_hash"`
	OutputIndex  uint32         `json:"output_index"`
	Amount       uint64         `json:"amount"`
	GlobalIndex  uint64         `json:"global_index"`
	BlockHeight  uint64         `json:"block_height"`
	EphemeralKey KeyPair        `json:"ephemeral_key"`
	KeyImage     types.KeyImage `json:"key_image"`
	Spent        bool           `json:"spent"`
	SpentHeight  uint64         `json:"spent_height"`
	Coinbase     bool           `json:"coinbase"`
	UnlockTime   uint64         `json:"unlock_time"`
}

// IsSpendable reports whether the transfer can be used as an input at the
// given chain height. A transfer is not spendable if it has already been
// spent, if it is a coinbase output that has not yet matured, or if its
// unlock time has not been reached.
func (t *Transfer) IsSpendable(chainHeight uint64, _ bool) bool {
	if t.Spent {
		return false
	}
	if t.Coinbase && t.BlockHeight+config.MinedMoneyUnlockWindow > chainHeight {
		return false
	}
	if t.UnlockTime > 0 && t.UnlockTime > chainHeight {
		return false
	}
	return true
}

// putTransfer serialises a transfer as JSON and stores it in the given store,
// keyed by the transfer's key image hex string.
func putTransfer(s *store.Store, tr *Transfer) error {
	val, err := json.Marshal(tr)
	if err != nil {
		return coreerr.E("putTransfer", "wallet: marshal transfer", err)
	}
	return s.Set(groupTransfers, tr.KeyImage.String(), string(val))
}

// getTransfer retrieves and deserialises a transfer by its key image.
func getTransfer(s *store.Store, ki types.KeyImage) (*Transfer, error) {
	val, err := s.Get(groupTransfers, ki.String())
	if err != nil {
		return nil, coreerr.E("getTransfer", fmt.Sprintf("wallet: get transfer %s", ki), err)
	}
	var tr Transfer
	if err := json.Unmarshal([]byte(val), &tr); err != nil {
		return nil, coreerr.E("getTransfer", "wallet: unmarshal transfer", err)
	}
	return &tr, nil
}

// markTransferSpent sets the spent flag and records the height at which the
// transfer was consumed.
func markTransferSpent(s *store.Store, ki types.KeyImage, height uint64) error {
	tr, err := getTransfer(s, ki)
	if err != nil {
		return err
	}
	tr.Spent = true
	tr.SpentHeight = height
	return putTransfer(s, tr)
}

// listTransfers returns all transfers stored in the given store.
func listTransfers(s *store.Store) ([]Transfer, error) {
	pairs, err := s.GetAll(groupTransfers)
	if err != nil {
		return nil, coreerr.E("listTransfers", "wallet: list transfers", err)
	}
	transfers := make([]Transfer, 0, len(pairs))
	for _, val := range pairs {
		var tr Transfer
		if err := json.Unmarshal([]byte(val), &tr); err != nil {
			continue
		}
		transfers = append(transfers, tr)
	}
	return transfers, nil
}
