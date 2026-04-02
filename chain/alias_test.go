package chain

import (
	"testing"

	store "dappco.re/go/core/store"
)

func TestAlias_PutGet_Good(t *testing.T) {
	s, _ := store.New(t.TempDir() + "/test.db")
	defer s.Close()
	c := New(s)

	err := c.PutAlias(Alias{Name: "charon", Address: "abc123", Comment: "v=lthn1"})
	if err != nil {
		t.Fatalf("PutAlias: %v", err)
	}

	alias, err := c.GetAlias("charon")
	if err != nil {
		t.Fatalf("GetAlias: %v", err)
	}
	if alias.Name != "charon" {
		t.Errorf("name: got %s", alias.Name)
	}
	if alias.Comment != "v=lthn1" {
		t.Errorf("comment: got %s", alias.Comment)
	}
}

func TestAlias_GetAll_Good(t *testing.T) {
	s, _ := store.New(t.TempDir() + "/test.db")
	defer s.Close()
	c := New(s)

	c.PutAlias(Alias{Name: "a", Address: "1", Comment: "x"})
	c.PutAlias(Alias{Name: "b", Address: "2", Comment: "y"})

	all := c.GetAllAliases()
	if len(all) != 2 {
		t.Errorf("expected 2 aliases, got %d", len(all))
	}
}

func TestAlias_Get_Bad_NotFound(t *testing.T) {
	s, _ := store.New(t.TempDir() + "/test.db")
	defer s.Close()
	c := New(s)

	_, err := c.GetAlias("nonexistent")
	if err == nil {
		t.Error("expected error for missing alias")
	}
}
