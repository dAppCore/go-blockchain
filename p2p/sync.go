// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package p2p

import (
	"dappco.re/go/core/blockchain/types"
	"dappco.re/go/core/p2p/node/levin"
)

// CoreSyncData is the blockchain state exchanged during handshake and timed sync.
type CoreSyncData struct {
	CurrentHeight        uint64
	TopID                types.Hash
	LastCheckpointHeight uint64
	CoreTime             uint64
	ClientVersion        string
	NonPruningMode       bool
}

// MarshalSection encodes CoreSyncData into a portable storage Section.
func (d *CoreSyncData) MarshalSection() levin.Section {
	return levin.Section{
		"current_height":           levin.Uint64Val(d.CurrentHeight),
		"top_id":                   levin.StringVal(d.TopID[:]),
		"last_checkpoint_height":   levin.Uint64Val(d.LastCheckpointHeight),
		"core_time":                levin.Uint64Val(d.CoreTime),
		"client_version":           levin.StringVal([]byte(d.ClientVersion)),
		"non_pruning_mode_enabled": levin.BoolVal(d.NonPruningMode),
	}
}

// UnmarshalSection decodes CoreSyncData from a portable storage Section.
func (d *CoreSyncData) UnmarshalSection(s levin.Section) error {
	if v, ok := s["current_height"]; ok {
		val, err := v.AsUint64()
		if err != nil {
			return err
		}
		d.CurrentHeight = val
	}
	if v, ok := s["top_id"]; ok {
		blob, err := v.AsString()
		if err != nil {
			return err
		}
		if len(blob) == 32 {
			copy(d.TopID[:], blob)
		}
	}
	if v, ok := s["last_checkpoint_height"]; ok {
		val, err := v.AsUint64()
		if err != nil {
			return err
		}
		d.LastCheckpointHeight = val
	}
	if v, ok := s["core_time"]; ok {
		val, err := v.AsUint64()
		if err != nil {
			return err
		}
		d.CoreTime = val
	}
	if v, ok := s["client_version"]; ok {
		blob, err := v.AsString()
		if err != nil {
			return err
		}
		d.ClientVersion = string(blob)
	}
	if v, ok := s["non_pruning_mode_enabled"]; ok {
		val, err := v.AsBool()
		if err != nil {
			return err
		}
		d.NonPruningMode = val
	}
	return nil
}
