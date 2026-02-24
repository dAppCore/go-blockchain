// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package p2p

import "forge.lthn.ai/core/go-p2p/node/levin"

// NewBlockNotification is NOTIFY_NEW_BLOCK (2001).
type NewBlockNotification struct {
	BlockBlob []byte   // Serialised block
	TxBlobs   [][]byte // Serialised transactions
	Height    uint64   // Current blockchain height
}

// Encode serialises the notification.
func (n *NewBlockNotification) Encode() ([]byte, error) {
	blockEntry := levin.Section{
		"block": levin.StringVal(n.BlockBlob),
		"txs":   levin.StringArrayVal(n.TxBlobs),
	}
	s := levin.Section{
		"b":                         levin.ObjectVal(blockEntry),
		"current_blockchain_height": levin.Uint64Val(n.Height),
	}
	return levin.EncodeStorage(s)
}

// Decode parses a new block notification from a storage blob.
func (n *NewBlockNotification) Decode(data []byte) error {
	s, err := levin.DecodeStorage(data)
	if err != nil {
		return err
	}
	if v, ok := s["current_blockchain_height"]; ok {
		n.Height, _ = v.AsUint64()
	}
	if v, ok := s["b"]; ok {
		obj, _ := v.AsSection()
		if blk, ok := obj["block"]; ok {
			n.BlockBlob, _ = blk.AsString()
		}
		if txs, ok := obj["txs"]; ok {
			n.TxBlobs, _ = txs.AsStringArray()
		}
	}
	return nil
}

// NewTransactionsNotification is NOTIFY_NEW_TRANSACTIONS (2002).
type NewTransactionsNotification struct {
	TxBlobs [][]byte
}

// Encode serialises the notification.
func (n *NewTransactionsNotification) Encode() ([]byte, error) {
	s := levin.Section{
		"txs": levin.StringArrayVal(n.TxBlobs),
	}
	return levin.EncodeStorage(s)
}

// Decode parses a new transactions notification.
func (n *NewTransactionsNotification) Decode(data []byte) error {
	s, err := levin.DecodeStorage(data)
	if err != nil {
		return err
	}
	if v, ok := s["txs"]; ok {
		n.TxBlobs, _ = v.AsStringArray()
	}
	return nil
}

// BlockCompleteEntry holds a block blob and its transaction blobs.
type BlockCompleteEntry struct {
	Block []byte   // Serialised block
	Txs   [][]byte // Serialised transactions
}

// RequestGetObjects is NOTIFY_REQUEST_GET_OBJECTS (2003).
type RequestGetObjects struct {
	Blocks [][]byte // 32-byte block hashes
	Txs    [][]byte // 32-byte tx hashes (usually empty for sync)
}

// Encode serialises the request.
// The C++ daemon uses KV_SERIALIZE_CONTAINER_POD_AS_BLOB for both blocks
// and txs, so we pack all hashes into single concatenated blobs.
func (r *RequestGetObjects) Encode() ([]byte, error) {
	blocksBlob := make([]byte, 0, len(r.Blocks)*32)
	for _, id := range r.Blocks {
		blocksBlob = append(blocksBlob, id...)
	}
	s := levin.Section{
		"blocks": levin.StringVal(blocksBlob),
	}
	if len(r.Txs) > 0 {
		txsBlob := make([]byte, 0, len(r.Txs)*32)
		for _, id := range r.Txs {
			txsBlob = append(txsBlob, id...)
		}
		s["txs"] = levin.StringVal(txsBlob)
	}
	return levin.EncodeStorage(s)
}

// Decode parses a get-objects request from a storage blob.
func (r *RequestGetObjects) Decode(data []byte) error {
	s, err := levin.DecodeStorage(data)
	if err != nil {
		return err
	}
	if v, ok := s["blocks"]; ok {
		blob, _ := v.AsString()
		r.Blocks = splitHashes(blob, 32)
	}
	if v, ok := s["txs"]; ok {
		blob, _ := v.AsString()
		r.Txs = splitHashes(blob, 32)
	}
	return nil
}

// ResponseGetObjects is NOTIFY_RESPONSE_GET_OBJECTS (2004).
type ResponseGetObjects struct {
	Blocks        []BlockCompleteEntry
	MissedIDs     [][]byte
	CurrentHeight uint64
}

