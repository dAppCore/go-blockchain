// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package wire

import (
	"fmt"

	coreerr "forge.lthn.ai/core/go-log"

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

// --- v2+ suffix (attachment + signatures + proofs) ---

func encodeSuffixV2(enc *Encoder, tx *types.Transaction) {
	enc.WriteBytes(tx.Attachment)
	enc.WriteBytes(tx.SignaturesRaw)
	enc.WriteBytes(tx.Proofs)
}

func decodeSuffixV2(dec *Decoder, tx *types.Transaction) {
	tx.Attachment = decodeRawVariantVector(dec)
	tx.SignaturesRaw = decodeRawVariantVector(dec)
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
		case types.TxInputHTLC:
			// Wire order: HTLCOrigin (string) is serialised before parent fields.
			encodeStringField(enc, v.HTLCOrigin)
			enc.WriteVarint(v.Amount)
			encodeKeyOffsets(enc, v.KeyOffsets)
			enc.WriteBlob32((*[32]byte)(&v.KeyImage))
			enc.WriteBytes(v.EtcDetails)
		case types.TxInputMultisig:
			enc.WriteVarint(v.Amount)
			enc.WriteBlob32((*[32]byte)(&v.MultisigOutID))
			enc.WriteVarint(v.SigsCount)
			enc.WriteBytes(v.EtcDetails)
		case types.TxInputZC:
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
		case types.InputTypeHTLC:
			var in types.TxInputHTLC
			in.HTLCOrigin = decodeStringField(dec)
			in.Amount = dec.ReadVarint()
			in.KeyOffsets = decodeKeyOffsets(dec)
			dec.ReadBlob32((*[32]byte)(&in.KeyImage))
			in.EtcDetails = decodeRawVariantVector(dec)
			vin = append(vin, in)
		case types.InputTypeMultisig:
			var in types.TxInputMultisig
			in.Amount = dec.ReadVarint()
			dec.ReadBlob32((*[32]byte)(&in.MultisigOutID))
			in.SigsCount = dec.ReadVarint()
			in.EtcDetails = decodeRawVariantVector(dec)
			vin = append(vin, in)
		case types.InputTypeZC:
			var in types.TxInputZC
			in.KeyOffsets = decodeKeyOffsets(dec)
			dec.ReadBlob32((*[32]byte)(&in.KeyImage))
			in.EtcDetails = decodeRawVariantVector(dec)
			vin = append(vin, in)
		default:
			dec.err = coreerr.E("decodeInputs", fmt.Sprintf("wire: unsupported input tag 0x%02x", tag), nil)
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
			enc.WriteUint64LE(ref.GlobalIndex)
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
			refs[i].GlobalIndex = dec.ReadUint64LE()
		case types.RefTypeByID:
			dec.ReadBlob32((*[32]byte)(&refs[i].TxID))
			refs[i].N = dec.ReadVarint()
		default:
			dec.err = coreerr.E("decodeKeyOffsets", fmt.Sprintf("wire: unsupported ref tag 0x%02x", refs[i].Tag), nil)
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
			switch t := v.Target.(type) {
			case types.TxOutToKey:
				enc.WriteVariantTag(types.TargetTypeToKey)
				enc.WriteBlob32((*[32]byte)(&t.Key))
				enc.WriteUint8(t.MixAttr)
			case types.TxOutMultisig:
				enc.WriteVariantTag(types.TargetTypeMultisig)
				enc.WriteVarint(t.MinimumSigs)
				enc.WriteVarint(uint64(len(t.Keys)))
				for k := range t.Keys {
					enc.WriteBlob32((*[32]byte)(&t.Keys[k]))
				}
			case types.TxOutHTLC:
				enc.WriteVariantTag(types.TargetTypeHTLC)
				enc.WriteBlob32((*[32]byte)(&t.HTLCHash))
				enc.WriteUint8(t.Flags)
				enc.WriteVarint(t.Expiration)
				enc.WriteBlob32((*[32]byte)(&t.PKRedeem))
				enc.WriteBlob32((*[32]byte)(&t.PKRefund))
			}
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
			var t types.TxOutToKey
			dec.ReadBlob32((*[32]byte)(&t.Key))
			t.MixAttr = dec.ReadUint8()
			out.Target = t
		case types.TargetTypeMultisig:
			var t types.TxOutMultisig
			t.MinimumSigs = dec.ReadVarint()
			keyCount := dec.ReadVarint()
			t.Keys = make([]types.PublicKey, keyCount)
			for k := uint64(0); k < keyCount; k++ {
				dec.ReadBlob32((*[32]byte)(&t.Keys[k]))
			}
			out.Target = t
		case types.TargetTypeHTLC:
			var t types.TxOutHTLC
			dec.ReadBlob32((*[32]byte)(&t.HTLCHash))
			t.Flags = dec.ReadUint8()
			t.Expiration = dec.ReadVarint()
			dec.ReadBlob32((*[32]byte)(&t.PKRedeem))
			dec.ReadBlob32((*[32]byte)(&t.PKRefund))
			out.Target = t
		default:
			dec.err = coreerr.E("decodeOutputsV1", fmt.Sprintf("wire: unsupported target tag 0x%02x", tag), nil)
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
			switch t := v.Target.(type) {
			case types.TxOutToKey:
				enc.WriteVariantTag(types.TargetTypeToKey)
				enc.WriteBlob32((*[32]byte)(&t.Key))
				enc.WriteUint8(t.MixAttr)
			case types.TxOutMultisig:
				enc.WriteVariantTag(types.TargetTypeMultisig)
				enc.WriteVarint(t.MinimumSigs)
				enc.WriteVarint(uint64(len(t.Keys)))
				for k := range t.Keys {
					enc.WriteBlob32((*[32]byte)(&t.Keys[k]))
				}
			case types.TxOutHTLC:
				enc.WriteVariantTag(types.TargetTypeHTLC)
				enc.WriteBlob32((*[32]byte)(&t.HTLCHash))
				enc.WriteUint8(t.Flags)
				enc.WriteVarint(t.Expiration)
				enc.WriteBlob32((*[32]byte)(&t.PKRedeem))
				enc.WriteBlob32((*[32]byte)(&t.PKRefund))
			}
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
			switch targetTag {
			case types.TargetTypeToKey:
				var t types.TxOutToKey
				dec.ReadBlob32((*[32]byte)(&t.Key))
				t.MixAttr = dec.ReadUint8()
				out.Target = t
			case types.TargetTypeMultisig:
				var t types.TxOutMultisig
				t.MinimumSigs = dec.ReadVarint()
				keyCount := dec.ReadVarint()
				t.Keys = make([]types.PublicKey, keyCount)
				for k := uint64(0); k < keyCount; k++ {
					dec.ReadBlob32((*[32]byte)(&t.Keys[k]))
				}
				out.Target = t
			case types.TargetTypeHTLC:
				var t types.TxOutHTLC
				dec.ReadBlob32((*[32]byte)(&t.HTLCHash))
				t.Flags = dec.ReadUint8()
				t.Expiration = dec.ReadVarint()
				dec.ReadBlob32((*[32]byte)(&t.PKRedeem))
				dec.ReadBlob32((*[32]byte)(&t.PKRefund))
				out.Target = t
			default:
				dec.err = coreerr.E("decodeOutputsV2", fmt.Sprintf("wire: unsupported target tag 0x%02x", targetTag), nil)
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
			dec.err = coreerr.E("decodeOutputsV2", fmt.Sprintf("wire: unsupported output tag 0x%02x", tag), nil)
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
	tagTxComment           = 7  // tx_comment — string
	tagTxPayerOld          = 8  // tx_payer_old — 2 public keys
	tagString              = 9  // std::string — string
	tagTxCryptoChecksum    = 10 // tx_crypto_checksum — two uint32 LE
	tagTxDerivationHint    = 11 // tx_derivation_hint — string
	tagTxServiceAttachment = 12 // tx_service_attachment — 3 strings + vector<key> + uint8
	tagUnlockTime          = 14 // etc_tx_details_unlock_time — varint
	tagExpirationTime      = 15 // etc_tx_details_expiration_time — varint
	tagTxDetailsFlags      = 16 // etc_tx_details_flags — varint
	tagSignedParts         = 17 // signed_parts — two varints (n_outs, n_extras)
	tagExtraAttachmentInfo = 18 // extra_attachment_info — string + hash + varint
	tagExtraUserData       = 19 // extra_user_data — string
	tagExtraAliasEntryOld  = 20 // extra_alias_entry_old — complex
	tagExtraPadding        = 21 // extra_padding — vector<uint8>
	tagPublicKey           = 22 // crypto::public_key — 32 bytes
	tagEtcTxFlags16        = 23 // etc_tx_flags16_t — uint16 LE
	tagUint16              = 24 // uint16_t — uint16 LE
	tagUint64              = 26 // uint64_t — varint
	tagEtcTxTime           = 27 // etc_tx_time — varint
	tagUint32              = 28 // uint32_t — uint32 LE
	tagTxReceiverOld       = 29 // tx_receiver_old — 2 public keys
	tagUnlockTime2         = 30 // etc_tx_details_unlock_time2 — vector of entries
	tagTxPayer             = 31 // tx_payer — 2 keys + optional flag
	tagTxReceiver          = 32 // tx_receiver — 2 keys + optional flag
	tagExtraAliasEntry     = 33 // extra_alias_entry — complex
	tagZarcanumTxDataV1    = 39 // zarcanum_tx_data_v1 — varint (fee)

	// Signature variant tags (signature_v).
	tagNLSAGSig    = 42 // NLSAG_sig — vector<signature>
	tagZCSig       = 43 // ZC_sig — 2 public_keys + CLSAG_GGX
	tagVoidSig     = 44 // void_sig — empty
	tagZarcanumSig = 45 // zarcanum_sig — complex

	// Asset operation tags (HF5 confidential assets).
	tagAssetDescriptorOperation        = 40 // asset_descriptor_operation
	tagAssetOperationProof             = 49 // asset_operation_proof
	tagAssetOperationOwnershipProof    = 50 // asset_operation_ownership_proof
	tagAssetOperationOwnershipProofETH = 51 // asset_operation_ownership_proof_eth

	// Proof variant tags (proof_v).
	tagZCAssetSurjectionProof = 46 // vector<BGE_proof_s>
	tagZCOutsRangeProof       = 47 // bpp_serialized + aggregation_proof
	tagZCBalanceProof         = 48 // generic_double_schnorr_sig_s (96 bytes)
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

	// Varint fields (structs with VARINT_FIELD)
	case tagUnlockTime, tagExpirationTime, tagTxDetailsFlags, tagEtcTxTime:
		v := dec.ReadVarint()
		if dec.err != nil {
			return nil
		}
		return EncodeVarint(v)

	// Fixed-size integer fields
	case tagUint64: // raw uint64_t — do_serialize → serialize_int → 8-byte LE
		return dec.ReadBytes(8)
	case tagTxCryptoChecksum: // two uint32 LE
		return dec.ReadBytes(8)
	case tagUint32: // uint32 LE
		return dec.ReadBytes(4)
	case tagSignedParts: // two varints: n_outs + n_extras
		return readSignedParts(dec)
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

	// Zarcanum extra variant
	case tagZarcanumTxDataV1: // fee — FIELD(fee) → serialize_int → 8-byte LE
		return dec.ReadBytes(8)

	// Signature variants
	case tagNLSAGSig: // vector<signature> (64 bytes each)
		return readVariantVectorFixed(dec, 64)
	case tagZCSig: // 2 public_keys + CLSAG_GGX_serialized
		return readZCSig(dec)
	case tagVoidSig: // empty struct
		return []byte{}
	case tagZarcanumSig: // complex: 10 scalars + bppe + public_key + CLSAG_GGXXG
		return readZarcanumSig(dec)

	// Asset operation variants (HF5)
	case tagAssetDescriptorOperation:
		return readAssetDescriptorOperation(dec)
	case tagAssetOperationProof:
		return readAssetOperationProof(dec)
	case tagAssetOperationOwnershipProof:
		return readAssetOperationOwnershipProof(dec)
	case tagAssetOperationOwnershipProofETH:
		return readAssetOperationOwnershipProofETH(dec)

	// Proof variants
	case tagZCAssetSurjectionProof: // vector<BGE_proof_s>
		return readZCAssetSurjectionProof(dec)
	case tagZCOutsRangeProof: // bpp_serialized + aggregation_proof
		return readZCOutsRangeProof(dec)
	case tagZCBalanceProof: // generic_double_schnorr_sig_s (3 scalars = 96 bytes)
		return dec.ReadBytes(96)

	default:
		dec.err = coreerr.E("readVariantElementData", fmt.Sprintf("wire: unsupported variant tag 0x%02x (%d)", tag, tag), nil)
		return nil
	}
}

// encodeStringField writes a string as a varint length prefix followed by
// the UTF-8 bytes.
func encodeStringField(enc *Encoder, s string) {
	enc.WriteVarint(uint64(len(s)))
	if len(s) > 0 {
		enc.WriteBytes([]byte(s))
	}
}

// decodeStringField reads a varint-prefixed string and returns the Go string.
func decodeStringField(dec *Decoder) string {
	length := dec.ReadVarint()
	if dec.err != nil || length == 0 {
		return ""
	}
	data := dec.ReadBytes(int(length))
	if dec.err != nil {
		return ""
	}
	return string(data)
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

// readSignedParts reads signed_parts (tag 17).
// Structure: n_outs (varint) + n_extras (varint).
func readSignedParts(dec *Decoder) []byte {
	v1 := dec.ReadVarint()
	if dec.err != nil {
		return nil
	}
	raw := EncodeVarint(v1)
	v2 := dec.ReadVarint()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, EncodeVarint(v2)...)
	return raw
}

// --- crypto blob readers ---
// These read variable-length serialised crypto structures and return raw bytes.
// All vectors are varint(count) + 32*count bytes (scalars or points).

// readVectorOfPoints reads a vector of 32-byte points/scalars.
// Returns raw bytes including the varint count prefix.
func readVectorOfPoints(dec *Decoder) []byte {
	return readVariantVectorFixed(dec, 32)
}

// readBPPSerialized reads a bpp_signature_serialized.
// Wire: vec(L) + vec(R) + A0(32) + A(32) + B(32) + r(32) + s(32) + delta(32).
func readBPPSerialized(dec *Decoder) []byte {
	var raw []byte
	// L: vector of points
	v := readVectorOfPoints(dec)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)
	// R: vector of points
	v = readVectorOfPoints(dec)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)
	// 6 fixed scalars: A0, A, B, r, s, delta
	b := dec.ReadBytes(6 * 32)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)
	return raw
}

