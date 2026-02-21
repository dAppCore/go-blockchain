// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package consensus

import (
	"fmt"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-blockchain/types"
)

// ValidateTransaction performs semantic validation on a regular (non-coinbase)
// transaction. Checks are ordered to match the C++ validate_tx_semantic().
func ValidateTransaction(tx *types.Transaction, txBlob []byte, forks []config.HardFork, height uint64) error {
	hf4Active := config.IsHardForkActive(forks, config.HF4Zarcanum, height)

	// 1. Blob size.
	if uint64(len(txBlob)) >= config.MaxTransactionBlobSize {
		return fmt.Errorf("%w: %d bytes", ErrTxTooLarge, len(txBlob))
	}

	// 2. Input count.
	if len(tx.Vin) == 0 {
		return ErrNoInputs
	}
	if uint64(len(tx.Vin)) > config.TxMaxAllowedInputs {
		return fmt.Errorf("%w: %d", ErrTooManyInputs, len(tx.Vin))
	}

	// 3. Input types — TxInputGenesis not allowed in regular transactions.
	if err := checkInputTypes(tx, hf4Active); err != nil {
		return err
	}

	// 4. Output validation.
	if err := checkOutputs(tx, hf4Active); err != nil {
		return err
	}

	// 5. Money overflow.
	if _, err := sumInputs(tx); err != nil {
		return err
	}
	if _, err := sumOutputs(tx); err != nil {
		return err
	}

	// 6. Key image uniqueness.
	if err := checkKeyImages(tx); err != nil {
		return err
	}

	// 7. Balance check (pre-HF4 only — post-HF4 uses commitment proofs).
	if !hf4Active {
		if _, err := TxFee(tx); err != nil {
			return err
		}
	}

	return nil
}

func checkInputTypes(tx *types.Transaction, hf4Active bool) error {
	for _, vin := range tx.Vin {
		switch vin.(type) {
		case types.TxInputToKey:
			// Always valid.
		case types.TxInputGenesis:
			return fmt.Errorf("%w: txin_gen in regular transaction", ErrInvalidInputType)
		default:
			// Future types (multisig, HTLC, ZC) — accept if HF4+.
			if !hf4Active {
				return fmt.Errorf("%w: tag %d pre-HF4", ErrInvalidInputType, vin.InputType())
			}
		}
	}
	return nil
}

func checkOutputs(tx *types.Transaction, hf4Active bool) error {
	if len(tx.Vout) == 0 {
		return ErrNoOutputs
	}

	if hf4Active && uint64(len(tx.Vout)) < config.TxMinAllowedOutputs {
		return fmt.Errorf("%w: %d (min %d)", ErrTooFewOutputs, len(tx.Vout), config.TxMinAllowedOutputs)
	}

	if uint64(len(tx.Vout)) > config.TxMaxAllowedOutputs {
		return fmt.Errorf("%w: %d", ErrTooManyOutputs, len(tx.Vout))
	}

	for i, vout := range tx.Vout {
		switch o := vout.(type) {
		case types.TxOutputBare:
			if o.Amount == 0 {
				return fmt.Errorf("%w: output %d has zero amount", ErrInvalidOutput, i)
			}
		case types.TxOutputZarcanum:
			// Validated by proof verification.
		}
	}

	return nil
}

func checkKeyImages(tx *types.Transaction) error {
	seen := make(map[types.KeyImage]struct{})
	for _, vin := range tx.Vin {
		toKey, ok := vin.(types.TxInputToKey)
		if !ok {
			continue
		}
		if _, exists := seen[toKey.KeyImage]; exists {
			return fmt.Errorf("%w: %s", ErrDuplicateKeyImage, toKey.KeyImage)
		}
		seen[toKey.KeyImage] = struct{}{}
	}
	return nil
}
