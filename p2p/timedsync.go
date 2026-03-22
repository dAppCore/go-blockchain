// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package p2p

import "dappco.re/go/core/p2p/node/levin"

// TimedSyncRequest is a COMMAND_TIMED_SYNC request.
type TimedSyncRequest struct {
	PayloadData CoreSyncData
}

// Encode serialises the timed sync request.
func (r *TimedSyncRequest) Encode() ([]byte, error) {
	s := levin.Section{
		"payload_data": levin.ObjectVal(r.PayloadData.MarshalSection()),
	}
	return levin.EncodeStorage(s)
}

// TimedSyncResponse is a COMMAND_TIMED_SYNC response.
type TimedSyncResponse struct {
	LocalTime    int64
	PayloadData  CoreSyncData
	PeerlistBlob []byte
}

// Decode parses a timed sync response from a storage blob.
func (r *TimedSyncResponse) Decode(data []byte) error {
	s, err := levin.DecodeStorage(data)
	if err != nil {
		return err
	}
	if v, ok := s["local_time"]; ok {
		r.LocalTime, _ = v.AsInt64()
	}
	if v, ok := s["payload_data"]; ok {
		obj, _ := v.AsSection()
		r.PayloadData.UnmarshalSection(obj)
	}
	if v, ok := s["local_peerlist"]; ok {
		r.PeerlistBlob, _ = v.AsString()
	}
	return nil
}
