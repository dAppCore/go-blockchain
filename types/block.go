// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// You may obtain a copy of the licence at:
//
//     https://joinup.ec.europa.eu/software/page/eupl/licence-eupl
//
// SPDX-License-Identifier: EUPL-1.2

package types

// BlockHeader contains the fields present in every block header. The fields
// are listed in wire serialisation order as defined by the C++ daemon
// (currency_basic.h:1123-1131).
//
// Wire format:
//
//	major_version   FIXED    uint8   (1 byte)
//	nonce           FIXED    uint64  (8 bytes LE)
//	prev_id         BLOB     hash    (32 bytes)
//	minor_version   VARINT   uint64
//	timestamp       VARINT   uint64
//	flags           FIXED    uint8   (1 byte)
type BlockHeader struct {
	// MajorVersion determines which consensus rules apply to this block.
	MajorVersion uint8

	// Nonce is iterated by the miner to find a valid PoW solution.
	// For PoS blocks this carries the stake modifier.
	Nonce uint64

	// PrevID is the hash of the previous block in the chain.
	PrevID Hash

	// MinorVersion is used for soft-fork signalling within a major version.
	// Encoded as varint on wire (uint64).
	MinorVersion uint64

	// Timestamp is the Unix epoch time (seconds) when the block was created.
	// Encoded as varint on wire.
	Timestamp uint64

	// Flags encodes block properties (e.g. PoS vs PoW).
	Flags uint8
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
