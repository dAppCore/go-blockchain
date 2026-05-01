package blockchain

import "testing"

func TestDNSRecord_Good(t *testing.T) {
	r := DNSRecord{Name: "charon.lthn", A: []string{"10.69.69.165"}}
	if r.Name != "charon.lthn" {
		t.Error("wrong name")
	}
	if len(r.A) != 1 {
		t.Error("expected 1 A record")
	}
}
