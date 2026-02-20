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
	"fmt"

	"forge.lthn.ai/core/go-blockchain/crypto"
	"forge.lthn.ai/core/go-blockchain/types"
)

// Signer produces signatures for transaction inputs.
type Signer interface {
	SignInput(prefixHash types.Hash, ephemeral KeyPair,
		ring []types.PublicKey, realIndex int) ([]types.Signature, error)
	Version() uint64
}

// NLSAGSigner signs using NLSAG ring signatures (v0/v1 transactions).
type NLSAGSigner struct{}

// SignInput generates a ring signature for a single transaction input. It
// derives the key image from the ephemeral key pair, then produces one
// signature element per ring member.
func (s *NLSAGSigner) SignInput(prefixHash types.Hash, ephemeral KeyPair,
	ring []types.PublicKey, realIndex int) ([]types.Signature, error) {

	ki, err := crypto.GenerateKeyImage(
		[32]byte(ephemeral.Public), [32]byte(ephemeral.Secret))
	if err != nil {
		return nil, fmt.Errorf("wallet: key image: %w", err)
	}

	pubs := make([][32]byte, len(ring))
	for i, k := range ring {
		pubs[i] = [32]byte(k)
	}

	rawSigs, err := crypto.GenerateRingSignature(
		[32]byte(prefixHash), ki, pubs,
		[32]byte(ephemeral.Secret), realIndex)
	if err != nil {
		return nil, fmt.Errorf("wallet: ring signature: %w", err)
	}

	sigs := make([]types.Signature, len(rawSigs))
	for i, rs := range rawSigs {
		sigs[i] = types.Signature(rs)
	}
	return sigs, nil
}

// Version returns the transaction version this signer targets.
func (s *NLSAGSigner) Version() uint64 { return 1 }
