// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

//go:build integration

package hsd

import (
	"testing"
)

func TestClient_GetBlockchainInfo_Good(t *testing.T) {
	client := NewClient("http://127.0.0.1:14037", "testkey")
	info, err := client.GetBlockchainInfo()
	if err != nil {
		t.Fatalf("GetBlockchainInfo: %v", err)
	}
	if info.Blocks == 0 {
		t.Error("expected non-zero block count")
	}
	if info.TreeRoot == "" {
		t.Error("expected non-empty tree root")
	}
}

func TestClient_GetNameResource_Good(t *testing.T) {
	client := NewClient("http://127.0.0.1:14037", "testkey")
	resource, err := client.GetNameResource("charon")
	if err != nil {
		t.Fatalf("GetNameResource: %v", err)
	}
	if len(resource.Records) == 0 {
		t.Error("expected records for charon")
	}
	hasGLUE4 := false
	for _, r := range resource.Records {
		if r.Type == "GLUE4" {
			hasGLUE4 = true
			if r.Address == "" {
				t.Error("GLUE4 record has no address")
			}
		}
	}
	if !hasGLUE4 {
		t.Error("expected GLUE4 record for charon")
	}
}

func TestClient_GetNameResource_Bad_NotFound(t *testing.T) {
	client := NewClient("http://127.0.0.1:14037", "testkey")
	resource, err := client.GetNameResource("nonexistent_name_12345")
	if err != nil {
		// Error is acceptable for non-existent names
		return
	}
	if resource != nil && len(resource.Records) > 0 {
		t.Error("expected no records for nonexistent name")
	}
}

func TestClient_GetHeight_Good(t *testing.T) {
	client := NewClient("http://127.0.0.1:14037", "testkey")
	height, err := client.GetHeight()
	if err != nil {
		t.Fatalf("GetHeight: %v", err)
	}
	if height == 0 {
		t.Error("expected non-zero height")
	}
}

func TestClient_Bad_WrongURL(t *testing.T) {
	client := NewClient("http://127.0.0.1:19999", "badkey")
	_, err := client.GetBlockchainInfo()
	if err == nil {
		t.Error("expected error for wrong URL")
	}
}
