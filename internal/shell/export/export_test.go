package export

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopySessionOK(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "initial.json")
	if err := os.WriteFile(src, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	res := CopySession(dstDir, map[string]string{"initial.json": src})
	if !res.OK || res.FriendlyKey != "msg.export_ok" {
		t.Fatalf("result = %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "initial.json")); err != nil {
		t.Fatal(err)
	}
}

func TestCopySessionReadOnly(t *testing.T) {
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "initial.json")
	if err := os.WriteFile(src, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "ro")
	if err := os.MkdirAll(dst, 0o555); err != nil {
		t.Fatal(err)
	}
	// Best-effort: on some systems root can still write; assert classified failure or ok.
	res := CopySession(dst, map[string]string{"initial.json": src})
	if res.OK {
		t.Skip("environment allows writes to 0555 directories")
	}
	if res.FriendlyKey != "msg.export_readonly" && res.FriendlyKey != "msg.export_failed" {
		t.Fatalf("unexpected key %q detail=%q", res.FriendlyKey, res.Detail)
	}
}
