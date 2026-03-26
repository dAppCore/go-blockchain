// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package consensus

import (
	"math/bits"

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"

	"dappco.re/go/core/blockchain/config"
)

// BaseReward returns the base block reward at the given height.
// Height 0 (genesis) returns the premine amount. All other heights
// return the fixed block reward (1 LTHN).
// Usage: consensus.BaseReward(...)
func BaseReward(height uint64) uint64 {
	if height == 0 {
		return config.Premine
	}
	return config.BlockReward
}

// BlockReward applies the size penalty to a base reward. If the block
// is within the granted full reward zone, the full base reward is returned.
// If the block exceeds 2*medianSize, an error is returned.
//
// The penalty formula matches the C++ get_block_reward():
//
//	reward = baseReward * (2*median - size) * size / median²
//
// Uses math/bits.Mul64 for 128-bit intermediate products to avoid overflow.
// Usage: consensus.BlockReward(...)
func BlockReward(baseReward, blockSize, medianSize uint64) (uint64, error) {
	effectiveMedian := medianSize
	if effectiveMedian < config.BlockGrantedFullRewardZone {
		effectiveMedian = config.BlockGrantedFullRewardZone
	}

	if blockSize <= effectiveMedian {
		return baseReward, nil
	}

	if blockSize > 2*effectiveMedian {
		return 0, coreerr.E("BlockReward", core.Sprintf("consensus: block size %d too large for median %d", blockSize, effectiveMedian), nil)
	}

	// penalty = baseReward * (2*median - size) * size / median²
	// Use 128-bit multiplication to avoid overflow.
	twoMedian := 2 * effectiveMedian
	factor := twoMedian - blockSize // (2*median - size)

	// hi1, lo1 = factor * blockSize
	hi1, lo1 := bits.Mul64(factor, blockSize)

	// Since hi1 should be 0 for reasonable block sizes, simplify:
	if hi1 > 0 {
		return 0, coreerr.E("BlockReward", "consensus: reward overflow", nil)
	}
	hi2, lo2 := bits.Mul64(baseReward, lo1)

	// Divide 128-bit result by median².
	medianSq_hi, medianSq_lo := bits.Mul64(effectiveMedian, effectiveMedian)
	_ = medianSq_hi // median² fits in 64 bits for any reasonable median

	reward, _ := bits.Div64(hi2, lo2, medianSq_lo)
	return reward, nil
}

// MinerReward calculates the total miner payout. Pre-HF4, transaction
// fees are added to the base reward. Post-HF4 (postHF4=true), fees are
// burned and the miner receives only the base reward.
// Usage: consensus.MinerReward(...)
func MinerReward(baseReward, totalFees uint64, postHF4 bool) uint64 {
	if postHF4 {
		return baseReward
	}
	return baseReward + totalFees
}
