// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package consensus

import (
	"testing"

	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/crypto"
	"dappco.re/go/core/blockchain/types"
	"dappco.re/go/core/blockchain/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyCrypto_VerifyV1Signatures_MockRing_Good(t *testing.T) {
	pub, sec, err := crypto.GenerateKeys()
	require.NoError(t, err)

	ki, err := crypto.GenerateKeyImage(pub, sec)
	require.NoError(t, err)

	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{
				Amount: 100,
				KeyOffsets: []types.TxOutRef{
					{Tag: types.RefTypeGlobalIndex, GlobalIndex: 0},
				},
				KeyImage: types.KeyImage(ki),
			},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 90, Target: types.TxOutToKey{Key: types.PublicKey(pub)}},
		},
	}

	prefixHash := wire.TransactionPrefixHash(tx)

	sigs, err := crypto.GenerateRingSignature(
		[32]byte(prefixHash), ki, [][32]byte{pub}, sec, 0)
	require.NoError(t, err)

	tx.Signatures = [][]types.Signature{make([]types.Signature, 1)}
	tx.Signatures[0][0] = types.Signature(sigs[0])

	getRing := func(amount uint64, offsets []uint64) ([]types.PublicKey, error) {
		return []types.PublicKey{types.PublicKey(pub)}, nil
	}

	err = VerifyTransactionSignatures(tx, config.MainnetForks, 100, getRing, nil)
	require.NoError(t, err)
}

func TestVerifyCrypto_VerifyV1Signatures_WrongSig_Bad(t *testing.T) {
	pub, sec, err := crypto.GenerateKeys()
	require.NoError(t, err)

	ki, err := crypto.GenerateKeyImage(pub, sec)
	require.NoError(t, err)

	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{
				Amount: 100,
				KeyOffsets: []types.TxOutRef{
					{Tag: types.RefTypeGlobalIndex, GlobalIndex: 0},
				},
				KeyImage: types.KeyImage(ki),
			},
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: 90, Target: types.TxOutToKey{Key: types.PublicKey(pub)}},
		},
		Signatures: [][]types.Signature{
			{types.Signature{}},
		},
	}

	getRing := func(amount uint64, offsets []uint64) ([]types.PublicKey, error) {
		return []types.PublicKey{types.PublicKey(pub)}, nil
	}

	err = VerifyTransactionSignatures(tx, config.MainnetForks, 100, getRing, nil)
	assert.Error(t, err)
}
