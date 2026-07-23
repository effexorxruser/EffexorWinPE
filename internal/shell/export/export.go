package export

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Request lists files to copy into a destination directory.
type Request struct {
	DestinationDir string
	Files          map[string]string // dest name -> source path
}

// Result describes export outcome.
type Result struct {
	OK          bool
	FriendlyKey string
	Detail      string
	Copied      []string
}

// CopySession copies report/diagnosis/session/journal into destinationDir.
func CopySession(destinationDir string, sources map[string]string) Result {
	destinationDir = strings.TrimSpace(destinationDir)
	if destinationDir == "" {
		return Result{OK: false, FriendlyKey: "msg.export_failed", Detail: "destination is empty"}
	}
	if err := os.MkdirAll(destinationDir, 0o755); err != nil {
		return classifyWriteError(err)
	}
	probe := filepath.Join(destinationDir, ".effexorwinpe-write-test")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return classifyWriteError(err)
	}
	_ = os.Remove(probe)

	var copied []string
	for name, src := range sources {
		if strings.TrimSpace(src) == "" {
			continue
		}
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dest := filepath.Join(destinationDir, name)
		if err := copyFile(src, dest); err != nil {
			return classifyWriteError(err)
		}
		copied = append(copied, dest)
	}
	if len(copied) == 0 {
		return Result{OK: false, FriendlyKey: "msg.export_failed", Detail: "no source files available"}
	}
	return Result{OK: true, FriendlyKey: "msg.export_ok", Copied: copied}
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func classifyWriteError(err error) Result {
	msg := err.Error()
	key := "msg.export_failed"
	lower := strings.ToLower(msg)
	switch {
	case errors.Is(err, os.ErrPermission),
		strings.Contains(lower, "access is denied"),
		strings.Contains(lower, "read-only"),
		strings.Contains(lower, "permission denied"):
		key = "msg.export_readonly"
	case strings.Contains(lower, "the device is not ready"),
		strings.Contains(lower, "no such device"),
		strings.Contains(lower, "not ready"):
		key = "msg.export_usb_failed"
	}
	return Result{OK: false, FriendlyKey: key, Detail: fmt.Sprintf("%v", err)}
}
