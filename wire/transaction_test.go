// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wire

import (
	"bytes"
	"testing"

	"forge.lthn.ai/core/go-blockchain/types"
)

func TestCoinbaseTxEncodeDecode_Good(t *testing.T) {
	// Build a minimal v1 coinbase transaction.
	tx := types.Transaction{
		Version: 1,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 42}},
		Vout: []types.TxOutput{types.TxOutputBare{
			Amount: 1000000,
			Target: types.TxOutToKey{
				Key:     types.PublicKey{0xDE, 0xAD},
				MixAttr: 0,
			},
		}},
		Extra: EncodeVarint(0), // empty extra (count=0)
	}

	// Encode prefix.
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransactionPrefix(enc, &tx)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	// Decode prefix.
	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	got := DecodeTransactionPrefix(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	if got.Version != tx.Version {
		t.Errorf("version: got %d, want %d", got.Version, tx.Version)
	}
	if len(got.Vin) != 1 {
		t.Fatalf("vin count: got %d, want 1", len(got.Vin))
	}
	gen, ok := got.Vin[0].(types.TxInputGenesis)
	if !ok {
		t.Fatalf("vin[0] type: got %T, want TxInputGenesis", got.Vin[0])
	}
	if gen.Height != 42 {
		t.Errorf("height: got %d, want 42", gen.Height)
	}
	if len(got.Vout) != 1 {
		t.Fatalf("vout count: got %d, want 1", len(got.Vout))
	}
	bare, ok := got.Vout[0].(types.TxOutputBare)
	if !ok {
		t.Fatalf("vout[0] type: got %T, want TxOutputBare", got.Vout[0])
	}
	if bare.Amount != 1000000 {
		t.Errorf("amount: got %d, want 1000000", bare.Amount)
	}
	toKey, ok := bare.Target.(types.TxOutToKey)
	if !ok {
		t.Fatalf("target type: got %T, want TxOutToKey", bare.Target)
	}
	if toKey.Key[0] != 0xDE || toKey.Key[1] != 0xAD {
		t.Errorf("target key: got %x, want DE AD...", toKey.Key[:2])
	}
}

func TestFullTxRoundTrip_Good(t *testing.T) {
	// Build a v1 coinbase transaction with empty signatures and attachment.
	tx := types.Transaction{
		Version: 1,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 0}},
		Vout: []types.TxOutput{types.TxOutputBare{
			Amount: 5000000000,
			Target: types.TxOutToKey{
				Key:     types.PublicKey{0x01, 0x02, 0x03},
				MixAttr: 0,
			},
		}},
		Extra:      EncodeVarint(0), // empty extra
		Attachment: EncodeVarint(0), // empty attachment
	}

	// Encode full transaction.
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransaction(enc, &tx)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}
	encoded := buf.Bytes()

	// Decode full transaction.
	dec := NewDecoder(bytes.NewReader(encoded))
	got := DecodeTransaction(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	// Re-encode and compare bytes.
	var rtBuf bytes.Buffer
	enc2 := NewEncoder(&rtBuf)
	EncodeTransaction(enc2, &got)
	if enc2.Err() != nil {
		t.Fatalf("re-encode error: %v", enc2.Err())
	}
	if !bytes.Equal(rtBuf.Bytes(), encoded) {
		t.Errorf("round-trip mismatch:\n  got:  %x\n  want: %x", rtBuf.Bytes(), encoded)
	}
}

