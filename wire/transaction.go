// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wire

import (
	"fmt"

	"forge.lthn.ai/core/go-blockchain/types"
)

// EncodeTransactionPrefix serialises the transaction prefix (without
// signatures, attachment, or proofs) in the consensus wire format.
//
// The version field determines the field ordering:
//
//	v0/v1: version, vin, vout, extra
//	v2+:   version, vin, extra, vout, [hardfork_id]
func EncodeTransactionPrefix(enc *Encoder, tx *types.Transaction) {
	enc.WriteVarint(tx.Version)
	if tx.Version <= types.VersionPreHF4 {
		encodePrefixV1(enc, tx)
	} else {
		encodePrefixV2(enc, tx)
	}
}

// EncodeTransaction serialises a full transaction including suffix fields.
func EncodeTransaction(enc *Encoder, tx *types.Transaction) {
	EncodeTransactionPrefix(enc, tx)
	if tx.Version <= types.VersionPreHF4 {
		encodeSuffixV1(enc, tx)
	} else {
		encodeSuffixV2(enc, tx)
	}
}

// DecodeTransactionPrefix deserialises a transaction prefix.
func DecodeTransactionPrefix(dec *Decoder) types.Transaction {
	var tx types.Transaction
	tx.Version = dec.ReadVarint()
	if dec.Err() != nil {
		return tx
	}
	if tx.Version <= types.VersionPreHF4 {
		decodePrefixV1(dec, &tx)
	} else {
		decodePrefixV2(dec, &tx)
	}
	return tx
}

// DecodeTransaction deserialises a full transaction.
func DecodeTransaction(dec *Decoder) types.Transaction {
	tx := DecodeTransactionPrefix(dec)
	if dec.Err() != nil {
		return tx
	}
	if tx.Version <= types.VersionPreHF4 {
		decodeSuffixV1(dec, &tx)
	} else {
		decodeSuffixV2(dec, &tx)
	}
	return tx
}

// --- v0/v1 prefix ---

func encodePrefixV1(enc *Encoder, tx *types.Transaction) {
	encodeInputs(enc, tx.Vin)
	encodeOutputsV1(enc, tx.Vout)
	enc.WriteBytes(tx.Extra) // raw wire bytes including varint count prefix
}

func decodePrefixV1(dec *Decoder, tx *types.Transaction) {
	tx.Vin = decodeInputs(dec)
	tx.Vout = decodeOutputsV1(dec)
	tx.Extra = decodeRawVariantVector(dec)
}

// --- v2+ prefix ---

func encodePrefixV2(enc *Encoder, tx *types.Transaction) {
	encodeInputs(enc, tx.Vin)
	enc.WriteBytes(tx.Extra)
	encodeOutputsV2(enc, tx.Vout)
	if tx.Version >= types.VersionPostHF5 {
		enc.WriteUint8(tx.HardforkID)
	}
}

func decodePrefixV2(dec *Decoder, tx *types.Transaction) {
	tx.Vin = decodeInputs(dec)
	tx.Extra = decodeRawVariantVector(dec)
	tx.Vout = decodeOutputsV2(dec)
	if tx.Version >= types.VersionPostHF5 {
		tx.HardforkID = dec.ReadUint8()
	}
}

// --- v0/v1 suffix (signatures + attachment) ---

func encodeSuffixV1(enc *Encoder, tx *types.Transaction) {
	enc.WriteVarint(uint64(len(tx.Signatures)))
	for _, ring := range tx.Signatures {
		enc.WriteVarint(uint64(len(ring)))
		for i := range ring {
			enc.WriteBlob64((*[64]byte)(&ring[i]))
		}
	}
	enc.WriteBytes(tx.Attachment)
}

func decodeSuffixV1(dec *Decoder, tx *types.Transaction) {
	sigCount := dec.ReadVarint()
	if sigCount > 0 && dec.Err() == nil {
		tx.Signatures = make([][]types.Signature, sigCount)
		for i := uint64(0); i < sigCount; i++ {
			ringSize := dec.ReadVarint()
			if ringSize > 0 && dec.Err() == nil {
				tx.Signatures[i] = make([]types.Signature, ringSize)
				for j := uint64(0); j < ringSize; j++ {
					dec.ReadBlob64((*[64]byte)(&tx.Signatures[i][j]))
				}
			}
		}
	}
	tx.Attachment = decodeRawVariantVector(dec)
}

