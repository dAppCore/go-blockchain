// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package rpc

import (
	"net/http"
	"testing"

	"dappco.re/go/core"
)

func mustJSONMarshal(t *testing.T, v any) []byte {
	t.Helper()

	result := core.JSONMarshal(v)
	if !result.OK {
		t.Fatalf("marshal json: %#v", result.Value)
	}

	data, ok := result.Value.([]byte)
	if !ok {
		t.Fatalf("marshal json returned %T, want []byte", result.Value)
	}
	return data
}

func mustJSONUnmarshal(t *testing.T, data []byte, target any) {
	t.Helper()

	result := core.JSONUnmarshal(data, target)
	if !result.OK {
		t.Fatalf("unmarshal json: %#v", result.Value)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(mustJSONMarshal(t, v)); err != nil {
		t.Fatalf("write json: %v", err)
	}
}
