// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package p2p

import (
	levin "dappco.re/go/core/p2p/node/levin"
)

// Encode serialises a HandshakeResponse to a portable storage blob.
//
//	data, err := resp.Encode()
func (r *HandshakeResponse) Encode() ([]byte, error) {
	s := levin.Section{
		"node_data":    levin.ObjectVal(r.NodeData.MarshalSection()),
		"payload_data": levin.ObjectVal(r.PayloadData.MarshalSection()),
	}
	if len(r.PeerlistBlob) > 0 {
		s["local_peerlist"] = levin.StringVal(r.PeerlistBlob)
	}
	return levin.EncodeStorage(s)
}

// Decode parses a RequestChain from a portable storage blob.
//
//	err := req.Decode(data)
func (r *RequestChain) Decode(data []byte) error {
	s, err := levin.DecodeStorage(data)
	if err != nil {
		return err
	}
	if v, ok := s["block_ids"]; ok {
		arr, err := v.AsStringArray()
		if err != nil {
			return err
		}
		r.BlockIDs = make([][]byte, len(arr))
		for i, b := range arr {
			r.BlockIDs[i] = b
		}
	}
	return nil
}

// Encode serialises a ResponseChainEntry to a portable storage blob.
//
//	data, err := resp.Encode()
func (r *ResponseChainEntry) Encode() ([]byte, error) {
	blockIDs := make([][]byte, len(r.BlockIDs))
	for i, id := range r.BlockIDs {
		blockIDs[i] = id
	}
	s := levin.Section{
		"start_height": levin.Uint64Val(r.StartHeight),
		"total_height": levin.Uint64Val(r.TotalHeight),
		"m_block_ids":  levin.StringArrayVal(blockIDs),
	}
	return levin.EncodeStorage(s)
}

// Decode parses a HandshakeRequest from a portable storage blob.
//
//	err := req.Decode(data)
func (r *HandshakeRequest) Decode(data []byte) error {
	s, err := levin.DecodeStorage(data)
	if err != nil {
		return err
	}
	if v, ok := s["node_data"]; ok {
		obj, err := v.AsSection()
		if err != nil {
			return err
		}
		if err := r.NodeData.UnmarshalSection(obj); err != nil {
			return err
		}
	}
	if v, ok := s["payload_data"]; ok {
		obj, err := v.AsSection()
		if err != nil {
			return err
		}
		if err := r.PayloadData.UnmarshalSection(obj); err != nil {
			return err
		}
	}
	return nil
}
