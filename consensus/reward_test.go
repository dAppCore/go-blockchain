//go:build !integration

package consensus

import (
	"testing"

	"dappco.re/go/core/blockchain/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReward_BaseReward_Good(t *testing.T) {
	assert.Equal(t, config.Premine, BaseReward(0), "genesis returns premine")
	assert.Equal(t, config.BlockReward, BaseReward(1), "block 1 returns standard reward")
	assert.Equal(t, config.BlockReward, BaseReward(10000), "arbitrary height")
}

func TestReward_BlockReward_Good(t *testing.T) {
	base := config.BlockReward

	// Small block: full reward.
	reward, err := BlockReward(base, 1000, config.BlockGrantedFullRewardZone)
	require.NoError(t, err)
	assert.Equal(t, base, reward)

	// Block at exactly the zone boundary: full reward.
	reward, err = BlockReward(base, config.BlockGrantedFullRewardZone, config.BlockGrantedFullRewardZone)
	require.NoError(t, err)
	assert.Equal(t, base, reward)
}

func TestReward_BlockReward_Bad(t *testing.T) {
	base := config.BlockReward
	median := config.BlockGrantedFullRewardZone

	// Block larger than 2*median: rejected.
	_, err := BlockReward(base, 2*median+1, median)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}

func TestReward_BlockReward_Ugly(t *testing.T) {
	base := config.BlockReward
	median := config.BlockGrantedFullRewardZone

	// Block slightly over zone: penalty applied, reward < base.
	reward, err := BlockReward(base, median+10_000, median)
	require.NoError(t, err)
	assert.Less(t, reward, base, "penalty should reduce reward")
	assert.Greater(t, reward, uint64(0), "reward should be positive")
}

func TestReward_MinerReward_Good(t *testing.T) {
	base := config.BlockReward
	fees := uint64(50_000_000_000) // 0.05 LTHN

	// Pre-HF4: fees added.
	total := MinerReward(base, fees, false)
	assert.Equal(t, base+fees, total)

	// Post-HF4: fees burned.
	total = MinerReward(base, fees, true)
	assert.Equal(t, base, total)
}
