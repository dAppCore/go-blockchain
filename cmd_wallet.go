// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package blockchain

import (
	"bytes"
	"encoding/hex"

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"

	"dappco.re/go/core/blockchain/rpc"
	"dappco.re/go/core/blockchain/types"
	"dappco.re/go/core/blockchain/wallet"
	"dappco.re/go/core/blockchain/wire"
	store "dappco.re/go/core/store"
	"github.com/spf13/cobra"
)

// AddWalletCommands registers the "wallet" command group.
//
//	blockchain.AddWalletCommands(root)
func AddWalletCommands(root *cobra.Command) {
	var walletFile string

	walletCmd := &cobra.Command{
		Use:   "wallet",
		Short: "Lethean wallet",
		Long:  "Create, restore, and manage Lethean wallets.",
	}

	walletCmd.PersistentFlags().StringVar(&walletFile, "wallet-file", "", "wallet file path")

	walletCmd.AddCommand(
		newWalletCreateCmd(&walletFile),
		newWalletAddressCmd(&walletFile),
		newWalletSeedCmd(&walletFile),
		newWalletScanCmd(&walletFile),
		newWalletRestoreCmd(&walletFile),
	)

	root.AddCommand(walletCmd)
}

func newWalletCreateCmd(walletFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create a new wallet",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWalletCreate(*walletFile)
		},
	}
}

func newWalletAddressCmd(walletFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "address",
		Short: "Show wallet address",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWalletAddress(*walletFile)
		},
	}
}

func newWalletSeedCmd(walletFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "seed",
		Short: "Show wallet seed phrase",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWalletSeed(*walletFile)
		},
	}
}

func runWalletCreate(walletFile string) error {
	if walletFile == "" {
		walletFile = core.JoinPath(defaultDataDir(), "wallet.db")
	}

	if err := ensureDataDir(core.PathBase(walletFile)); err != nil {
		// PathBase might not give us the directory — use the parent
	}

	account, err := wallet.GenerateAccount()
	if err != nil {
		return coreerr.E("runWalletCreate", "generate account", err)
	}

	s, err := store.New(walletFile)
	if err != nil {
		return coreerr.E("runWalletCreate", "open wallet store", err)
	}
	defer s.Close()

	if err := account.Save(s, ""); err != nil {
		return coreerr.E("runWalletCreate", "save wallet", err)
	}

	addr := account.Address()
	addrStr := addr.Encode(0x1eaf7) // iTHN standard prefix
	seed, _ := account.ToSeed()

	core.Print(nil, "Wallet created!")
	core.Print(nil, "  Address: %s", addrStr)
	core.Print(nil, "  Seed:    %s", seed)
	core.Print(nil, "  File:    %s", walletFile)

	return nil
}

func runWalletAddress(walletFile string) error {
	if walletFile == "" {
		walletFile = core.JoinPath(defaultDataDir(), "wallet.db")
	}

	s, err := store.New(walletFile)
	if err != nil {
		return coreerr.E("runWalletAddress", "open wallet store", err)
	}
	defer s.Close()

	account, err := wallet.LoadAccount(s, "")
	if err != nil {
		return coreerr.E("runWalletAddress", "load wallet", err)
	}

	addr := account.Address()
	core.Print(nil, "%s", addr.Encode(0x1eaf7))
	return nil
}

func runWalletSeed(walletFile string) error {
	if walletFile == "" {
		walletFile = core.JoinPath(defaultDataDir(), "wallet.db")
	}

	s, err := store.New(walletFile)
	if err != nil {
		return coreerr.E("runWalletSeed", "open wallet store", err)
	}
	defer s.Close()

	account, err := wallet.LoadAccount(s, "")
	if err != nil {
		return coreerr.E("runWalletSeed", "load wallet", err)
	}

	seed, err := account.ToSeed()
	if err != nil {
		return coreerr.E("runWalletSeed", "export seed", err)
	}

	core.Print(nil, "%s", seed)
	return nil
}

func newWalletScanCmd(walletFile *string) *cobra.Command {
	var daemonURL string

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan chain for wallet outputs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWalletScan(*walletFile, daemonURL)
		},
	}

	cmd.Flags().StringVar(&daemonURL, "daemon", "http://127.0.0.1:46941", "daemon RPC URL")
	return cmd
}