func TestTransactionHash_Good(t *testing.T) {
	// TransactionHash should equal TransactionPrefixHash for all versions.
	// Confirmed from C++ source: get_transaction_hash delegates to
	// get_transaction_prefix_hash for all transaction versions.
	tx := types.Transaction{
		Version: 1,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 0}},
		Vout: []types.TxOutput{types.TxOutputBare{
			Amount: 5000000000,
			Target: types.TxOutToKey{Key: types.PublicKey{0x01, 0x02, 0x03}},
		}},
		Extra:      EncodeVarint(0),
		Attachment: EncodeVarint(0),
	}

	txHash := TransactionHash(&tx)
	prefixHash := TransactionPrefixHash(&tx)

	// TransactionHash always delegates to TransactionPrefixHash.
	if txHash != prefixHash {
		t.Error("TransactionHash should equal TransactionPrefixHash")
	}

	// Verify manual consistency.
	var prefBuf bytes.Buffer
	enc := NewEncoder(&prefBuf)
	EncodeTransactionPrefix(enc, &tx)

	if Keccak256(prefBuf.Bytes()) != [32]byte(prefixHash) {
		t.Error("TransactionPrefixHash does not match manual prefix encoding")
	}
}

func TestTxInputToKeyRoundTrip_Good(t *testing.T) {
	tx := types.Transaction{
		Version: 1,
		Vin: []types.TxInput{types.TxInputToKey{
			Amount: 100,
			KeyOffsets: []types.TxOutRef{
				{Tag: types.RefTypeGlobalIndex, GlobalIndex: 42},
				{Tag: types.RefTypeGlobalIndex, GlobalIndex: 7},
			},
			KeyImage:   types.KeyImage{0xFF},
			EtcDetails: EncodeVarint(0),
		}},
		Vout: []types.TxOutput{types.TxOutputBare{
			Amount: 50,
			Target: types.TxOutToKey{Key: types.PublicKey{0xAB}},
		}},
		Extra:      EncodeVarint(0),
		Attachment: EncodeVarint(0),
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransaction(enc, &tx)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	got := DecodeTransaction(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	toKey, ok := got.Vin[0].(types.TxInputToKey)
	if !ok {
		t.Fatalf("vin[0] type: got %T, want TxInputToKey", got.Vin[0])
	}
	if toKey.Amount != 100 {
		t.Errorf("amount: got %d, want 100", toKey.Amount)
	}
	if len(toKey.KeyOffsets) != 2 {
		t.Fatalf("key_offsets: got %d, want 2", len(toKey.KeyOffsets))
	}
	if toKey.KeyOffsets[0].GlobalIndex != 42 {
		t.Errorf("key_offsets[0]: got %d, want 42", toKey.KeyOffsets[0].GlobalIndex)
	}
	if toKey.KeyImage[0] != 0xFF {
		t.Errorf("key_image[0]: got 0x%02x, want 0xFF", toKey.KeyImage[0])
	}
}

func TestExtraVariantTags_Good(t *testing.T) {
	// Test that various extra variant tags decode and re-encode correctly.
	tests := []struct {
		name string
		data []byte // raw extra bytes (including varint count prefix)
	}{
		{
			name: "public_key",
			// count=1, tag=22 (crypto::public_key), 32 bytes of key
			data: append([]byte{0x01, tagPublicKey}, make([]byte, 32)...),
		},
		{
			name: "unlock_time",
			// count=1, tag=14 (etc_tx_details_unlock_time), varint(100)=0x64
			data: []byte{0x01, tagUnlockTime, 0x64},
		},
		{
			name: "tx_details_flags",
			// count=1, tag=16, varint(1)=0x01
			data: []byte{0x01, tagTxDetailsFlags, 0x01},
		},
		{
			name: "derivation_hint",
			// count=1, tag=11, string len=3, "abc"
			data: []byte{0x01, tagTxDerivationHint, 0x03, 'a', 'b', 'c'},
		},
		{
			name: "user_data",
			// count=1, tag=19, string len=2, "hi"
			data: []byte{0x01, tagExtraUserData, 0x02, 'h', 'i'},
		},
		{
			name: "extra_padding",
			// count=1, tag=21, vector count=4, 4 bytes
			data: []byte{0x01, tagExtraPadding, 0x04, 0x00, 0x00, 0x00, 0x00},
		},
		{
			name: "crypto_checksum",
			// count=1, tag=10, 8 bytes (two uint32 LE)
			data: []byte{0x01, tagTxCryptoChecksum, 0x01, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00},
		},
		{
			name: "signed_parts",
			// count=1, tag=17, two varints: n_outs=2, n_extras=1
			data: []byte{0x01, tagSignedParts, 0x02, 0x01},
		},
		{
			name: "etc_tx_flags16",
			// count=1, tag=23, uint16 LE
			data: []byte{0x01, tagEtcTxFlags16, 0x01, 0x00},
		},
		{
			name: "etc_tx_time",
			// count=1, tag=27, varint(42)
			data: []byte{0x01, tagEtcTxTime, 0x2A},
		},
		{
			name: "tx_comment",
			// count=1, tag=7, string len=5, "hello"
			data: []byte{0x01, tagTxComment, 0x05, 'h', 'e', 'l', 'l', 'o'},
		},
		{
			name: "tx_payer_old",
			// count=1, tag=8, 64 bytes (2 public keys)
			data: append([]byte{0x01, tagTxPayerOld}, make([]byte, 64)...),
		},
		{
			name: "tx_receiver_old",
			// count=1, tag=29, 64 bytes
			data: append([]byte{0x01, tagTxReceiverOld}, make([]byte, 64)...),
		},
		{
			name: "tx_payer_not_auditable",
			// count=1, tag=31, 64 bytes (2 keys) + marker=0 (no auditable flag)
			data: append(append([]byte{0x01, tagTxPayer}, make([]byte, 64)...), 0x00),
		},
		{
			name: "tx_payer_auditable",
			// count=1, tag=31, 64 bytes (2 keys) + marker=1 + auditable_flag=1
			data: append(append([]byte{0x01, tagTxPayer}, make([]byte, 64)...), 0x01, 0x01),
		},
		{
			name: "extra_attachment_info",
			// count=1, tag=18, cnt_type(string len=0) + hash(32 zeros) + sz(varint 0)
			data: append([]byte{0x01, tagExtraAttachmentInfo, 0x00}, append(make([]byte, 32), 0x00)...),
		},
		{
			name: "unlock_time2",
			// count=1, tag=30, vector count=1, entry: {unlock_time=10, output_index=0}
			data: []byte{0x01, tagUnlockTime2, 0x01, 0x0A, 0x00},
		},
		{
			name: "tx_service_attachment",
			// count=1, tag=12, 3 empty strings + empty key vec + flags=0
			data: []byte{0x01, tagTxServiceAttachment, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		{
			name: "multiple_elements",
			// count=2: public_key + unlock_time
			data: append(
				append([]byte{0x02, tagPublicKey}, make([]byte, 32)...),
				tagUnlockTime, 0x64,
			),
		},
		{
			name: "empty_extra",
			// count=0
			data: []byte{0x00},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a v1 tx with this extra data.
			tx := types.Transaction{
				Version: 1,
				Vin:     []types.TxInput{types.TxInputGenesis{Height: 0}},
				Vout: []types.TxOutput{types.TxOutputBare{
					Amount: 1000,
					Target: types.TxOutToKey{Key: types.PublicKey{0xAA}},
				}},
				Extra:      tt.data,
				Attachment: EncodeVarint(0),
			}

			// Encode.
			var buf bytes.Buffer
			enc := NewEncoder(&buf)
			EncodeTransaction(enc, &tx)
			if enc.Err() != nil {
				t.Fatalf("encode error: %v", enc.Err())
			}

			// Decode.
			dec := NewDecoder(bytes.NewReader(buf.Bytes()))
			got := DecodeTransaction(dec)
			if dec.Err() != nil {
				t.Fatalf("decode error: %v", dec.Err())
			}

			// Re-encode and compare.
			var rtBuf bytes.Buffer
			enc2 := NewEncoder(&rtBuf)
			EncodeTransaction(enc2, &got)
			if enc2.Err() != nil {
				t.Fatalf("re-encode error: %v", enc2.Err())
			}
			if !bytes.Equal(rtBuf.Bytes(), buf.Bytes()) {
				t.Errorf("round-trip mismatch:\n  got:  %x\n  want: %x", rtBuf.Bytes(), buf.Bytes())
			}
		})
	}
}

func TestTxWithSignaturesRoundTrip_Good(t *testing.T) {
	// Test v1 transaction with non-empty signatures.
	sig := types.Signature{}
	sig[0] = 0xAA
	sig[63] = 0xBB

	tx := types.Transaction{
		Version: 1,
		Vin: []types.TxInput{types.TxInputToKey{
			Amount: 100,
			KeyOffsets: []types.TxOutRef{
				{Tag: types.RefTypeGlobalIndex, GlobalIndex: 42},
			},
			KeyImage:   types.KeyImage{0xFF},
			EtcDetails: EncodeVarint(0),
		}},
		Vout: []types.TxOutput{types.TxOutputBare{
			Amount: 50,
			Target: types.TxOutToKey{Key: types.PublicKey{0xAB}},
		}},
		Extra: EncodeVarint(0),
		Signatures: [][]types.Signature{
			{sig},
		},
		Attachment: EncodeVarint(0),
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransaction(enc, &tx)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	got := DecodeTransaction(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	if len(got.Signatures) != 1 {
		t.Fatalf("signatures count: got %d, want 1", len(got.Signatures))
	}
	if len(got.Signatures[0]) != 1 {
		t.Fatalf("ring[0] size: got %d, want 1", len(got.Signatures[0]))
	}
	if got.Signatures[0][0][0] != 0xAA || got.Signatures[0][0][63] != 0xBB {
		t.Error("signature data mismatch")
	}

	// Round-trip byte comparison.
	var rtBuf bytes.Buffer
	enc2 := NewEncoder(&rtBuf)
	EncodeTransaction(enc2, &got)
	if !bytes.Equal(rtBuf.Bytes(), buf.Bytes()) {
		t.Errorf("round-trip mismatch")
	}
}

func TestRefByIDRoundTrip_Good(t *testing.T) {
	// Test TxOutRef with RefTypeByID tag.
	tx := types.Transaction{
		Version: 1,
		Vin: []types.TxInput{types.TxInputToKey{
			Amount: 100,
			KeyOffsets: []types.TxOutRef{
				{
					Tag:  types.RefTypeByID,
					TxID: types.Hash{0xDE, 0xAD},
					N:    3,
				},
			},
			KeyImage:   types.KeyImage{0xFF},
			EtcDetails: EncodeVarint(0),
		}},
		Vout: []types.TxOutput{types.TxOutputBare{
			Amount: 50,
			Target: types.TxOutToKey{Key: types.PublicKey{0xAB}},
		}},
		Extra:      EncodeVarint(0),
		Attachment: EncodeVarint(0),
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransaction(enc, &tx)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	got := DecodeTransaction(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	toKey := got.Vin[0].(types.TxInputToKey)
	if len(toKey.KeyOffsets) != 1 {
		t.Fatalf("key_offsets: got %d, want 1", len(toKey.KeyOffsets))
	}
	ref := toKey.KeyOffsets[0]
	if ref.Tag != types.RefTypeByID {
		t.Errorf("ref tag: got %d, want %d", ref.Tag, types.RefTypeByID)
	}
	if ref.TxID[0] != 0xDE || ref.TxID[1] != 0xAD {
		t.Errorf("ref txid: got %x, want DEAD...", ref.TxID[:2])
	}
	if ref.N != 3 {
		t.Errorf("ref N: got %d, want 3", ref.N)
	}
}

func TestHTLCInputRoundTrip_Good(t *testing.T) {
	tx := types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputHTLC{
				HTLCOrigin: "test_origin",
				Amount:     42000,
				KeyOffsets: []types.TxOutRef{
					{Tag: types.RefTypeGlobalIndex, GlobalIndex: 100},
				},
				KeyImage:   types.KeyImage{0xAA},
				EtcDetails: EncodeVarint(0),
			},
		},
		Vout: []types.TxOutput{types.TxOutputBare{
			Amount: 41000,
			Target: types.TxOutToKey{Key: types.PublicKey{0x01}},
		}},
		Extra: EncodeVarint(0),
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransactionPrefix(enc, &tx)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	got := DecodeTransactionPrefix(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	if len(got.Vin) != 1 {
		t.Fatalf("vin count: got %d, want 1", len(got.Vin))
	}
	htlc, ok := got.Vin[0].(types.TxInputHTLC)
	if !ok {
		t.Fatalf("vin[0] type: got %T, want TxInputHTLC", got.Vin[0])
	}
	if htlc.HTLCOrigin != "test_origin" {
		t.Errorf("HTLCOrigin: got %q, want %q", htlc.HTLCOrigin, "test_origin")
	}
	if htlc.Amount != 42000 {
		t.Errorf("Amount: got %d, want 42000", htlc.Amount)
	}
	if htlc.KeyImage[0] != 0xAA {
		t.Errorf("KeyImage[0]: got 0x%02x, want 0xAA", htlc.KeyImage[0])
	}

	// Byte-level round-trip.
	var rtBuf bytes.Buffer
	enc2 := NewEncoder(&rtBuf)
	EncodeTransactionPrefix(enc2, &got)
	if enc2.Err() != nil {
		t.Fatalf("re-encode error: %v", enc2.Err())
	}
	if !bytes.Equal(rtBuf.Bytes(), buf.Bytes()) {
		t.Errorf("round-trip mismatch:\n  got:  %x\n  want: %x", rtBuf.Bytes(), buf.Bytes())
	}
}

func TestMultisigInputRoundTrip_Good(t *testing.T) {
	tx := types.Transaction{
		Version: types.VersionPreHF4,
		Vin: []types.TxInput{
			types.TxInputMultisig{
				Amount:        50000,
				MultisigOutID: types.Hash{0xBB},
				SigsCount:     3,
				EtcDetails:    EncodeVarint(0),
			},
		},
		Vout: []types.TxOutput{types.TxOutputBare{
			Amount: 49000,
			Target: types.TxOutToKey{Key: types.PublicKey{0x02}},
		}},
		Extra: EncodeVarint(0),
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransactionPrefix(enc, &tx)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	got := DecodeTransactionPrefix(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	if len(got.Vin) != 1 {
		t.Fatalf("vin count: got %d, want 1", len(got.Vin))
	}
	msig, ok := got.Vin[0].(types.TxInputMultisig)
	if !ok {
		t.Fatalf("vin[0] type: got %T, want TxInputMultisig", got.Vin[0])
	}
	if msig.Amount != 50000 {
		t.Errorf("Amount: got %d, want 50000", msig.Amount)
	}
	if msig.MultisigOutID[0] != 0xBB {
		t.Errorf("MultisigOutID[0]: got 0x%02x, want 0xBB", msig.MultisigOutID[0])
	}
	if msig.SigsCount != 3 {
		t.Errorf("SigsCount: got %d, want 3", msig.SigsCount)
	}

	// Byte-level round-trip.
	var rtBuf bytes.Buffer
	enc2 := NewEncoder(&rtBuf)
	EncodeTransactionPrefix(enc2, &got)
	if enc2.Err() != nil {
		t.Fatalf("re-encode error: %v", enc2.Err())
	}
	if !bytes.Equal(rtBuf.Bytes(), buf.Bytes()) {
		t.Errorf("round-trip mismatch:\n  got:  %x\n  want: %x", rtBuf.Bytes(), buf.Bytes())
	}
}

func TestMultisigTargetV1RoundTrip_Good(t *testing.T) {
	tx := types.Transaction{
		Version: types.VersionPreHF4,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 1}},
		Vout: []types.TxOutput{types.TxOutputBare{
			Amount: 5000,
			Target: types.TxOutMultisig{
				MinimumSigs: 2,
				Keys:        []types.PublicKey{{0x01}, {0x02}, {0x03}},
			},
		}},
		Extra: EncodeVarint(0),
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransactionPrefix(enc, &tx)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	got := DecodeTransactionPrefix(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	bare, ok := got.Vout[0].(types.TxOutputBare)
	if !ok {
		t.Fatalf("vout[0] type: got %T, want TxOutputBare", got.Vout[0])
	}
	msig, ok := bare.Target.(types.TxOutMultisig)
	if !ok {
		t.Fatalf("target type: got %T, want TxOutMultisig", bare.Target)
	}
	if msig.MinimumSigs != 2 {
		t.Errorf("MinimumSigs: got %d, want 2", msig.MinimumSigs)
	}
	if len(msig.Keys) != 3 {
		t.Errorf("Keys count: got %d, want 3", len(msig.Keys))
	}

	// Byte-level round-trip.
	var rtBuf bytes.Buffer
	enc2 := NewEncoder(&rtBuf)
	EncodeTransactionPrefix(enc2, &got)
	if enc2.Err() != nil {
		t.Fatalf("re-encode error: %v", enc2.Err())
	}
	if !bytes.Equal(rtBuf.Bytes(), buf.Bytes()) {
		t.Errorf("round-trip mismatch:\n  got:  %x\n  want: %x", rtBuf.Bytes(), buf.Bytes())
	}
}

func TestHTLCTargetV1RoundTrip_Good(t *testing.T) {
	tx := types.Transaction{
		Version: types.VersionPreHF4,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 1}},
		Vout: []types.TxOutput{types.TxOutputBare{
			Amount: 7000,
			Target: types.TxOutHTLC{
				HTLCHash:   types.Hash{0xCC},
				Flags:      1, // RIPEMD160
				Expiration: 20000,
				PKRedeem:   types.PublicKey{0xDD},
				PKRefund:   types.PublicKey{0xEE},
			},
		}},
		Extra: EncodeVarint(0),
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransactionPrefix(enc, &tx)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	got := DecodeTransactionPrefix(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	bare, ok := got.Vout[0].(types.TxOutputBare)
	if !ok {
		t.Fatalf("vout[0] type: got %T, want TxOutputBare", got.Vout[0])
	}
	htlc, ok := bare.Target.(types.TxOutHTLC)
	if !ok {
		t.Fatalf("target type: got %T, want TxOutHTLC", bare.Target)
	}
	if htlc.HTLCHash[0] != 0xCC {
		t.Errorf("HTLCHash[0]: got 0x%02x, want 0xCC", htlc.HTLCHash[0])
	}
	if htlc.Flags != 1 {
		t.Errorf("Flags: got %d, want 1", htlc.Flags)
	}
	if htlc.Expiration != 20000 {
		t.Errorf("Expiration: got %d, want 20000", htlc.Expiration)
	}
	if htlc.PKRedeem[0] != 0xDD {
		t.Errorf("PKRedeem[0]: got 0x%02x, want 0xDD", htlc.PKRedeem[0])
	}
	if htlc.PKRefund[0] != 0xEE {
		t.Errorf("PKRefund[0]: got 0x%02x, want 0xEE", htlc.PKRefund[0])
	}

	// Byte-level round-trip.
	var rtBuf bytes.Buffer
	enc2 := NewEncoder(&rtBuf)
	EncodeTransactionPrefix(enc2, &got)
	if enc2.Err() != nil {
		t.Fatalf("re-encode error: %v", enc2.Err())
	}
	if !bytes.Equal(rtBuf.Bytes(), buf.Bytes()) {
		t.Errorf("round-trip mismatch:\n  got:  %x\n  want: %x", rtBuf.Bytes(), buf.Bytes())
	}
}

func TestMultisigTargetV2RoundTrip_Good(t *testing.T) {
	tx := types.Transaction{
		Version: types.VersionPostHF4,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 1}},
		Vout: []types.TxOutput{types.TxOutputBare{
			Amount: 5000,
			Target: types.TxOutMultisig{
				MinimumSigs: 2,
				Keys:        []types.PublicKey{{0x01}, {0x02}},
			},
		}},
		Extra: EncodeVarint(0),
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransactionPrefix(enc, &tx)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	got := DecodeTransactionPrefix(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	bare, ok := got.Vout[0].(types.TxOutputBare)
	if !ok {
		t.Fatalf("vout[0] type: got %T, want TxOutputBare", got.Vout[0])
	}
	msig, ok := bare.Target.(types.TxOutMultisig)
	if !ok {
		t.Fatalf("target type: got %T, want TxOutMultisig", bare.Target)
	}
	if msig.MinimumSigs != 2 {
		t.Errorf("MinimumSigs: got %d, want 2", msig.MinimumSigs)
	}
	if len(msig.Keys) != 2 {
		t.Errorf("Keys count: got %d, want 2", len(msig.Keys))
	}

	// Byte-level round-trip.
	var rtBuf bytes.Buffer
	enc2 := NewEncoder(&rtBuf)
	EncodeTransactionPrefix(enc2, &got)
	if enc2.Err() != nil {
		t.Fatalf("re-encode error: %v", enc2.Err())
	}
	if !bytes.Equal(rtBuf.Bytes(), buf.Bytes()) {
		t.Errorf("round-trip mismatch:\n  got:  %x\n  want: %x", rtBuf.Bytes(), buf.Bytes())
	}
}

func TestHTLCTargetV2RoundTrip_Good(t *testing.T) {
	tx := types.Transaction{
		Version: types.VersionPostHF4,
		Vin:     []types.TxInput{types.TxInputGenesis{Height: 1}},
		Vout: []types.TxOutput{types.TxOutputBare{
			Amount: 7000,
			Target: types.TxOutHTLC{
				HTLCHash:   types.Hash{0xCC},
				Flags:      0, // SHA256
				Expiration: 15000,
				PKRedeem:   types.PublicKey{0xDD},
				PKRefund:   types.PublicKey{0xEE},
			},
		}},
		Extra: EncodeVarint(0),
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	EncodeTransactionPrefix(enc, &tx)
	if enc.Err() != nil {
		t.Fatalf("encode error: %v", enc.Err())
	}

	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	got := DecodeTransactionPrefix(dec)
	if dec.Err() != nil {
		t.Fatalf("decode error: %v", dec.Err())
	}

	bare, ok := got.Vout[0].(types.TxOutputBare)
	if !ok {
		t.Fatalf("vout[0] type: got %T, want TxOutputBare", got.Vout[0])
	}
	htlc, ok := bare.Target.(types.TxOutHTLC)
	if !ok {
		t.Fatalf("target type: got %T, want TxOutHTLC", bare.Target)
	}
	if htlc.HTLCHash[0] != 0xCC {
		t.Errorf("HTLCHash[0]: got 0x%02x, want 0xCC", htlc.HTLCHash[0])
	}
	if htlc.Flags != 0 {
		t.Errorf("Flags: got %d, want 0", htlc.Flags)
	}
	if htlc.Expiration != 15000 {
		t.Errorf("Expiration: got %d, want 15000", htlc.Expiration)
	}
	if htlc.PKRedeem[0] != 0xDD {
		t.Errorf("PKRedeem[0]: got 0x%02x, want 0xDD", htlc.PKRedeem[0])
	}
	if htlc.PKRefund[0] != 0xEE {
		t.Errorf("PKRefund[0]: got 0x%02x, want 0xEE", htlc.PKRefund[0])
	}

	// Byte-level round-trip.
	var rtBuf bytes.Buffer
	enc2 := NewEncoder(&rtBuf)
	EncodeTransactionPrefix(enc2, &got)
	if enc2.Err() != nil {
		t.Fatalf("re-encode error: %v", enc2.Err())
	}
	if !bytes.Equal(rtBuf.Bytes(), buf.Bytes()) {
		t.Errorf("round-trip mismatch:\n  got:  %x\n  want: %x", rtBuf.Bytes(), buf.Bytes())
	}
}
