// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package consensus

import (
	"fmt"
	"sort"

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
		return fmt.Errorf("%w: %d > %d + %d", ErrTimestampFuture,
			blockTimestamp, adjustedTime, limit)
	}

	// Median check — only when we have enough history.
	if uint64(len(recentTimestamps)) < config.TimestampCheckWindow {
		return nil
	}

	median := medianTimestamp(recentTimestamps)
	if blockTimestamp < median {
		return fmt.Errorf("%w: %d < median %d", ErrTimestampOld,
			blockTimestamp, median)
	}

	return nil
}

// medianTimestamp returns the median of a slice of timestamps.
func medianTimestamp(timestamps []uint64) uint64 {
	sorted := make([]uint64, len(timestamps))
	copy(sorted, timestamps)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

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
		return fmt.Errorf("%w: no inputs", ErrMinerTxInputs)
	}

	// First input must be TxInputGenesis.
	gen, ok := tx.Vin[0].(types.TxInputGenesis)
	if !ok {
		return fmt.Errorf("%w: first input is not txin_gen", ErrMinerTxInputs)
	}
	if gen.Height != height {
		return fmt.Errorf("%w: got %d, expected %d", ErrMinerTxHeight, gen.Height, height)
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
				return fmt.Errorf("%w: invalid PoS stake input type", ErrMinerTxInputs)
			}
			// Post-HF4: accept ZC inputs.
		}
	} else {
		return fmt.Errorf("%w: %d inputs (expected 1 or 2)", ErrMinerTxInputs, len(tx.Vin))
	}

	return nil
}
