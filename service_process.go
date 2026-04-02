// Copyright (c) 2017-2026 Lethean (https://lt.hn)
// SPDX-License-Identifier: EUPL-1.2

package blockchain

import (
	"dappco.re/go/core"
	"dappco.re/go/core/process"
)

// NewDaemonProcess creates a go-process managed daemon for the blockchain service.
//
//	d := blockchain.NewDaemonProcess(dataDir)
//	d.Start()
//	d.SetReady(true)
//	d.Run(ctx)
func NewDaemonProcess(dataDir string) *process.Daemon {
	return process.NewDaemon(process.DaemonOptions{
		PIDFile:    core.JoinPath(dataDir, "blockchain.pid"),
		HealthAddr: ":47942",
		Registry:   process.DefaultRegistry(),
		RegistryEntry: process.DaemonEntry{
			Code:   "dappco.re/go/core/blockchain",
			Daemon: "chain",
		},
	})
}