// Encode serialises the response.
func (r *ResponseGetObjects) Encode() ([]byte, error) {
	sections := make([]levin.Section, len(r.Blocks))
	for i, entry := range r.Blocks {
		sections[i] = levin.Section{
			"block": levin.StringVal(entry.Block),
			"txs":   levin.StringArrayVal(entry.Txs),
		}
	}
	s := levin.Section{
		"blocks":                    levin.ObjectArrayVal(sections),
		"current_blockchain_height": levin.Uint64Val(r.CurrentHeight),
	}
	if len(r.MissedIDs) > 0 {
		// missed_ids uses KV_SERIALIZE_CONTAINER_POD_AS_BLOB in C++.
		blob := make([]byte, 0, len(r.MissedIDs)*32)
		for _, id := range r.MissedIDs {
			blob = append(blob, id...)
		}
		s["missed_ids"] = levin.StringVal(blob)
	}
	return levin.EncodeStorage(s)
}

// Decode parses a get-objects response from a storage blob.
func (r *ResponseGetObjects) Decode(data []byte) error {
	s, err := levin.DecodeStorage(data)
	if err != nil {
		return err
	}
	if v, ok := s["current_blockchain_height"]; ok {
		r.CurrentHeight, _ = v.AsUint64()
	}
	if v, ok := s["blocks"]; ok {
		sections, _ := v.AsSectionArray()
		r.Blocks = make([]BlockCompleteEntry, len(sections))
		for i, sec := range sections {
			if blk, ok := sec["block"]; ok {
				r.Blocks[i].Block, _ = blk.AsString()
			}
			if txs, ok := sec["txs"]; ok {
				r.Blocks[i].Txs, _ = txs.AsStringArray()
			}
		}
	}
	if v, ok := s["missed_ids"]; ok {
		// missed_ids uses KV_SERIALIZE_CONTAINER_POD_AS_BLOB in C++.
		blob, _ := v.AsString()
		r.MissedIDs = splitHashes(blob, 32)
	}
	return nil
}

// RequestChain is NOTIFY_REQUEST_CHAIN (2006).
type RequestChain struct {
	BlockIDs [][]byte // Array of 32-byte block hashes
}

// Encode serialises the request.
// The C++ daemon uses KV_SERIALIZE_CONTAINER_POD_AS_BLOB for block_ids,
// so we pack all hashes into a single concatenated blob.
func (r *RequestChain) Encode() ([]byte, error) {
	blob := make([]byte, 0, len(r.BlockIDs)*32)
	for _, id := range r.BlockIDs {
		blob = append(blob, id...)
	}
	s := levin.Section{
		"block_ids": levin.StringVal(blob),
	}
	return levin.EncodeStorage(s)
}

// BlockContextInfo holds a block hash and cumulative size from a chain
// entry response. Mirrors the C++ block_context_info struct.
type BlockContextInfo struct {
	Hash      []byte // 32-byte block hash (KV_SERIALIZE_VAL_POD_AS_BLOB)
	CumulSize uint64 // Cumulative block size
}

// ResponseChainEntry is NOTIFY_RESPONSE_CHAIN_ENTRY (2007).
type ResponseChainEntry struct {
	StartHeight uint64
	TotalHeight uint64
	BlockIDs    [][]byte           // Convenience: just the hashes
	Blocks      []BlockContextInfo // Full entries with cumulative sizes
}

// Decode parses a chain entry response.
// m_block_ids is an object array of block_context_info, each with
// "h" (hash blob) and "cumul_size" (uint64).
func (r *ResponseChainEntry) Decode(data []byte) error {
	s, err := levin.DecodeStorage(data)
	if err != nil {
		return err
	}
	if v, ok := s["start_height"]; ok {
		r.StartHeight, _ = v.AsUint64()
	}
	if v, ok := s["total_height"]; ok {
		r.TotalHeight, _ = v.AsUint64()
	}
	if v, ok := s["m_block_ids"]; ok {
		sections, _ := v.AsSectionArray()
		r.Blocks = make([]BlockContextInfo, len(sections))
		r.BlockIDs = make([][]byte, len(sections))
		for i, sec := range sections {
			if hv, ok := sec["h"]; ok {
				r.Blocks[i].Hash, _ = hv.AsString()
				r.BlockIDs[i] = r.Blocks[i].Hash
			}
			if cv, ok := sec["cumul_size"]; ok {
				r.Blocks[i].CumulSize, _ = cv.AsUint64()
			}
		}
	}
	return nil
}

// splitHashes divides a concatenated blob into fixed-size hash slices.
func splitHashes(blob []byte, size int) [][]byte {
	n := len(blob) / size
	out := make([][]byte, n)
	for i := range n {
		out[i] = blob[i*size : (i+1)*size]
	}
	return out
}
