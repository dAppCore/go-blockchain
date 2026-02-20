package wallet

import (
	"strings"
	"testing"
)

func TestWordlistLength(t *testing.T) {
	if len(wordlist) != 1626 {
		t.Fatalf("wordlist length = %d, want 1626", len(wordlist))
	}
}

func TestWordlistFirstLast(t *testing.T) {
	if wordlist[0] != "like" {
		t.Errorf("wordlist[0] = %q, want %q", wordlist[0], "like")
	}
	if wordlist[1625] != "weary" {
		t.Errorf("wordlist[1625] = %q, want %q", wordlist[1625], "weary")
	}
}

func TestWordIndexConsistency(t *testing.T) {
	for i, w := range wordlist {
		idx, ok := wordIndex[w]
		if !ok {
			t.Fatalf("word %q not in index", w)
		}
		if idx != i {
			t.Fatalf("wordIndex[%q] = %d, want %d", w, idx, i)
		}
	}
}

func TestMnemonicRoundTrip(t *testing.T) {
	var key [32]byte

	phrase, err := MnemonicEncode(key[:])
	if err != nil {
		t.Fatal(err)
	}

	words := strings.Fields(phrase)
	if len(words) != 25 {
		t.Fatalf("got %d words, want 25", len(words))
	}

	decoded, err := MnemonicDecode(phrase)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != key {
		t.Fatalf("round-trip failed: got %x, want %x", decoded, key)
	}
}

func TestMnemonicRoundTripNonZero(t *testing.T) {
	var key [32]byte
	for i := range key {
		key[i] = byte(i * 7)
	}

	phrase, err := MnemonicEncode(key[:])
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := MnemonicDecode(phrase)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != key {
		t.Fatalf("round-trip failed: got %x, want %x", decoded, key)
	}
}

func TestMnemonicInvalidWordCount(t *testing.T) {
	_, err := MnemonicDecode("like just love")
	if err == nil {
		t.Fatal("expected error for 3 words")
	}
}

func TestMnemonicInvalidWord(t *testing.T) {
	phrase := "like just love know never want time out there make look eye down only think call hand high keep last long make new zzzznotaword"
	_, err := MnemonicDecode(phrase)
	if err == nil {
		t.Fatal("expected error for invalid word")
	}
}

func TestMnemonicBadChecksum(t *testing.T) {
	var key [32]byte

	phrase, _ := MnemonicEncode(key[:])
	words := strings.Fields(phrase)
	words[24] = "never"
	if words[24] == words[0] {
		words[24] = "want"
	}

	_, err := MnemonicDecode(strings.Join(words, " "))
	if err == nil {
		t.Fatal("expected checksum error")
	}
}

func TestMnemonicInvalidLength(t *testing.T) {
	_, err := MnemonicEncode([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("expected error for non-32-byte input")
	}
}
