package export

import (
	"fmt"
	"strings"
)

// PathDecision is the export-policy outcome for a concrete filesystem path.
type PathDecision int

const (
	// PathAllowRemovable means the path is on an allowed removable volume.
	PathAllowRemovable PathDecision = iota
	// PathRequireFixedConfirm means the path is on a fixed/internal volume and
	// may be used only after explicit advanced confirmation.
	PathRequireFixedConfirm
	// PathRejectExcluded means the volume root is banned (WinPE X:\ or a
	// discovered Windows installation volume).
	PathRejectExcluded
	// PathRejectUnknown means the volume type is missing/unknown/other.
	PathRejectUnknown
)

// PathEvaluation is the result of validating an export target path.
type PathEvaluation struct {
	Decision   PathDecision
	Path       string
	VolumeRoot string
	Drive      DriveInfo
	ReasonKey  string
}

// VolumeRoot returns the drive root for a Windows-style path (e.g. C:\ from
// C:\Users\x\reports). Nested directories cannot bypass volume policy.
func VolumeRoot(path string) string {
	return volumeRoot(path)
}

// EvaluateExportPath applies ExportPolicy to any chosen path (folder dialog,
// typed path, or nested directory). Removable volumes are allowed; fixed
// volumes require advanced confirmation; excluded and unknown kinds are rejected.
func EvaluateExportPath(path string, scanner DriveScanner, policy ExportPolicy) (PathEvaluation, error) {
	path = strings.TrimSpace(path)
	eval := PathEvaluation{Path: path, ReasonKey: "msg.export_path_unknown"}
	if path == "" {
		eval.Decision = PathRejectUnknown
		return eval, nil
	}
	root := VolumeRoot(path)
	eval.VolumeRoot = root
	if root == "" {
		eval.Decision = PathRejectUnknown
		return eval, nil
	}
	if excluded(root, policy.ExcludeRoots) {
		eval.Decision = PathRejectExcluded
		eval.ReasonKey = "msg.export_path_excluded"
		return eval, nil
	}
	if scanner == nil {
		return eval, fmt.Errorf("drive scanner is nil")
	}
	drives, err := scanner.List()
	if err != nil {
		return eval, err
	}
	var matched *DriveInfo
	for i := range drives {
		if normalizeRoot(drives[i].Root) == normalizeRoot(root) {
			d := drives[i]
			d.Root = normalizeRoot(d.Root)
			matched = &d
			break
		}
	}
	if matched == nil {
		eval.Decision = PathRejectUnknown
		eval.ReasonKey = "msg.export_path_unknown"
		return eval, nil
	}
	eval.Drive = *matched
	switch matched.Kind {
	case DriveRemovable:
		eval.Decision = PathAllowRemovable
		eval.ReasonKey = ""
		return eval, nil
	case DriveFixed:
		eval.Decision = PathRequireFixedConfirm
		eval.ReasonKey = "msg.export_fixed_confirm"
		return eval, nil
	default:
		eval.Decision = PathRejectUnknown
		eval.ReasonKey = "msg.export_path_unknown"
		return eval, nil
	}
}