// readBPPESerialized reads a bppe_signature_serialized.
// Wire: vec(L) + vec(R) + A0(32) + A(32) + B(32) + r(32) + s(32) + delta_1(32) + delta_2(32).
func readBPPESerialized(dec *Decoder) []byte {
	var raw []byte
	v := readVectorOfPoints(dec)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)
	v = readVectorOfPoints(dec)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)
	// 7 fixed scalars: A0, A, B, r, s, delta_1, delta_2
	b := dec.ReadBytes(7 * 32)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)
	return raw
}

// readBGEProof reads a BGE_proof_s.
// Wire: A(32) + B(32) + vec(Pk) + vec(f) + y(32) + z(32).
func readBGEProof(dec *Decoder) []byte {
	var raw []byte
	// A + B: 2 fixed points
	b := dec.ReadBytes(64)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)
	// Pk: vector of points
	v := readVectorOfPoints(dec)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)
	// f: vector of scalars
	v = readVectorOfPoints(dec)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)
	// y + z: 2 fixed scalars
	b = dec.ReadBytes(64)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)
	return raw
}

// readAggregationProof reads a vector_UG_aggregation_proof_serialized.
// Wire: vec(commitments) + vec(y0s) + vec(y1s) + c(32).
func readAggregationProof(dec *Decoder) []byte {
	var raw []byte
	// 3 vectors of points/scalars
	for range 3 {
		v := readVectorOfPoints(dec)
		if dec.err != nil {
			return nil
		}
		raw = append(raw, v...)
	}
	// c: 1 fixed scalar
	b := dec.ReadBytes(32)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)
	return raw
}

