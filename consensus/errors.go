// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package consensus

import "dappco.re/go/core"

// Sentinel errors for consensus validation failures.
var (
	// Transaction structural errors.
	// Usage: value := consensus.ErrTxTooLarge
	ErrTxTooLarge = core.E("", "consensus: transaction too large", nil)
	// Usage: value := consensus.ErrNoInputs
	ErrNoInputs = core.E("", "consensus: transaction has no inputs", nil)
	// Usage: value := consensus.ErrTooManyInputs
	ErrTooManyInputs = core.E("", "consensus: transaction exceeds max inputs", nil)
	// Usage: value := consensus.ErrInvalidInputType
	ErrInvalidInputType = core.E("", "consensus: unsupported input type", nil)
	// Usage: value := consensus.ErrNoOutputs
	ErrNoOutputs = core.E("", "consensus: transaction has no outputs", nil)
	// Usage: value := consensus.ErrTooFewOutputs
	ErrTooFewOutputs = core.E("", "consensus: transaction below min outputs", nil)
	// Usage: value := consensus.ErrTooManyOutputs
	ErrTooManyOutputs = core.E("", "consensus: transaction exceeds max outputs", nil)
	// Usage: value := consensus.ErrInvalidOutput
	ErrInvalidOutput = core.E("", "consensus: invalid output", nil)
	// Usage: value := consensus.ErrDuplicateKeyImage
	ErrDuplicateKeyImage = core.E("", "consensus: duplicate key image in transaction", nil)
	// Usage: value := consensus.ErrInvalidExtra
	ErrInvalidExtra = core.E("", "consensus: invalid extra field", nil)
	// Usage: value := consensus.ErrTxVersionInvalid
	ErrTxVersionInvalid = core.E("", "consensus: invalid transaction version for current hardfork", nil)
	// Usage: value := consensus.ErrPreHardforkFreeze
	ErrPreHardforkFreeze = core.E("", "consensus: non-coinbase transaction rejected during pre-hardfork freeze", nil)

	// Transaction economic errors.
	// Usage: value := consensus.ErrInputOverflow
	ErrInputOverflow = core.E("", "consensus: input amount overflow", nil)
	// Usage: value := consensus.ErrOutputOverflow
	ErrOutputOverflow = core.E("", "consensus: output amount overflow", nil)
	// Usage: value := consensus.ErrNegativeFee
	ErrNegativeFee = core.E("", "consensus: outputs exceed inputs", nil)

	// Block errors.
	// Usage: value := consensus.ErrBlockTooLarge
	ErrBlockTooLarge = core.E("", "consensus: block exceeds max size", nil)
	// Usage: value := consensus.ErrBlockMajorVersion
	ErrBlockMajorVersion = core.E("", "consensus: invalid block major version for height", nil)
	// Usage: value := consensus.ErrTimestampFuture
	ErrTimestampFuture = core.E("", "consensus: block timestamp too far in future", nil)
	// Usage: value := consensus.ErrTimestampOld
	ErrTimestampOld = core.E("", "consensus: block timestamp below median", nil)
	// Usage: value := consensus.ErrMinerTxInputs
	ErrMinerTxInputs = core.E("", "consensus: invalid miner transaction inputs", nil)
	// Usage: value := consensus.ErrMinerTxHeight
	ErrMinerTxHeight = core.E("", "consensus: miner transaction height mismatch", nil)
	// Usage: value := consensus.ErrMinerTxUnlock
	ErrMinerTxUnlock = core.E("", "consensus: miner transaction unlock time invalid", nil)
	// Usage: value := consensus.ErrRewardMismatch
	ErrRewardMismatch = core.E("", "consensus: block reward mismatch", nil)
	// Usage: value := consensus.ErrMinerTxProofs
	ErrMinerTxProofs = core.E("", "consensus: miner transaction proof count invalid", nil)

	// ErrBlockVersion is an alias for ErrBlockMajorVersion, used by
	// checkBlockVersion when the block major version does not match
	// the expected version for the height in the hardfork schedule.
	// Usage: value := consensus.ErrBlockVersion
	ErrBlockVersion = ErrBlockMajorVersion
)
