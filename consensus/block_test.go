//go:build !integration

package consensus

import (
	"testing"
	"time"

	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/types"
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

func TestValidateBlockReward_Good(t *testing.T) {
	height := uint64(100)
	tx := validMinerTx(height)
	err := ValidateBlockReward(tx, height, 1000, config.BlockGrantedFullRewardZone, 0, config.MainnetForks)
	require.NoError(t, err)
}

func TestValidateBlockReward_Bad_TooMuch(t *testing.T) {
	height := uint64(100)
	tx := &types.Transaction{
		Version: types.VersionInitial,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: height}},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: config.BlockReward + 1, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	err := ValidateBlockReward(tx, height, 1000, config.BlockGrantedFullRewardZone, 0, config.MainnetForks)
	assert.ErrorIs(t, err, ErrRewardMismatch)
}

func TestValidateBlockReward_Good_WithFees(t *testing.T) {
	height := uint64(100)
	fees := uint64(50_000_000_000)
	tx := &types.Transaction{
		Version: types.VersionInitial,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: height}},
		Vout: []types.TxOutput{
			types.TxOutputBare{Amount: config.BlockReward + fees, Target: types.TxOutToKey{Key: types.PublicKey{1}}},
		},
	}
	err := ValidateBlockReward(tx, height, 1000, config.BlockGrantedFullRewardZone, fees, config.MainnetForks)
	require.NoError(t, err)
}

func TestValidateBlock_Good(t *testing.T) {
	now := uint64(time.Now().Unix())
	height := uint64(100)
	blk := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 0, // pre-HF1 on mainnet
			Timestamp:    now,
			Flags:        0, // PoW
		},
		MinerTx: *validMinerTx(height),
	}

	err := ValidateBlock(blk, height, 1000, config.BlockGrantedFullRewardZone, 0, now, nil, config.MainnetForks)
	require.NoError(t, err)
}

func TestValidateBlock_Bad_Timestamp(t *testing.T) {
	now := uint64(time.Now().Unix())
	height := uint64(100)
	blk := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 0, // pre-HF1 on mainnet
			Timestamp:    now + config.BlockFutureTimeLimit + 100,
			Flags:        0,
		},
		MinerTx: *validMinerTx(height),
	}

	err := ValidateBlock(blk, height, 1000, config.BlockGrantedFullRewardZone, 0, now, nil, config.MainnetForks)
	assert.ErrorIs(t, err, ErrTimestampFuture)
}

func TestValidateBlock_Bad_MinerTx(t *testing.T) {
	now := uint64(time.Now().Unix())
	height := uint64(100)
	blk := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 0, // pre-HF1 on mainnet
			Timestamp:    now,
			Flags:        0,
		},
		MinerTx: *validMinerTx(200), // wrong height
	}

	err := ValidateBlock(blk, height, 1000, config.BlockGrantedFullRewardZone, 0, now, nil, config.MainnetForks)
	assert.ErrorIs(t, err, ErrMinerTxHeight)
}

// --- Block major version tests (Task 10) ---

