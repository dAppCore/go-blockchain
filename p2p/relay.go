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
func (r *RequestGetObjects) Encode() ([]byte, error) {
	s := levin.Section{
		"blocks": levin.StringArrayVal(r.Blocks),
	}
	if len(r.Txs) > 0 {
		s["txs"] = levin.StringArrayVal(r.Txs)
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
		r.Blocks, _ = v.AsStringArray()
	}
	if v, ok := s["txs"]; ok {
		r.Txs, _ = v.AsStringArray()
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
		s["missed_ids"] = levin.StringArrayVal(r.MissedIDs)
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
		r.MissedIDs, _ = v.AsStringArray()
	}
	return nil
}

// RequestChain is NOTIFY_REQUEST_CHAIN (2006).
type RequestChain struct {
	BlockIDs [][]byte // Array of 32-byte block hashes
}

// Encode serialises the request.
func (r *RequestChain) Encode() ([]byte, error) {
	s := levin.Section{
		"block_ids": levin.StringArrayVal(r.BlockIDs),
	}
	return levin.EncodeStorage(s)
}

// ResponseChainEntry is NOTIFY_RESPONSE_CHAIN_ENTRY (2007).
type ResponseChainEntry struct {
	StartHeight uint64
	TotalHeight uint64
	BlockIDs    [][]byte
}

// Decode parses a chain entry response.
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
		r.BlockIDs, _ = v.AsStringArray()
	}
	return nil
}