// --- v2+ suffix (attachment + signatures_raw + proofs) ---

func encodeSuffixV2(enc *Encoder, tx *types.Transaction) {
	enc.WriteBytes(tx.Attachment)
	// v2+ signatures and proofs are stored as raw wire bytes
	enc.WriteBytes(tx.Proofs)
}

func decodeSuffixV2(dec *Decoder, tx *types.Transaction) {
	tx.Attachment = decodeRawVariantVector(dec)
	tx.Proofs = decodeRawVariantVector(dec)
}

// --- inputs ---

func encodeInputs(enc *Encoder, vin []types.TxInput) {
	enc.WriteVarint(uint64(len(vin)))
	for _, in := range vin {
		enc.WriteVariantTag(in.InputType())
		switch v := in.(type) {
		case types.TxInputGenesis:
			enc.WriteVarint(v.Height)
		case types.TxInputToKey:
			enc.WriteVarint(v.Amount)
			encodeKeyOffsets(enc, v.KeyOffsets)
			enc.WriteBlob32((*[32]byte)(&v.KeyImage))
			enc.WriteBytes(v.EtcDetails)
		}
	}
}

func decodeInputs(dec *Decoder) []types.TxInput {
	n := dec.ReadVarint()
	if n == 0 || dec.Err() != nil {
		return nil
	}
	vin := make([]types.TxInput, 0, n)
	for i := uint64(0); i < n; i++ {
		tag := dec.ReadVariantTag()
		if dec.Err() != nil {
			return vin
		}
		switch tag {
		case types.InputTypeGenesis:
			vin = append(vin, types.TxInputGenesis{Height: dec.ReadVarint()})
		case types.InputTypeToKey:
			var in types.TxInputToKey
			in.Amount = dec.ReadVarint()
			in.KeyOffsets = decodeKeyOffsets(dec)
			dec.ReadBlob32((*[32]byte)(&in.KeyImage))
			in.EtcDetails = decodeRawVariantVector(dec)
			vin = append(vin, in)
		default:
			dec.err = fmt.Errorf("wire: unsupported input tag 0x%02x", tag)
			return vin
		}
	}
	return vin
}

// --- key offsets (txout_ref_v variant vector) ---

func encodeKeyOffsets(enc *Encoder, refs []types.TxOutRef) {
	enc.WriteVarint(uint64(len(refs)))
	for _, ref := range refs {
		enc.WriteVariantTag(ref.Tag)
		switch ref.Tag {
		case types.RefTypeGlobalIndex:
			enc.WriteVarint(ref.GlobalIndex)
		case types.RefTypeByID:
			enc.WriteBlob32((*[32]byte)(&ref.TxID))
			enc.WriteVarint(ref.N)
		}
	}
}

func decodeKeyOffsets(dec *Decoder) []types.TxOutRef {
	n := dec.ReadVarint()
	if n == 0 || dec.Err() != nil {
		return nil
	}
	refs := make([]types.TxOutRef, n)
	for i := uint64(0); i < n; i++ {
		refs[i].Tag = dec.ReadVariantTag()
		switch refs[i].Tag {
		case types.RefTypeGlobalIndex:
			refs[i].GlobalIndex = dec.ReadVarint()
		case types.RefTypeByID:
			dec.ReadBlob32((*[32]byte)(&refs[i].TxID))
			refs[i].N = dec.ReadVarint()
		default:
			dec.err = fmt.Errorf("wire: unsupported ref tag 0x%02x", refs[i].Tag)
			return refs
		}
	}
	return refs
}

// --- outputs ---

// encodeOutputsV1 serialises v0/v1 outputs. In v0/v1, outputs are tx_out_bare
// directly without an outer tx_out_v variant tag.
func encodeOutputsV1(enc *Encoder, vout []types.TxOutput) {
	enc.WriteVarint(uint64(len(vout)))
	for _, out := range vout {
		switch v := out.(type) {
		case types.TxOutputBare:
			enc.WriteVarint(v.Amount)
			// Target is a variant (txout_target_v)
			enc.WriteVariantTag(types.TargetTypeToKey)
			enc.WriteBlob32((*[32]byte)(&v.Target.Key))
			enc.WriteUint8(v.Target.MixAttr)
		}
	}
}

