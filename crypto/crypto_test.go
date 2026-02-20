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

// ── Standard Signatures ──────────────────────────────────

func TestSignature_Good_Roundtrip(t *testing.T) {
	pub, sec, _ := crypto.GenerateKeys()

	// Sign a message hash.
	msg := crypto.FastHash([]byte("test message"))
	sig, err := crypto.GenerateSignature(msg, pub, sec)
	if err != nil {
		t.Fatalf("GenerateSignature: %v", err)
	}

	// Verify with correct key.
	if !crypto.CheckSignature(msg, pub, sig) {
		t.Fatal("valid signature failed verification")
	}
}

func TestSignature_Bad_WrongKey(t *testing.T) {
	pub, sec, _ := crypto.GenerateKeys()
	pub2, _, _ := crypto.GenerateKeys()

	msg := crypto.FastHash([]byte("test"))
	sig, _ := crypto.GenerateSignature(msg, pub, sec)

	// Verify with wrong public key should fail.
	if crypto.CheckSignature(msg, pub2, sig) {
		t.Fatal("signature verified with wrong public key")
	}
}

func TestSignature_Bad_WrongMessage(t *testing.T) {
	pub, sec, _ := crypto.GenerateKeys()

	msg1 := crypto.FastHash([]byte("message 1"))
	msg2 := crypto.FastHash([]byte("message 2"))
	sig, _ := crypto.GenerateSignature(msg1, pub, sec)

	if crypto.CheckSignature(msg2, pub, sig) {
		t.Fatal("signature verified with wrong message")
	}
}

// ── Ring Signatures (NLSAG) ─────────────────────────────

func TestRingSignature_Good_Roundtrip(t *testing.T) {
	// Create a ring of 4 public keys. The real signer is at index 1.
	ringSize := 4
	realIndex := 1

	pubs := make([][32]byte, ringSize)
	var realSec [32]byte
	for i := 0; i < ringSize; i++ {
		pub, sec, _ := crypto.GenerateKeys()
		pubs[i] = pub
		if i == realIndex {
			realSec = sec
		}
	}

	// Generate key image for the real key.
	ki, _ := crypto.GenerateKeyImage(pubs[realIndex], realSec)

	// Sign.
	msg := crypto.FastHash([]byte("ring sig test"))
	sigs, err := crypto.GenerateRingSignature(msg, ki, pubs, realSec, realIndex)
	if err != nil {
		t.Fatalf("GenerateRingSignature: %v", err)
	}

	// Verify.
	if !crypto.CheckRingSignature(msg, ki, pubs, sigs) {
		t.Fatal("valid ring signature failed verification")
	}
}

func TestRingSignature_Bad_WrongMessage(t *testing.T) {
	pubs := make([][32]byte, 3)
	var sec [32]byte
	for i := range pubs {
		pub, s, _ := crypto.GenerateKeys()
		pubs[i] = pub
		if i == 0 {
			sec = s
		}
	}
	ki, _ := crypto.GenerateKeyImage(pubs[0], sec)

	msg1 := crypto.FastHash([]byte("msg1"))
	msg2 := crypto.FastHash([]byte("msg2"))
	sigs, _ := crypto.GenerateRingSignature(msg1, ki, pubs, sec, 0)

	if crypto.CheckRingSignature(msg2, ki, pubs, sigs) {
		t.Fatal("ring signature verified with wrong message")
	}
}

// ── CLSAG ────────────────────────────────────────────────

