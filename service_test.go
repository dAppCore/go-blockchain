package blockchain

import (
	"testing"

	"dappco.re/go/core"
)

func TestBlockchainService_New_Good(t *testing.T) {
	c := core.New()
	opts := BlockchainOptions{
		DataDir: t.TempDir(),
		Seed:    "127.0.0.1:46942",
		Testnet: true,
		RPCPort: "48999",
		RPCBind: "127.0.0.1",
	}
	svc := NewBlockchainService(c, opts)
	if svc == nil {
		t.Fatal("service is nil")
	}
	if svc.opts.Testnet != true {
		t.Error("expected testnet")
	}
	expectedActions := []string{
		"blockchain.chain.height",
		"blockchain.chain.info",
		"blockchain.chain.block",
		"blockchain.alias.list",
		"blockchain.alias.get",
		"blockchain.wallet.create",
	}
	for _, name := range expectedActions {
		if !c.Action(name).Exists() {
			t.Fatalf("expected action %s to be registered", name)
		}
	}
}

func TestBlockchainService_SeedToRPC_Good(t *testing.T) {
	tests := []struct {
		seed string
		want string
	}{
		{"127.0.0.1:46942", "http://127.0.0.1:46941"},
		{"127.0.0.1:36942", "http://127.0.0.1:36941"},
		{"10.0.0.1:9999", "http://10.0.0.1:9999"},
	}
	for _, tt := range tests {
		got := seedToRPC(tt.seed)
		if got != tt.want {
			t.Errorf("seedToRPC(%s): got %s, want %s", tt.seed, got, tt.want)
		}
	}
}
