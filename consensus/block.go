// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package consensus

import (
	"fmt"
	"slices"

	coreerr "forge.lthn.ai/core/go-log"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-blockchain/types"
)

// IsPoS returns true if the block flags indicate a Proof-of-Stake block.
// Bit 0 of the flags byte is the PoS indicator.
func IsPoS(flags uint8) bool {
	return flags&1 != 0
}

// CheckTimestamp validates a block's timestamp against future limits and
// the median of recent timestamps.
func CheckTimestamp(blockTimestamp uint64, flags uint8, adjustedTime uint64, recentTimestamps []uint64) error {
	// Future time limit.
	limit := config.BlockFutureTimeLimit
	if IsPoS(flags) {
		limit = config.PosBlockFutureTimeLimit
	}
	if blockTimestamp > adjustedTime+limit {
		return coreerr.E("CheckTimestamp", fmt.Sprintf("%d > %d + %d",
			blockTimestamp, adjustedTime, limit), ErrTimestampFuture)
	}

	// Median check — only when we have enough history.
	if uint64(len(recentTimestamps)) < config.TimestampCheckWindow {
		return nil
	}

	median := medianTimestamp(recentTimestamps)
	if blockTimestamp < median {
		return coreerr.E("CheckTimestamp", fmt.Sprintf("%d < median %d",
			blockTimestamp, median), ErrTimestampOld)
	}

	return nil
}

// medianTimestamp returns the median of a slice of timestamps.
func medianTimestamp(timestamps []uint64) uint64 {
	sorted := make([]uint64, len(timestamps))
	copy(sorted, timestamps)
	slices.Sort(sorted)

	n := len(sorted)
	if n == 0 {
		return 0
	}
	return sorted[n/2]
}

// ValidateMinerTx checks the structure of a coinbase (miner) transaction.
// For PoW blocks: exactly 1 input (TxInputGenesis). For PoS blocks: exactly
// 2 inputs (TxInputGenesis + stake input).
func ValidateMinerTx(tx *types.Transaction, height uint64, forks []config.HardFork) error {
	if len(tx.Vin) == 0 {
		return coreerr.E("ValidateMinerTx", "no inputs", ErrMinerTxInputs)
	}

	// First input must be TxInputGenesis.
	gen, ok := tx.Vin[0].(types.TxInputGenesis)
	if !ok {
		return coreerr.E("ValidateMinerTx", "first input is not txin_gen", ErrMinerTxInputs)
	}
	if gen.Height != height {
		return coreerr.E("ValidateMinerTx", fmt.Sprintf("got %d, expected %d", gen.Height, height), ErrMinerTxHeight)
	}

	// PoW blocks: exactly 1 input. PoS: exactly 2.
	if len(tx.Vin) == 1 {
		// PoW — valid.
	} else if len(tx.Vin) == 2 {
		// PoS — second input must be a spend input.
		switch tx.Vin[1].(type) {
		case types.TxInputToKey:
			// Pre-HF4 PoS.
		default:
			hf4Active := config.IsHardForkActive(forks, config.HF4Zarcanum, height)
			if !hf4Active {
				return coreerr.E("ValidateMinerTx", "invalid PoS stake input type", ErrMinerTxInputs)
			}
			// Post-HF4: accept ZC inputs.
		}
	} else {
		return coreerr.E("ValidateMinerTx", fmt.Sprintf("%d inputs (expected 1 or 2)", len(tx.Vin)), ErrMinerTxInputs)
	}

	return nil
}

// ValidateBlockReward checks that the miner transaction outputs do not
// exceed the expected reward (base reward + fees for pre-HF4).
func ValidateBlockReward(minerTx *types.Transaction, height, blockSize, medianSize, totalFees uint64, forks []config.HardFork) error {
	base := BaseReward(height)
	reward, err := BlockReward(base, blockSize, medianSize)
	if err != nil {
		return err
	}

	hf4Active := config.IsHardForkActive(forks, config.HF4Zarcanum, height)
	expected := MinerReward(reward, totalFees, hf4Active)

	// Sum miner tx outputs.
	var outputSum uint64
	for _, vout := range minerTx.Vout {
		if bare, ok := vout.(types.TxOutputBare); ok {
			outputSum += bare.Amount
		}
	}

	if outputSum > expected {
		return coreerr.E("ValidateBlockReward", fmt.Sprintf("outputs %d > expected %d", outputSum, expected), ErrRewardMismatch)
	}

	return nil
}

