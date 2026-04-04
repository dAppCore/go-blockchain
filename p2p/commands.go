// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

// Package p2p implements the CryptoNote P2P protocol for the Lethean blockchain.
package p2p

import "dappco.re/go/core/p2p/node/levin"

// Re-export command IDs from the levin package for convenience.
const (
	// Usage: value := p2p.CommandHandshake
	CommandHandshake = levin.CommandHandshake
	// Usage: value := p2p.CommandTimedSync
	CommandTimedSync = levin.CommandTimedSync
	// Usage: value := p2p.CommandPing
	CommandPing = levin.CommandPing
	// Usage: value := p2p.CommandNewBlock
	CommandNewBlock = levin.CommandNewBlock
	// Usage: value := p2p.CommandNewTransactions
	CommandNewTransactions = levin.CommandNewTransactions
	// Usage: value := p2p.CommandRequestObjects
	CommandRequestObjects = levin.CommandRequestObjects
	// Usage: value := p2p.CommandResponseObjects
	CommandResponseObjects = levin.CommandResponseObjects
	// Usage: value := p2p.CommandRequestChain
	CommandRequestChain = levin.CommandRequestChain
	// Usage: value := p2p.CommandResponseChain
	CommandResponseChain = levin.CommandResponseChain
)
