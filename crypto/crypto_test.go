// SPDX-Licence-Identifier: EUPL-1.2

package crypto_test

import (
	"encoding/hex"
	"testing"

	"forge.lthn.ai/core/go-blockchain/crypto"
)

func TestFastHash_Good_KnownVector(t *testing.T) {
	// Empty input → known Keccak-256 hash.
	input := []byte{}
	expected := "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"

	got := crypto.FastHash(input)
	if hex.EncodeToString(got[:]) != expected {
		t.Fatalf("FastHash(empty)\n  got:  %x\n  want: %s", got, expected)
	}
}

func TestFastHash_Good_HelloWorld(t *testing.T) {
	input := []byte("Hello, World!")
	got := crypto.FastHash(input)
	var zero [32]byte
	if got == zero {
		t.Fatal("FastHash returned zero hash")
	}
}

func TestGenerateKeys_Good_Roundtrip(t *testing.T) {
	pub, sec, err := crypto.GenerateKeys()
	if err != nil {
		t.Fatalf("GenerateKeys: %v", err)
	}

	if !crypto.CheckKey(pub) {
		t.Fatal("generated public key failed CheckKey")
	}

	pub2, err := crypto.SecretToPublic(sec)
	if err != nil {
		t.Fatalf("SecretToPublic: %v", err)
	}
	if pub != pub2 {
		t.Fatalf("SecretToPublic mismatch:\n  GenerateKeys: %x\n  SecretToPublic: %x", pub, pub2)
	}
}

func TestCheckKey_Bad_Invalid(t *testing.T) {
	// A random 32-byte value is overwhelmingly unlikely to be a valid curve point.
	bad := [32]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	if crypto.CheckKey(bad) {
		t.Fatal("0xFF...FF should fail CheckKey")
	}
}

func TestGenerateKeys_Good_Unique(t *testing.T) {
	pub1, _, _ := crypto.GenerateKeys()
	pub2, _, _ := crypto.GenerateKeys()
	if pub1 == pub2 {
		t.Fatal("two GenerateKeys calls returned identical public keys")
	}
}
