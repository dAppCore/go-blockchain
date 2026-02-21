//go:build !integration

package consensus

import (
	"testing"
	"time"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-blockchain/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckTimestamp_Good(t *testing.T) {
	now := uint64(time.Now().Unix())

	// PoW block within limits.
	err := CheckTimestamp(now, 0, now, nil) // flags=0 -> PoW
	require.NoError(t, err)

	// With sufficient history, timestamp above median.
	timestamps := make([]uint64, config.TimestampCheckWindow)
	for i := range timestamps {
		timestamps[i] = now - 100 + uint64(i)
	}
	err = CheckTimestamp(now, 0, now, timestamps)
	require.NoError(t, err)
}

func TestCheckTimestamp_Bad(t *testing.T) {
	now := uint64(time.Now().Unix())

	// PoW block too far in future.
	future := now + config.BlockFutureTimeLimit + 1
	err := CheckTimestamp(future, 0, now, nil)
	assert.ErrorIs(t, err, ErrTimestampFuture)

	// PoS block too far in future (tighter limit).
	posFlags := uint8(1) // bit 0 = PoS
	posFuture := now + config.PosBlockFutureTimeLimit + 1
	err = CheckTimestamp(posFuture, posFlags, now, nil)
	assert.ErrorIs(t, err, ErrTimestampFuture)

	// Timestamp below median of last 60 blocks.
	timestamps := make([]uint64, config.TimestampCheckWindow)
	for i := range timestamps {
		timestamps[i] = now - 60 + uint64(i) // median ~ now - 30
	}
	oldTimestamp := now - 100 // well below median
	err = CheckTimestamp(oldTimestamp, 0, now, timestamps)
	assert.ErrorIs(t, err, ErrTimestampOld)
}

func TestCheckTimestamp_Ugly(t *testing.T) {
	now := uint64(time.Now().Unix())

	// Fewer than 60 timestamps: skip median check.
	timestamps := make([]uint64, 10)
	for i := range timestamps {
		timestamps[i] = now - 100
	}
	err := CheckTimestamp(now-200, 0, now, timestamps) // old but under 60 entries
	require.NoError(t, err)
}

func validMinerTx(height uint64) *types.Transaction {
	return &types.Transaction{
		Version: types.VersionInitial,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: height}},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: config.BlockReward, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
}

func TestValidateMinerTx_Good(t *testing.T) {
	tx := validMinerTx(100)
	err := ValidateMinerTx(tx, 100, config.MainnetForks)
	require.NoError(t, err)
}

func TestValidateMinerTx_Bad_WrongHeight(t *testing.T) {
	tx := validMinerTx(100)
	err := ValidateMinerTx(tx, 200, config.MainnetForks) // height mismatch
	assert.ErrorIs(t, err, ErrMinerTxHeight)
}

func TestValidateMinerTx_Bad_NoInputs(t *testing.T) {
	tx := &types.Transaction{Version: types.VersionInitial}
	err := ValidateMinerTx(tx, 100, config.MainnetForks)
	assert.ErrorIs(t, err, ErrMinerTxInputs)
}

func TestValidateMinerTx_Bad_WrongFirstInput(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionInitial,
		Vin:     []types.TxInput{types.TxInputToKey{Amount: 1}},
	}
	err := ValidateMinerTx(tx, 100, config.MainnetForks)
	assert.ErrorIs(t, err, ErrMinerTxInputs)
}

func TestValidateMinerTx_Good_PoS(t *testing.T) {
	tx := &types.Transaction{
		Version: types.VersionInitial,
		Vin: []types.TxInput{
			types.TxInputGenesis{Height: 100},
			types.TxInputToKey{Amount: 1}, // PoS stake input
		},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: config.BlockReward, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	err := ValidateMinerTx(tx, 100, config.MainnetForks)
	// 2 inputs with genesis + TxInputToKey is valid PoS structure.
	require.NoError(t, err)
}
