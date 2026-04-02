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

func TestAction_AssetInfo_Good(t *testing.T) {
	result := actionAssetInfo(context.Background(), core.NewOptions(core.Option{Key: "asset_id", Value: "LTHN"}))
	if !result.OK { t.Fatal("failed") }
	m := result.Value.(map[string]interface{})
	if m["ticker"] != "LTHN" { t.Error("wrong ticker") }
}

func TestAction_AssetList_Good(t *testing.T) {
	result := actionAssetList(context.Background(), core.Options{})
	if !result.OK { t.Fatal("failed") }
}

func TestAction_AssetDeploy_Bad(t *testing.T) {
	result := actionAssetDeploy(context.Background(), core.Options{})
	if result.OK { t.Error("should fail without ticker") }
}

func TestAction_RegisterAll_Good(t *testing.T) {
	c := core.New()
	RegisterAllActions(c, nil)
	// Verify actions exist
	if !c.Action("blockchain.chain.height").Exists() { t.Error("chain.height not registered") }
	if !c.Action("blockchain.wallet.create").Exists() { t.Error("wallet.create not registered") }
	if !c.Action("blockchain.crypto.hash").Exists() { t.Error("crypto.hash not registered") }
	if !c.Action("blockchain.asset.info").Exists() { t.Error("asset.info not registered") }
	if !c.Action("blockchain.forge.release").Exists() { t.Error("forge.release not registered") }
}

func TestAction_HSDResolve_Bad_NoName(t *testing.T) {
	handler := makeHSDResolve("http://127.0.0.1:14037", "testkey")
	result := handler(context.Background(), core.Options{})
	if result.OK {
		t.Error("should fail without name")
	}
}

func TestAction_RegisterAllActions_Good_Count(t *testing.T) {
	c := core.New()
	// Can't call RegisterAllActions with nil chain for some actions
	// but we can verify the action count
	RegisterWalletActions(c)
	RegisterCryptoActions(c)
	RegisterAssetActions(c)
	RegisterForgeActions(c)

	allActions := c.Actions()
	if len(allActions) < 14 {
		t.Errorf("expected 14+ actions, got %d", len(allActions))
	}
}
