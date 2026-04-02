package blockchain

import (
	"context"
	"testing"

	"dappco.re/go/core"
)

func TestAction_WalletCreate_Good(t *testing.T) {
	result := actionWalletCreate(context.Background(), core.Options{})
	if !result.OK {
		t.Fatal("wallet create failed")
	}
	m := result.Value.(map[string]interface{})
	addr := m["address"].(string)
	if len(addr) < 90 || addr[:4] != "iTHN" {
		t.Errorf("bad address: %s", addr[:20])
	}
	seed := m["seed"].(string)
	if len(seed) < 50 {
		t.Errorf("bad seed: too short")
	}
}

func TestAction_WalletSeed_Good(t *testing.T) {
	result := actionWalletSeed(context.Background(), core.Options{})
	if !result.OK {
		t.Fatal("wallet seed failed")
	}
}

func TestAction_Hash_Good(t *testing.T) {
	opts := core.NewOptions(core.Option{Key: "data", Value: "hello"})
	result := actionHash(context.Background(), opts)
	if !result.OK {
		t.Fatal("hash failed")
	}
	hash := result.Value.(string)
	if len(hash) != 64 {
		t.Errorf("hash length: %d, want 64", len(hash))
	}
}

func TestAction_GenerateKeys_Good(t *testing.T) {
	result := actionGenerateKeys(context.Background(), core.Options{})
	if !result.OK {
		t.Fatal("generate keys failed")
	}
	m := result.Value.(map[string]interface{})
	if len(m["public"].(string)) != 64 {
		t.Error("bad public key length")
	}
}

func TestAction_ValidateAddress_Good(t *testing.T) {
	// First create a wallet to get a valid address
	createResult := actionWalletCreate(context.Background(), core.Options{})
	addr := createResult.Value.(map[string]interface{})["address"].(string)

	opts := core.NewOptions(core.Option{Key: "address", Value: addr})
	result := actionValidateAddress(context.Background(), opts)
	if !result.OK {
		t.Fatal("validate failed")
	}
	m := result.Value.(map[string]interface{})
	if !m["valid"].(bool) {
		t.Error("expected valid")
	}
	if m["type"] != "standard" {
		t.Errorf("type: got %v, want standard", m["type"])
	}
}
