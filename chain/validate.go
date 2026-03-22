// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"bytes"
	"fmt"

	coreerr "dappco.re/go/core/log"

	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/types"
	"dappco.re/go/core/blockchain/wire"
)

// ValidateHeader checks a block header before storage.
// expectedHeight is the height at which this block would be stored.
func (c *Chain) ValidateHeader(b *types.Block, expectedHeight uint64) error {
	currentHeight, err := c.Height()
	if err != nil {
		return coreerr.E("Chain.ValidateHeader", "validate: get height", err)
	}

	// Height sequence check.
	if expectedHeight != currentHeight {
		return coreerr.E("Chain.ValidateHeader", fmt.Sprintf("validate: expected height %d but chain is at %d", expectedHeight, currentHeight), nil)
	}

	// Genesis block: prev_id must be zero.
	if expectedHeight == 0 {
		if !b.PrevID.IsZero() {
			return coreerr.E("Chain.ValidateHeader", "validate: genesis block has non-zero prev_id", nil)
		}
		return nil
	}

	// Non-genesis: prev_id must match top block hash.
	_, topMeta, err := c.TopBlock()
	if err != nil {
		return coreerr.E("Chain.ValidateHeader", "validate: get top block", err)
	}
	if b.PrevID != topMeta.Hash {
		return coreerr.E("Chain.ValidateHeader", fmt.Sprintf("validate: prev_id %s does not match top block %s", b.PrevID, topMeta.Hash), nil)
	}

	// Block size check.
	var buf bytes.Buffer
	enc := wire.NewEncoder(&buf)
	wire.EncodeBlock(enc, b)
	if enc.Err() == nil && uint64(buf.Len()) > config.MaxBlockSize {
		return coreerr.E("Chain.ValidateHeader", fmt.Sprintf("validate: block size %d exceeds max %d", buf.Len(), config.MaxBlockSize), nil)
	}

	return nil
}
