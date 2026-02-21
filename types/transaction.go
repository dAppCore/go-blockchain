// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// You may obtain a copy of the licence at:
//
//     https://joinup.ec.europa.eu/software/page/eupl/licence-eupl
//
// SPDX-License-Identifier: EUPL-1.2

package types

// Transaction version constants matching the C++ TRANSACTION_VERSION_* defines.
// On the wire, version is encoded as a varint (uint64).
const (
	VersionInitial uint64 = 0 // genesis/coinbase
	VersionPreHF4  uint64 = 1 // standard pre-HF4
	VersionPostHF4 uint64 = 2 // Zarcanum (HF4+)
	VersionPostHF5 uint64 = 3 // confidential assets (HF5+)
)

// Input variant tags (txin_v) — values from SET_VARIANT_TAGS in currency_basic.h.
const (
	InputTypeGenesis  uint8 = 0  // txin_gen (coinbase)
	InputTypeToKey    uint8 = 1  // txin_to_key (standard spend)
	InputTypeMultisig uint8 = 2  // txin_multisig
	InputTypeHTLC     uint8 = 34 // txin_htlc (0x22)
	InputTypeZC       uint8 = 37 // txin_zc_input (0x25)
)

// Output variant tags (tx_out_v).
const (
	OutputTypeBare     uint8 = 36 // tx_out_bare (0x24)
	OutputTypeZarcanum uint8 = 38 // tx_out_zarcanum (0x26)
)

// Output target variant tags (txout_target_v).
const (
	TargetTypeToKey    uint8 = 3  // txout_to_key (33-byte blob: key + mix_attr)
	TargetTypeMultisig uint8 = 4  // txout_multisig
	TargetTypeHTLC     uint8 = 35 // txout_htlc (0x23)
)

// Key offset variant tags (txout_ref_v).
const (
	RefTypeGlobalIndex uint8 = 26 // uint64 varint (0x1A)
	RefTypeByID        uint8 = 25 // ref_by_id {hash, varint} (0x19)
)

// Signature variant tags (signature_v).
const (
	SigTypeNLSAG    uint8 = 42 // NLSAG_sig (0x2A)
	SigTypeZC       uint8 = 43 // ZC_sig (0x2B)
	SigTypeVoid     uint8 = 44 // void_sig (0x2C)
	SigTypeZarcanum uint8 = 45 // zarcanum_sig (0x2D)
)

// Transaction represents a Lethean blockchain transaction. The wire format
// differs between versions:
//
//	v0/v1: version, vin, vout, extra, [signatures, attachment]
//	v2+:   version, vin, extra, vout, [hardfork_id], [attachment, signatures, proofs]
type Transaction struct {
	// Version determines the transaction format and consensus rules.
	// Encoded as varint on wire.
	Version uint64

	// Vin contains all transaction inputs.
	Vin []TxInput

	// Vout contains all transaction outputs.
	Vout []TxOutput

	// Extra holds the serialised variant vector of per-transaction metadata
	// (public key, payment IDs, unlock time, etc.). Stored as raw wire bytes
	// to enable bit-identical round-tripping.
	Extra []byte

	// HardforkID identifies the hardfork version for v3+ transactions.
	// Only present on wire when Version >= VersionPostHF5.
	HardforkID uint8

	// Signatures holds ring signatures for v0/v1 transactions.
	// Each element corresponds to one input; inner slice is the ring.
	Signatures [][]Signature

	// SignaturesRaw holds the serialised variant vector of v2+ signatures
	// (NLSAG_sig, ZC_sig, void_sig, zarcanum_sig). Stored as raw wire bytes.
	// V0/v1 uses the structured Signatures field; v2+ uses this field.
	SignaturesRaw []byte

	// Attachment holds the serialised variant vector of transaction attachments.
	// Stored as raw wire bytes.
	Attachment []byte

	// Proofs holds the serialised variant vector of proofs (v2+ only).
	// Stored as raw wire bytes.
	Proofs []byte
}

// TxInput is the interface implemented by all transaction input types.
type TxInput interface {
	InputType() uint8
}

// TxOutput is the interface implemented by all transaction output types.
type TxOutput interface {
	OutputType() uint8
}

// TxInputGenesis is the coinbase input that appears in miner transactions.
type TxInputGenesis struct {
	Height uint64
}

// InputType returns the wire variant tag for genesis inputs.
func (t TxInputGenesis) InputType() uint8 { return InputTypeGenesis }

// TxOutToKey is the txout_to_key target variant. On the wire it is
// serialised as a 33-byte packed blob: 32-byte public key + 1-byte mix_attr.
type TxOutToKey struct {
	Key     PublicKey
	MixAttr uint8
}

// TxOutRef is one element of a txin_to_key key_offsets vector.
// Each element is a variant: either a uint64 global index or a ref_by_id.
type TxOutRef struct {
	Tag         uint8  // RefTypeGlobalIndex or RefTypeByID
	GlobalIndex uint64 // valid when Tag == RefTypeGlobalIndex
	TxID        Hash   // valid when Tag == RefTypeByID
	N           uint64 // valid when Tag == RefTypeByID
}

// TxInputToKey is a standard input that spends a previously received output
// by proving knowledge of the corresponding secret key via a ring signature.
type TxInputToKey struct {
	// Amount in atomic units. Zero for HF4+ Zarcanum transactions.
	Amount uint64

	// KeyOffsets contains the output references forming the decoy ring.
	// Each element is a variant (global index or ref_by_id).
	KeyOffsets []TxOutRef

	// KeyImage prevents double-spending of this input.
	KeyImage KeyImage

	// EtcDetails holds the serialised variant vector of input-level details
	// (signed_parts, attachment_info). Stored as raw wire bytes.
	EtcDetails []byte
}

// InputType returns the wire variant tag for key inputs.
func (t TxInputToKey) InputType() uint8 { return InputTypeToKey }

// TxInputZC is a Zarcanum confidential input (HF4+). Unlike TxInputToKey,
// there is no amount field — amounts are hidden by commitments.
// Wire order: key_offsets, k_image, etc_details.
type TxInputZC struct {
	// KeyOffsets contains the output references forming the decoy ring.
	KeyOffsets []TxOutRef

	// KeyImage prevents double-spending of this input.
	KeyImage KeyImage

	// EtcDetails holds the serialised variant vector of input-level details.
	EtcDetails []byte
}

// InputType returns the wire variant tag for ZC inputs.
func (t TxInputZC) InputType() uint8 { return InputTypeZC }

// TxOutputBare is a transparent (pre-Zarcanum) transaction output.
type TxOutputBare struct {
	// Amount in atomic units.
	Amount uint64

	// Target is the one-time output destination (key + mix attribute).
	Target TxOutToKey
}

// OutputType returns the wire variant tag for bare outputs.
func (t TxOutputBare) OutputType() uint8 { return OutputTypeBare }

// TxOutputZarcanum is a confidential (HF4+) transaction output.
type TxOutputZarcanum struct {
	StealthAddress   PublicKey
	ConcealingPoint  PublicKey
	AmountCommitment PublicKey
	BlindedAssetID   PublicKey
	EncryptedAmount  uint64
	MixAttr          uint8
}

// OutputType returns the wire variant tag for Zarcanum outputs.
func (t TxOutputZarcanum) OutputType() uint8 { return OutputTypeZarcanum }