func TestCLSAG_GG_Good_Roundtrip(t *testing.T) {
	// CLSAG_GG is a 2-dimensional linkable ring signature:
	//   Layer 0: stealth addresses (P_i), secret_x for real signer
	//   Layer 1: commitment difference (A_i - pseudo_out), secret_f
	//
	// Ring commitments are stored premultiplied by 1/8 (on-chain form).
	// generate takes pseudo_out as the FULL point (not premultiplied).
	// verify takes pseudo_out as the PREMULTIPLIED form.
	//
	// When pseudo_out matches the real commitment: secret_f = 0.
	// generate pseudo_out = 8 * ring_commitment (full point).
	// verify pseudo_out = ring_commitment (premultiplied form, as stored).
	ringSize := 4
	realIndex := 2

	ring := make([]byte, ringSize*64)
	var realStealthSec [32]byte
	var secretF [32]byte // zero — pseudo_out matches real commitment
	var ki [32]byte

	for i := 0; i < ringSize; i++ {
		pub, sec, _ := crypto.GenerateKeys()
		copy(ring[i*64:], pub[:])

		cPub, _, _ := crypto.GenerateKeys()
		// Store commitment as-is. CLSAG treats this as premultiplied by 1/8.
		copy(ring[i*64+32:], cPub[:])

		if i == realIndex {
			realStealthSec = sec
			var err error
			ki, err = crypto.GenerateKeyImage(pub, sec)
			if err != nil {
				t.Fatalf("GenerateKeyImage: %v", err)
			}
		}
	}

	// For generate: pseudo_out = 8 * commitment (full point).
	var commitmentPremul [32]byte
	copy(commitmentPremul[:], ring[realIndex*64+32:realIndex*64+64])
	pseudoOutFull, err := crypto.PointMul8(commitmentPremul)
	if err != nil {
		t.Fatalf("PointMul8: %v", err)
	}

	msg := crypto.FastHash([]byte("clsag gg test"))
	sig, err := crypto.GenerateCLSAGGG(msg, ring, ringSize, pseudoOutFull, ki,
		realStealthSec, secretF, realIndex)
	if err != nil {
		t.Fatalf("GenerateCLSAGGG: %v", err)
	}

	expectedSize := crypto.CLSAGGGSigSize(ringSize)
	if len(sig) != expectedSize {
		t.Fatalf("sig size: got %d, want %d", len(sig), expectedSize)
	}

	// For verify: pseudo_out = commitment (premultiplied form).
	if !crypto.VerifyCLSAGGG(msg, ring, ringSize, commitmentPremul, ki, sig) {
		t.Fatal("valid CLSAG_GG signature failed verification")
	}
}

func TestCLSAG_GG_Bad_WrongMessage(t *testing.T) {
	ringSize := 3
	realIndex := 0

	ring := make([]byte, ringSize*64)
	var realStealthSec [32]byte
	var secretF [32]byte
	var ki [32]byte

	for i := 0; i < ringSize; i++ {
		pub, sec, _ := crypto.GenerateKeys()
		copy(ring[i*64:], pub[:])
		cPub, _, _ := crypto.GenerateKeys()
		copy(ring[i*64+32:], cPub[:])
		if i == realIndex {
			realStealthSec = sec
			ki, _ = crypto.GenerateKeyImage(pub, sec)
		}
	}

	var commitmentPremul [32]byte
	copy(commitmentPremul[:], ring[realIndex*64+32:realIndex*64+64])
	pseudoOutFull, _ := crypto.PointMul8(commitmentPremul)

	msg1 := crypto.FastHash([]byte("msg1"))
	msg2 := crypto.FastHash([]byte("msg2"))
	sig, _ := crypto.GenerateCLSAGGG(msg1, ring, ringSize, pseudoOutFull, ki,
		realStealthSec, secretF, realIndex)

	if crypto.VerifyCLSAGGG(msg2, ring, ringSize, commitmentPremul, ki, sig) {
		t.Fatal("CLSAG_GG verified with wrong message")
	}
}

func TestCLSAG_GGX_Good_SigSize(t *testing.T) {
	// Verify sig size calculation is consistent.
	if crypto.CLSAGGGXSigSize(4) != 32+4*64+64 {
		t.Fatalf("GGX sig size for ring=4: got %d, want %d", crypto.CLSAGGGXSigSize(4), 32+4*64+64)
	}
}

func TestCLSAG_GGXXG_Good_SigSize(t *testing.T) {
	// Verify sig size calculation is consistent.
	if crypto.CLSAGGGXXGSigSize(4) != 32+4*64+128 {
		t.Fatalf("GGXXG sig size for ring=4: got %d, want %d", crypto.CLSAGGGXXGSigSize(4), 32+4*64+128)
	}
}

// ── Range Proofs / Zarcanum (stubs) ──────────────────────

func TestBPPE_Stub_NotImplemented(t *testing.T) {
	t.Skip("BPPE verification needs on-chain proof data — Phase 4")
}

func TestBGE_Stub_NotImplemented(t *testing.T) {
	t.Skip("BGE verification needs on-chain proof data — Phase 4")
}

func TestZarcanum_Stub_NotImplemented(t *testing.T) {
	t.Skip("Zarcanum verification needs on-chain proof data — Phase 4")
}
