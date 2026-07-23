package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestShellWindowsAMD64CrossBuild(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	modRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	out := filepath.Join(t.TempDir(), "effexorwinpe-shell.exe")
	cmd := exec.Command("go", "build", "-trimpath", "-o", out, ".")
	cmd.Dir = filepath.Join(modRoot, "cmd", "effexorwinpe-shell")
	cmd.Env = append(os.Environ(),
		"GOOS=windows",
		"GOARCH=amd64",
		"CGO_ENABLED=0",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cross-build failed: %v\n%s", err, output)
	}
	st, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() < 1024 {
		t.Fatalf("exe too small: %d", st.Size())
	}
	t.Logf("effexorwinpe-shell.exe size=%d bytes", st.Size())
}
