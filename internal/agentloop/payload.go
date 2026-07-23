package agentloop

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

const maxEvidenceFacts = 64
const maxEvidenceRefCount = 32

// ValidateEvidencePayload checks collector output against the approved request.
func ValidateEvidencePayload(payload EvidencePayload, request EvidenceRequest) error {
	if err := validateID("evidence payload request", payload.RequestID); err != nil {
		return err
	}
	if payload.RequestID != request.ID {
		return fmt.Errorf("evidence payload request_id %q does not match request %q", payload.RequestID, request.ID)
	}
	if payload.Operation != request.Operation {
		return fmt.Errorf("evidence payload operation %q does not match request %q", payload.Operation, request.Operation)
	}
	if !IsAllowedEvidenceOperation(payload.Operation) {
		return fmt.Errorf("evidence payload uses unknown operation %q", payload.Operation)
	}
	if payload.CollectedAt.IsZero() {
		return fmt.Errorf("evidence payload collected_at is required")
	}
	if payload.Facts == nil {
		return fmt.Errorf("evidence payload facts must be present")
	}
	if len(payload.Facts) > maxEvidenceFacts {
		return fmt.Errorf("evidence payload facts exceed %d entries", maxEvidenceFacts)
	}
	for key, value := range payload.Facts {
		if strings.TrimSpace(key) == "" || len(key) > 128 {
			return fmt.Errorf("evidence payload fact key is invalid")
		}
		if forbiddenPath(key) {
			return fmt.Errorf("evidence payload fact key %q is forbidden", key)
		}
		switch typed := value.(type) {
		case nil, bool, string, float64, int, int64:
			if str, ok := value.(string); ok {
				if err := boundedText("evidence fact", str, 2000); err != nil {
					return err
				}
				if err := RejectCommandText("evidence fact", str); err != nil {
					return err
				}
			}
			_ = typed
		default:
			return fmt.Errorf("evidence payload fact %q has unsupported type", key)
		}
	}
	if payload.EvidenceRefs == nil {
		return fmt.Errorf("evidence payload evidence_refs must be present")
	}
	if len(payload.EvidenceRefs) > maxEvidenceRefCount {
		return fmt.Errorf("evidence payload evidence_refs exceed %d entries", maxEvidenceRefCount)
	}
	seen := map[string]struct{}{}
	for _, ref := range payload.EvidenceRefs {
		if err := boundedText("evidence ref", ref, 512); err != nil {
			return err
		}
		if forbiddenPath(ref) {
			return fmt.Errorf("evidence payload evidence_ref %q is forbidden", ref)
		}
		if _, duplicate := seen[ref]; duplicate {
			return fmt.Errorf("evidence payload repeats evidence_ref %q", ref)
		}
		seen[ref] = struct{}{}
	}
	return validateSize("evidence payload", payload, MaxEvidencePayload)
}

func forbiddenPath(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return true
	}
	if strings.Contains(trimmed, "\x00") || strings.Contains(trimmed, "..") {
		return true
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(trimmed, `\\`) || strings.HasPrefix(trimmed, "//") {
		return true
	}
	if strings.HasPrefix(lower, `\\.`) || strings.HasPrefix(lower, `\\?`) {
		return true
	}
	if strings.Contains(lower, `\\.`) || strings.Contains(lower, `\\?`) {
		return true
	}
	if strings.Contains(lower, "/dev/") || strings.Contains(lower, "\\\\.\\") {
		return true
	}
	return false
}

func argumentMatchesReport(name string, value any, report diagnostics.Report) error {
	switch name {
	case "root":
		root, ok := value.(string)
		if !ok {
			return fmt.Errorf("root must be a string")
		}
		if forbiddenPath(root) {
			return fmt.Errorf("root path is forbidden")
		}
		for _, install := range report.Installations {
			if install.Root == root {
				return nil
			}
		}
		return fmt.Errorf("root %q is not a discovered Windows installation", root)
	case "store_path":
		path, ok := value.(string)
		if !ok {
			return fmt.Errorf("store_path must be a string")
		}
		// BCD paths may start with a single backslash; reject UNC and device namespaces.
		if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") ||
			strings.Contains(path, "..") || strings.Contains(path, "\x00") ||
			strings.Contains(strings.ToLower(path), `\\.`) || strings.Contains(path, `\\?`) {
			return fmt.Errorf("store_path is forbidden")
		}
		for _, store := range report.Boot.BCDStores {
			if store.Path == path {
				return nil
			}
		}
		return fmt.Errorf("store_path %q is not a discovered BCD store", path)
	case "device_id":
		deviceID := stringifyArgument(value)
		for _, health := range report.Storage.DriveHealth {
			if health.DeviceID == deviceID {
				return nil
			}
		}
		for _, disk := range report.Storage.Disks {
			if strconv.Itoa(disk.Number) == deviceID {
				return nil
			}
		}
		return fmt.Errorf("device_id %q is not present in the report", deviceID)
	case "disk_number":
		number, ok := asInt(value)
		if !ok {
			return fmt.Errorf("disk_number must be an integer")
		}
		for _, disk := range report.Storage.Disks {
			if disk.Number == number {
				return nil
			}
		}
		return fmt.Errorf("disk_number %d is not present in the report", number)
	case "mount_point":
		mount, ok := value.(string)
		if !ok {
			return fmt.Errorf("mount_point must be a string")
		}
		if forbiddenPath(mount) {
			return fmt.Errorf("mount_point is forbidden")
		}
		for _, volume := range report.Storage.BitLockerVolumes {
			if volume.MountPoint == mount {
				return nil
			}
		}
		for _, partition := range report.Storage.Partitions {
			if partition.DriveLetter != "" && (partition.DriveLetter == mount || partition.DriveLetter+":" == mount || partition.DriveLetter+`:\` == mount) {
				return nil
			}
		}
		return fmt.Errorf("mount_point %q is not present in the report", mount)
	case "check_id":
		checkID, ok := value.(string)
		if !ok {
			return fmt.Errorf("check_id must be a string")
		}
		for _, check := range report.Checks {
			if check.ID == checkID {
				return nil
			}
		}
		return fmt.Errorf("check_id %q is not present in the report", checkID)
	default:
		return fmt.Errorf("unsupported argument %q", name)
	}
}

func asInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		if typed != float64(int64(typed)) {
			return 0, false
		}
		return int(typed), true
	default:
		return 0, false
	}
}
