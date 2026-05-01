package daemon

import "testing"

func TestWalletProxy_IsWalletMethod_Good(t *testing.T) {
	walletMethods := []string{"getbalance", "getaddress", "transfer", "deploy_asset", "register_alias"}
	for _, m := range walletMethods {
		if !IsWalletMethod(m) {
			t.Errorf("%s should be a wallet method", m)
		}
	}
}

func TestWalletProxy_IsWalletMethod_Bad(t *testing.T) {
	chainMethods := []string{"getinfo", "getheight", "get_all_alias_details", "search"}
	for _, m := range chainMethods {
		if IsWalletMethod(m) {
			t.Errorf("%s should NOT be a wallet method", m)
		}
	}
}

func TestWalletProxy_New_Good(t *testing.T) {
	proxy := NewWalletProxy("http://127.0.0.1:46944")
	if proxy == nil {
		t.Fatal("proxy is nil")
	}
	if proxy.walletURL != "http://127.0.0.1:46944" {
		t.Errorf("url: %s", proxy.walletURL)
	}
}
