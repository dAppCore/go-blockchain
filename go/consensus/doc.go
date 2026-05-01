// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

// Package consensus implements Lethean blockchain validation rules.
//
// Validation is organised in three layers:
//
//   - Structural: transaction size, input/output counts, key image
//     uniqueness. No cryptographic operations required.
//   - Economic: block reward calculation, fee extraction, balance
//     checks, overflow detection.
//   - Cryptographic: PoW hash verification (RandomX via CGo),
//     ring signature verification, proof verification.
//
// All functions take *config.ChainConfig and a block height for
// hardfork-aware validation. The package has no dependency on chain/.
package consensus
