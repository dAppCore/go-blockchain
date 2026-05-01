// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"os"

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
)

// WriteAtomic writes data to a file atomically (temp → rename).
// Safe for block/tx storage — no partial writes on crash.
//
//	chain.WriteAtomic("/data/blocks/11000.bin", blockBytes)
func WriteAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return coreerr.E("WriteAtomic", core.Sprintf("write temp %s", tmp), err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return coreerr.E("WriteAtomic", core.Sprintf("rename %s → %s", tmp, path), err)
	}
	return nil
}

// EnsureDir creates a directory and all parents if needed.
//
//	chain.EnsureDir("/data/blocks")
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}
