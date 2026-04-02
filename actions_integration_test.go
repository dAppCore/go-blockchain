//go:build integration

package blockchain

import (
	"context"
	"testing"

	"dappco.re/go/core"
	"dappco.re/go/core/blockchain/chain"
	store "dappco.re/go/core/store"
)

func setupLiveChain(t *testing.T) (*core.Core, *chain.Chain) {
	t.Helper()
	s, err := store.New("/tmp/go-full-sync/chain.db")
	if err != nil {
		t.Skip("no synced chain data at /tmp/go-full-sync/chain.db")
	}
	t.Cleanup(func() { s.Close() })
	ch := chain.New(s)
	c := core.New()
	RegisterAllActions(c, ch, "http://127.0.0.1:14037", "testkey")
	return c, ch
}

func TestActionsIntegration_ChainHeight_Good(t *testing.T) {
	c, _ := setupLiveChain(t)
	result := c.Action("blockchain.chain.height").Run(context.Background(), core.Options{})
	if !result.OK { t.Fatal("failed") }
	height := result.Value.(uint64)
	if height < 11000 {
		t.Errorf("expected height > 11000, got %d", height)
	}
}

func TestActionsIntegration_Aliases_Good(t *testing.T) {
	c, _ := setupLiveChain(t)
	result := c.Action("blockchain.alias.list").Run(context.Background(), core.Options{})
	if !result.OK { t.Fatal("failed") }
	aliases := result.Value.([]chain.Alias)
	if len(aliases) != 14 {
		t.Errorf("expected 14 aliases, got %d", len(aliases))
	}
}

func TestActionsIntegration_GatewayDiscovery_Good(t *testing.T) {
	c, _ := setupLiveChain(t)
	result := c.Action("blockchain.network.gateways").Run(context.Background(), core.Options{})
	if !result.OK { t.Fatal("failed") }
	gateways := result.Value.([]map[string]string)
	if len(gateways) == 0 {
		t.Error("expected at least 1 gateway")
	}
	// Verify charon is a gateway
	found := false
	for _, g := range gateways {
		if g["name"] == "charon" { found = true }
	}
	if !found {
		t.Error("charon not found in gateways")
	}
}

func TestActionsIntegration_Supply_Good(t *testing.T) {
	c, _ := setupLiveChain(t)
	result := c.Action("blockchain.supply.total").Run(context.Background(), core.Options{})
	if !result.OK { t.Fatal("failed") }
	supply := result.Value.(map[string]interface{})
	total := supply["total"].(uint64)
	if total < 10011000 {
		t.Errorf("expected total > 10011000, got %d", total)
	}
}

func TestActionsIntegration_DNSDiscover_Good(t *testing.T) {
	c, _ := setupLiveChain(t)
	result := c.Action("blockchain.dns.discover").Run(context.Background(), core.Options{})
	if !result.OK { t.Fatal("failed") }
	names := result.Value.([]string)
	if len(names) < 10 {
		t.Errorf("expected 10+ DNS names, got %d", len(names))
	}
}

func TestActionsIntegration_WalletCreateVerify_Good(t *testing.T) {
	c, _ := setupLiveChain(t)

	// Create wallet
	createResult := c.Action("blockchain.wallet.create").Run(context.Background(), core.Options{})
	if !createResult.OK { t.Fatal("create failed") }
	m := createResult.Value.(map[string]interface{})
	addr := m["address"].(string)
	seed := m["seed"].(string)

	// Validate the created address
	validateResult := c.Action("blockchain.crypto.validate_address").Run(context.Background(),
		core.NewOptions(core.Option{Key: "address", Value: addr}))
	if !validateResult.OK { t.Fatal("validate failed") }
	v := validateResult.Value.(map[string]interface{})
	if !v["valid"].(bool) {
		t.Error("created address failed validation")
	}

	// Restore from seed
	addrResult := c.Action("blockchain.wallet.address").Run(context.Background(),
		core.NewOptions(core.Option{Key: "seed", Value: seed}))
	if !addrResult.OK { t.Fatal("restore failed") }
	derived := addrResult.Value.(map[string]interface{})
	if derived["standard"] != addr {
		t.Error("seed restore produced different address")
	}
}