// readCLSAG_GGX reads a CLSAG_GGX_signature_serialized.
// Wire: c(32) + vec(r_g) + vec(r_x) + K1(32) + K2(32).
func readCLSAG_GGX(dec *Decoder) []byte {
	var raw []byte
	// c: 1 fixed scalar
	b := dec.ReadBytes(32)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)
	// r_g, r_x: 2 vectors
	for range 2 {
		v := readVectorOfPoints(dec)
		if dec.err != nil {
			return nil
		}
		raw = append(raw, v...)
	}
	// K1 + K2: 2 fixed points
	b = dec.ReadBytes(64)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)
	return raw
}

// readCLSAG_GGXXG reads a CLSAG_GGXXG_signature_serialized.
// Wire: c(32) + vec(r_g) + vec(r_x) + K1(32) + K2(32) + K3(32) + K4(32).
func readCLSAG_GGXXG(dec *Decoder) []byte {
	var raw []byte
	// c: 1 fixed scalar
	b := dec.ReadBytes(32)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)
	// r_g, r_x: 2 vectors
	for range 2 {
		v := readVectorOfPoints(dec)
		if dec.err != nil {
			return nil
		}
		raw = append(raw, v...)
	}
	// K1 + K2 + K3 + K4: 4 fixed points
	b = dec.ReadBytes(128)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)
	return raw
}

