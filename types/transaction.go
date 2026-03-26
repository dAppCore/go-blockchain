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
	// Usage: value := types.VersionInitial
	VersionInitial uint64 = 0 // genesis/coinbase
	// Usage: value := types.VersionPreHF4
	VersionPreHF4 uint64 = 1 // standard pre-HF4
	// Usage: value := types.VersionPostHF4
	VersionPostHF4 uint64 = 2 // Zarcanum (HF4+)
	// Usage: value := types.VersionPostHF5
	VersionPostHF5 uint64 = 3 // confidential assets (HF5+)
)

// Input variant tags (txin_v) — values from SET_VARIANT_TAGS in currency_basic.h.
const (
	// Usage: value := types.InputTypeGenesis
	InputTypeGenesis uint8 = 0 // txin_gen (coinbase)
	// Usage: value := types.InputTypeToKey
	InputTypeToKey uint8 = 1 // txin_to_key (standard spend)
	// Usage: value := types.InputTypeMultisig
	InputTypeMultisig uint8 = 2 // txin_multisig
	// Usage: value := types.InputTypeHTLC
	InputTypeHTLC uint8 = 34 // txin_htlc (0x22)
	// Usage: value := types.InputTypeZC
	InputTypeZC uint8 = 37 // txin_zc_input (0x25)
)

// Output variant tags (tx_out_v).
const (
	// Usage: value := types.OutputTypeBare
	OutputTypeBare uint8 = 36 // tx_out_bare (0x24)
	// Usage: value := types.OutputTypeZarcanum
	OutputTypeZarcanum uint8 = 38 // tx_out_zarcanum (0x26)
)

// Output target variant tags (txout_target_v).
const (
	// Usage: value := types.TargetTypeToKey
	TargetTypeToKey uint8 = 3 // txout_to_key (33-byte blob: key + mix_attr)
	// Usage: value := types.TargetTypeMultisig
	TargetTypeMultisig uint8 = 4 // txout_multisig
	// Usage: value := types.TargetTypeHTLC
	TargetTypeHTLC uint8 = 35 // txout_htlc (0x23)
)

// Key offset variant tags (txout_ref_v).
const (
	// Usage: value := types.RefTypeGlobalIndex
	RefTypeGlobalIndex uint8 = 26 // uint64 varint (0x1A)
	// Usage: value := types.RefTypeByID
	RefTypeByID uint8 = 25 // ref_by_id {hash, varint} (0x19)
)

// Signature variant tags (signature_v).
const (
	// Usage: value := types.SigTypeNLSAG
	SigTypeNLSAG uint8 = 42 // NLSAG_sig (0x2A)
	// Usage: value := types.SigTypeZC
	SigTypeZC uint8 = 43 // ZC_sig (0x2B)
	// Usage: value := types.SigTypeVoid
	SigTypeVoid uint8 = 44 // void_sig (0x2C)
	// Usage: value := types.SigTypeZarcanum
	SigTypeZarcanum uint8 = 45 // zarcanum_sig (0x2D)
)

// Transaction represents a Lethean blockchain transaction. The wire format
// differs between versions:
//
//	v0/v1: version, vin, vout, extra, [signatures, attachment]
//	v2+:   version, vin, extra, vout, [hardfork_id], [attachment, signatures, proofs]
//
// Usage: var value types.Transaction
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
// Usage: var value types.TxInput
type TxInput interface {
	InputType() uint8
}

// TxOutput is the interface implemented by all transaction output types.
// Usage: var value types.TxOutput
type TxOutput interface {
	OutputType() uint8
}

// TxInputGenesis is the coinbase input that appears in miner transactions.
// Usage: var value types.TxInputGenesis
type TxInputGenesis struct {
	Height uint64
}

// InputType returns the wire variant tag for genesis inputs.
// Usage: value.InputType(...)
func (t TxInputGenesis) InputType() uint8 { return InputTypeGenesis }

// TxOutTarget is the interface implemented by all output target types
// within a TxOutputBare. Each target variant has a unique wire tag.
// Usage: var value types.TxOutTarget
type TxOutTarget interface {
	TargetType() uint8
}

// TxOutToKey is the txout_to_key target variant. On the wire it is
// serialised as a 33-byte packed blob: 32-byte public key + 1-byte mix_attr.
// Usage: var value types.TxOutToKey
type TxOutToKey struct {
	Key     PublicKey
	MixAttr uint8
}

// TargetType returns the wire variant tag for to_key targets.
// Usage: value.TargetType(...)
func (t TxOutToKey) TargetType() uint8 { return TargetTypeToKey }

// TxOutMultisig is the txout_multisig target variant (HF1+).
// Spendable when minimum_sigs of the listed keys sign.
// Usage: var value types.TxOutMultisig
type TxOutMultisig struct {
	MinimumSigs uint64
	Keys        []PublicKey
}

