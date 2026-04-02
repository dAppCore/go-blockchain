package blockchain

import (
	"context"
	"testing"

	"dappco.re/go/core"
)

func TestForge_PublishRelease_Good(t *testing.T) {
	opts := core.NewOptions(core.Option{Key: "version", Value: "0.3.0"})
	result := forgePublishRelease(context.Background(), opts)
	if !result.OK {
		t.Error("expected OK")
	}
}

func TestForge_PublishRelease_Bad_NoVersion(t *testing.T) {
	result := forgePublishRelease(context.Background(), core.Options{})
	if result.OK {
		t.Error("expected failure without version")
	}
}

func TestForge_DispatchBuild_Good(t *testing.T) {
	result := forgeDispatchBuild(context.Background(), core.Options{})
	if !result.OK {
		t.Error("expected OK")
	}
	m := result.Value.(map[string]interface{})
	if m["target"] != "testnet" {
		t.Errorf("default target: got %v, want testnet", m["target"])
	}
}
