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
const (
	// VersionInitial is the genesis/coinbase transaction version.
	VersionInitial uint8 = 0

	// VersionPreHF4 is the standard transaction version before hardfork 4.
	VersionPreHF4 uint8 = 1

	// VersionPostHF4 is the Zarcanum transaction version introduced at HF4.
	VersionPostHF4 uint8 = 2

	// VersionPostHF5 is the confidential assets transaction version from HF5.
	VersionPostHF5 uint8 = 3
)

// Transaction represents a Lethean blockchain transaction. The structure
// covers all transaction versions (0 through 3) with version-dependent
// interpretation of inputs and outputs.
type Transaction struct {
	// Version determines the transaction format and which consensus rules
	// apply to validation.
	Version uint8

	// UnlockTime is the block height or Unix timestamp after which the
	// outputs of this transaction become spendable. A value of 0 means
	// immediately spendable (after the standard unlock window).
	UnlockTime uint64

	// Vin contains all transaction inputs.
	Vin []TxInput

	// Vout contains all transaction outputs.
	Vout []TxOutput

	// Extra holds auxiliary data such as the transaction public key,
	// payment IDs, and other per-transaction metadata. The format is a
	// sequence of tagged TLV fields.
	Extra []byte
}

// TxInput is the interface implemented by all transaction input types.
// Each concrete type corresponds to a different input variant in the
// CryptoNote protocol.
type TxInput interface {
	// InputType returns the wire type tag for this input variant.
	InputType() uint8
}

// TxOutput is the interface implemented by all transaction output types.
type TxOutput interface {
	// OutputType returns the wire type tag for this output variant.
	OutputType() uint8
}

// Input type tags matching the C++ serialisation tags.
const (
	InputTypeGenesis uint8 = 0xFF // txin_gen (coinbase)
	InputTypeToKey   uint8 = 0x02 // txin_to_key (standard spend)
)

// Output type tags.
const (
	OutputTypeBare     uint8 = 0x02 // tx_out_bare (transparent output)
	OutputTypeZarcanum uint8 = 0x03 // tx_out_zarcanum (confidential output)
)

// TxInputGenesis is the coinbase input that appears in miner transactions.
// It has no real input data; only the block height is recorded.
type TxInputGenesis struct {
	// Height is the block height this coinbase transaction belongs to.
	Height uint64
}

// InputType returns the wire type tag for genesis (coinbase) inputs.
func (t TxInputGenesis) InputType() uint8 { return InputTypeGenesis }

// TxInputToKey is a standard input that spends a previously received output
// by proving knowledge of the corresponding secret key via a ring signature.
type TxInputToKey struct {
	// Amount is the input amount in atomic units. For pre-HF4 transparent
	// transactions this is the real amount; for HF4+ Zarcanum transactions
	// this is zero (amounts are hidden in Pedersen commitments).
	Amount uint64

	// KeyOffsets contains the relative offsets into the global output index
	// that form the decoy ring. The first offset is absolute; subsequent
	// offsets are relative to the previous one.
	KeyOffsets []uint64

	// KeyImage is the key image that prevents double-spending of this input.
	KeyImage KeyImage
}

// InputType returns the wire type tag for key inputs.
func (t TxInputToKey) InputType() uint8 { return InputTypeToKey }

// TxOutputBare is a transparent (pre-Zarcanum) transaction output.
type TxOutputBare struct {
	// Amount is the output amount in atomic units.
	Amount uint64

	// TargetKey is the one-time public key derived from the recipient's
	// address and the transaction secret key.
	TargetKey PublicKey
}

// OutputType returns the wire type tag for bare outputs.
func (t TxOutputBare) OutputType() uint8 { return OutputTypeBare }

// TxOutputZarcanum is a confidential (HF4+) transaction output where the
// amount is hidden inside a Pedersen commitment.
type TxOutputZarcanum struct {
	// StealthAddress is the one-time stealth address for this output.
	StealthAddress PublicKey

	// AmountCommitment is the Pedersen commitment to the output amount.
	AmountCommitment PublicKey

	// ConcealingPoint is an additional point used in the Zarcanum protocol
	// for blinding.
	ConcealingPoint PublicKey

	// EncryptedAmount is the amount encrypted with a key derived from the
	// shared secret between sender and recipient.
	EncryptedAmount [32]byte

	// MixAttr encodes the minimum ring size and other mixing attributes.
	MixAttr uint8
}

// OutputType returns the wire type tag for Zarcanum outputs.
func (t TxOutputZarcanum) OutputType() uint8 { return OutputTypeZarcanum }
