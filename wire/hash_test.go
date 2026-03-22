// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wire

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"dappco.re/go/core/blockchain/types"
)

// testnetGenesisRawTx returns the raw wire bytes of the testnet genesis
// coinbase transaction, constructed from the packed C struct in
// genesis/_genesis_tn.cpp.gen.
func testnetGenesisRawTx() []byte {
	u64s := [25]uint64{
		0xa080800100000101, 0x03018ae3c8e0c8cf, 0x7b0287d2a2218485, 0x720c5b385edbe3dd,
		0x178e7c64d18a598f, 0x98bb613ff63e6d03, 0x3814f971f9160500, 0x1c595f65f55d872e,
		0x835e5fd926b1f78d, 0xf597c7f5a33b6131, 0x2074496b139c8341, 0x64612073656b6174,
		0x20656761746e6176, 0x6e2065687420666f, 0x666f206572757461, 0x616d726f666e6920,
		0x696562206e6f6974, 0x207973616520676e, 0x6165727073206f74, 0x6168207475622064,
		0x7473206f74206472, 0x202d202e656c6669, 0x206968736f746153, 0x6f746f6d616b614e,
		0x0a0e0d66020b0015,
	}
	u8s := [2]uint8{0x00, 0x00}

	buf := make([]byte, 25*8+2)
	for i, v := range u64s {
		binary.LittleEndian.PutUint64(buf[i*8:], v)
	}
	buf[200] = u8s[0]
	buf[201] = u8s[1]
	return buf
}

// TestGenesisBlockHash_Good is the definitive correctness test for wire
// serialisation. It constructs the testnet genesis block, computes its
// hash, and verifies it matches the hash returned by the C++ daemon.
//
// If this test passes, the block header serialisation, transaction prefix
// serialisation, tree hash, and Keccak-256 implementation are all
// bit-identical to the C++ reference.
func TestGenesisBlockHash_Good(t *testing.T) {
	wantHash := "cb9d5455ccb79451931003672c405f5e2ac51bff54021aa30bc4499b1ffc4963"

	// Parse the raw genesis coinbase transaction.
	rawTx := testnetGenesisRawTx()
	dec := NewDecoder(bytes.NewReader(rawTx))
	minerTx := DecodeTransaction(dec)
	if dec.Err() != nil {
		t.Fatalf("failed to decode genesis miner tx: %v", dec.Err())
	}

	// Verify basic transaction structure.
	if minerTx.Version != 1 {
		t.Fatalf("miner tx version: got %d, want 1", minerTx.Version)
	}
	if len(minerTx.Vin) != 1 {
		t.Fatalf("miner tx vin count: got %d, want 1", len(minerTx.Vin))
	}
	gen, ok := minerTx.Vin[0].(types.TxInputGenesis)
	if !ok {
		t.Fatalf("miner tx vin[0]: got %T, want TxInputGenesis", minerTx.Vin[0])
	}
	if gen.Height != 0 {
		t.Fatalf("miner tx genesis height: got %d, want 0", gen.Height)
	}
	if len(minerTx.Vout) != 1 {
		t.Fatalf("miner tx vout count: got %d, want 1", len(minerTx.Vout))
	}

	// Verify round-trip: re-encode and compare to original bytes.
	var rtBuf bytes.Buffer
	enc := NewEncoder(&rtBuf)
	EncodeTransaction(enc, &minerTx)
	if enc.Err() != nil {
		t.Fatalf("re-encode error: %v", enc.Err())
	}
	if !bytes.Equal(rtBuf.Bytes(), rawTx) {
		t.Fatalf("round-trip mismatch:\n  got: %x\n  want: %x", rtBuf.Bytes(), rawTx)
	}

	// Construct the genesis block.
	block := types.Block{
		BlockHeader: testnetGenesisHeader(),
		MinerTx:     minerTx,
		TxHashes:    nil, // genesis has no other transactions
	}

	// Compute and verify the block hash.
	gotHash := BlockHash(&block)
	if hex.EncodeToString(gotHash[:]) != wantHash {
		t.Errorf("genesis block hash:\n  got:  %x\n  want: %s", gotHash, wantHash)

		// Debug: dump intermediate values.
		prefixHash := TransactionPrefixHash(&block.MinerTx)
		t.Logf("miner tx prefix hash: %x", prefixHash)

		blob := BlockHashingBlob(&block)
		t.Logf("block hashing blob (%d bytes): %x", len(blob), blob)
	}
}

func TestTransactionPrefixHashRoundTrip_Good(t *testing.T) {
	rawTx := testnetGenesisRawTx()

	// Decode.
	dec := NewDecoder(bytes.NewReader(rawTx))
	tx := DecodeTransaction(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	// Prefix hash should be deterministic.
	h1 := TransactionPrefixHash(&tx)
	h2 := TransactionPrefixHash(&tx)
	if h1 != h2 {
		t.Error("prefix hash not deterministic")
	}

	// The prefix hash should equal Keccak-256 of the prefix bytes (first 200 bytes).
	wantPrefixHash := Keccak256(rawTx[:200])
	if types.Hash(wantPrefixHash) != h1 {
		t.Errorf("prefix hash mismatch:\n  got:  %x\n  want: %x", h1, wantPrefixHash)
	}
}
