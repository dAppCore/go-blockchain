// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package mining

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

func testnetGenesisHeader() types.BlockHeader {
	return types.BlockHeader{
		MajorVersion: 1,
		Nonce:        101011010221,
		PrevID:       types.Hash{},
		MinorVersion: 0,
		Timestamp:    1770897600,
		Flags:        0,
	}
}

func testnetGenesisRawTx() []byte {
	u64s := [25]uint64{
		0xa080800100000101, 0x03018ae3c8e0c8cf, 0x7b0287d2a2218485, 0x720c5b385edbe3dd,
		0x178e7c64d18a598f, 0x98bb613ff63e6d03, 0x3814f971f9160500, 0x1c595f65f55d872e,
		0x835e5fd926b1f78d, 0xf597c7f5a33b6131, 0x2074496b139c8341, 0x64612073656b6174,
		0x20656761746e6176, 0x6e2065687420666f, 0x666f206572757461, 0x616d726f666e6920,
		0x696562206e6f6974, 0x207973616520676e, 0x6165727073206f74, 0x6168207475622064,
		0x7473206f74206472, 0x202d202e656c6966, 0x206968736f746153, 0x6f746f6d616b614e,
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

func TestHeaderMiningHash_Good(t *testing.T) {
	// Build the genesis block from the known raw coinbase transaction.
	rawTx := testnetGenesisRawTx()
	dec := wire.NewDecoder(bytes.NewReader(rawTx))
	minerTx := wire.DecodeTransaction(dec)
	if dec.Err() != nil {
		t.Fatalf("decode genesis tx: %v", dec.Err())
	}

	block := types.Block{
		BlockHeader: testnetGenesisHeader(),
		MinerTx:     minerTx,
	}

	got := HeaderMiningHash(&block)

	// The header mining hash is computed with nonce=0, so manually compute
	// it to get the expected value.
	block.Nonce = 0
	blob := wire.BlockHashingBlob(&block)
	want := wire.Keccak256(blob)

	if got != want {
		t.Errorf("HeaderMiningHash:\n  got:  %s\n  want: %s",
			hex.EncodeToString(got[:]), hex.EncodeToString(want[:]))
	}
}

func TestHeaderMiningHash_Good_NonceIgnored(t *testing.T) {
	// HeaderMiningHash must produce the same result regardless of the
	// block's current nonce value.
	rawTx := testnetGenesisRawTx()
	dec := wire.NewDecoder(bytes.NewReader(rawTx))
	minerTx := wire.DecodeTransaction(dec)
	if dec.Err() != nil {
		t.Fatalf("decode genesis tx: %v", dec.Err())
	}

	block1 := types.Block{
		BlockHeader: testnetGenesisHeader(),
		MinerTx:     minerTx,
	}
	block2 := block1
	block2.Nonce = 999999

	h1 := HeaderMiningHash(&block1)
	h2 := HeaderMiningHash(&block2)

	if h1 != h2 {
		t.Errorf("HeaderMiningHash changed with different nonce:\n  nonce=%d: %x\n  nonce=%d: %x",
			block1.Nonce, h1, block2.Nonce, h2)
	}
}

func TestCheckNonce_Good_LowDifficulty(t *testing.T) {
	// Build a genesis block and compute its header hash.
	rawTx := testnetGenesisRawTx()
	dec := wire.NewDecoder(bytes.NewReader(rawTx))
	minerTx := wire.DecodeTransaction(dec)
	if dec.Err() != nil {
		t.Fatalf("decode genesis tx: %v", dec.Err())
	}

	block := types.Block{
		BlockHeader: testnetGenesisHeader(),
		MinerTx:     minerTx,
	}
	headerHash := HeaderMiningHash(&block)

	// With difficulty=1, any nonce should produce a valid solution.
	ok, err := CheckNonce(headerHash, 0, 1)
	if err != nil {
		t.Fatalf("CheckNonce: %v", err)
	}
	if !ok {
		t.Error("CheckNonce should pass with difficulty=1")
	}
}

func TestCheckNonce_Good_HighDifficulty(t *testing.T) {
	// With extremely high difficulty, nonce=0 should NOT produce a valid solution.
	rawTx := testnetGenesisRawTx()
	dec := wire.NewDecoder(bytes.NewReader(rawTx))
	minerTx := wire.DecodeTransaction(dec)
	if dec.Err() != nil {
		t.Fatalf("decode genesis tx: %v", dec.Err())
	}

	block := types.Block{
		BlockHeader: testnetGenesisHeader(),
		MinerTx:     minerTx,
	}
	headerHash := HeaderMiningHash(&block)

	// With difficulty = max uint64, virtually no hash passes.
	ok, err := CheckNonce(headerHash, 0, ^uint64(0))
	if err != nil {
		t.Fatalf("CheckNonce: %v", err)
	}
	if ok {
		t.Error("CheckNonce should fail with max difficulty")
	}
}
