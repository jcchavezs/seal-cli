package upgrade

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReplaceExecutableWritesNewBytesAndPreservesExecutableMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows replacement is manual")
	}

	dir := t.TempDir()
	exe := filepath.Join(dir, "seal")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := ReplaceExecutable(exe, []byte("new")); err != nil {
		t.Fatalf("ReplaceExecutable() error = %v", err)
	}

	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("replacement bytes = %q, want new", got)
	}

	info, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("replacement mode = %v, want executable bit set", info.Mode().Perm())
	}
}