// TargetType returns the wire variant tag for multisig targets.
// Usage: value.TargetType(...)
func (t TxOutMultisig) TargetType() uint8 { return TargetTypeMultisig }

// TxOutHTLC is the txout_htlc target variant (HF1+).
// Hash Time-Locked Contract: redeemable with hash preimage before
// expiration, refundable after expiration.
// Usage: var value types.TxOutHTLC
type TxOutHTLC struct {
	HTLCHash   Hash      // 32-byte hash lock
	Flags      uint8     // bit 0: 0=SHA256, 1=RIPEMD160
	Expiration uint64    // block height deadline
	PKRedeem   PublicKey // recipient key (can redeem before expiration)
	PKRefund   PublicKey // sender key (can refund after expiration)
}

// TargetType returns the wire variant tag for HTLC targets.
// Usage: value.TargetType(...)
func (t TxOutHTLC) TargetType() uint8 { return TargetTypeHTLC }

// TxOutRef is one element of a txin_to_key key_offsets vector.
// Each element is a variant: either a uint64 global index or a ref_by_id.
// Usage: var value types.TxOutRef
type TxOutRef struct {
	Tag         uint8  // RefTypeGlobalIndex or RefTypeByID
	GlobalIndex uint64 // valid when Tag == RefTypeGlobalIndex
	TxID        Hash   // valid when Tag == RefTypeByID
	N           uint64 // valid when Tag == RefTypeByID
}

// TxInputToKey is a standard input that spends a previously received output
// by proving knowledge of the corresponding secret key via a ring signature.
// Usage: var value types.TxInputToKey
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
// Usage: value.InputType(...)
func (t TxInputToKey) InputType() uint8 { return InputTypeToKey }

// TxInputZC is a Zarcanum confidential input (HF4+). Unlike TxInputToKey,
// there is no amount field — amounts are hidden by commitments.
// Wire order: key_offsets, k_image, etc_details.
// Usage: var value types.TxInputZC
type TxInputZC struct {
	// KeyOffsets contains the output references forming the decoy ring.
	KeyOffsets []TxOutRef

	// KeyImage prevents double-spending of this input.
	KeyImage KeyImage

	// EtcDetails holds the serialised variant vector of input-level details.
	EtcDetails []byte
}

// InputType returns the wire variant tag for ZC inputs.
// Usage: value.InputType(...)
func (t TxInputZC) InputType() uint8 { return InputTypeZC }

// TxInputHTLC extends TxInputToKey with an HTLC origin hash (HF1+).
// Wire order: HTLCOrigin (string) serialised BEFORE parent fields (C++ quirk).
// Carries Amount, KeyOffsets, KeyImage, EtcDetails -- same as TxInputToKey.
// Usage: var value types.TxInputHTLC
type TxInputHTLC struct {
	HTLCOrigin string // C++ field: hltc_origin (transposed in source)
	Amount     uint64
	KeyOffsets []TxOutRef
	KeyImage   KeyImage
	EtcDetails []byte // opaque variant vector
}

// InputType returns the wire variant tag for HTLC inputs.
// Usage: value.InputType(...)
func (t TxInputHTLC) InputType() uint8 { return InputTypeHTLC }

// TxInputMultisig spends from a multisig output (HF1+).
// Usage: var value types.TxInputMultisig
type TxInputMultisig struct {
	Amount        uint64
	MultisigOutID Hash // 32-byte hash identifying the multisig output
	SigsCount     uint64
	EtcDetails    []byte // opaque variant vector
}

// InputType returns the wire variant tag for multisig inputs.
// Usage: value.InputType(...)
func (t TxInputMultisig) InputType() uint8 { return InputTypeMultisig }

// TxOutputBare is a transparent (pre-Zarcanum) transaction output.
// Usage: var value types.TxOutputBare
type TxOutputBare struct {
	// Amount in atomic units.
	Amount uint64

	// Target is the output destination. Before HF1 this is always TxOutToKey;
	// after HF1 it may also be TxOutMultisig or TxOutHTLC.
	Target TxOutTarget
}

// OutputType returns the wire variant tag for bare outputs.
// Usage: value.OutputType(...)
func (t TxOutputBare) OutputType() uint8 { return OutputTypeBare }

// TxOutputZarcanum is a confidential (HF4+) transaction output.
// Usage: var value types.TxOutputZarcanum
type TxOutputZarcanum struct {
	StealthAddress   PublicKey
	ConcealingPoint  PublicKey
	AmountCommitment PublicKey
	BlindedAssetID   PublicKey
	EncryptedAmount  uint64
	MixAttr          uint8
}

// OutputType returns the wire variant tag for Zarcanum outputs.
// Usage: value.OutputType(...)
func (t TxOutputZarcanum) OutputType() uint8 { return OutputTypeZarcanum }