func decodeOutputsV1(dec *Decoder) []types.TxOutput {
	n := dec.ReadVarint()
	if n == 0 || dec.Err() != nil {
		return nil
	}
	vout := make([]types.TxOutput, 0, n)
	for i := uint64(0); i < n; i++ {
		var out types.TxOutputBare
		out.Amount = dec.ReadVarint()
		tag := dec.ReadVariantTag()
		if dec.Err() != nil {
			return vout
		}
		switch tag {
		case types.TargetTypeToKey:
			dec.ReadBlob32((*[32]byte)(&out.Target.Key))
			out.Target.MixAttr = dec.ReadUint8()
		default:
			dec.err = fmt.Errorf("wire: unsupported target tag 0x%02x", tag)
			return vout
		}
		vout = append(vout, out)
	}
	return vout
}

// encodeOutputsV2 serialises v2+ outputs with outer tx_out_v variant tags.
func encodeOutputsV2(enc *Encoder, vout []types.TxOutput) {
	enc.WriteVarint(uint64(len(vout)))
	for _, out := range vout {
		enc.WriteVariantTag(out.OutputType())
		switch v := out.(type) {
		case types.TxOutputBare:
			enc.WriteVarint(v.Amount)
			enc.WriteVariantTag(types.TargetTypeToKey)
			enc.WriteBlob32((*[32]byte)(&v.Target.Key))
			enc.WriteUint8(v.Target.MixAttr)
		case types.TxOutputZarcanum:
			enc.WriteBlob32((*[32]byte)(&v.StealthAddress))
			enc.WriteBlob32((*[32]byte)(&v.ConcealingPoint))
			enc.WriteBlob32((*[32]byte)(&v.AmountCommitment))
			enc.WriteBlob32((*[32]byte)(&v.BlindedAssetID))
			enc.WriteUint64LE(v.EncryptedAmount)
			enc.WriteUint8(v.MixAttr)
		}
	}
}

func decodeOutputsV2(dec *Decoder) []types.TxOutput {
	n := dec.ReadVarint()
	if n == 0 || dec.Err() != nil {
		return nil
	}
	vout := make([]types.TxOutput, 0, n)
	for i := uint64(0); i < n; i++ {
		tag := dec.ReadVariantTag()
		if dec.Err() != nil {
			return vout
		}
		switch tag {
		case types.OutputTypeBare:
			var out types.TxOutputBare
			out.Amount = dec.ReadVarint()
			targetTag := dec.ReadVariantTag()
			if targetTag == types.TargetTypeToKey {
				dec.ReadBlob32((*[32]byte)(&out.Target.Key))
				out.Target.MixAttr = dec.ReadUint8()
			} else {
				dec.err = fmt.Errorf("wire: unsupported target tag 0x%02x", targetTag)
				return vout
			}
			vout = append(vout, out)
		case types.OutputTypeZarcanum:
			var out types.TxOutputZarcanum
			dec.ReadBlob32((*[32]byte)(&out.StealthAddress))
			dec.ReadBlob32((*[32]byte)(&out.ConcealingPoint))
			dec.ReadBlob32((*[32]byte)(&out.AmountCommitment))
			dec.ReadBlob32((*[32]byte)(&out.BlindedAssetID))
			out.EncryptedAmount = dec.ReadUint64LE()
			out.MixAttr = dec.ReadUint8()
			vout = append(vout, out)
		default:
			dec.err = fmt.Errorf("wire: unsupported output tag 0x%02x", tag)
			return vout
		}
	}
	return vout
}

// --- raw variant vector encoding ---
// These helpers handle variant vectors stored as opaque raw bytes.
// The raw bytes include the varint count prefix and all tagged elements.

