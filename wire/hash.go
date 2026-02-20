// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wire

import (
	"bytes"

	"forge.lthn.ai/core/go-blockchain/types"
)

// BlockHashingBlob builds the blob used to compute a block's hash.
//
// The format (from currency_format_utils_blocks.cpp) is:
//
//	serialised_block_header || tree_root_hash || varint(tx_count)
//
// where tx_count = 1 (miner_tx) + len(tx_hashes).
func BlockHashingBlob(b *types.Block) []byte {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeBlockHeader(enc, &b.BlockHeader)

	// Compute tree hash over all transaction hashes.
	txCount := 1 + len(b.TxHashes)
	hashes := make([][32]byte, txCount)
	hashes[0] = [32]byte(TransactionPrefixHash(&b.MinerTx))
	for i, h := range b.TxHashes {
		hashes[i+1] = [32]byte(h)
	}
	treeRoot := TreeHash(hashes)

	buf.Write(treeRoot[:])
	buf.Write(EncodeVarint(uint64(txCount)))
	return buf.Bytes()
}

// BlockHash computes the block ID (Keccak-256 of the block hashing blob).
//
// The C++ code calls get_object_hash(blobdata) which serialises the string
// through binary_archive before hashing. For std::string this prepends a
// varint length prefix, so the actual hash input is:
//
//	varint(len(blob)) || blob
func BlockHash(b *types.Block) types.Hash {
	blob := BlockHashingBlob(b)
	var prefixed []byte
	prefixed = append(prefixed, EncodeVarint(uint64(len(blob)))...)
	prefixed = append(prefixed, blob...)
	return types.Hash(Keccak256(prefixed))
}

// TransactionHash computes the full transaction hash (tx_id).
//
// For v0/v1 transactions this is Keccak-256 of the full serialised transaction
// (prefix + signatures + attachment). For v2+ it delegates to the prefix hash
// (Zano computes v2+ hashes from prefix data only).
func TransactionHash(tx *types.Transaction) types.Hash {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransaction(enc, tx)
	return types.Hash(Keccak256(buf.Bytes()))
}

// TransactionPrefixHash computes the hash of a transaction prefix.
// This is Keccak-256 of the serialised transaction prefix (version + vin +
// vout + extra, in version-dependent order).
func TransactionPrefixHash(tx *types.Transaction) types.Hash {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransactionPrefix(enc, tx)
	return types.Hash(Keccak256(buf.Bytes()))
}