// expectedBlockMajorVersion returns the expected block major version for a
// given height and fork schedule. This maps hardfork eras to block versions:
//
//	HF0 (genesis) -> 0
//	HF1           -> 1
//	HF3           -> 2
//	HF4+          -> 3
func expectedBlockMajorVersion(forks []config.HardFork, height uint64) uint8 {
	if config.IsHardForkActive(forks, config.HF4Zarcanum, height) {
		return config.CurrentBlockMajorVersion // 3
	}
	if config.IsHardForkActive(forks, config.HF3, height) {
		return config.HF3BlockMajorVersion // 2
	}
	if config.IsHardForkActive(forks, config.HF1, height) {
		return config.HF1BlockMajorVersion // 1
	}
	return config.BlockMajorVersionInitial // 0
}

// checkBlockVersion validates that the block's major version matches
// what is expected at the given height in the fork schedule.
func checkBlockVersion(blk *types.Block, forks []config.HardFork, height uint64) error {
	expected := expectedBlockMajorVersion(forks, height)
	if blk.MajorVersion != expected {
		return fmt.Errorf("%w: got %d, want %d at height %d",
			ErrBlockMajorVersion, blk.MajorVersion, expected, height)
	}
	return nil
}

// ValidateBlock performs full consensus validation on a block. It checks
// the block version, timestamp, miner transaction structure, and reward.
// Transaction semantic validation for regular transactions should be done
// separately via ValidateTransaction for each tx in the block.
func ValidateBlock(blk *types.Block, height, blockSize, medianSize, totalFees, adjustedTime uint64,
	recentTimestamps []uint64, forks []config.HardFork) error {

	// Block major version check.
	if err := checkBlockVersion(blk, forks, height); err != nil {
		return err
	}

	// Timestamp validation.
	if err := CheckTimestamp(blk.Timestamp, blk.Flags, adjustedTime, recentTimestamps); err != nil {
		return err
	}

	// Miner transaction structure.
	if err := ValidateMinerTx(&blk.MinerTx, height, forks); err != nil {
		return err
	}

	// Block reward.
	if err := ValidateBlockReward(&blk.MinerTx, height, blockSize, medianSize, totalFees, forks); err != nil {
		return err
	}

	return nil
}

// IsPreHardforkFreeze reports whether the given height falls within the
// pre-hardfork transaction freeze window for the specified fork version.
// The freeze window is the PreHardforkTxFreezePeriod blocks immediately
// before the fork activation height (inclusive).
//
// For a fork with activation height H (active at heights > H):
//
//	freeze applies at heights (H - period + 1) .. H
//
// Returns false if the fork version is not found or if the activation height
// is too low for a meaningful freeze window.
func IsPreHardforkFreeze(forks []config.HardFork, version uint8, height uint64) bool {
	activationHeight, ok := config.HardforkActivationHeight(forks, version)
	if !ok {
		return false
	}

	// A fork at height 0 means active from genesis — no freeze window.
	if activationHeight == 0 {
		return false
	}

	// Guard against underflow: if activation height < period, freeze starts at 1.
	freezeStart := uint64(1)
	if activationHeight >= config.PreHardforkTxFreezePeriod {
		freezeStart = activationHeight - config.PreHardforkTxFreezePeriod + 1
	}

	return height >= freezeStart && height <= activationHeight
}

// ValidateTransactionInBlock performs transaction validation including the
// pre-hardfork freeze check. This wraps ValidateTransaction with an
// additional check: during the freeze window before HF5, non-coinbase
// transactions are rejected.
func ValidateTransactionInBlock(tx *types.Transaction, txBlob []byte, forks []config.HardFork, height uint64) error {
	// Pre-hardfork freeze: reject non-coinbase transactions in the freeze window.
	if !isCoinbase(tx) && IsPreHardforkFreeze(forks, config.HF5, height) {
		return fmt.Errorf("%w: height %d is within HF5 freeze window", ErrPreHardforkFreeze, height)
	}

	return ValidateTransaction(tx, txBlob, forks, height)
}
