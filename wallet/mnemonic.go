package wallet

import (
	"encoding/binary"
	"hash/crc32"

	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
)

const numWords = 1626

// MnemonicEncode converts a 32-byte secret key to a 25-word mnemonic phrase.
func MnemonicEncode(key []byte) (string, error) {
	if len(key) != 32 {
		return "", coreerr.E("MnemonicEncode", core.Sprintf("wallet: mnemonic encode requires 32 bytes, got %d", len(key)), nil)
	}

	words := make([]string, 0, 25)
	n := uint32(numWords)

	for i := 0; i < 32; i += 4 {
		val := binary.LittleEndian.Uint32(key[i : i+4])
		w1 := val % n
		w2 := ((val / n) + w1) % n
		w3 := (((val / n) / n) + w2) % n
		words = append(words, wordlist[w1], wordlist[w2], wordlist[w3])
	}

	checkIdx := checksumIndex(words)
	words = append(words, words[checkIdx])

	return core.Join(" ", words...), nil
}

// MnemonicDecode converts a 25-word mnemonic phrase to a 32-byte secret key.
func MnemonicDecode(phrase string) ([32]byte, error) {
	var key [32]byte

	words := mnemonicWords(phrase)
	if len(words) != 25 {
		return key, coreerr.E("MnemonicDecode", core.Sprintf("wallet: mnemonic requires 25 words, got %d", len(words)), nil)
	}

	expected := checksumIndex(words[:24])
	if words[24] != words[expected] {
		return key, coreerr.E("MnemonicDecode", "wallet: mnemonic checksum failed", nil)
	}

	n := uint32(numWords)

	for i := range 8 {
		w1, ok1 := wordIndex[words[i*3]]
		w2, ok2 := wordIndex[words[i*3+1]]
		w3, ok3 := wordIndex[words[i*3+2]]
		if !ok1 || !ok2 || !ok3 {
			word := words[i*3]
			if !ok2 {
				word = words[i*3+1]
			}
			if !ok3 {
				word = words[i*3+2]
			}
			return key, coreerr.E("MnemonicDecode", core.Sprintf("wallet: unknown mnemonic word %q", word), nil)
		}

		val := uint32(w1) +
			n*(((n-uint32(w1))+uint32(w2))%n) +
			n*n*(((n-uint32(w2))+uint32(w3))%n)
		binary.LittleEndian.PutUint32(key[i*4:i*4+4], val)
	}

	return key, nil
}

func mnemonicWords(phrase string) []string {
	normalised := core.Trim(phrase)
	for _, ws := range []string{"\n", "\r", "\t"} {
		normalised = core.Replace(normalised, ws, " ")
	}
	parts := core.Split(normalised, " ")
	words := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			words = append(words, part)
		}
	}
	return words
}

func checksumIndex(words []string) int {
	var prefixes string
	for _, w := range words {
		if len(w) >= 3 {
			prefixes += w[:3]
		} else {
			prefixes += w
		}
	}
	return int(crc32.ChecksumIEEE([]byte(prefixes))) % len(words)
}
