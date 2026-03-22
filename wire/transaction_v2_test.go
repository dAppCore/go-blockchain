// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wire

import (
	"bytes"
	"encoding/hex"
	"testing"

	"dappco.re/go/core/blockchain/types"
)

// Block 101 coinbase transaction from testnet (post-HF4, version 2).
// TX hash: 543bc3c29e9f4c5d1fc566be03fb4da1f2ce2d70d4312fdcc3e4eed7ca3b61e0
// 1323 bytes.
const testnetCoinbaseV2Hex = "020100650616f8b44403a658f6a5cf66a20edeb5cda69913a6057d5bda321e39e4ce69c197c41316362e302e312e325b666131363038632d64697274795d15000b02e35a0b020e620e6f02260b219a40a8799bd76dc19a4db2d3eda28997bcb85341475b33db8dc928a81e1bdf0cd0bbe4f58a988e6ca84e1925b45be8ada18b17908187da9a78d9e58553834ced9654f46406bb69a1cd6d39a574dac9ed62fb4f144640a6680ca7de072fc974c32d3eaafafc623bf483e858d42e8bf4ec7df064ada2e34934469cff6b62685e583b2120b1052c0026875db29b157807ad75cb803dddc54892e4c96238e36773ae1e311a3032698c47a29ba8b274a9f113960a09f09b88d03c72ece51ad8034b7542a9e7c186a5f95d37fff60a0f2cf5885d2279f6e3750feb276a4d8f65d1725ad255e47bfd65c5cc74c32d3eaafafc623bf483e858d42e8bf4ec7df064ada2e34934469cff6b6268ee18b9ce799d5705000000032e002f0747c3d2db565bf368c9879dd1b08899a5e69bc956bc0a89b0cb456a54e066aa8589c6a14ac578409af177d9605f03fe61dfae8067338a40ca551b34580350f694bf389b62ae684146d33632e1ca5bf51dc6fed884780443489ee8fd68a535ddcf5299c9ca2f400fd0a978b1c0ef55a03922549ef78b8b5cd268bd4df5eb32f1fc4d4543466be7e9b9ceb6051955a815427bd773ad9a5cc9fec49fae4c0ab46fc753d1c86e8c04cea4b903fc8c42f404bc8c85b25eb2468f8de75171e2bf802ae96daa934cdb68b0609eb8942b3691c2bf1a122068bcd18d8ef4dd08abb292978007599626cfd549b317eb92667ab1783a730cab6528326b581034a60b8f2582ff61d8ac1ec8395699a170dc9ca5ae2a87cd70388083475e38763aa327e31b27bd691ebd37e22b4eda43001e908fdb211de548ed942766139c8197201d9cc53c744a88ee20995c5f0b64d1bc48293f9c3b8799b2866e473915871df9d55ac065c58ebd51887763709d9e9992d317c12bd27ad933452d06b821b4ee282de78e7cc56102ad119d2f1fb9a79a709614cb24dcb83119bb5734a70c923c4c9586afe1e5bdfedc244f7568a3cd9de95d9d240fb01ee0e0695f6d2066d085457054e78dead4b47ed0fcf2de7c5a9802352f7ab65667c468e32329964723a2084ae0dd38ae9779ff7e0870d6b664275b8207e545f4f80537e7cc4c9f81098eefa42e5efc1c8586bc9e7a48652c1f1e1e28e86b6e7ea80079dd6db7c78d083235ede6ae359631dbcb543d3a6e8b8692fb013832acb9b93ee717c72e832fd78c3a2d5f4fcb380fe3e21825ce80bda2c30b499ae87ce5e9daedde3f8885bcd7463fccaa88d375007c8c94c0f95faa4d0c9e14811442ecb3dc78bd46dafba93e648324b7d0a36d0c02c61a3937e37ff91acd74bd2877bb47e236c36315744a8031339a02c41481d52b5157b6954e712187a14edcd6faf0e6adbb8ec66f2d9260bd238106e076d3098e02943b66ec6f0ff40b3a9e7c9d0f6064317ff7ecc86a4891cfef46e82b0d9f940b4915135e692c042cbcedfe3220c71d6a8935a83b675d93dc70e9035181183f0402527d9d6113e2238807fa6f4646def4d88675ed76862f40918e7249de1d338a053abaed2c406e2824c3df27e147de78a0080a0ca633d4710c068ee126071dbd09c81ec2b948b73997f38c89ca1cd49a2338b49380bbe32c9ee6035b5c2532800e3096fcda7a6274966a8e4a72980655e3683a0ea9317585777add458cb728b2850ef1b6204572195b08407da42101436bfb768f5f13a27fc3766a41cc7812d4f1006e8c64688a9ce9d8666f29abbff012bc86a4b985697cccde23ec916b012ff40a"

