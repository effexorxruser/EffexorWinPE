package agentloop

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

const maxEvidenceFacts = 64
const maxEvidenceRefCount = 32

// NormalizeAndValidateEvidencePayload validates collector facts against the
// operation schema, applies privacy redaction checks, and replaces any
// collector-supplied evidence refs with loop-generated refs.
func NormalizeAndValidateEvidencePayload(payload EvidencePayload, request EvidenceRequest) (EvidencePayload, error) {
	if err := validateID("evidence payload request", payload.RequestID); err != nil {
		return EvidencePayload{}, err
	}
	if payload.RequestID != request.ID {
		return EvidencePayload{}, fmt.Errorf("evidence payload request_id %q does not match request %q", payload.RequestID, request.ID)
	}
	if payload.Operation != request.Operation {
		return EvidencePayload{}, fmt.Errorf("evidence payload operation %q does not match request %q", payload.Operation, request.Operation)
	}
	if !IsAllowedEvidenceOperation(payload.Operation) {
		return EvidencePayload{}, fmt.Errorf("evidence payload uses unknown operation %q", payload.Operation)
	}
	if payload.CollectedAt.IsZero() {
		return EvidencePayload{}, fmt.Errorf("evidence payload collected_at is required")
	}
	if payload.Facts == nil {
		return EvidencePayload{}, fmt.Errorf("evidence payload facts must be present")
	}
	// Drop collector-supplied refs; they are never trusted.
	payload.EvidenceRefs = nil
	payload.PrivacyClass = request.PrivacyClass
	payload.Facts = ApplyPrivacyRedactions(payload.Facts, request.PrivacyClass)
	if err := ValidateOperationFacts(payload.Operation, payload.Facts); err != nil {
		return EvidencePayload{}, err
	}
	refs, err := GenerateEvidenceRefs(payload.Operation, payload.Facts)
	if err != nil {
		return EvidencePayload{}, err
	}
	payload.EvidenceRefs = refs
	if err := validateSize("evidence payload", payload, MaxEvidencePayload); err != nil {
		return EvidencePayload{}, err
	}
	return payload, nil
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

func argumentMatchesReport(name string, value any, report diagnostics.Report) (any, error) {
	switch name {
	case "root":
		root, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("root must be a string")
		}
		if forbiddenPath(root) {
			return nil, fmt.Errorf("root path is forbidden")
		}
		for _, install := range report.Installations {
			if WindowsPathsEqual(install.Root, root) {
				return install.Root, nil
			}
		}
		return nil, fmt.Errorf("root %q is not a discovered Windows installation", root)
	case "store_path":
		path, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("store_path must be a string")
		}
		if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") ||
			strings.Contains(path, "..") || strings.Contains(path, "\x00") ||
			strings.Contains(strings.ToLower(path), `\\.`) || strings.Contains(path, `\\?`) {
			return nil, fmt.Errorf("store_path is forbidden")
		}
		for _, store := range report.Boot.BCDStores {
			if WindowsPathsEqual(store.Path, path) {
				return store.Path, nil
			}
		}
		return nil, fmt.Errorf("store_path %q is not a discovered BCD store", path)
	case "device_id":
		deviceID := stringifyArgument(value)
		for _, health := range report.Storage.DriveHealth {
			if strings.EqualFold(health.DeviceID, deviceID) {
				return health.DeviceID, nil
			}
		}
		for _, disk := range report.Storage.Disks {
			if strconv.Itoa(disk.Number) == deviceID {
				return strconv.Itoa(disk.Number), nil
			}
		}
		return nil, fmt.Errorf("device_id %q is not present in the report", deviceID)
	case "disk_number":
		number, ok := asInt(value)
		if !ok {
			return nil, fmt.Errorf("disk_number must be an integer")
		}
		for _, disk := range report.Storage.Disks {
			if disk.Number == number {
				return number, nil
			}
		}
		return nil, fmt.Errorf("disk_number %d is not present in the report", number)
	case "mount_point":
		mount, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("mount_point must be a string")
		}
		if forbiddenPath(mount) {
			return nil, fmt.Errorf("mount_point is forbidden")
		}
		for _, volume := range report.Storage.BitLockerVolumes {
			if WindowsPathsEqual(volume.MountPoint, mount) {
				return volume.MountPoint, nil
			}
		}
		for _, partition := range report.Storage.Partitions {
			if partition.DriveLetter == "" {
				continue
			}
			canonical := partition.DriveLetter + ":"
			candidates := []string{
				partition.DriveLetter,
				canonical,
				partition.DriveLetter + `:\`,
			}
			for _, candidate := range candidates {
				if WindowsPathsEqual(candidate, mount) {
					return canonical, nil
				}
			}
		}
		return nil, fmt.Errorf("mount_point %q is not present in the report", mount)
	case "check_id":
		checkID, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("check_id must be a string")
		}
		for _, check := range report.Checks {
			if check.ID == checkID {
				return check.ID, nil
			}
		}
		return nil, fmt.Errorf("check_id %q is not present in the report", checkID)
	default:
		return nil, fmt.Errorf("unsupported argument %q", name)
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
