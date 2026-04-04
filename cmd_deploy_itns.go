// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2
package blockchain

import (

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"

	"dappco.re/go/core/blockchain/rpc"
	"github.com/spf13/cobra"
)

// AddAssetCommands registers the "asset" command group.
//
//	blockchain.AddAssetCommands(root)
func AddAssetCommands(root *cobra.Command) {
	assetCmd := &cobra.Command{
		Use:   "asset",
		Short: "Confidential asset operations (HF5+)",
	}

	var walletRPC string
	assetCmd.PersistentFlags().StringVar(&walletRPC, "wallet-rpc", "http://127.0.0.1:46944", "wallet RPC URL")

	assetCmd.AddCommand(
		newDeployITNSCmd(&walletRPC),
		newAssetInfoCmd(&walletRPC),
	)

	root.AddCommand(assetCmd)
}

func newDeployITNSCmd(walletRPC *string) *cobra.Command {
	return &cobra.Command{
		Use:   "deploy-itns",
		Short: "Deploy the ITNS trust token (requires HF5)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeployITNS(*walletRPC)
		},
	}
}

func newAssetInfoCmd(walletRPC *string) *cobra.Command {
	var assetID string
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Get asset info by ID or ticker",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAssetInfo(*walletRPC, assetID)
		},
	}
	cmd.Flags().StringVar(&assetID, "asset", "LTHN", "asset ID or ticker")
	return cmd
}

func runDeployITNS(walletRPC string) error {
	client := rpc.NewClient(walletRPC)

	desc := rpc.AssetDescriptor{
		Ticker:       "ITNS",
		FullName:     "IntenseCoin",
		TotalMax:     AtomicUnit * 1000000, // 1B at 12 decimals
		CurrentSup:   0,
		DecimalPoint: 12,
		HiddenSupply: false,
		MetaInfo:     core.Concat(`{"network":"lethean","type":"trust","purpose":"sidechain gateway trust token"}`),
	}

	core.Print(nil, "Deploying ITNS (IntenseCoin) confidential asset...")
	core.Print(nil, "  Ticker:     %s", desc.Ticker)
	core.Print(nil, "  Name:       %s", desc.FullName)
	core.Print(nil, "  Max supply: %d (1B ITNS)", desc.TotalMax)
	core.Print(nil, "  Decimals:   %d", desc.DecimalPoint)

	resp, err := client.DeployAsset(desc)
	if err != nil {
		return coreerr.E("runDeployITNS", "deploy failed", err)
	}

	core.Print(nil, "")
	core.Print(nil, "ITNS DEPLOYED!")
	core.Print(nil, "  Asset ID: %s", resp.AssetID)
	core.Print(nil, "  TX Hash:  %s", resp.TxID)

	return nil
}

func runAssetInfo(walletRPC, assetID string) error {
	client := rpc.NewClient(walletRPC)

	info, err := client.GetAssetInfo(assetID)
	if err != nil {
		return coreerr.E("runAssetInfo", core.Sprintf("asset %s", assetID), err)
	}

	core.Print(nil, "Asset: %s (%s)", info.Ticker, info.FullName)
	core.Print(nil, "  Max supply:     %d", info.TotalMax)
	core.Print(nil, "  Current supply: %d", info.CurrentSup)
	core.Print(nil, "  Decimals:       %d", info.DecimalPoint)
	core.Print(nil, "  Hidden supply:  %v", info.HiddenSupply)

	return nil
}
