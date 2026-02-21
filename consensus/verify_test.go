//go:build !integration

package consensus

import (
	"testing"

	"forge.lthn.ai/core/go-blockchain/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyTransactionSignatures_Good_Coinbase(t *testing.T) {
	// Coinbase transactions have no signatures to verify.
	tx := validMinerTx(100)
	err := VerifyTransactionSignatures(tx, config.MainnetForks, 100, nil)
	require.NoError(t, err)
}

func TestVerifyTransactionSignatures_Bad_MissingSigs(t *testing.T) {
	tx := validV1Tx()
	tx.Signatures = nil // no signatures
	err := VerifyTransactionSignatures(tx, config.MainnetForks, 100, nil)
	assert.Error(t, err)
}
