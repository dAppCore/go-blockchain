//go:build !integration

package consensus

import (
	"testing"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-blockchain/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyTransactionSignatures_Good_Coinbase(t *testing.T) {
	// Coinbase transactions have no signatures to verify.
	tx := validMinerTx(100)
	err := VerifyTransactionSignatures(tx, config.MainnetForks, 100, nil, nil)
	require.NoError(t, err)
}

func TestVerifyTransactionSignatures_Bad_MissingSigs(t *testing.T) {
	tx := validV1Tx()
	tx.Signatures = nil // no signatures
	err := VerifyTransactionSignatures(tx, config.MainnetForks, 100, nil, nil)
	assert.Error(t, err)
}

// --- HTLC signature verification tests (Task 9) ---

func TestVerifyV1Signatures_MixedHTLC_Good(t *testing.T) {
	// Structural check only (getRingOutputs = nil).
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: 100, KeyImage: types.KeyImage{1}},
			types.TxInputHTLC{Amount: 50, KeyImage: types.KeyImage{2}},
		},
		Signatures: [][]types.Signature{
			{{1}}, // sig for TxInputToKey
			{{2}}, // sig for TxInputHTLC
		},
	}
	err := VerifyTransactionSignatures(tx, config.MainnetForks, 20000, nil, nil)
	require.NoError(t, err)
}

func TestVerifyV1Signatures_MixedHTLC_Bad(t *testing.T) {
	// Wrong signature count.
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: 100, KeyImage: types.KeyImage{1}},
			types.TxInputHTLC{Amount: 50, KeyImage: types.KeyImage{2}},
		},
		Signatures: [][]types.Signature{
			{{1}}, // only 1 sig for 2 ring inputs
		},
	}
	err := VerifyTransactionSignatures(tx, config.MainnetForks, 20000, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature count")
}

func TestVerifyV1Signatures_HTLCOnly_Good(t *testing.T) {
	// Transaction with only HTLC inputs.
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputHTLC{Amount: 50, KeyImage: types.KeyImage{1}},
			types.TxInputHTLC{Amount: 30, KeyImage: types.KeyImage{2}},
		},
		Signatures: [][]types.Signature{
			{{1}},
			{{2}},
		},
	}
	err := VerifyTransactionSignatures(tx, config.MainnetForks, 20000, nil, nil)
	require.NoError(t, err)
}

func TestVerifyV1Signatures_MultisigSkipped_Good(t *testing.T) {
	// Multisig inputs do not participate in NLSAG signatures.
	tx := &types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputToKey{Amount: 100, KeyImage: types.KeyImage{1}},
			types.TxInputMultisig{Amount: 50},
		},
		Signatures: [][]types.Signature{
			{{1}}, // only 1 sig, multisig is not counted
		},
	}
	err := VerifyTransactionSignatures(tx, config.MainnetForks, 20000, nil, nil)
	require.NoError(t, err)
}
