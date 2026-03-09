// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package main

import (
	cli "forge.lthn.ai/core/cli/pkg/cli"
	blockchain "forge.lthn.ai/core/go-blockchain"
)

func main() {
	cli.WithAppName("core-chain")
	cli.Main(
		cli.WithCommands("chain", blockchain.AddChainCommands),
	)
}
