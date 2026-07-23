package export

import (
	"fmt"
	"path/filepath"
	"strings"
)

// DriveKind classifies a volume for export safety.
type DriveKind string

const (
	DriveRemovable DriveKind = "removable"
	DriveFixed     DriveKind = "fixed"
	DriveOther     DriveKind = "other"
)

// DriveInfo describes a candidate export target.
type DriveInfo struct {
	Root      string // e.g. E:\
	Letter    string // e.g. E
	Label     string
	SizeBytes uint64
	Kind      DriveKind
}

// DriveScanner lists volumes. Tests inject fakes; production uses OS scanner.
type DriveScanner interface {
	List() ([]DriveInfo, error)
}

// ExportPolicy filters drives for safe technician export.
type ExportPolicy struct {
	ExcludeRoots []string // always excluded (e.g. X:\, Windows install roots)
}

// DefaultExcludeRoots returns roots that must never be default export targets.
func DefaultExcludeRoots(windowsInstallRoots []string) []string {
	out := []string{`X:\`, `X:/`, `X:`}
	for _, root := range windowsInstallRoots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		// Normalize install root like C:\Windows -> C:\
		vol := volumeRoot(root)
		if vol != "" {
			out = append(out, vol)
		}
	}
	return out
}

func volumeRoot(path string) string {
	path = strings.ReplaceAll(path, "/", `\`)
	if len(path) >= 2 && path[1] == ':' {
		return strings.ToUpper(path[:1]) + `:\`
	}
	return ""
}

func normalizeRoot(root string) string {
	root = strings.TrimSpace(root)
	root = strings.ReplaceAll(root, "/", `\`)
	if root == "" {
		return ""
	}
	if !strings.HasSuffix(root, `\`) {
		root += `\`
	}
	if len(root) >= 2 && root[1] == ':' {
		return strings.ToUpper(root[:1]) + root[1:]
	}
	return root
}

func excluded(root string, exclude []string) bool {
	root = normalizeRoot(root)
	for _, ex := range exclude {
		if normalizeRoot(ex) == root {
			return true
		}
	}
	return false
}

// CandidateDrives returns removable drives first. Fixed drives are returned
// separately and require advanced confirmation before use.
func CandidateDrives(scanner DriveScanner, policy ExportPolicy) (removable []DriveInfo, fixed []DriveInfo, err error) {
	if scanner == nil {
		return nil, nil, fmt.Errorf("drive scanner is nil")
	}
	all, err := scanner.List()
	if err != nil {
		return nil, nil, err
	}
	for _, d := range all {
		d.Root = normalizeRoot(d.Root)
		if d.Root == "" || excluded(d.Root, policy.ExcludeRoots) {
			continue
		}
		switch d.Kind {
		case DriveRemovable:
			removable = append(removable, d)
		case DriveFixed:
			fixed = append(fixed, d)
		}
	}
	return removable, fixed, nil
}

// FormatDrive returns a human-readable drive line for UI prompts.
func FormatDrive(d DriveInfo) string {
	label := d.Label
	if label == "" {
		label = "-"
	}
	size := formatSize(d.SizeBytes)
	return fmt.Sprintf("%s  %s  %s  [%s]", d.Root, label, size, d.Kind)
}

func formatSize(v uint64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
		tb = gb * 1024
	)
	switch {
	case v >= tb:
		return fmt.Sprintf("%.1f TiB", float64(v)/float64(tb))
	case v >= gb:
		return fmt.Sprintf("%.1f GiB", float64(v)/float64(gb))
	case v >= mb:
		return fmt.Sprintf("%.1f MiB", float64(v)/float64(mb))
	default:
		return fmt.Sprintf("%d B", v)
	}
}

// ExportDirForDrive builds the default export folder on a drive.
func ExportDirForDrive(root string) string {
	return filepath.Join(normalizeRoot(root), "EffexorWinPE-reports")
}
