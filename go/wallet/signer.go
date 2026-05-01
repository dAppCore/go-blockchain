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
	"dappco.re/go/core/blockchain/crypto"
	"dappco.re/go/core/blockchain/types"
	coreerr "dappco.re/go/core/log"
)

// Signer produces signatures for transaction inputs.
// Usage: var value wallet.Signer
type Signer interface {
	SignInput(prefixHash types.Hash, ephemeral KeyPair,
		ring []types.PublicKey, realIndex int) ([]types.Signature, error)
	Version() uint64
}

// NLSAGSigner signs using NLSAG ring signatures (v0/v1 transactions).
// Usage: var value wallet.NLSAGSigner
type NLSAGSigner struct{}

// SignInput generates a ring signature for a single transaction input. It
// derives the key image from the ephemeral key pair, then produces one
// signature element per ring member.
// Usage: value.SignInput(...)
func (s *NLSAGSigner) SignInput(prefixHash types.Hash, ephemeral KeyPair,
	ring []types.PublicKey, realIndex int) ([]types.Signature, error) {

	ki, err := crypto.GenerateKeyImage(
		[32]byte(ephemeral.Public), [32]byte(ephemeral.Secret))
	if err != nil {
		return nil, coreerr.E("NLSAGSigner.SignInput", "wallet: key image", err)
	}

	pubs := make([][32]byte, len(ring))
	for i, k := range ring {
		pubs[i] = [32]byte(k)
	}

	rawSigs, err := crypto.GenerateRingSignature(
		[32]byte(prefixHash), ki, pubs,
		[32]byte(ephemeral.Secret), realIndex)
	if err != nil {
		return nil, coreerr.E("NLSAGSigner.SignInput", "wallet: ring signature", err)
	}

	sigs := make([]types.Signature, len(rawSigs))
	for i, rs := range rawSigs {
		sigs[i] = types.Signature(rs)
	}
	return sigs, nil
}

// Version returns the transaction version this signer targets.
// Usage: value.Version(...)
func (s *NLSAGSigner) Version() uint64 { return 1 }
