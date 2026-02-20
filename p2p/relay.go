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
