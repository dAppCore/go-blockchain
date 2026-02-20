// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package p2p

import "forge.lthn.ai/core/go-p2p/node/levin"

// EncodePingRequest returns an encoded empty ping request payload.
func EncodePingRequest() ([]byte, error) {
	return levin.EncodeStorage(levin.Section{})
}

// DecodePingResponse parses a ping response payload.
func DecodePingResponse(data []byte) (status string, peerID uint64, err error) {
	s, err := levin.DecodeStorage(data)
	if err != nil {
		return "", 0, err
	}
	if v, ok := s["status"]; ok {
		blob, e := v.AsString()
		if e != nil {
			return "", 0, e
		}
		status = string(blob)
	}
	if v, ok := s["peer_id"]; ok {
		peerID, err = v.AsUint64()
		if err != nil {
			return "", 0, err
		}
	}
	return status, peerID, nil
}
