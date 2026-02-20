// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// You may obtain a copy of the licence at:
//
//     https://joinup.ec.europa.eu/software/page/eupl/licence-eupl
//
// SPDX-License-Identifier: EUPL-1.2

package types

// BlockHeader contains the fields present in every block header. These fields
// are consensus-critical and must be serialised in the exact order defined by
// the CryptoNote wire format.
type BlockHeader struct {
	// MajorVersion determines which consensus rules apply to this block.
	// The version increases at hardfork boundaries.
	MajorVersion uint8

	// MinorVersion is used for soft-fork signalling within a major version.
	MinorVersion uint8

	// Timestamp is the Unix epoch time (seconds) when the block was created.
	// For PoS blocks this is the kernel timestamp; for PoW blocks it is the
	// miner's claimed time.
	Timestamp uint64

	// PrevID is the hash of the previous block in the chain.
	PrevID Hash

	// Nonce is the value iterated by the miner to find a valid PoW solution.
	// For PoS blocks this field carries the stake modifier.
	Nonce uint64
}

// Block is a complete block including the header, miner (coinbase) transaction,
// and the hashes of all other transactions included in the block.
type Block struct {
	BlockHeader

	// MinerTx is the coinbase transaction that pays the block reward.
	MinerTx Transaction

	// TxHashes contains the hashes of all non-coinbase transactions included
	// in this block, in the order they appear.
	TxHashes []Hash
}
