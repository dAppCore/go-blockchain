//go:build !integration

package consensus

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrors_Good(t *testing.T) {
	assert.True(t, errors.Is(ErrTxTooLarge, ErrTxTooLarge))
	assert.False(t, errors.Is(ErrTxTooLarge, ErrNoInputs))
	assert.Contains(t, ErrTxTooLarge.Error(), "too large")
}
