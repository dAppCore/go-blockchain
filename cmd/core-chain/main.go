// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package main

import (
	blockchain "dappco.re/go/core/blockchain"
	cli "dappco.re/go/core/cli/pkg/cli"
)

func main() {
	cli.WithAppName("core-chain")
	cli.Main(
		cli.WithCommands("chain", blockchain.AddChainCommands),
	)
}
