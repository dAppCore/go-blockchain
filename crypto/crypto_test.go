// SPDX-Licence-Identifier: EUPL-1.2

package crypto_test

import (
	"encoding/hex"
	"testing"

	"dappco.re/go/core/blockchain/crypto"
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

// ── Range Proofs (BPP — Bulletproofs++) ──────────────────

func TestBPP_Bad_EmptyProof(t *testing.T) {
	commitment := [32]byte{0x01}
	if crypto.VerifyBPP([]byte{}, [][32]byte{commitment}) {
		t.Fatal("empty BPP proof should fail")
	}
}

func TestBPP_Bad_GarbageProof(t *testing.T) {
	// Build a minimal valid-shaped proof: L(0) + R(0) + 6 * 32-byte fields.
	proof := make([]byte, 0, 2+6*32)
	proof = append(proof, 0x00) // varint 0: L length
	proof = append(proof, 0x00) // varint 0: R length
	proof = append(proof, make([]byte, 6*32)...)

	commitment := [32]byte{0x01}
	if crypto.VerifyBPP(proof, [][32]byte{commitment}) {
		t.Fatal("garbage BPP proof should fail verification")
	}
}

func TestBPP_Good_TestnetCoinbase101(t *testing.T) {
	// Real BPP range proof from testnet block 101 (first post-HF4 coinbase).
	// TX hash: 543bc3c29e9f4c5d1fc566be03fb4da1f2ce2d70d4312fdcc3e4eed7ca3b61e0
	//
	// This is the zc_outs_range_proof.bpp field, verified by bpp_crypto_trait_ZC_out.
	// Commitments are the amount_commitments_for_rp_aggregation (E'_j) from the
	// aggregation proof, NOT the raw output amount_commitments.
	proofHex := "07" +
		// L[0..6]
		"47c3d2db565bf368c9879dd1b08899a5e69bc956bc0a89b0cb456a54e066aa85" +
		"89c6a14ac578409af177d9605f03fe61dfae8067338a40ca551b34580350f694" +
		"bf389b62ae684146d33632e1ca5bf51dc6fed884780443489ee8fd68a535ddcf" +
		"5299c9ca2f400fd0a978b1c0ef55a03922549ef78b8b5cd268bd4df5eb32f1fc" +
		"4d4543466be7e9b9ceb6051955a815427bd773ad9a5cc9fec49fae4c0ab46fc7" +
		"53d1c86e8c04cea4b903fc8c42f404bc8c85b25eb2468f8de75171e2bf802ae9" +
		"6daa934cdb68b0609eb8942b3691c2bf1a122068bcd18d8ef4dd08abb2929780" +
		"07" + // R count
		// R[0..6]
		"599626cfd549b317eb92667ab1783a730cab6528326b581034a60b8f2582ff61" +
		"d8ac1ec8395699a170dc9ca5ae2a87cd70388083475e38763aa327e31b27bd69" +
		"1ebd37e22b4eda43001e908fdb211de548ed942766139c8197201d9cc53c744a" +
		"88ee20995c5f0b64d1bc48293f9c3b8799b2866e473915871df9d55ac065c58e" +
		"bd51887763709d9e9992d317c12bd27ad933452d06b821b4ee282de78e7cc561" +
		"02ad119d2f1fb9a79a709614cb24dcb83119bb5734a70c923c4c9586afe1e5bd" +
		"fedc244f7568a3cd9de95d9d240fb01ee0e0695f6d2066d085457054e78dead4" +
		// A0
		"b47ed0fcf2de7c5a9802352f7ab65667c468e32329964723a2084ae0dd38ae97" +
		// A
		"79ff7e0870d6b664275b8207e545f4f80537e7cc4c9f81098eefa42e5efc1c85" +
		// B
		"86bc9e7a48652c1f1e1e28e86b6e7ea80079dd6db7c78d083235ede6ae359631" +
		// r
		"dbcb543d3a6e8b8692fb013832acb9b93ee717c72e832fd78c3a2d5f4fcb380f" +
		// s
		"e3e21825ce80bda2c30b499ae87ce5e9daedde3f8885bcd7463fccaa88d37500" +
		// delta
		"7c8c94c0f95faa4d0c9e14811442ecb3dc78bd46dafba93e648324b7d0a36d0c"

	proof, err := hex.DecodeString(proofHex)
	if err != nil {
		t.Fatalf("decode proof hex: %v", err)
	}

	// Aggregation proof commitments (E'_j, premultiplied by 1/8).
	var c0, c1 [32]byte
	h0, _ := hex.DecodeString("c61a3937e37ff91acd74bd2877bb47e236c36315744a8031339a02c41481d52b")
	h1, _ := hex.DecodeString("5157b6954e712187a14edcd6faf0e6adbb8ec66f2d9260bd238106e076d3098e")
	copy(c0[:], h0)
	copy(c1[:], h1)

	if !crypto.VerifyBPP(proof, [][32]byte{c0, c1}) {
		t.Fatal("real testnet BPP proof should verify successfully")
	}
}

func TestBPP_Bad_TestnetWrongCommitment(t *testing.T) {
	// Same proof as above but with corrupted commitment — must fail.
	proofHex := "07" +
		"47c3d2db565bf368c9879dd1b08899a5e69bc956bc0a89b0cb456a54e066aa85" +
		"89c6a14ac578409af177d9605f03fe61dfae8067338a40ca551b34580350f694" +
		"bf389b62ae684146d33632e1ca5bf51dc6fed884780443489ee8fd68a535ddcf" +
		"5299c9ca2f400fd0a978b1c0ef55a03922549ef78b8b5cd268bd4df5eb32f1fc" +
		"4d4543466be7e9b9ceb6051955a815427bd773ad9a5cc9fec49fae4c0ab46fc7" +
		"53d1c86e8c04cea4b903fc8c42f404bc8c85b25eb2468f8de75171e2bf802ae9" +
		"6daa934cdb68b0609eb8942b3691c2bf1a122068bcd18d8ef4dd08abb2929780" +
		"07" +
		"599626cfd549b317eb92667ab1783a730cab6528326b581034a60b8f2582ff61" +
		"d8ac1ec8395699a170dc9ca5ae2a87cd70388083475e38763aa327e31b27bd69" +
		"1ebd37e22b4eda43001e908fdb211de548ed942766139c8197201d9cc53c744a" +
		"88ee20995c5f0b64d1bc48293f9c3b8799b2866e473915871df9d55ac065c58e" +
		"bd51887763709d9e9992d317c12bd27ad933452d06b821b4ee282de78e7cc561" +
		"02ad119d2f1fb9a79a709614cb24dcb83119bb5734a70c923c4c9586afe1e5bd" +
		"fedc244f7568a3cd9de95d9d240fb01ee0e0695f6d2066d085457054e78dead4" +
		"b47ed0fcf2de7c5a9802352f7ab65667c468e32329964723a2084ae0dd38ae97" +
		"79ff7e0870d6b664275b8207e545f4f80537e7cc4c9f81098eefa42e5efc1c85" +
		"86bc9e7a48652c1f1e1e28e86b6e7ea80079dd6db7c78d083235ede6ae359631" +
		"dbcb543d3a6e8b8692fb013832acb9b93ee717c72e832fd78c3a2d5f4fcb380f" +
		"e3e21825ce80bda2c30b499ae87ce5e9daedde3f8885bcd7463fccaa88d37500" +
		"7c8c94c0f95faa4d0c9e14811442ecb3dc78bd46dafba93e648324b7d0a36d0c"

	proof, _ := hex.DecodeString(proofHex)

	// Corrupted commitment (flipped first byte).
	var c0, c1 [32]byte
	h0, _ := hex.DecodeString("d61a3937e37ff91acd74bd2877bb47e236c36315744a8031339a02c41481d52b")
	h1, _ := hex.DecodeString("5157b6954e712187a14edcd6faf0e6adbb8ec66f2d9260bd238106e076d3098e")
	copy(c0[:], h0)
	copy(c1[:], h1)

	if crypto.VerifyBPP(proof, [][32]byte{c0, c1}) {
		t.Fatal("BPP proof with corrupted commitment should fail")
	}
}

// ── Range Proofs (BPPE — Bulletproofs++ Enhanced) ────────

func TestBPPE_Bad_EmptyProof(t *testing.T) {
	// Empty proof must return false (not crash).
	commitment := [32]byte{0x01}
	if crypto.VerifyBPPE([]byte{}, [][32]byte{commitment}) {
		t.Fatal("empty BPPE proof should fail")
	}
}

func TestBPPE_Bad_GarbageProof(t *testing.T) {
	// Garbage bytes should deserialise (valid varint + blobs) but fail verification.
	// Build a minimal valid-shaped proof: L(0 entries) + R(0 entries) + 7 * 32-byte fields.
	proof := make([]byte, 0, 2+7*32)
	proof = append(proof, 0x00) // varint 0: L length
	proof = append(proof, 0x00) // varint 0: R length
	proof = append(proof, make([]byte, 7*32)...)

	commitment := [32]byte{0x01}
	if crypto.VerifyBPPE(proof, [][32]byte{commitment}) {
		t.Fatal("garbage BPPE proof should fail verification")
	}
}

func TestBGE_Bad_EmptyProof(t *testing.T) {
	ctx := [32]byte{0x01}
	ring := [][32]byte{{0x02}}
	if crypto.VerifyBGE(ctx, ring, []byte{}) {
		t.Fatal("empty BGE proof should fail")
	}
}

func TestBGE_Bad_GarbageProof(t *testing.T) {
	// Build a minimal valid-shaped proof: A(32) + B(32) + Pk(0) + f(0) + y(32) + z(32).
	proof := make([]byte, 0, 4*32+2)
	proof = append(proof, make([]byte, 32)...) // A
	proof = append(proof, make([]byte, 32)...) // B
	proof = append(proof, 0x00)                // varint 0: Pk length
	proof = append(proof, 0x00)                // varint 0: f length
	proof = append(proof, make([]byte, 32)...) // y
	proof = append(proof, make([]byte, 32)...) // z

	ctx := [32]byte{0x01}
	ring := [][32]byte{{0x02}}
	if crypto.VerifyBGE(ctx, ring, proof) {
		t.Fatal("garbage BGE proof should fail verification")
	}
}

func TestZarcanum_Stub_NotImplemented(t *testing.T) {
	// Zarcanum bridge API needs extending — verify it returns false.
	hash := [32]byte{0x01}
	if crypto.VerifyZarcanum(hash, []byte{0x00}) {
		t.Fatal("Zarcanum stub should return false")
	}
}
