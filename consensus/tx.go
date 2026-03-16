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
	hf1Active := config.IsHardForkActive(forks, config.HF1, height)
	hf4Active := config.IsHardForkActive(forks, config.HF4Zarcanum, height)

	// 0. Transaction version for current hardfork.
	if err := checkTxVersion(tx, forks, height); err != nil {
		return err
	}

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

func checkInputTypes(tx *types.Transaction, hf1Active, hf4Active bool) error {
	for _, vin := range tx.Vin {
		switch vin.(type) {
		case types.TxInputToKey:
			// Always valid.
		case types.TxInputGenesis:
			return fmt.Errorf("%w: txin_gen in regular transaction", ErrInvalidInputType)
		case types.TxInputHTLC, types.TxInputMultisig:
			if !hf1Active {
				return fmt.Errorf("%w: tag %d pre-HF1", ErrInvalidInputType, vin.InputType())
			}
		default:
			// Future types (ZC, etc.) — accept if HF4+.
			if !hf4Active {
				return fmt.Errorf("%w: tag %d pre-HF4", ErrInvalidInputType, vin.InputType())
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
			// Check target type gating.
			switch o.Target.(type) {
			case types.TxOutToKey:
				// Always valid.
			case types.TxOutMultisig, types.TxOutHTLC:
				if !hf1Active {
					return fmt.Errorf("%w: output %d has target type %d pre-HF1",
						ErrInvalidOutput, i, o.Target.TargetType())
				}
			}
		case types.TxOutputZarcanum:
			// Validated by proof verification.
		}
	}

	return nil
}

// checkTxVersion validates that the transaction version is correct for the
// current hardfork era. After HF5, version must be 3. Before HF5, version 3
// is rejected.
func checkTxVersion(tx *types.Transaction, forks []config.HardFork, height uint64) error {
	hf5Active := config.IsHardForkActive(forks, config.HF5, height)

	if hf5Active {
		// After HF5: must be version 3.
		if tx.Version != types.VersionPostHF5 {
			return fmt.Errorf("%w: got version %d, require %d after HF5",
				ErrTxVersionInvalid, tx.Version, types.VersionPostHF5)
		}
	} else {
		// Before HF5: version 3 is not allowed.
		if tx.Version >= types.VersionPostHF5 {
			return fmt.Errorf("%w: version %d not allowed before HF5",
				ErrTxVersionInvalid, tx.Version)
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
			return fmt.Errorf("%w: %s", ErrDuplicateKeyImage, ki)
		}
		seen[ki] = struct{}{}
	}
	return nil
}
