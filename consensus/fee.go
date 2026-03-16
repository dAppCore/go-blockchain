// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package consensus

import (
	"fmt"
	"math"

	coreerr "forge.lthn.ai/core/go-log"

	"forge.lthn.ai/core/go-blockchain/types"
)

// TxFee calculates the transaction fee for pre-HF4 (v0/v1) transactions.
// Coinbase transactions return 0. For standard transactions, fee equals
// the difference between total input amounts and total output amounts.
func TxFee(tx *types.Transaction) (uint64, error) {
	if isCoinbase(tx) {
		return 0, nil
	}

	inputSum, err := sumInputs(tx)
	if err != nil {
		return 0, err
	}

	outputSum, err := sumOutputs(tx)
	if err != nil {
		return 0, err
	}

	if outputSum > inputSum {
		return 0, coreerr.E("TxFee", fmt.Sprintf("inputs=%d, outputs=%d", inputSum, outputSum), ErrNegativeFee)
	}

	return inputSum - outputSum, nil
}

// isCoinbase returns true if the transaction's first input is TxInputGenesis.
func isCoinbase(tx *types.Transaction) bool {
	if len(tx.Vin) == 0 {
		return false
	}
	_, ok := tx.Vin[0].(types.TxInputGenesis)
	return ok
}

// sumInputs totals all transparent input amounts, checking for overflow.
// Covers TxInputToKey, TxInputHTLC, and TxInputMultisig.
func sumInputs(tx *types.Transaction) (uint64, error) {
	var total uint64
	for _, vin := range tx.Vin {
		var amount uint64
		switch v := vin.(type) {
		case types.TxInputToKey:
			amount = v.Amount
		case types.TxInputHTLC:
			amount = v.Amount
		case types.TxInputMultisig:
			amount = v.Amount
		default:
			continue
		}
		if total > math.MaxUint64-amount {
			return 0, ErrInputOverflow
		}
		total += amount
	}
	return total, nil
}

// sumOutputs totals all TxOutputBare amounts, checking for overflow.
func sumOutputs(tx *types.Transaction) (uint64, error) {
	var total uint64
	for _, vout := range tx.Vout {
		bare, ok := vout.(types.TxOutputBare)
		if !ok {
			continue
		}
		if total > math.MaxUint64-bare.Amount {
			return 0, ErrOutputOverflow
		}
		total += bare.Amount
	}
	return total, nil
}
