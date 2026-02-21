//go:build !integration

package consensus

import (
	"testing"
	"time"

	"forge.lthn.ai/core/go-blockchain/config"
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
