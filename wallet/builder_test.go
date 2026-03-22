// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// You may obtain a copy of the licence at:
//
//     https://joinup.ec.europa.eu/software/page/eupl/licence-eupl
//
// SPDX-License-Identifier: EUPL-1.2

package wallet

import (
	"testing"

	"dappco.re/go/core/blockchain/config"
	"dappco.re/go/core/blockchain/crypto"
	"dappco.re/go/core/blockchain/types"
)

// mockRingSelector returns fixed ring members for testing.
type mockRingSelector struct{}

func (m *mockRingSelector) SelectRing(amount uint64, realGlobalIndex uint64,
	ringSize int) ([]RingMember, error) {
	members := make([]RingMember, ringSize)
	for i := range members {
		pub, _, _ := crypto.GenerateKeys()
		members[i] = RingMember{
			PublicKey:   types.PublicKey(pub),
			GlobalIndex: uint64(i * 100),
		}
	}
	return members, nil
}

// makeTestSource generates a Transfer with fresh ephemeral keys and a key image.
func makeTestSource(amount uint64, globalIndex uint64) Transfer {
	pub, sec, _ := crypto.GenerateKeys()
	ki, _ := crypto.GenerateKeyImage(pub, sec)
	return Transfer{
		Amount:      amount,
		GlobalIndex: globalIndex,
		EphemeralKey: KeyPair{
			Public: types.PublicKey(pub),
			Secret: types.SecretKey(sec),
		},
		KeyImage: types.KeyImage(ki),
	}
}

func TestV1BuilderBasic(t *testing.T) {
	signer := &NLSAGSigner{}
	selector := &mockRingSelector{}
	builder := NewV1Builder(signer, selector)

	sender, err := GenerateAccount()
	if err != nil {
		t.Fatalf("generate sender: %v", err)
	}
	recipient, err := GenerateAccount()
	if err != nil {
		t.Fatalf("generate recipient: %v", err)
	}

	source := makeTestSource(5000, 42)
	req := &BuildRequest{
		Sources:       []Transfer{source},
		Destinations:  []Destination{{Address: recipient.Address(), Amount: 3000}},
		Fee:           1000,
		SenderAddress: sender.Address(),
	}

	tx, err := builder.Build(req)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Version must be 1.
	if tx.Version != types.VersionPreHF4 {
		t.Errorf("version: got %d, want %d", tx.Version, types.VersionPreHF4)
	}

	// One input.
	if len(tx.Vin) != 1 {
		t.Fatalf("inputs: got %d, want 1", len(tx.Vin))
	}
	inp, ok := tx.Vin[0].(types.TxInputToKey)
	if !ok {
		t.Fatalf("input type: got %T, want TxInputToKey", tx.Vin[0])
	}
	if inp.Amount != 5000 {
		t.Errorf("input amount: got %d, want 5000", inp.Amount)
	}

	// Ring size = decoys + real output.
	expectedRingSize := int(config.DefaultDecoySetSize) + 1
	if len(inp.KeyOffsets) != expectedRingSize {
		t.Errorf("ring size: got %d, want %d", len(inp.KeyOffsets), expectedRingSize)
	}

	// Key offsets must be sorted by global index.
	for j := 1; j < len(inp.KeyOffsets); j++ {
		if inp.KeyOffsets[j].GlobalIndex < inp.KeyOffsets[j-1].GlobalIndex {
			t.Errorf("key offsets not sorted at index %d: %d < %d",
				j, inp.KeyOffsets[j].GlobalIndex, inp.KeyOffsets[j-1].GlobalIndex)
		}
	}

	// Two outputs: destination (3000) + change (1000).
	if len(tx.Vout) != 2 {
		t.Fatalf("outputs: got %d, want 2", len(tx.Vout))
	}
	out0, ok := tx.Vout[0].(types.TxOutputBare)
	if !ok {
		t.Fatalf("output 0 type: got %T, want TxOutputBare", tx.Vout[0])
	}
	if out0.Amount != 3000 {
		t.Errorf("output 0 amount: got %d, want 3000", out0.Amount)
	}
	out1, ok := tx.Vout[1].(types.TxOutputBare)
	if !ok {
		t.Fatalf("output 1 type: got %T, want TxOutputBare", tx.Vout[1])
	}
	if out1.Amount != 1000 {
		t.Errorf("output 1 amount: got %d, want 1000", out1.Amount)
	}

	// Output keys must be non-zero and unique.
	toKey0, ok := out0.Target.(types.TxOutToKey)
	if !ok {
		t.Fatalf("output 0 target type: got %T, want TxOutToKey", out0.Target)
	}
	toKey1, ok := out1.Target.(types.TxOutToKey)
	if !ok {
		t.Fatalf("output 1 target type: got %T, want TxOutToKey", out1.Target)
	}
	if toKey0.Key == (types.PublicKey{}) {
		t.Error("output 0 key is zero")
	}
	if toKey1.Key == (types.PublicKey{}) {
		t.Error("output 1 key is zero")
	}
	if toKey0.Key == toKey1.Key {
		t.Error("output keys are identical; derivation broken")
	}

	// One signature set per input.
	if len(tx.Signatures) != 1 {
		t.Fatalf("signature sets: got %d, want 1", len(tx.Signatures))
	}
	if len(tx.Signatures[0]) != expectedRingSize {
		t.Errorf("signatures[0] length: got %d, want %d",
			len(tx.Signatures[0]), expectedRingSize)
	}

	// Extra must be non-empty.
	if len(tx.Extra) == 0 {
		t.Error("extra is empty")
	}

	// Attachment must be non-empty (varint(0) = 1 byte).
	if len(tx.Attachment) == 0 {
		t.Error("attachment is empty")
	}

	// Serialisation must succeed.
	raw, err := SerializeTransaction(tx)
	if err != nil {
		t.Fatalf("SerializeTransaction: %v", err)
	}
	if len(raw) == 0 {
		t.Error("serialised transaction is empty")
	}
}

