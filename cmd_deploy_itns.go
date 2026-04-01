// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package blockchain

import (
	"log"

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
		TotalMax:     1000000000000000000, // 1B at 12 decimals
		CurrentSup:   0,
		DecimalPoint: 12,
		HiddenSupply: false,
		MetaInfo:     core.Concat(`{"network":"lethean","type":"trust","purpose":"sidechain gateway trust token"}`),
	}

	log.Println("Deploying ITNS (IntenseCoin) confidential asset...")
	log.Printf("  Ticker:     %s", desc.Ticker)
	log.Printf("  Name:       %s", desc.FullName)
	log.Printf("  Max supply: %d (1B ITNS)", desc.TotalMax)
	log.Printf("  Decimals:   %d", desc.DecimalPoint)

	resp, err := client.DeployAsset(desc)
	if err != nil {
		return coreerr.E("runDeployITNS", "deploy failed", err)
	}

	log.Println("")
	log.Println("ITNS DEPLOYED!")
	log.Printf("  Asset ID: %s", resp.AssetID)
	log.Printf("  TX Hash:  %s", resp.TxID)

	return nil
}

func runAssetInfo(walletRPC, assetID string) error {
	client := rpc.NewClient(walletRPC)

	info, err := client.GetAssetInfo(assetID)
	if err != nil {
		return coreerr.E("runAssetInfo", core.Sprintf("asset %s", assetID), err)
	}

	log.Printf("Asset: %s (%s)", info.Ticker, info.FullName)
	log.Printf("  Max supply:     %d", info.TotalMax)
	log.Printf("  Current supply: %d", info.CurrentSup)
	log.Printf("  Decimals:       %d", info.DecimalPoint)
	log.Printf("  Hidden supply:  %v", info.HiddenSupply)

	return nil
}
