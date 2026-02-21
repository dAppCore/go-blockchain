// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package consensus

import "errors"

// Sentinel errors for consensus validation failures.
var (
	// Transaction structural errors.
	ErrTxTooLarge        = errors.New("consensus: transaction too large")
	ErrNoInputs          = errors.New("consensus: transaction has no inputs")
	ErrTooManyInputs     = errors.New("consensus: transaction exceeds max inputs")
	ErrInvalidInputType  = errors.New("consensus: unsupported input type")
	ErrNoOutputs         = errors.New("consensus: transaction has no outputs")
	ErrTooFewOutputs     = errors.New("consensus: transaction below min outputs")
	ErrTooManyOutputs    = errors.New("consensus: transaction exceeds max outputs")
	ErrInvalidOutput     = errors.New("consensus: invalid output")
	ErrDuplicateKeyImage = errors.New("consensus: duplicate key image in transaction")
	ErrInvalidExtra      = errors.New("consensus: invalid extra field")

	// Transaction economic errors.
	ErrInputOverflow  = errors.New("consensus: input amount overflow")
	ErrOutputOverflow = errors.New("consensus: output amount overflow")
	ErrNegativeFee    = errors.New("consensus: outputs exceed inputs")

	// Block errors.
	ErrBlockTooLarge   = errors.New("consensus: block exceeds max size")
	ErrTimestampFuture = errors.New("consensus: block timestamp too far in future")
	ErrTimestampOld    = errors.New("consensus: block timestamp below median")
	ErrMinerTxInputs   = errors.New("consensus: invalid miner transaction inputs")
	ErrMinerTxHeight   = errors.New("consensus: miner transaction height mismatch")
	ErrMinerTxUnlock   = errors.New("consensus: miner transaction unlock time invalid")
	ErrRewardMismatch  = errors.New("consensus: block reward mismatch")
	ErrMinerTxProofs   = errors.New("consensus: miner transaction proof count invalid")
)
