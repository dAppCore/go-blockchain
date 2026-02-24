// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package p2p

import (
	"encoding/binary"

	"forge.lthn.ai/core/go-p2p/node/levin"
)

// PeerlistEntrySize is the packed size of a peerlist entry (ip + port + id + last_seen).
const PeerlistEntrySize = 24

// NodeData contains the node identity exchanged during handshake.
type NodeData struct {
	NetworkID [16]byte
	PeerID    uint64
	LocalTime int64
	MyPort    uint32
}

// MarshalSection encodes NodeData into a portable storage Section.
func (n *NodeData) MarshalSection() levin.Section {
	return levin.Section{
		"network_id": levin.StringVal(n.NetworkID[:]),
		"peer_id":    levin.Uint64Val(n.PeerID),
		"local_time": levin.Int64Val(n.LocalTime),
		"my_port":    levin.Uint32Val(n.MyPort),
	}
}

// UnmarshalSection decodes NodeData from a portable storage Section.
func (n *NodeData) UnmarshalSection(s levin.Section) error {
	if v, ok := s["network_id"]; ok {
		blob, err := v.AsString()
		if err != nil {
			return err
		}
		if len(blob) >= 16 {
			copy(n.NetworkID[:], blob[:16])
		}
	}
	if v, ok := s["peer_id"]; ok {
		val, err := v.AsUint64()
		if err != nil {
			return err
		}
		n.PeerID = val
	}
	if v, ok := s["local_time"]; ok {
		val, err := v.AsInt64()
		if err != nil {
			return err
		}
		n.LocalTime = val
	}
	if v, ok := s["my_port"]; ok {
		val, err := v.AsUint32()
		if err != nil {
			return err
		}
		n.MyPort = val
	}
	return nil
}

// PeerlistEntry is a decoded peerlist entry from a handshake response.
type PeerlistEntry struct {
	IP       uint32
	Port     uint32
	ID       uint64
	LastSeen int64
}

// DecodePeerlist splits a packed peerlist blob into entries.
func DecodePeerlist(blob []byte) []PeerlistEntry {
	n := len(blob) / PeerlistEntrySize
	entries := make([]PeerlistEntry, n)
	for i := range n {
		off := i * PeerlistEntrySize
		entries[i] = PeerlistEntry{
			IP:       binary.LittleEndian.Uint32(blob[off : off+4]),
			Port:     binary.LittleEndian.Uint32(blob[off+4 : off+8]),
			ID:       binary.LittleEndian.Uint64(blob[off+8 : off+16]),
			LastSeen: int64(binary.LittleEndian.Uint64(blob[off+16 : off+24])),
		}
	}
	return entries
}

// HandshakeRequest is a COMMAND_HANDSHAKE request.
type HandshakeRequest struct {
	NodeData    NodeData
	PayloadData CoreSyncData
}

// MarshalSection encodes the request.
func (r *HandshakeRequest) MarshalSection() levin.Section {
	return levin.Section{
		"node_data":    levin.ObjectVal(r.NodeData.MarshalSection()),
		"payload_data": levin.ObjectVal(r.PayloadData.MarshalSection()),
	}
}

// UnmarshalSection decodes the request.
func (r *HandshakeRequest) UnmarshalSection(s levin.Section) error {
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

// EncodeHandshakeRequest serialises a handshake request into a storage blob.
func EncodeHandshakeRequest(req *HandshakeRequest) ([]byte, error) {
	return levin.EncodeStorage(req.MarshalSection())
}

// HandshakeResponse is a COMMAND_HANDSHAKE response.
type HandshakeResponse struct {
	NodeData     NodeData
	PayloadData  CoreSyncData
	PeerlistBlob []byte // Raw packed peerlist (24 bytes per entry)
}

// Decode parses a handshake response from a storage blob.
func (r *HandshakeResponse) Decode(data []byte) error {
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
	if v, ok := s["local_peerlist"]; ok {
		blob, err := v.AsString()
		if err != nil {
			return err
		}
		r.PeerlistBlob = blob
	}
	return nil
}
