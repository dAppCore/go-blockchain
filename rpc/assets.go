// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import (
	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
)

// AssetDescriptor describes a confidential asset for deployment.
//
//	desc := rpc.AssetDescriptor{Ticker: "ITNS", FullName: "IntenseCoin"}
type AssetDescriptor struct {
	Ticker       string `json:"ticker"`
	FullName     string `json:"full_name"`
	TotalMax     uint64 `json:"total_max_supply"`
	CurrentSup   uint64 `json:"current_supply"`
	DecimalPoint uint8  `json:"decimal_point"`
	HiddenSupply bool   `json:"hidden_supply"`
	MetaInfo     string `json:"meta_info"`
}

// DeployAssetResponse is the response from deploy_asset.
//
//	resp.AssetID // hex string
//	resp.TxID    // transaction hash
type DeployAssetResponse struct {
	AssetID string `json:"new_asset_id"`
	TxID    string `json:"tx_id"`
}

// DeployAsset deploys a new confidential asset on the chain (HF5+).
//
//	resp, err := client.DeployAsset(desc)
func (c *Client) DeployAsset(desc AssetDescriptor) (*DeployAssetResponse, error) {
	params := struct {
		Descriptor AssetDescriptor `json:"asset_descriptor"`
	}{Descriptor: desc}

	var resp DeployAssetResponse
	if err := c.call("deploy_asset", params, &resp); err != nil {
		return nil, coreerr.E("Client.DeployAsset", "deploy_asset", err)
	}
	return &resp, nil
}

// EmitAsset emits (mints) additional supply of an existing asset.
//
//	resp, err := client.EmitAsset(assetID, amount)
func (c *Client) EmitAsset(assetID string, amount uint64) (string, error) {
	params := struct {
		AssetID string `json:"asset_id"`
		Amount  uint64 `json:"amount"`
	}{AssetID: assetID, Amount: amount}

	var resp struct {
		TxID string `json:"tx_id"`
	}
	if err := c.call("emit_asset", params, &resp); err != nil {
		return "", coreerr.E("Client.EmitAsset", "emit_asset", err)
	}
	return resp.TxID, nil
}

// BurnAsset burns (destroys) supply of an asset.
//
//	resp, err := client.BurnAsset(assetID, amount)
func (c *Client) BurnAsset(assetID string, amount uint64) (string, error) {
	params := struct {
		AssetID string `json:"asset_id"`
		Amount  uint64 `json:"amount"`
	}{AssetID: assetID, Amount: amount}

	var resp struct {
		TxID string `json:"tx_id"`
	}
	if err := c.call("burn_asset", params, &resp); err != nil {
		return "", coreerr.E("Client.BurnAsset", "burn_asset", err)
	}
	return resp.TxID, nil
}

// GetAssetInfo retrieves the descriptor for an asset by ID or ticker.
//
//	info, err := client.GetAssetInfo("LTHN")
func (c *Client) GetAssetInfo(assetIDOrTicker string) (*AssetDescriptor, error) {
	params := struct {
		AssetID string `json:"asset_id"`
	}{AssetID: assetIDOrTicker}

	var resp struct {
		Descriptor AssetDescriptor `json:"asset_descriptor"`
		AssetID    string          `json:"asset_id"`
	}
	if err := c.call("get_asset_info", params, &resp); err != nil {
		return nil, coreerr.E("Client.GetAssetInfo", core.Sprintf("get_asset_info %s", assetIDOrTicker), err)
	}
	return &resp.Descriptor, nil
}