func TestV2CoinbaseRoundTrip_Good(t *testing.T) {
	blob, err := hex.DecodeString(testnetCoinbaseV2Hex)
	if err != nil {
		t.Fatalf("bad test hex: %v", err)
	}

	// Decode
	dec := NewDecoder(bytes.NewReader(blob))
	tx := DecodeTransaction(dec)
	if dec.Err() != nil {
		t.Fatalf("decode failed: %v", dec.Err())
	}

	// Re-encode
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransaction(enc, &tx)
	if enc.Err() != nil {
		t.Fatalf("encode failed: %v", enc.Err())
	}

	// Byte-for-byte comparison
	got := buf.Bytes()
	if !bytes.Equal(got, blob) {
		t.Fatalf("round-trip mismatch: got %d bytes, want %d bytes", len(got), len(blob))
	}
}

func TestV2CoinbaseTxHash_Good(t *testing.T) {
	blob, _ := hex.DecodeString(testnetCoinbaseV2Hex)

	// Decode full transaction to get the prefix boundary.
	dec := NewDecoder(bytes.NewReader(blob))
	tx := DecodeTransaction(dec)
	if dec.Err() != nil {
		t.Fatalf("decode failed: %v", dec.Err())
	}

	// Hash the prefix
	got := TransactionHash(&tx)

	wantHex := "543bc3c29e9f4c5d1fc566be03fb4da1f2ce2d70d4312fdcc3e4eed7ca3b61e0"
	wantBytes, _ := hex.DecodeString(wantHex)
	var want types.Hash
	copy(want[:], wantBytes)

	if got != want {
		t.Fatalf("tx hash mismatch:\n  got  %x\n  want %x", got, want)
	}
}

func TestV2CoinbaseFields_Good(t *testing.T) {
	blob, _ := hex.DecodeString(testnetCoinbaseV2Hex)
	dec := NewDecoder(bytes.NewReader(blob))
	tx := DecodeTransaction(dec)
	if dec.Err() != nil {
		t.Fatalf("decode failed: %v", dec.Err())
	}

	// Version
	if tx.Version != types.VersionPostHF4 {
		t.Errorf("version: got %d, want %d", tx.Version, types.VersionPostHF4)
	}

	// Inputs: should be 1 coinbase input
	if len(tx.Vin) != 1 {
		t.Fatalf("input count: got %d, want 1", len(tx.Vin))
	}
	gen, ok := tx.Vin[0].(types.TxInputGenesis)
	if !ok {
		t.Fatalf("input[0] type: got %T, want TxInputGenesis", tx.Vin[0])
	}
	if gen.Height != 101 {
		t.Errorf("coinbase height: got %d, want 101", gen.Height)
	}

	// Outputs: should be 2 Zarcanum outputs
	if len(tx.Vout) != 2 {
		t.Fatalf("output count: got %d, want 2", len(tx.Vout))
	}
	for i, out := range tx.Vout {
		if _, ok := out.(types.TxOutputZarcanum); !ok {
			t.Errorf("output[%d] type: got %T, want TxOutputZarcanum", i, out)
		}
	}

	// Suffix: attachment=empty, signatures=empty, proofs=3 elements
	if !bytes.Equal(tx.Attachment, EncodeVarint(0)) {
		t.Errorf("attachment: expected empty vector, got %d bytes", len(tx.Attachment))
	}
	if !bytes.Equal(tx.SignaturesRaw, EncodeVarint(0)) {
		t.Errorf("signatures_raw: expected empty vector, got %d bytes", len(tx.SignaturesRaw))
	}

	// Proofs should start with varint(3) = 0x03
	if len(tx.Proofs) == 0 {
		t.Fatal("proofs: expected non-empty")
	}
	proofsCount, n, err := DecodeVarint(tx.Proofs)
	if err != nil {
		t.Fatalf("proofs: failed to decode count varint: %v", err)
	}
	if proofsCount != 3 {
		t.Errorf("proofs count: got %d, want 3", proofsCount)
	}

	// First proof should be tag 0x2E (46) = zc_asset_surjection_proof
	if n < len(tx.Proofs) && tx.Proofs[n] != 0x2E {
		t.Errorf("proofs[0] tag: got 0x%02x, want 0x2E", tx.Proofs[n])
	}
}

func TestV2PrefixDecode_Good(t *testing.T) {
	blob, _ := hex.DecodeString(testnetCoinbaseV2Hex)
	dec := NewDecoder(bytes.NewReader(blob))
	tx := DecodeTransactionPrefix(dec)
	if dec.Err() != nil {
		t.Fatalf("prefix decode failed: %v", dec.Err())
	}

	// Prefix re-encodes correctly
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransactionPrefix(enc, &tx)
	if enc.Err() != nil {
		t.Fatalf("prefix encode failed: %v", enc.Err())
	}

	// The prefix should be a proper prefix of the full blob
	prefix := buf.Bytes()
	if len(prefix) >= len(blob) {
		t.Fatalf("prefix (%d bytes) not shorter than full blob (%d bytes)", len(prefix), len(blob))
	}
	if !bytes.Equal(prefix, blob[:len(prefix)]) {
		t.Fatal("prefix bytes don't match start of full blob")
	}
}
