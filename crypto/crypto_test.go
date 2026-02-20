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

// ── Key Derivation ────────────────────────────────────────

func TestKeyDerivation_Good_Roundtrip(t *testing.T) {
	// Alice and Bob generate key pairs; shared derivation must match.
	pubA, secA, err := crypto.GenerateKeys()
	if err != nil {
		t.Fatalf("GenerateKeys (Alice): %v", err)
	}
	pubB, secB, err := crypto.GenerateKeys()
	if err != nil {
		t.Fatalf("GenerateKeys (Bob): %v", err)
	}

	// D = secA * pubB  must equal  secB * pubA  (ECDH commutativity).
	dAB, err := crypto.GenerateKeyDerivation(pubB, secA)
	if err != nil {
		t.Fatalf("GenerateKeyDerivation(pubB, secA): %v", err)
	}
	dBA, err := crypto.GenerateKeyDerivation(pubA, secB)
	if err != nil {
		t.Fatalf("GenerateKeyDerivation(pubA, secB): %v", err)
	}
	if dAB != dBA {
		t.Fatalf("ECDH mismatch:\n  dAB: %x\n  dBA: %x", dAB, dBA)
	}
}

func TestDerivePublicKey_Good_OutputScanning(t *testing.T) {
	// Simulate one-time address generation and scanning.
	// Sender knows (txPub, recipientPub). Recipient knows (txPub, recipientSec).
	recipientPub, recipientSec, err := crypto.GenerateKeys()
	if err != nil {
		t.Fatalf("GenerateKeys (recipient): %v", err)
	}
	txPub, txSec, err := crypto.GenerateKeys()
	if err != nil {
		t.Fatalf("GenerateKeys (tx): %v", err)
	}

	// Sender derives ephemeral output key.
	derivSender, err := crypto.GenerateKeyDerivation(recipientPub, txSec)
	if err != nil {
		t.Fatalf("sender derivation: %v", err)
	}
	ephPub, err := crypto.DerivePublicKey(derivSender, 0, recipientPub)
	if err != nil {
		t.Fatalf("DerivePublicKey: %v", err)
	}
	if !crypto.CheckKey(ephPub) {
		t.Fatal("ephemeral public key failed CheckKey")
	}

	// Recipient re-derives and must get the same key.
	derivRecipient, err := crypto.GenerateKeyDerivation(txPub, recipientSec)
	if err != nil {
		t.Fatalf("recipient derivation: %v", err)
	}
	ephPub2, err := crypto.DerivePublicKey(derivRecipient, 0, recipientPub)
	if err != nil {
		t.Fatalf("DerivePublicKey (recipient): %v", err)
	}
	if ephPub != ephPub2 {
		t.Fatalf("output scanning mismatch:\n  sender:    %x\n  recipient: %x", ephPub, ephPub2)
	}

	// Recipient derives the secret key for spending.
	ephSec, err := crypto.DeriveSecretKey(derivRecipient, 0, recipientSec)
	if err != nil {
		t.Fatalf("DeriveSecretKey: %v", err)
	}

	// Verify: ephSec → ephPub must match.
	ephPub3, err := crypto.SecretToPublic(ephSec)
	if err != nil {
		t.Fatalf("SecretToPublic(ephSec): %v", err)
	}
	if ephPub != ephPub3 {
		t.Fatalf("ephemeral key pair mismatch:\n  derived pub: %x\n  sec→pub:     %x", ephPub, ephPub3)
	}
}

// ── Key Images ────────────────────────────────────────────

func TestKeyImage_Good_Roundtrip(t *testing.T) {
	pub, sec, err := crypto.GenerateKeys()
	if err != nil {
		t.Fatalf("GenerateKeys: %v", err)
	}

	ki, err := crypto.GenerateKeyImage(pub, sec)
	if err != nil {
		t.Fatalf("GenerateKeyImage: %v", err)
	}

	var zero [32]byte
	if ki == zero {
		t.Fatal("key image is zero")
	}

	if !crypto.ValidateKeyImage(ki) {
		t.Fatal("generated key image failed validation")
	}
}

func TestKeyImage_Good_Deterministic(t *testing.T) {
	pub, sec, err := crypto.GenerateKeys()
	if err != nil {
		t.Fatalf("GenerateKeys: %v", err)
	}

	ki1, err := crypto.GenerateKeyImage(pub, sec)
	if err != nil {
		t.Fatalf("GenerateKeyImage (1): %v", err)
	}
	ki2, err := crypto.GenerateKeyImage(pub, sec)
	if err != nil {
		t.Fatalf("GenerateKeyImage (2): %v", err)
	}

	if ki1 != ki2 {
		t.Fatalf("key image not deterministic:\n  ki1: %x\n  ki2: %x", ki1, ki2)
	}
}

func TestKeyImage_Bad_Invalid(t *testing.T) {
	// 0xFF...FF is not a valid curve point, so not a valid key image.
	bad := [32]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	if crypto.ValidateKeyImage(bad) {
		t.Fatal("0xFF...FF should fail ValidateKeyImage")
	}
}
