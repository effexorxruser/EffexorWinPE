package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Journal stores UI and subprocess messages.
type Journal struct {
	mu      sync.Mutex
	entries []string
	path    string
}

// New creates a journal that optionally mirrors to path.
func New(path string) *Journal {
	return &Journal{path: path, entries: make([]string, 0, 64)}
}

// Path returns the on-disk journal path.
func (j *Journal) Path() string {
	if j == nil {
		return ""
	}
	return j.path
}

// Append adds a line with timestamp.
func (j *Journal) Append(format string, args ...any) {
	if j == nil {
		return
	}
	line := fmt.Sprintf(format, args...)
	stamp := time.Now().UTC().Format(time.RFC3339)
	entry := stamp + " " + line
	j.mu.Lock()
	defer j.mu.Unlock()
	j.entries = append(j.entries, entry)
	if j.path != "" {
		_ = os.MkdirAll(filepath.Dir(j.path), 0o755)
		f, err := os.OpenFile(j.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = f.WriteString(entry + "\n")
			_ = f.Close()
		}
	}
}

// Entries returns a copy of in-memory lines.
func (j *Journal) Entries() []string {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]string, len(j.entries))
	copy(out, j.entries)
	return out
}

// Text returns the full journal as one string.
func (j *Journal) Text() string {
	return strings.Join(j.Entries(), "\n")
}
