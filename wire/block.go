// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wire

import "dappco.re/go/core/blockchain/types"

// EncodeBlockHeader serialises a block header in the consensus wire format.
//
// Wire order (currency_basic.h:1123-1131):
//
//	major_version  uint8   (1 byte)
//	nonce          uint64  (8 bytes LE)
//	prev_id        hash    (32 bytes)
//	minor_version  varint
//	timestamp      varint
//	flags          uint8   (1 byte)
func EncodeBlockHeader(enc *Encoder, h *types.BlockHeader) {
	enc.WriteUint8(h.MajorVersion)
	enc.WriteUint64LE(h.Nonce)
	enc.WriteBlob32((*[32]byte)(&h.PrevID))
	enc.WriteVarint(h.MinorVersion)
	enc.WriteVarint(h.Timestamp)
	enc.WriteUint8(h.Flags)
}

// DecodeBlockHeader deserialises a block header from the consensus wire format.
func DecodeBlockHeader(dec *Decoder) types.BlockHeader {
	var h types.BlockHeader
	h.MajorVersion = dec.ReadUint8()
	h.Nonce = dec.ReadUint64LE()
	dec.ReadBlob32((*[32]byte)(&h.PrevID))
	h.MinorVersion = dec.ReadVarint()
	h.Timestamp = dec.ReadVarint()
	h.Flags = dec.ReadUint8()
	return h
}

// EncodeBlock serialises a full block (header + miner tx + tx hashes).
func EncodeBlock(enc *Encoder, b *types.Block) {
	EncodeBlockHeader(enc, &b.BlockHeader)
	EncodeTransaction(enc, &b.MinerTx)
	enc.WriteVarint(uint64(len(b.TxHashes)))
	for i := range b.TxHashes {
		enc.WriteBlob32((*[32]byte)(&b.TxHashes[i]))
	}
}

// DecodeBlock deserialises a full block.
func DecodeBlock(dec *Decoder) types.Block {
	var b types.Block
	b.BlockHeader = DecodeBlockHeader(dec)
	b.MinerTx = DecodeTransaction(dec)
	n := dec.ReadVarint()
	if n > 0 && dec.Err() == nil {
		b.TxHashes = make([]types.Hash, n)
		for i := uint64(0); i < n; i++ {
			dec.ReadBlob32((*[32]byte)(&b.TxHashes[i]))
		}
	}
	return b
}