func TestExpectedBlockMajorVersion_Good(t *testing.T) {
	tests := []struct {
		name   string
		forks  []config.HardFork
		height uint64
		want   uint8
	}{
		// --- Mainnet ---
		{
			name:   "mainnet/genesis",
			forks:  config.MainnetForks,
			height: 0,
			want:   config.BlockMajorVersionInitial, // 0
		},
		{
			name:   "mainnet/pre_HF1",
			forks:  config.MainnetForks,
			height: 5000,
			want:   config.BlockMajorVersionInitial, // 0
		},
		{
			name:   "mainnet/at_HF1_boundary",
			forks:  config.MainnetForks,
			height: 10080,
			want:   config.BlockMajorVersionInitial, // 0 (fork at height > 10080)
		},
		{
			name:   "mainnet/post_HF1",
			forks:  config.MainnetForks,
			height: 10081,
			want:   config.HF1BlockMajorVersion, // 1
		},
		{
			name:   "mainnet/well_past_HF1",
			forks:  config.MainnetForks,
			height: 100000,
			want:   config.HF1BlockMajorVersion, // 1 (HF3 not yet active)
		},

		// --- Testnet (HF3 active from genesis) ---
		{
			name:   "testnet/genesis",
			forks:  config.TestnetForks,
			height: 0,
			want:   config.HF3BlockMajorVersion, // 2 (HF3 at 0)
		},
		{
			name:   "testnet/pre_HF4",
			forks:  config.TestnetForks,
			height: 50,
			want:   config.HF3BlockMajorVersion, // 2 (HF4 at >100)
		},
		{
			name:   "testnet/post_HF4",
			forks:  config.TestnetForks,
			height: 101,
			want:   config.CurrentBlockMajorVersion, // 3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expectedBlockMajorVersion(tt.forks, tt.height)
			if got != tt.want {
				t.Errorf("expectedBlockMajorVersion(%d) = %d, want %d", tt.height, got, tt.want)
			}
		})
	}
}

func TestCheckBlockVersion_Good(t *testing.T) {
	now := uint64(time.Now().Unix())
	// Correct version at each mainnet/testnet era.
	tests := []struct {
		name    string
		version uint8
		height  uint64
		forks   []config.HardFork
	}{
		{"mainnet/v0_pre_HF1", config.BlockMajorVersionInitial, 5000, config.MainnetForks},
		{"mainnet/v1_post_HF1", config.HF1BlockMajorVersion, 10081, config.MainnetForks},
		{"testnet/v2_genesis", config.HF3BlockMajorVersion, 0, config.TestnetForks},
		{"testnet/v3_post_HF4", config.CurrentBlockMajorVersion, 101, config.TestnetForks},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blk := &types.Block{
				BlockHeader: types.BlockHeader{
					MajorVersion: tt.version,
					Timestamp:    now,
					Flags:        0,
				},
			}
			err := checkBlockVersion(blk, tt.forks, tt.height)
			require.NoError(t, err)
		})
	}
}

func TestCheckBlockVersion_Bad(t *testing.T) {
	now := uint64(time.Now().Unix())
	tests := []struct {
		name    string
		version uint8
		height  uint64
		forks   []config.HardFork
	}{
		{"mainnet/v1_pre_HF1", config.HF1BlockMajorVersion, 5000, config.MainnetForks},
		{"mainnet/v0_post_HF1", config.BlockMajorVersionInitial, 10081, config.MainnetForks},
		{"mainnet/v2_post_HF1", config.HF3BlockMajorVersion, 10081, config.MainnetForks},
		{"testnet/v1_genesis", config.HF1BlockMajorVersion, 0, config.TestnetForks},
		{"testnet/v2_post_HF4", config.HF3BlockMajorVersion, 101, config.TestnetForks},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blk := &types.Block{
				BlockHeader: types.BlockHeader{
					MajorVersion: tt.version,
					Timestamp:    now,
					Flags:        0,
				},
			}
			err := checkBlockVersion(blk, tt.forks, tt.height)
			assert.ErrorIs(t, err, ErrBlockVersion)
		})
	}
}

func TestCheckBlockVersion_Ugly(t *testing.T) {
	now := uint64(time.Now().Unix())

	// Version 255 should never be valid at any height.
	blk := &types.Block{
		BlockHeader: types.BlockHeader{MajorVersion: 255, Timestamp: now},
	}
	err := checkBlockVersion(blk, config.MainnetForks, 0)
	assert.ErrorIs(t, err, ErrBlockVersion)

	err = checkBlockVersion(blk, config.MainnetForks, 10081)
	assert.ErrorIs(t, err, ErrBlockVersion)

	// Version 0 at the exact HF1 boundary (height 10080 -- fork not yet active).
	blk0 := &types.Block{
		BlockHeader: types.BlockHeader{MajorVersion: config.BlockMajorVersionInitial, Timestamp: now},
	}
	err = checkBlockVersion(blk0, config.MainnetForks, 10080)
	require.NoError(t, err)
}

