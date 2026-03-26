// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// You may obtain a copy of the licence at:
//
//     https://joinup.ec.europa.eu/software/page/eupl/licence-eupl
//
// SPDX-License-Identifier: EUPL-1.2

package wallet

import (
	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"

	"dappco.re/go/core/blockchain/rpc"
	"dappco.re/go/core/blockchain/types"
)

// RingMember is a public key and global index used in ring construction.
type RingMember struct {
	PublicKey   types.PublicKey
	GlobalIndex uint64
}

// RingSelector picks decoy outputs for ring signatures.
type RingSelector interface {
	SelectRing(amount uint64, realGlobalIndex uint64, ringSize int) ([]RingMember, error)
}

// RPCRingSelector fetches decoys from the daemon via RPC.
type RPCRingSelector struct {
	client *rpc.Client
}

// NewRPCRingSelector returns a RingSelector backed by the given RPC client.
func NewRPCRingSelector(client *rpc.Client) *RPCRingSelector {
	return &RPCRingSelector{client: client}
}

// SelectRing fetches random outputs from the daemon and returns ringSize
// decoy members, excluding the real output and any duplicates.
func (s *RPCRingSelector) SelectRing(amount uint64, realGlobalIndex uint64, ringSize int) ([]RingMember, error) {
	outs, err := s.client.GetRandomOutputs(amount, ringSize+5)
	if err != nil {
		return nil, coreerr.E("RPCRingSelector.SelectRing", "wallet: get random outputs", err)
	}

	var members []RingMember
	seen := map[uint64]bool{realGlobalIndex: true}

	for _, out := range outs {
		if seen[out.GlobalIndex] {
			continue
		}
		seen[out.GlobalIndex] = true

		pk, err := types.PublicKeyFromHex(out.PublicKey)
		if err != nil {
			continue
		}
		members = append(members, RingMember{
			PublicKey:   pk,
			GlobalIndex: out.GlobalIndex,
		})
		if len(members) >= ringSize {
			break
		}
	}

	if len(members) < ringSize {
		return nil, coreerr.E("RPCRingSelector.SelectRing", core.Sprintf("wallet: insufficient decoys: got %d, need %d", len(members), ringSize), nil)
	}
	return members, nil
}
