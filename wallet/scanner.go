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
	"forge.lthn.ai/core/go-blockchain/crypto"
	"forge.lthn.ai/core/go-blockchain/types"
)

// Scanner detects outputs belonging to a wallet within a transaction.
type Scanner interface {
	ScanTransaction(tx *types.Transaction, txHash types.Hash,
		blockHeight uint64, extra *TxExtra) ([]Transfer, error)
}

// V1Scanner implements Scanner for v0/v1 transactions using ECDH derivation.
// For each output it performs: derivation = viewSecret * txPubKey, then checks
// whether DerivePublicKey(derivation, i, spendPub) matches the output key.
type V1Scanner struct {
	account *Account
}

// NewV1Scanner returns a scanner bound to the given account.
func NewV1Scanner(acc *Account) *V1Scanner {
	return &V1Scanner{account: acc}
}

// ScanTransaction examines every output in tx and returns a Transfer for each
// output that belongs to the scanner's account. The caller must supply a
// pre-parsed TxExtra so that the tx public key is available.
func (s *V1Scanner) ScanTransaction(tx *types.Transaction, txHash types.Hash,
	blockHeight uint64, extra *TxExtra) ([]Transfer, error) {

	if extra.TxPublicKey.IsZero() {
		return nil, nil
	}

	derivation, err := crypto.GenerateKeyDerivation(
		[32]byte(extra.TxPublicKey),
		[32]byte(s.account.ViewSecretKey))
	if err != nil {
		return nil, nil
	}

	isCoinbase := len(tx.Vin) > 0 && tx.Vin[0].InputType() == types.InputTypeGenesis

	var transfers []Transfer
	for i, out := range tx.Vout {
		bare, ok := out.(types.TxOutputBare)
		if !ok {
			continue
		}

		expectedPub, err := crypto.DerivePublicKey(
			derivation, uint64(i), [32]byte(s.account.SpendPublicKey))
		if err != nil {
			continue
		}

		if types.PublicKey(expectedPub) != bare.Target.Key {
			continue
		}

		ephSec, err := crypto.DeriveSecretKey(
			derivation, uint64(i), [32]byte(s.account.SpendSecretKey))
		if err != nil {
			continue
		}

		ki, err := crypto.GenerateKeyImage(expectedPub, ephSec)
		if err != nil {
			continue
		}

		transfers = append(transfers, Transfer{
			TxHash:      txHash,
			OutputIndex: uint32(i),
			Amount:      bare.Amount,
			BlockHeight: blockHeight,
			EphemeralKey: KeyPair{
				Public: types.PublicKey(expectedPub),
				Secret: types.SecretKey(ephSec),
			},
			KeyImage:   types.KeyImage(ki),
			Coinbase:   isCoinbase,
			UnlockTime: extra.UnlockTime,
		})
	}

	return transfers, nil
}
