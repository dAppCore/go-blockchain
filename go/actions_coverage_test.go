package blockchain

import (
	"context"
	"testing"

	"dappco.re/go/core"
	"dappco.re/go/core/blockchain/chain"
	store "dappco.re/go/core/store"
)

func TestActions_AllRegistered_Good(t *testing.T) {
	c := core.New()
	dir := t.TempDir()
	s, _ := store.New(dir + "/test.db")
	defer s.Close()
	ch := chain.New(s)

	RegisterAllActions(c, ch, "http://127.0.0.1:14037", "testkey")

	expected := []string{
		// Chain (10)
		"blockchain.chain.height", "blockchain.chain.info", "blockchain.chain.block",
		"blockchain.chain.synced", "blockchain.chain.hardforks", "blockchain.chain.stats",
		"blockchain.chain.search", "blockchain.chain.difficulty", "blockchain.chain.transaction",
		"blockchain.chain.peers",
		// Alias (3)
		"blockchain.alias.list", "blockchain.alias.get", "blockchain.alias.capabilities",
		// Network (8)
		"blockchain.network.gateways", "blockchain.network.topology",
		"blockchain.network.vpn", "blockchain.network.dns",
		"blockchain.network.services", "blockchain.network.discover",
		"blockchain.network.vpn.endpoints", "blockchain.network.gateway.register",
		// Supply (4)
		"blockchain.supply.total", "blockchain.supply.hashrate",
		"blockchain.supply.emission", "blockchain.supply.circulating",
		// Wallet (8)
		"blockchain.wallet.create", "blockchain.wallet.address", "blockchain.wallet.seed",
		"blockchain.wallet.restore", "blockchain.wallet.info", "blockchain.wallet.validate",
		"blockchain.wallet.balance", "blockchain.wallet.history",
		// Crypto (4)
		"blockchain.crypto.hash", "blockchain.crypto.generate_keys",
		"blockchain.crypto.check_key", "blockchain.crypto.validate_address",
		// Asset (8)
		"blockchain.asset.info", "blockchain.asset.list", "blockchain.asset.deploy",
		"blockchain.asset.emit", "blockchain.asset.burn", "blockchain.asset.balance",
		"blockchain.asset.transfer", "blockchain.asset.whitelist",
		// Escrow (5)
		"blockchain.escrow.create", "blockchain.escrow.fund",
		"blockchain.escrow.release", "blockchain.escrow.refund", "blockchain.escrow.status",
		// Forge (4)
		"blockchain.forge.release", "blockchain.forge.issue",
		"blockchain.forge.build", "blockchain.forge.event",
		// HSD (3)
		"blockchain.hsd.info", "blockchain.hsd.resolve", "blockchain.hsd.height",
		// DNS (3)
		"blockchain.dns.resolve", "blockchain.dns.names", "blockchain.dns.discover",
		// Relay (1)
		"blockchain.relay.info",
		// Identity (2)
		"blockchain.identity.lookup", "blockchain.identity.verify",
		// Mining (3)
		"blockchain.mining.template", "blockchain.mining.difficulty", "blockchain.mining.reward",
		// Estimate (3)
		"blockchain.estimate.supply_at_height", "blockchain.estimate.block_time",
		"blockchain.estimate.height_at_time",
	}

	missing := 0
	for _, name := range expected {
		if !c.Action(name).Exists() {
			t.Errorf("MISSING: %s", name)
			missing++
		}
	}
	if missing > 0 {
		t.Fatalf("%d/%d actions missing", missing, len(expected))
	}
	t.Logf("%d/%d actions registered", len(expected), len(expected))
}

func TestActions_ChainActionsWork_Good(t *testing.T) {
	c := core.New()
	dir := t.TempDir()
	s, _ := store.New(dir + "/test.db")
	defer s.Close()
	ch := chain.New(s)

	RegisterActions(c, ch)

	// Test each chain action returns OK on empty chain
	chainActions := []string{
		"blockchain.chain.height", "blockchain.chain.info",
		"blockchain.chain.synced", "blockchain.chain.hardforks",
		"blockchain.chain.stats",
	}

	for _, name := range chainActions {
		result := c.Action(name).Run(context.Background(), core.Options{})
		if !result.OK {
			t.Errorf("%s returned OK=false on empty chain", name)
		}
	}
}

func TestActions_WalletActionsWork_Good(t *testing.T) {
	c := core.New()
	RegisterWalletActions(c)

	// Create should work
	result := c.Action("blockchain.wallet.create").Run(context.Background(), core.Options{})
	if !result.OK {
		t.Fatal("wallet.create failed")
	}

	m := result.Value.(map[string]interface{})
	addr := m["address"].(string)
	seed := m["seed"].(string)

	if addr[:4] != "iTHN" {
		t.Errorf("bad address prefix: %s", addr[:4])
	}

	// Verify seed can derive address
	addrResult := c.Action("blockchain.wallet.address").Run(context.Background(),
		core.NewOptions(core.Option{Key: "seed", Value: seed}))
	if !addrResult.OK {
		t.Fatal("wallet.address failed")
	}
	derived := addrResult.Value.(map[string]interface{})
	if derived["standard"] != addr {
		t.Errorf("address mismatch: created=%s derived=%s", addr[:20], derived["standard"].(string)[:20])
	}
}

func TestActions_CryptoActionsWork_Good(t *testing.T) {
	c := core.New()
	RegisterCryptoActions(c)

	// Hash
	hashResult := c.Action("blockchain.crypto.hash").Run(context.Background(),
		core.NewOptions(core.Option{Key: "data", Value: "test"}))
	if !hashResult.OK {
		t.Fatal("crypto.hash failed")
	}
	hash := hashResult.Value.(string)
	if len(hash) != 64 {
		t.Errorf("hash length: %d", len(hash))
	}

	// Same input = same hash
	hashResult2 := c.Action("blockchain.crypto.hash").Run(context.Background(),
		core.NewOptions(core.Option{Key: "data", Value: "test"}))
	if hashResult2.Value != hashResult.Value {
		t.Error("deterministic hash failed")
	}

	// Generate keys
	keyResult := c.Action("blockchain.crypto.generate_keys").Run(context.Background(), core.Options{})
	if !keyResult.OK {
		t.Fatal("crypto.generate_keys failed")
	}
	keys := keyResult.Value.(map[string]interface{})
	if len(keys["public"].(string)) != 64 {
		t.Error("bad public key")
	}
	if len(keys["secret"].(string)) != 64 {
		t.Error("bad secret key")
	}
}

func TestActions_SupplyCalculation_Good(t *testing.T) {
	c := core.New()
	dir := t.TempDir()
	s, _ := store.New(dir + "/test.db")
	defer s.Close()
	ch := chain.New(s)

	RegisterActions(c, ch)

	result := c.Action("blockchain.supply.total").Run(context.Background(), core.Options{})
	if !result.OK {
		t.Fatal("supply.total failed")
	}
	supply := result.Value.(map[string]interface{})
	premine := supply["premine"].(uint64)
	if premine != PremineAmount {
		t.Errorf("premine: %d, want %d", premine, PremineAmount)
	}
}
