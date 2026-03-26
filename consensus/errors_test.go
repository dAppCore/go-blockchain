//go:build !integration

package consensus

import (
	"testing"

	"dappco.re/go/core"
	"github.com/stretchr/testify/assert"
)

func TestErrors_Errors_Good(t *testing.T) {
	assert.True(t, core.Is(ErrTxTooLarge, ErrTxTooLarge))
	assert.False(t, core.Is(ErrTxTooLarge, ErrNoInputs))
	assert.Contains(t, ErrTxTooLarge.Error(), "too large")
}