// --- signature variant readers ---

// readZCSig reads ZC_sig (tag 43).
// Wire: pseudo_out_amount_commitment(32) + pseudo_out_blinded_asset_id(32) + CLSAG_GGX.
func readZCSig(dec *Decoder) []byte {
	var raw []byte
	// 2 public keys
	b := dec.ReadBytes(64)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)
	// CLSAG_GGX_serialized
	v := readCLSAG_GGX(dec)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)
	return raw
}

// readZarcanumSig reads zarcanum_sig (tag 45).
// Wire: d(32) + C(32) + C'(32) + E(32) + c(32) + y0(32) + y1(32) + y2(32) + y3(32) + y4(32)
//
//   - bppe_serialized + pseudo_out_amount_commitment(32) + CLSAG_GGXXG.
func readZarcanumSig(dec *Decoder) []byte {
	var raw []byte
	// 10 fixed scalars/points
	b := dec.ReadBytes(10 * 32)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)
	// E_range_proof: bppe_signature_serialized
	v := readBPPESerialized(dec)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)
	// pseudo_out_amount_commitment: 1 public key
	b = dec.ReadBytes(32)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)
	// clsag_ggxxg: CLSAG_GGXXG_signature_serialized
	v = readCLSAG_GGXXG(dec)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)
	return raw
}