// decodeRawVariantVector reads a variant vector from the decoder and returns
// the raw wire bytes (including the varint count prefix). This enables
// bit-identical round-tripping of extra, attachment, etc_details, and proofs
// without needing to understand every variant type.
//
// For each element, the tag byte determines how to find the element boundary.
// Known fixed-size tags are skipped by size; unknown tags cause an error.
func decodeRawVariantVector(dec *Decoder) []byte {
	if dec.err != nil {
		return nil
	}

	// Read the count and capture the varint bytes.
	count := dec.ReadVarint()
	if dec.err != nil {
		return nil
	}
	if count == 0 {
		return EncodeVarint(0) // just the count prefix
	}

	// Build the raw bytes: start with the count varint.
	raw := EncodeVarint(count)

	for i := uint64(0); i < count; i++ {
		tag := dec.ReadUint8()
		if dec.err != nil {
			return nil
		}
		raw = append(raw, tag)

		data := readVariantElementData(dec, tag)
		if dec.err != nil {
			return nil
		}
		raw = append(raw, data...)
	}

	return raw
}

// Variant element tags from SET_VARIANT_TAGS (currency_basic.h:1249-1322).
// These are used by readVariantElementData to determine element boundaries
// when reading raw variant vectors (extra, attachment, etc_details).
const (
	tagTxComment              = 7  // tx_comment — string
	tagTxPayerOld             = 8  // tx_payer_old — 2 public keys
	tagString                 = 9  // std::string — string
	tagTxCryptoChecksum       = 10 // tx_crypto_checksum — two uint32 LE
	tagTxDerivationHint       = 11 // tx_derivation_hint — string
	tagTxServiceAttachment    = 12 // tx_service_attachment — 3 strings + vector<key> + uint8
	tagUnlockTime             = 14 // etc_tx_details_unlock_time — varint
	tagExpirationTime         = 15 // etc_tx_details_expiration_time — varint
	tagTxDetailsFlags         = 16 // etc_tx_details_flags — varint
	tagSignedParts            = 17 // signed_parts — uint32 LE
	tagExtraAttachmentInfo    = 18 // extra_attachment_info — string + hash + varint
	tagExtraUserData          = 19 // extra_user_data — string
	tagExtraAliasEntryOld     = 20 // extra_alias_entry_old — complex
	tagExtraPadding           = 21 // extra_padding — vector<uint8>
	tagPublicKey              = 22 // crypto::public_key — 32 bytes
	tagEtcTxFlags16           = 23 // etc_tx_flags16_t — uint16 LE
	tagUint16                 = 24 // uint16_t — uint16 LE
	tagUint64                 = 26 // uint64_t — varint
	tagEtcTxTime              = 27 // etc_tx_time — varint
	tagUint32                 = 28 // uint32_t — uint32 LE
	tagTxReceiverOld          = 29 // tx_receiver_old — 2 public keys
	tagUnlockTime2            = 30 // etc_tx_details_unlock_time2 — vector of entries
	tagTxPayer                = 31 // tx_payer — 2 keys + optional flag
	tagTxReceiver             = 32 // tx_receiver — 2 keys + optional flag
	tagExtraAliasEntry        = 33 // extra_alias_entry — complex
	tagZarcanumTxDataV1       = 39 // zarcanum_tx_data_v1 — complex
)

// readVariantElementData reads the data portion of a variant element (after the
// tag byte) and returns the raw bytes. The tag determines the expected size.
func readVariantElementData(dec *Decoder, tag uint8) []byte {
	switch tag {
	// 32-byte fixed blob
	case tagPublicKey:
		return dec.ReadBytes(32)

	// String fields (varint length + bytes)
	case tagTxComment, tagString, tagTxDerivationHint, tagExtraUserData:
		return readStringBlob(dec)

	// Varint fields
	case tagUnlockTime, tagExpirationTime, tagTxDetailsFlags, tagUint64, tagEtcTxTime:
		v := dec.ReadVarint()
		if dec.err != nil {
			return nil
		}
		return EncodeVarint(v)

	// Fixed-size integer fields
	case tagTxCryptoChecksum: // two uint32 LE
		return dec.ReadBytes(8)
	case tagSignedParts, tagUint32: // uint32 LE
		return dec.ReadBytes(4)
	case tagEtcTxFlags16, tagUint16: // uint16 LE
		return dec.ReadBytes(2)

	// Vector of uint8 (varint count + bytes)
	case tagExtraPadding:
		return readVariantVectorFixed(dec, 1)

	// Struct types: 2 public keys (64 bytes)
	case tagTxPayerOld, tagTxReceiverOld:
		return dec.ReadBytes(64)

	// Struct types: 2 public keys + optional flag
	case tagTxPayer, tagTxReceiver:
		return readTxPayer(dec)

	// Composite types
	case tagExtraAttachmentInfo:
		return readExtraAttachmentInfo(dec)
	case tagUnlockTime2:
		return readUnlockTime2(dec)
	case tagTxServiceAttachment:
		return readTxServiceAttachment(dec)

	default:
		dec.err = fmt.Errorf("wire: unsupported variant tag 0x%02x (%d)", tag, tag)
		return nil
	}
}

