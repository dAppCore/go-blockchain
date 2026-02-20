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
	"encoding/binary"
	"fmt"

	"forge.lthn.ai/core/go-blockchain/types"
	"forge.lthn.ai/core/go-blockchain/wire"
)

// Extra field tag constants from the CryptoNote variant vector encoding.
const (
	extraTagDerivationHint = 11 // string-encoded: varint(length) + bytes
	extraTagUnlockTime     = 14 // varint-encoded
	extraTagPublicKey      = 22 // fixed 32 bytes
)

// TxExtra holds wallet-critical fields parsed from a transaction's raw extra
// bytes. The Raw field preserves the original bytes for round-tripping.
type TxExtra struct {
	TxPublicKey    types.PublicKey
	UnlockTime     uint64
	DerivationHint uint16
	Raw            []byte
}

// ParseTxExtra decodes a CryptoNote variant vector (raw tx extra bytes) and
// extracts the wallet-critical fields: tx public key (tag 22), unlock time
// (tag 14), and derivation hint (tag 11). Unknown tags are skipped.
func ParseTxExtra(raw []byte) (*TxExtra, error) {
	extra := &TxExtra{Raw: make([]byte, len(raw))}
	copy(extra.Raw, raw)

	if len(raw) == 0 {
		return extra, nil
	}

	count, n, err := wire.DecodeVarint(raw)
	if err != nil {
		return extra, fmt.Errorf("wallet: extra: invalid varint count: %w", err)
	}
	pos := n

	for i := uint64(0); i < count && pos < len(raw); i++ {
		tag := raw[pos]
		pos++

		switch tag {
		case extraTagPublicKey:
			if pos+32 > len(raw) {
				return extra, fmt.Errorf("wallet: extra: truncated public key at offset %d", pos)
			}
			copy(extra.TxPublicKey[:], raw[pos:pos+32])
			pos += 32

		case extraTagUnlockTime:
			val, vn, vErr := wire.DecodeVarint(raw[pos:])
			if vErr != nil {
				return extra, fmt.Errorf("wallet: extra: invalid unlock_time varint: %w", vErr)
			}
			extra.UnlockTime = val
			pos += vn

		case extraTagDerivationHint:
			length, vn, vErr := wire.DecodeVarint(raw[pos:])
			if vErr != nil {
				return extra, fmt.Errorf("wallet: extra: invalid hint length varint: %w", vErr)
			}
			pos += vn
			if length == 2 && pos+2 <= len(raw) {
				extra.DerivationHint = binary.LittleEndian.Uint16(raw[pos : pos+2])
			}
			pos += int(length)

		default:
			skip, skipErr := skipExtraElement(raw[pos:], tag)
			if skipErr != nil {
				// Unknown or malformed element; stop parsing but return what we have.
				return extra, nil
			}
			pos += skip
		}
	}

	return extra, nil
}

// BuildTxExtra constructs a minimal raw extra containing only the tx public
// key (tag 22). This is the minimum required for a valid transaction.
func BuildTxExtra(txPubKey types.PublicKey) []byte {
	raw := wire.EncodeVarint(1)
	raw = append(raw, extraTagPublicKey)
	raw = append(raw, txPubKey[:]...)
	return raw
}

// skipExtraElement returns the number of data bytes to skip for a given tag,
// based on the CryptoNote variant vector element sizes.
func skipExtraElement(data []byte, tag uint8) (int, error) {
	switch tag {
	// String types: varint(length) + length bytes.
	case 7, 9, 11, 19:
		if len(data) == 0 {
			return 0, fmt.Errorf("wallet: extra: no data for string tag %d", tag)
		}
		length, n, err := wire.DecodeVarint(data)
		if err != nil {
			return 0, fmt.Errorf("wallet: extra: invalid string length for tag %d: %w", tag, err)
		}
		return n + int(length), nil

	// Varint types: single varint value.
	case 14, 15, 16, 26, 27:
		_, n, err := wire.DecodeVarint(data)
		if err != nil {
			return 0, fmt.Errorf("wallet: extra: invalid varint for tag %d: %w", tag, err)
		}
		return n, nil

	// Fixed-size types.
	case 10:
		return 8, nil // 8 bytes
	case 17, 28:
		return 4, nil // 4 bytes
	case 23, 24:
		return 2, nil // 2 bytes
	case 22:
		return 32, nil // public key
	case 8, 29:
		return 64, nil // signature

	default:
		return 0, fmt.Errorf("wallet: extra: unknown tag %d", tag)
	}
}