// --- proof variant readers ---

// --- HF5 asset operation readers ---

// readAssetDescriptorOperation reads asset_descriptor_operation (tag 40).
// Wire: version(uint8) + operation_type(uint8) + opt_asset_id(uint8 marker
// + 32 bytes if present) + opt_descriptor(uint8 marker + descriptor if
// present) + amount_to_emit(uint64 LE) + amount_to_burn(uint64 LE) +
// etc(vector<uint8>).
//
// Descriptor (AssetDescriptorBase): ticker(string) + full_name(string) +
// total_max_supply(uint64 LE) + current_supply(uint64 LE) +
// decimal_point(uint8) + meta_info(string) + owner_key(32 bytes) +
// etc(vector<uint8>).
func readAssetDescriptorOperation(dec *Decoder) []byte {
	var raw []byte

	// ver: uint8
	ver := dec.ReadUint8()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, ver)

	// operation_type: uint8
	opType := dec.ReadUint8()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, opType)

	// opt_asset_id: uint8 presence marker + 32 bytes if present
	assetMarker := dec.ReadUint8()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, assetMarker)
	if assetMarker != 0 {
		b := dec.ReadBytes(32)
		if dec.err != nil {
			return nil
		}
		raw = append(raw, b...)
	}

	// opt_descriptor: uint8 presence marker + descriptor if present
	descMarker := dec.ReadUint8()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, descMarker)
	if descMarker != 0 {
		// AssetDescriptorBase
		// ticker: string
		s := readStringBlob(dec)
		if dec.err != nil {
			return nil
		}
		raw = append(raw, s...)
		// full_name: string
		s = readStringBlob(dec)
		if dec.err != nil {
			return nil
		}
		raw = append(raw, s...)
		// total_max_supply: uint64 LE
		b := dec.ReadBytes(8)
		if dec.err != nil {
			return nil
		}
		raw = append(raw, b...)
		// current_supply: uint64 LE
		b = dec.ReadBytes(8)
		if dec.err != nil {
			return nil
		}
		raw = append(raw, b...)
		// decimal_point: uint8
		dp := dec.ReadUint8()
		if dec.err != nil {
			return nil
		}
		raw = append(raw, dp)
		// meta_info: string
		s = readStringBlob(dec)
		if dec.err != nil {
			return nil
		}
		raw = append(raw, s...)
		// owner_key: 32 bytes
		b = dec.ReadBytes(32)
		if dec.err != nil {
			return nil
		}
		raw = append(raw, b...)
		// etc: vector<uint8>
		v := readVariantVectorFixed(dec, 1)
		if dec.err != nil {
			return nil
		}
		raw = append(raw, v...)
	}

	// amount_to_emit: uint64 LE
	b := dec.ReadBytes(8)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)
	// amount_to_burn: uint64 LE
	b = dec.ReadBytes(8)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)
	// etc: vector<uint8>
	v := readVariantVectorFixed(dec, 1)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)

	return raw
}