// readStringBlob reads a varint-prefixed string and returns the raw bytes
// including the length varint.
func readStringBlob(dec *Decoder) []byte {
	length := dec.ReadVarint()
	if dec.err != nil {
		return nil
	}
	raw := EncodeVarint(length)
	if length > 0 {
		data := dec.ReadBytes(int(length))
		if dec.err != nil {
			return nil
		}
		raw = append(raw, data...)
	}
	return raw
}

// readVariantVectorFixed reads a vector of fixed-size elements and returns
// the raw bytes including the count varint.
func readVariantVectorFixed(dec *Decoder, elemSize int) []byte {
	count := dec.ReadVarint()
	if dec.err != nil {
		return nil
	}
	raw := EncodeVarint(count)
	if count > 0 {
		data := dec.ReadBytes(int(count) * elemSize)
		if dec.err != nil {
			return nil
		}
		raw = append(raw, data...)
	}
	return raw
}

// readExtraAttachmentInfo reads the extra_attachment_info variant (tag 18).
// Structure: cnt_type (string) + hash (32 bytes) + sz (varint).
func readExtraAttachmentInfo(dec *Decoder) []byte {
	var raw []byte
	// cnt_type: string
	str := readStringBlob(dec)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, str...)
	// hash: 32 bytes
	h := dec.ReadBytes(32)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, h...)
	// sz: varint
	v := dec.ReadVarint()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, EncodeVarint(v)...)
	return raw
}

// readUnlockTime2 reads etc_tx_details_unlock_time2 (tag 30).
// Structure: vector of {varint unlock_time, varint output_index}.
func readUnlockTime2(dec *Decoder) []byte {
	count := dec.ReadVarint()
	if dec.err != nil {
		return nil
	}
	raw := EncodeVarint(count)
	for i := uint64(0); i < count; i++ {
		v1 := dec.ReadVarint()
		if dec.err != nil {
			return nil
		}
		raw = append(raw, EncodeVarint(v1)...)
		v2 := dec.ReadVarint()
		if dec.err != nil {
			return nil
		}
		raw = append(raw, EncodeVarint(v2)...)
	}
	return raw
}

// readTxPayer reads tx_payer / tx_receiver (tags 31 / 32).
// Structure: spend_public_key (32 bytes) + view_public_key (32 bytes)
// + optional_field marker. In the binary_archive, the optional is
// serialised as uint8(1)+data or uint8(0) for empty.
func readTxPayer(dec *Decoder) []byte {
	var raw []byte
	// Two public keys
	keys := dec.ReadBytes(64)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, keys...)
	// is_auditable flag (optional_field serialised as uint8 presence + data)
	marker := dec.ReadUint8()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, marker)
	if marker != 0 {
		b := dec.ReadUint8()
		if dec.err != nil {
			return nil
		}
		raw = append(raw, b)
	}
	return raw
}

// readTxServiceAttachment reads tx_service_attachment (tag 12).
// Structure: service_id (string) + instruction (string) + body (string)
// + security (vector<public_key>) + flags (uint8).
func readTxServiceAttachment(dec *Decoder) []byte {
	var raw []byte
	// Three string fields
	for range 3 {
		s := readStringBlob(dec)
		if dec.err != nil {
			return nil
		}
		raw = append(raw, s...)
	}
	// security: vector<crypto::public_key> (varint count + 32*N bytes)
	v := readVariantVectorFixed(dec, 32)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)
	// flags: uint8
	b := dec.ReadUint8()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b)
	return raw
}
