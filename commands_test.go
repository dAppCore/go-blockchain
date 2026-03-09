// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package blockchain

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddChainCommands_Good_RegistersParent(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	AddChainCommands(root)

	chainCmd, _, err := root.Find([]string{"chain"})
	require.NoError(t, err)
	assert.Equal(t, "chain", chainCmd.Name())
}

func TestAddChainCommands_Good_HasSubcommands(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	AddChainCommands(root)

	chainCmd, _, _ := root.Find([]string{"chain"})

	var names []string
	for _, sub := range chainCmd.Commands() {
		names = append(names, sub.Name())
	}
	assert.Contains(t, names, "explorer")
	assert.Contains(t, names, "sync")
}

func TestAddChainCommands_Good_PersistentFlags(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	AddChainCommands(root)

	chainCmd, _, _ := root.Find([]string{"chain"})

	assert.NotNil(t, chainCmd.PersistentFlags().Lookup("data-dir"))
	assert.NotNil(t, chainCmd.PersistentFlags().Lookup("seed"))
	assert.NotNil(t, chainCmd.PersistentFlags().Lookup("testnet"))
}
