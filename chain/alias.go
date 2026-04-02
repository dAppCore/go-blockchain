// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"bytes"
	"encoding/hex"

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"

	"dappco.re/go/core/blockchain/wire"
)

const groupAliases = "aliases"

// Alias represents a registered blockchain alias.
//
//	alias := chain.Alias{Name: "charon", Address: "iTHN...", Comment: "v=lthn1;type=gateway"}
type Alias struct {
	Name    string `json:"alias"`
	Address string `json:"address"`
	Comment string `json:"comment"`
}

// PutAlias stores an alias registration.
//
//	c.PutAlias(alias)
func (c *Chain) PutAlias(a Alias) error {
	value := core.Concat(a.Address, "|", a.Comment)
	return c.store.Set(groupAliases, a.Name, value)
}

// GetAlias retrieves an alias by name.
//
//	alias, err := c.GetAlias("charon")
func (c *Chain) GetAlias(name string) (*Alias, error) {
	data, err := c.store.Get(groupAliases, name)
	if err != nil {
		return nil, coreerr.E("Chain.GetAlias", core.Sprintf("alias %s not found", name), err)
	}

	parts := core.SplitN(data, "|", 2)
	alias := &Alias{Name: name}
	if len(parts) >= 1 {
		alias.Address = parts[0]
	}
	if len(parts) >= 2 {
		alias.Comment = parts[1]
	}
	return alias, nil
}

// GetAllAliases returns all registered aliases.
//
//	aliases := c.GetAllAliases()
func (c *Chain) GetAllAliases() []Alias {
	var aliases []Alias
	all, err := c.store.GetAll(groupAliases)
	if err != nil {
		return aliases
	}
	for name, value := range all {
		parts := core.SplitN(value, "|", 2)
		a := Alias{Name: name}
		if len(parts) >= 1 {
			a.Address = parts[0]
		}
		if len(parts) >= 2 {
			a.Comment = parts[1]
		}
		aliases = append(aliases, a)
	}
	return aliases
}

// ExtractAliasFromExtra parses an extra_alias_entry from raw extra bytes.
// The extra field is a variant vector — we scan for tag 33 (extra_alias_entry).
//
//	alias := chain.ExtractAliasFromExtra(extraBytes)
func ExtractAliasFromExtra(extra []byte) *Alias {
	if len(extra) == 0 {
		return nil
	}

	dec := wire.NewDecoder(bytes.NewReader(extra))
	count := dec.ReadVarint()
	if dec.Err() != nil {
		return nil
	}

	for i := uint64(0); i < count; i++ {
		tag := dec.ReadUint8()
		if dec.Err() != nil {
			return nil
		}

		if tag == 33 { // tagExtraAliasEntry
			return parseAliasEntry(dec)
		}

		// Skip non-alias entries by reading their data.
		// We use the same skip logic as the wire package.
		skipVariantElement(dec, tag)
		if dec.Err() != nil {
			return nil
		}
	}
	return nil
}

// parseAliasEntry reads the fields of an extra_alias_entry after the tag byte.
func parseAliasEntry(dec *wire.Decoder) *Alias {
	// m_alias — string (varint length + bytes)
	aliasLen := dec.ReadVarint()
	if dec.Err() != nil || aliasLen > 256 {
		return nil
	}
	aliasBytes := dec.ReadBytes(int(aliasLen))
	if dec.Err() != nil {
		return nil
	}

	// m_address — account_public_address (32 + 32 + 1 = 65 bytes)
	addrBytes := dec.ReadBytes(65)
	if dec.Err() != nil {
		return nil
	}

	// m_text_comment — string
	commentLen := dec.ReadVarint()
	if dec.Err() != nil || commentLen > 4096 {
		return nil
	}
	commentBytes := dec.ReadBytes(int(commentLen))
	if dec.Err() != nil {
		return nil
	}

	// Skip m_view_key and m_sign (we don't need them for the alias index)
	// m_view_key — vector<secret_key>
	vkCount := dec.ReadVarint()
	if dec.Err() != nil {
		return nil
	}
	dec.ReadBytes(int(vkCount) * 32)

	// m_sign — vector<signature>
	sigCount := dec.ReadVarint()
	if dec.Err() != nil {
		return nil
	}
	dec.ReadBytes(int(sigCount) * 64)

	// Build the address from spend+view public keys.
	// For now, just store the hex of the spend key as a placeholder.
	// TODO: encode proper base58 iTHN address from the keys.
	spendKey := hex.EncodeToString(addrBytes[:32])

	return &Alias{
		Name:    string(aliasBytes),
		Address: spendKey,
		Comment: string(commentBytes),
	}
}

// skipVariantElement reads and discards a variant element of the given tag.
// This is a simplified version that handles the common tags.
func skipVariantElement(dec *wire.Decoder, tag uint8) {
	switch tag {
	case 22: // public_key — 32 bytes
		dec.ReadBytes(32)
	case 7, 9, 11, 19: // string types
		l := dec.ReadVarint()
		if dec.Err() == nil && l <= 65536 {
			dec.ReadBytes(int(l))
		}
	case 14, 15, 16, 27: // varint types
		dec.ReadVarint()
	case 10: // crypto_checksum — 8 bytes
		dec.ReadBytes(8)
	case 17: // signed_parts — 2 varints
		dec.ReadVarint()
		dec.ReadVarint()
	case 23, 24: // uint16
		dec.ReadBytes(2)
	case 26: // uint64
		dec.ReadBytes(8)
	case 28: // uint32
		dec.ReadBytes(4)
	case 8, 29: // old payer/receiver — 64 bytes
		dec.ReadBytes(64)
	case 30: // unlock_time2 — vector of entries
		cnt := dec.ReadVarint()
		if dec.Err() == nil {
			for j := uint64(0); j < cnt; j++ {
				dec.ReadVarint() // expiration
				dec.ReadVarint() // unlock_time
			}
		}
	case 31, 32: // payer/receiver — 64 bytes + optional flag
		dec.ReadBytes(64)
		marker := dec.ReadUint8()
		if marker != 0 {
			dec.ReadUint8()
		}
	case 39: // zarcanum_tx_data_v1 — 8 bytes (fee)
		dec.ReadBytes(8)
	case 21: // extra_padding — vector<uint8>
		l := dec.ReadVarint()
		if dec.Err() == nil && l <= 65536 {
			dec.ReadBytes(int(l))
		}
	case 18: // extra_attachment_info — string + hash + varint
		l := dec.ReadVarint()
		if dec.Err() == nil {
			dec.ReadBytes(int(l))
		}
		dec.ReadBytes(32)
		dec.ReadVarint()
	case 12: // tx_service_attachment — 3 strings + vector<key> + uint8
		for i := 0; i < 3; i++ {
			l := dec.ReadVarint()
			if dec.Err() == nil && l <= 65536 {
				dec.ReadBytes(int(l))
			}
		}
		cnt := dec.ReadVarint()
		if dec.Err() == nil {
			dec.ReadBytes(int(cnt) * 32)
		}
		dec.ReadUint8()
	default:
		// Unknown tag — can't skip safely, set error to abort
		dec.ReadBytes(0) // trigger EOF if nothing left
	}
}
