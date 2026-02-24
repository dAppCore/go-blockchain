// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"bytes"
	"errors"
	"fmt"

	"forge.lthn.ai/core/go-blockchain/config"
	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

// ValidateHeader checks a block header before storage.
// expectedHeight is the height at which this block would be stored.
func (c *Chain) ValidateHeader(b *types.Block, expectedHeight uint64) error {
	currentHeight, err := c.Height()
	if err != nil {
		return fmt.Errorf("validate: get height: %w", err)
	}

	// Height sequence check.
	if expectedHeight != currentHeight {
		return fmt.Errorf("validate: expected height %d but chain is at %d",
			expectedHeight, currentHeight)
	}

	// Genesis block: prev_id must be zero.
	if expectedHeight == 0 {
		if !b.PrevID.IsZero() {
			return errors.New("validate: genesis block has non-zero prev_id")
		}
		return nil
	}

	// Non-genesis: prev_id must match top block hash.
	_, topMeta, err := c.TopBlock()
	if err != nil {
		return fmt.Errorf("validate: get top block: %w", err)
	}
	if b.PrevID != topMeta.Hash {
		return fmt.Errorf("validate: prev_id %s does not match top block %s",
			b.PrevID, topMeta.Hash)
	}

	// Block size check.
	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	wire.EncodeBlock(enc, b)
	if enc.Err() == nil && uint64(buf.Len()) > config.MaxBlockSize {
		return fmt.Errorf("validate: block size %d exceeds max %d",
			buf.Len(), config.MaxBlockSize)
	}

	return nil
}
