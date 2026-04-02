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
}

func TestBlockchainService_SeedToRPC_Good(t *testing.T) {
	tests := []struct{ seed string; testnet bool; want string }{
		{"127.0.0.1:46942", true, "http://127.0.0.1:46941"},
		{"127.0.0.1:36942", false, "http://127.0.0.1:36941"},
		{"10.0.0.1:9999", false, "http://10.0.0.1:9999"},
	}
	for _, tt := range tests {
		got := seedToRPC(tt.seed, tt.testnet)
		if got != tt.want {
			t.Errorf("seedToRPC(%s): got %s, want %s", tt.seed, got, tt.want)
		}
	}
}
