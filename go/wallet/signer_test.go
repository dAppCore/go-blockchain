// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wallet

import (
	"testing"

	"dappco.re/go/core/blockchain/crypto"
	"dappco.re/go/core/blockchain/types"
)

func TestSigner_NLSAGSignerRoundTrip_Good(t *testing.T) {
	pub, sec, err := crypto.GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}
	ki, err := crypto.GenerateKeyImage(pub, sec)
	if err != nil {
		t.Fatal(err)
	}

	// Build ring of 3 with our key at index 1.
	ring := make([]types.PublicKey, 3)
	for i := range ring {
		p, _, err := crypto.GenerateKeys()
		if err != nil {
			t.Fatal(err)
		}
		ring[i] = types.PublicKey(p)
	}
	ring[1] = types.PublicKey(pub)

	var prefixHash types.Hash
	prefixHash[0] = 0xFF

	signer := &NLSAGSigner{}
	sigs, err := signer.SignInput(prefixHash, KeyPair{
		Public: types.PublicKey(pub),
		Secret: types.SecretKey(sec),
	}, ring, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(sigs) != 3 {
		t.Fatalf("got %d sigs, want 3", len(sigs))
	}

	// Verify with crypto.CheckRingSignature.
	pubs := make([][32]byte, len(ring))
	for i, k := range ring {
		pubs[i] = [32]byte(k)
	}
	rawSigs := make([][64]byte, len(sigs))
	for i, s := range sigs {
		rawSigs[i] = [64]byte(s)
	}
	if !crypto.CheckRingSignature([32]byte(prefixHash), ki, pubs, rawSigs) {
		t.Fatal("ring signature verification failed")
	}
}

func TestSigner_NLSAGSignerVersion_Good(t *testing.T) {
	signer := &NLSAGSigner{}
	if signer.Version() != 1 {
		t.Fatalf("version = %d, want 1", signer.Version())
	}
}

func TestSigner_NLSAGSignerLargeRing_Ugly(t *testing.T) {
	pub, sec, err := crypto.GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}
	ki, err := crypto.GenerateKeyImage(pub, sec)
	if err != nil {
		t.Fatal(err)
	}

	ring := make([]types.PublicKey, 10)
	for i := range ring {
		p, _, err := crypto.GenerateKeys()
		if err != nil {
			t.Fatal(err)
		}
		ring[i] = types.PublicKey(p)
	}
	ring[5] = types.PublicKey(pub) // real at index 5

	var prefixHash types.Hash
	for i := range prefixHash {
		prefixHash[i] = byte(i)
	}

	signer := &NLSAGSigner{}
	sigs, err := signer.SignInput(prefixHash, KeyPair{
		Public: types.PublicKey(pub),
		Secret: types.SecretKey(sec),
	}, ring, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(sigs) != 10 {
		t.Fatalf("got %d sigs, want 10", len(sigs))
	}

	pubs := make([][32]byte, len(ring))
	for i, k := range ring {
		pubs[i] = [32]byte(k)
	}
	rawSigs := make([][64]byte, len(sigs))
	for i, s := range sigs {
		rawSigs[i] = [64]byte(s)
	}
	if !crypto.CheckRingSignature([32]byte(prefixHash), ki, pubs, rawSigs) {
		t.Fatal("large ring signature verification failed")
	}
}

func TestSigner_NLSAGSignerInterface_Ugly(t *testing.T) {
	// Compile-time check that NLSAGSigner satisfies the Signer interface.
	var _ Signer = (*NLSAGSigner)(nil)
}
