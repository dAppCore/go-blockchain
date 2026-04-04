package chain

import (
	"os"
	"testing"
)

func TestBlockdata_WriteAtomic_Good(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.bin"
	data := []byte("hello block")

	err := WriteAtomic(path, data)
	if err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	read, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(read) != "hello block" {
		t.Errorf("content: got %s", string(read))
	}

	// Verify no temp file left behind
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("temp file should not exist after atomic write")
	}
}

func TestBlockdata_EnsureDir_Good(t *testing.T) {
	dir := t.TempDir() + "/a/b/c"
	err := EnsureDir(dir)
	if err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Error("directory should exist")
	}
}