// readAssetOperationProof reads asset_operation_proof (tag 49).
// Wire: version(uint8) + generic_schnorr_sig_s(64 bytes) + asset_id(32 bytes)
// + etc(vector<uint8>).
func readAssetOperationProof(dec *Decoder) []byte {
	var raw []byte

	// ver: uint8
	ver := dec.ReadUint8()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, ver)

	// gss: generic_schnorr_sig_s — 2 scalars (s, c) = 64 bytes
	b := dec.ReadBytes(64)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)

	// asset_id: 32-byte hash
	b = dec.ReadBytes(32)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)

	// etc: vector<uint8>
	v := readVariantVectorFixed(dec, 1)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)

	return raw
}

// readAssetOperationOwnershipProof reads asset_operation_ownership_proof (tag 50).
// Wire: version(uint8) + generic_schnorr_sig_s(64 bytes) + etc(vector<uint8>).
func readAssetOperationOwnershipProof(dec *Decoder) []byte {
	var raw []byte

	// ver: uint8
	ver := dec.ReadUint8()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, ver)

	// gss: generic_schnorr_sig_s — 2 scalars (s, c) = 64 bytes
	b := dec.ReadBytes(64)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)

	// etc: vector<uint8>
	v := readVariantVectorFixed(dec, 1)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)

	return raw
}

// readAssetOperationOwnershipProofETH reads asset_operation_ownership_proof_eth
// (tag 51). Wire: version(uint8) + eth_sig(65 bytes) + etc(vector<uint8>).
func readAssetOperationOwnershipProofETH(dec *Decoder) []byte {
	var raw []byte

	// ver: uint8
	ver := dec.ReadUint8()
	if dec.err != nil {
		return nil
	}
	raw = append(raw, ver)

	// eth_sig: 65 bytes (r=32 + s=32 + v=1)
	b := dec.ReadBytes(65)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, b...)

	// etc: vector<uint8>
	v := readVariantVectorFixed(dec, 1)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)

	return raw
}

// --- proof variant readers ---

// readZCAssetSurjectionProof reads zc_asset_surjection_proof (tag 46).
// Wire: varint(count) + count * BGE_proof_s.
func readZCAssetSurjectionProof(dec *Decoder) []byte {
	count := dec.ReadVarint()
	if dec.err != nil {
		return nil
	}
	raw := EncodeVarint(count)
	for i := uint64(0); i < count; i++ {
		b := readBGEProof(dec)
		if dec.err != nil {
			return nil
		}
		raw = append(raw, b...)
	}
	return raw
}

// readZCOutsRangeProof reads zc_outs_range_proof (tag 47).
// Wire: bpp_signature_serialized + vector_UG_aggregation_proof_serialized.
func readZCOutsRangeProof(dec *Decoder) []byte {
	var raw []byte
	// bpp
	v := readBPPSerialized(dec)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)
	// aggregation_proof
	v = readAggregationProof(dec)
	if dec.err != nil {
		return nil
	}
	raw = append(raw, v...)
	return raw
}
