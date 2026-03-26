// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package consensus

import "dappco.re/go/core"

// Sentinel errors for consensus validation failures.
var (
	// Transaction structural errors.
	ErrTxTooLarge        = core.E("", "consensus: transaction too large", nil)
	ErrNoInputs          = core.E("", "consensus: transaction has no inputs", nil)
	ErrTooManyInputs     = core.E("", "consensus: transaction exceeds max inputs", nil)
	ErrInvalidInputType  = core.E("", "consensus: unsupported input type", nil)
	ErrNoOutputs         = core.E("", "consensus: transaction has no outputs", nil)
	ErrTooFewOutputs     = core.E("", "consensus: transaction below min outputs", nil)
	ErrTooManyOutputs    = core.E("", "consensus: transaction exceeds max outputs", nil)
	ErrInvalidOutput     = core.E("", "consensus: invalid output", nil)
	ErrDuplicateKeyImage = core.E("", "consensus: duplicate key image in transaction", nil)
	ErrInvalidExtra      = core.E("", "consensus: invalid extra field", nil)
	ErrTxVersionInvalid  = core.E("", "consensus: invalid transaction version for current hardfork", nil)
	ErrPreHardforkFreeze = core.E("", "consensus: non-coinbase transaction rejected during pre-hardfork freeze", nil)

	// Transaction economic errors.
	ErrInputOverflow  = core.E("", "consensus: input amount overflow", nil)
	ErrOutputOverflow = core.E("", "consensus: output amount overflow", nil)
	ErrNegativeFee    = core.E("", "consensus: outputs exceed inputs", nil)

	// Block errors.
	ErrBlockTooLarge     = core.E("", "consensus: block exceeds max size", nil)
	ErrBlockMajorVersion = core.E("", "consensus: invalid block major version for height", nil)
	ErrTimestampFuture   = core.E("", "consensus: block timestamp too far in future", nil)
	ErrTimestampOld      = core.E("", "consensus: block timestamp below median", nil)
	ErrMinerTxInputs     = core.E("", "consensus: invalid miner transaction inputs", nil)
	ErrMinerTxHeight     = core.E("", "consensus: miner transaction height mismatch", nil)
	ErrMinerTxUnlock     = core.E("", "consensus: miner transaction unlock time invalid", nil)
	ErrRewardMismatch    = core.E("", "consensus: block reward mismatch", nil)
	ErrMinerTxProofs     = core.E("", "consensus: miner transaction proof count invalid", nil)

	// ErrBlockVersion is an alias for ErrBlockMajorVersion, used by
	// checkBlockVersion when the block major version does not match
	// the expected version for the height in the hardfork schedule.
	ErrBlockVersion = ErrBlockMajorVersion
)
