// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import "fmt"

// SubmitBlock submits a mined block to the daemon.
// The hexBlob is the hex-encoded serialised block.
// Note: submitblock takes a JSON array as params, not an object.
func (c *Client) SubmitBlock(hexBlob string) error {
	// submitblock expects params as an array: ["hexblob"]
	params := []string{hexBlob}
	var resp struct {
		Status string `json:"status"`
	}
	if err := c.call("submitblock", params, &resp); err != nil {
		return err
	}
	if resp.Status != "OK" {
		return fmt.Errorf("submitblock: status %q", resp.Status)
	}
	return nil
}
