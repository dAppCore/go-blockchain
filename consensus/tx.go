// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package consensus

import (
	"fmt"

	coreerr "dappco.re/go/core/log"

	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/types"
)

// ValidateTransaction performs semantic validation on a regular (non-coinbase)
// transaction. Checks are ordered to match the C++ validate_tx_semantic().
func ValidateTransaction(tx *types.Transaction, txBlob []byte, forks []config.HardFork, height uint64) error {
	hf4Active := config.IsHardForkActive(forks, config.HF4Zarcanum, height)

	// 0. Transaction version.
	if err := checkTxVersion(tx, forks, height); err != nil {
		return err
	}

	// 1. Blob size.
	if uint64(len(txBlob)) >= config.MaxTransactionBlobSize {
		return coreerr.E("ValidateTransaction", fmt.Sprintf("%d bytes", len(txBlob)), ErrTxTooLarge)
	}

	// 2. Input count.
	if len(tx.Vin) == 0 {
		return ErrNoInputs
	}
	if uint64(len(tx.Vin)) > config.TxMaxAllowedInputs {
		return coreerr.E("ValidateTransaction", fmt.Sprintf("%d", len(tx.Vin)), ErrTooManyInputs)
	}

	hf1Active := config.IsHardForkActive(forks, config.HF1, height)

	// 3. Input types — TxInputGenesis not allowed in regular transactions.
	if err := checkInputTypes(tx, hf1Active, hf4Active); err != nil {
		return err
	}

	// 4. Output validation.
	if err := checkOutputs(tx, hf1Active, hf4Active); err != nil {
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

// checkTxVersion validates that the transaction version is appropriate for the
// current hardfork era.
//
// After HF5: transaction version must be >= VersionPostHF5 (3).
// Before HF5: transaction version 3 is rejected (too early).
func checkTxVersion(tx *types.Transaction, forks []config.HardFork, height uint64) error {
	hf5Active := config.IsHardForkActive(forks, config.HF5, height)

	if hf5Active && tx.Version < types.VersionPostHF5 {
		return coreerr.E("checkTxVersion",
			fmt.Sprintf("version %d too low after HF5 at height %d", tx.Version, height),
			ErrTxVersionInvalid)
	}

	if !hf5Active && tx.Version >= types.VersionPostHF5 {
		return coreerr.E("checkTxVersion",
			fmt.Sprintf("version %d not allowed before HF5 at height %d", tx.Version, height),
			ErrTxVersionInvalid)
	}

	return nil
}

func checkInputTypes(tx *types.Transaction, hf1Active, hf4Active bool) error {
	for _, vin := range tx.Vin {
		switch vin.(type) {
		case types.TxInputToKey:
			// Always valid.
		case types.TxInputGenesis:
			return coreerr.E("checkInputTypes", "txin_gen in regular transaction", ErrInvalidInputType)
		case types.TxInputHTLC, types.TxInputMultisig:
			// HTLC and multisig inputs require at least HF1.
			if !hf1Active {
				return coreerr.E("checkInputTypes", fmt.Sprintf("tag %d pre-HF1", vin.InputType()), ErrInvalidInputType)
			}
		default:
			// Future types (ZC) — accept if HF4+.
			if !hf4Active {
				return coreerr.E("checkInputTypes", fmt.Sprintf("tag %d pre-HF4", vin.InputType()), ErrInvalidInputType)
			}
		}
	}
	return nil
}

func checkOutputs(tx *types.Transaction, hf1Active, hf4Active bool) error {
	if len(tx.Vout) == 0 {
		return ErrNoOutputs
	}

	if hf4Active && uint64(len(tx.Vout)) < config.TxMinAllowedOutputs {
		return coreerr.E("checkOutputs", fmt.Sprintf("%d (min %d)", len(tx.Vout), config.TxMinAllowedOutputs), ErrTooFewOutputs)
	}

	if uint64(len(tx.Vout)) > config.TxMaxAllowedOutputs {
		return coreerr.E("checkOutputs", fmt.Sprintf("%d", len(tx.Vout)), ErrTooManyOutputs)
	}

	for i, vout := range tx.Vout {
		switch o := vout.(type) {
		case types.TxOutputBare:
			if o.Amount == 0 {
				return coreerr.E("checkOutputs", fmt.Sprintf("output %d has zero amount", i), ErrInvalidOutput)
			}
			// HTLC and Multisig output targets require at least HF1.
			switch o.Target.(type) {
			case types.TxOutHTLC, types.TxOutMultisig:
				if !hf1Active {
					return coreerr.E("checkOutputs", fmt.Sprintf("output %d: HTLC/multisig target pre-HF1", i), ErrInvalidOutput)
				}
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
		var ki types.KeyImage
		switch v := vin.(type) {
		case types.TxInputToKey:
			ki = v.KeyImage
		case types.TxInputHTLC:
			ki = v.KeyImage
		default:
			continue
		}
		if _, exists := seen[ki]; exists {
			return coreerr.E("checkKeyImages", ki.String(), ErrDuplicateKeyImage)
		}
		seen[ki] = struct{}{}
	}
	return nil
}