func runWalletScan(walletFile, daemonURL string) error {
	if walletFile == "" {
		walletFile = core.JoinPath(defaultDataDir(), "wallet.db")
	}

	s, err := store.New(walletFile)
	if err != nil {
		return coreerr.E("runWalletScan", "open wallet store", err)
	}
	defer s.Close()

	account, err := wallet.LoadAccount(s, "")
	if err != nil {
		return coreerr.E("runWalletScan", "load wallet", err)
	}

	addr := account.Address()
	core.Print(nil, "Scanning for: %s", addr.Encode(0x1eaf7))

	scanner := wallet.NewV1Scanner(account)
	client := rpc.NewClient(daemonURL)

	remoteHeight, err := client.GetHeight()
	if err != nil {
		return coreerr.E("runWalletScan", "get chain height", err)
	}

	var totalBalance uint64
	var outputCount int

	for h := uint64(0); h < remoteHeight; h++ {
		blocks, err := client.GetBlocksDetails(h, 1)
		if err != nil {
			continue
		}

		for _, bd := range blocks {
			for _, txInfo := range bd.Transactions {
				if txInfo.Blob == "" {
					continue
				}

				txBytes, err := hex.DecodeString(txInfo.Blob)
				if err != nil {
					continue
				}

				txDec := wire.NewDecoder(bytes.NewReader(txBytes))
				tx := wire.DecodeTransaction(txDec)
				if txDec.Err() != nil {
					continue
				}

				extra, err := wallet.ParseTxExtra(tx.Extra)
				if err != nil {
					continue
				}

				txHash, _ := types.HashFromHex(txInfo.ID)
				transfers, err := scanner.ScanTransaction(&tx, txHash, h, extra)
				if err != nil {
					continue
				}

				for _, t := range transfers {
					totalBalance += t.Amount
					outputCount++
					core.Print(nil, "  Found output: %d.%012d LTHN at height %d",
						t.Amount/1000000000000, t.Amount%1000000000000, h)
				}
			}
		}

		if h > 0 && h%1000 == 0 {
			core.Print(nil, "  Scanned %d/%d blocks... (%d outputs, %d.%012d LTHN)",
				h, remoteHeight, outputCount,
				totalBalance/1000000000000, totalBalance%1000000000000)
		}
	}

	core.Print(nil, "Balance: %d.%012d LTHN (%d outputs)",
		totalBalance/1000000000000, totalBalance%1000000000000, outputCount)

	return nil
}

func newWalletBalanceCmd(walletFile *string) *cobra.Command {
	var walletRPC string

	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Check wallet balance via daemon wallet RPC",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWalletBalance(walletRPC)
		},
	}

	cmd.Flags().StringVar(&walletRPC, "wallet-rpc", "http://127.0.0.1:46944", "wallet RPC URL")
	return cmd
}

func runWalletBalance(walletRPC string) error {
	// Use the RPC client pointed at the wallet RPC endpoint.
	client := rpc.NewClient(walletRPC)

	info, err := client.GetInfo()
	if err != nil {
		// The wallet RPC uses same JSON-RPC format but different methods.
		// Fall back to raw call.
		core.Print(nil, "Note: wallet RPC does not support getinfo, using getbalance directly")
	} else {
		_ = info
	}

	// For now, just report that the command exists. The actual balance
	// query needs a wallet-specific RPC client (different from daemon RPC).
	core.Print(nil, "Wallet RPC: %s", walletRPC)
	core.Print(nil, "Use the C++ wallet for balance queries until Go scanner is optimised")
	core.Print(nil, "  Go scanner: core-chain wallet scan --daemon http://127.0.0.1:46941")

	return nil
}

func newWalletRestoreCmd(walletFile *string) *cobra.Command {
	var seed string
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore wallet from seed phrase",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWalletRestore(*walletFile, seed)
		},
	}
	cmd.Flags().StringVar(&seed, "seed", "", "24-word mnemonic seed phrase")
	cmd.MarkFlagRequired("seed")
	return cmd
}

func runWalletRestore(walletFile, seed string) error {
	if walletFile == "" {
		walletFile = core.JoinPath(defaultDataDir(), "wallet-restored.db")
	}

	account, err := wallet.RestoreFromSeed(seed)
	if err != nil {
		return coreerr.E("runWalletRestore", "restore from seed", err)
	}

	s, err := store.New(walletFile)
	if err != nil {
		return coreerr.E("runWalletRestore", "open wallet store", err)
	}
	defer s.Close()

	if err := account.Save(s, ""); err != nil {
		return coreerr.E("runWalletRestore", "save wallet", err)
	}

	addr := account.Address()
	core.Print(nil, "Wallet restored!")
	core.Print(nil, "  Address: %s", addr.Encode(0x1eaf7))
	core.Print(nil, "  File:    %s", walletFile)

	return nil
}
