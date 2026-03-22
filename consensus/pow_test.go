//go:build !integration

package consensus

import (
	"testing"

	"dappco.re/go/core/blockchain/types"
	"github.com/stretchr/testify/assert"
)

func TestCheckDifficulty_Good(t *testing.T) {
	// A zero hash meets any difficulty.
	hash := types.Hash{}
	assert.True(t, CheckDifficulty(hash, 1))
}

func TestCheckDifficulty_Bad(t *testing.T) {
	// Max hash (all 0xFF) should fail high difficulty.
	hash := types.Hash{}
	for i := range hash {
		hash[i] = 0xFF
	}
	assert.False(t, CheckDifficulty(hash, ^uint64(0)))
}