func TestValidateBlock_MajorVersion_Good(t *testing.T) {
	now := uint64(time.Now().Unix())
	tests := []struct {
		name    string
		forks   []config.HardFork
		height  uint64
		version uint8
	}{
		// Mainnet: pre-HF1 expects version 0.
		{name: "mainnet_preHF1", forks: config.MainnetForks, height: 5000, version: 0},
		// Mainnet: post-HF1 expects version 1.
		{name: "mainnet_postHF1", forks: config.MainnetForks, height: 20000, version: 1},
		// Testnet: HF1 active from genesis, HF3 active from genesis, expects version 2.
		{name: "testnet_genesis", forks: config.TestnetForks, height: 5, version: 2},
		// Testnet: post-HF4 (height > 100) expects version 3.
		{name: "testnet_postHF4", forks: config.TestnetForks, height: 200, version: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blk := &types.Block{
				BlockHeader: types.BlockHeader{
					MajorVersion: tt.version,
					Timestamp:    now,
					Flags:        0,
				},
				MinerTx: *validMinerTx(tt.height),
			}
			err := ValidateBlock(blk, tt.height, 1000, config.BlockGrantedFullRewardZone, 0, now, nil, tt.forks)
			require.NoError(t, err)
		})
	}
}

func TestValidateBlock_MajorVersion_Bad(t *testing.T) {
	now := uint64(time.Now().Unix())
	tests := []struct {
		name    string
		forks   []config.HardFork
		height  uint64
		version uint8
	}{
		// Mainnet: pre-HF1 with wrong version 1.
		{name: "mainnet_preHF1_v1", forks: config.MainnetForks, height: 5000, version: 1},
		// Mainnet: post-HF1 with wrong version 0.
		{name: "mainnet_postHF1_v0", forks: config.MainnetForks, height: 20000, version: 0},
		// Mainnet: post-HF1 with wrong version 2.
		{name: "mainnet_postHF1_v2", forks: config.MainnetForks, height: 20000, version: 2},
		// Testnet: post-HF4 with wrong version 2.
		{name: "testnet_postHF4_v2", forks: config.TestnetForks, height: 200, version: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blk := &types.Block{
				BlockHeader: types.BlockHeader{
					MajorVersion: tt.version,
					Timestamp:    now,
					Flags:        0,
				},
				MinerTx: *validMinerTx(tt.height),
			}
			err := ValidateBlock(blk, tt.height, 1000, config.BlockGrantedFullRewardZone, 0, now, nil, tt.forks)
			assert.ErrorIs(t, err, ErrBlockMajorVersion)
		})
	}
}

func TestValidateBlock_MajorVersion_Ugly(t *testing.T) {
	now := uint64(time.Now().Unix())
	// Boundary test: exactly at HF1 activation height (10080) on mainnet.
	// HF1 activates at heights strictly greater than 10080, so at height
	// 10080 itself HF1 is NOT active; version must be 0.
	blk := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 0,
			Timestamp:    now,
			Flags:        0,
		},
		MinerTx: *validMinerTx(10080),
	}
	err := ValidateBlock(blk, 10080, 1000, config.BlockGrantedFullRewardZone, 0, now, nil, config.MainnetForks)
	require.NoError(t, err)

	// At height 10081, HF1 IS active; version must be 1.
	blk2 := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 1,
			Timestamp:    now,
			Flags:        0,
		},
		MinerTx: *validMinerTx(10081),
	}
	err = ValidateBlock(blk2, 10081, 1000, config.BlockGrantedFullRewardZone, 0, now, nil, config.MainnetForks)
	require.NoError(t, err)
}
