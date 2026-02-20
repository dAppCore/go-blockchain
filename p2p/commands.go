// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

// Package p2p implements the CryptoNote P2P protocol for the Lethean blockchain.
package p2p

import "forge.lthn.ai/core/go-p2p/node/levin"

// Re-export command IDs from the levin package for convenience.
const (
	CommandHandshake       = levin.CommandHandshake
	CommandTimedSync       = levin.CommandTimedSync
	CommandPing            = levin.CommandPing
	CommandNewBlock        = levin.CommandNewBlock
	CommandNewTransactions = levin.CommandNewTransactions
	CommandRequestObjects  = levin.CommandRequestObjects
	CommandResponseObjects = levin.CommandResponseObjects
	CommandRequestChain    = levin.CommandRequestChain
	CommandResponseChain   = levin.CommandResponseChain
)