func TestV1BuilderInsufficientFunds(t *testing.T) {
	signer := &NLSAGSigner{}
	selector := &mockRingSelector{}
	builder := NewV1Builder(signer, selector)

	sender, _ := GenerateAccount()
	recipient, _ := GenerateAccount()

	source := makeTestSource(1000, 10)
	req := &BuildRequest{
		Sources:       []Transfer{source},
		Destinations:  []Destination{{Address: recipient.Address(), Amount: 2000}},
		Fee:           500,
		SenderAddress: sender.Address(),
	}

	_, err := builder.Build(req)
	if err == nil {
		t.Fatal("expected error for insufficient funds")
	}
}

func TestV1BuilderExactAmount(t *testing.T) {
	signer := &NLSAGSigner{}
	selector := &mockRingSelector{}
	builder := NewV1Builder(signer, selector)

	sender, _ := GenerateAccount()
	recipient, _ := GenerateAccount()

	// Source exactly covers destination + fee: no change output.
	source := makeTestSource(3000, 55)
	req := &BuildRequest{
		Sources:       []Transfer{source},
		Destinations:  []Destination{{Address: recipient.Address(), Amount: 2000}},
		Fee:           1000,
		SenderAddress: sender.Address(),
	}

	tx, err := builder.Build(req)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Exactly one output (no change).
	if len(tx.Vout) != 1 {
		t.Fatalf("outputs: got %d, want 1", len(tx.Vout))
	}

	out, ok := tx.Vout[0].(types.TxOutputBare)
	if !ok {
		t.Fatalf("output type: got %T, want TxOutputBare", tx.Vout[0])
	}
	if out.Amount != 2000 {
		t.Errorf("output amount: got %d, want 2000", out.Amount)
	}
}
