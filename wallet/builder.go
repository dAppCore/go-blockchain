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
	"bytes"
	"cmp"
	"fmt"
	"slices"

	coreerr "dappco.re/go/core/log"

	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/crypto"
	"dappco.re/go/core/blockchain/types"
	"dappco.re/go/core/blockchain/wire"
)

// Destination is a recipient address and amount.
type Destination struct {
	Address types.Address
	Amount  uint64
}

// BuildRequest holds the parameters for building a transaction.
type BuildRequest struct {
	Sources       []Transfer
	Destinations  []Destination
	Fee           uint64
	SenderAddress types.Address
}

// Builder constructs signed transactions.
type Builder interface {
	Build(req *BuildRequest) (*types.Transaction, error)
}

// V1Builder constructs v1 transactions with NLSAG ring signatures.
type V1Builder struct {
	signer       Signer
	ringSelector RingSelector
}

// inputMeta holds the signing context for a single transaction input.
type inputMeta struct {
	ephemeral KeyPair
	ring      []types.PublicKey
	realIndex int
}

// NewV1Builder returns a Builder that produces version 1 transactions.
func NewV1Builder(signer Signer, ringSelector RingSelector) *V1Builder {
	return &V1Builder{
		signer:       signer,
		ringSelector: ringSelector,
	}
}

// Build constructs a signed v1 transaction from the given request.
//
// Algorithm:
//  1. Validate that source amounts cover destinations plus fee.
//  2. Generate a one-time transaction key pair.
//  3. Build inputs with ring decoys sorted by global index.
//  4. Build outputs with ECDH-derived one-time keys.
//  5. Add a change output if there is surplus.
//  6. Compute the prefix hash and sign each input.
func (b *V1Builder) Build(req *BuildRequest) (*types.Transaction, error) {
	// 1. Validate amounts.
	var sourceTotal uint64
	for _, src := range req.Sources {
		sourceTotal += src.Amount
	}
	var destTotal uint64
	for _, dst := range req.Destinations {
		destTotal += dst.Amount
	}
	if sourceTotal < destTotal+req.Fee {
		return nil, coreerr.E("V1Builder.Build", fmt.Sprintf("wallet: insufficient funds: have %d, need %d", sourceTotal, destTotal+req.Fee), nil)
	}
	change := sourceTotal - destTotal - req.Fee

	// 2. Generate one-time TX key pair.
	txPub, txSec, err := crypto.GenerateKeys()
	if err != nil {
		return nil, coreerr.E("V1Builder.Build", "wallet: generate tx keys", err)
	}

	tx := &types.Transaction{Version: types.VersionPreHF4}

	// 3. Build inputs.
	metas := make([]inputMeta, len(req.Sources))

	for i, src := range req.Sources {
		input, meta, buildErr := b.buildInput(&src)
		if buildErr != nil {
			return nil, coreerr.E("V1Builder.Build", fmt.Sprintf("wallet: input %d", i), buildErr)
		}
		tx.Vin = append(tx.Vin, input)
		metas[i] = meta
	}

	// 4. Build destination outputs.
	outputIdx := uint64(0)
	for _, dst := range req.Destinations {
		out, outErr := deriveOutput(txSec, dst.Address, outputIdx, dst.Amount)
		if outErr != nil {
			return nil, coreerr.E("V1Builder.Build", fmt.Sprintf("wallet: output %d", outputIdx), outErr)
		}
		tx.Vout = append(tx.Vout, out)
		outputIdx++
	}

	// 5. Change output.
	if change > 0 {
		out, outErr := deriveOutput(txSec, req.SenderAddress, outputIdx, change)
		if outErr != nil {
			return nil, coreerr.E("V1Builder.Build", "wallet: change output", outErr)
		}
		tx.Vout = append(tx.Vout, out)
	}

	// 6. Extra and attachment.
	tx.Extra = BuildTxExtra(types.PublicKey(txPub))
	tx.Attachment = wire.EncodeVarint(0)

	// 7. Compute prefix hash and sign.
	prefixHash := wire.TransactionPrefixHash(tx)
	for i, meta := range metas {
		sigs, signErr := b.signer.SignInput(prefixHash, meta.ephemeral, meta.ring, meta.realIndex)
		if signErr != nil {
			return nil, coreerr.E("V1Builder.Build", fmt.Sprintf("wallet: sign input %d", i), signErr)
		}
		tx.Signatures = append(tx.Signatures, sigs)
	}

	return tx, nil
}

// buildInput constructs a single TxInputToKey with its decoy ring.
func (b *V1Builder) buildInput(src *Transfer) (types.TxInputToKey, inputMeta, error) {
	decoys, err := b.ringSelector.SelectRing(
		src.Amount, src.GlobalIndex, int(config.DefaultDecoySetSize))
	if err != nil {
		return types.TxInputToKey{}, inputMeta{}, err
	}

	// Insert the real output into the ring.
	ring := append(decoys, RingMember{
		PublicKey:   src.EphemeralKey.Public,
		GlobalIndex: src.GlobalIndex,
	})

	// Sort by global index (consensus rule).
	slices.SortFunc(ring, func(a, b RingMember) int {
		return cmp.Compare(a.GlobalIndex, b.GlobalIndex)
	})

	// Find real index after sorting.
	realIdx := slices.IndexFunc(ring, func(m RingMember) bool {
		return m.GlobalIndex == src.GlobalIndex
	})
	if realIdx < 0 {
		return types.TxInputToKey{}, inputMeta{}, coreerr.E("V1Builder.buildInput", "real output not found in ring", nil)
	}

	// Build key offsets and public key list.
	offsets := make([]types.TxOutRef, len(ring))
	pubs := make([]types.PublicKey, len(ring))
	for j, m := range ring {
		offsets[j] = types.TxOutRef{
			Tag:         types.RefTypeGlobalIndex,
			GlobalIndex: m.GlobalIndex,
		}
		pubs[j] = m.PublicKey
	}

	input := types.TxInputToKey{
		Amount:     src.Amount,
		KeyOffsets: offsets,
		KeyImage:   src.KeyImage,
		EtcDetails: wire.EncodeVarint(0),
	}

	meta := inputMeta{
		ephemeral: src.EphemeralKey,
		ring:      pubs,
		realIndex: realIdx,
	}

	return input, meta, nil
}

// deriveOutput creates a TxOutputBare with an ECDH-derived one-time key.
func deriveOutput(txSec [32]byte, addr types.Address, index uint64, amount uint64) (types.TxOutputBare, error) {
	derivation, err := crypto.GenerateKeyDerivation(
		[32]byte(addr.ViewPublicKey), txSec)
	if err != nil {
		return types.TxOutputBare{}, coreerr.E("deriveOutput", "key derivation", err)
	}

	ephPub, err := crypto.DerivePublicKey(
		derivation, index, [32]byte(addr.SpendPublicKey))
	if err != nil {
		return types.TxOutputBare{}, coreerr.E("deriveOutput", "derive public key", err)
	}

	return types.TxOutputBare{
		Amount: amount,
		Target: types.TxOutToKey{Key: types.PublicKey(ephPub)},
	}, nil
}

// SerializeTransaction encodes a transaction into its wire-format bytes.
func SerializeTransaction(tx *types.Transaction) ([]byte, error) {
	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	wire.EncodeTransaction(enc, tx)
	if err := enc.Err(); err != nil {
		return nil, coreerr.E("SerializeTransaction", "wallet: encode tx", err)
	}
	return buf.Bytes(), nil
}
