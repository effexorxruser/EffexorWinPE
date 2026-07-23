package agentloop

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

// Closed read-only evidence operations. No repair or shell execution is implied.
const (
	OpReviewMissingSources        = "review_missing_sources"
	OpIdentifyWindowsInstallation = "identify_windows_installation"
	OpSelectWindowsTarget         = "select_windows_target"
	OpInspectBCDEntries           = "inspect_bcd_entries"
	OpInspectStorageHealth        = "inspect_storage_health"
	OpReviewBitLockerAccess       = "review_bitlocker_access"
	OpInspectNetworkStatus        = "inspect_network_status"
	OpInspectPartitionLayout      = "inspect_partition_layout"
	OpInspectBootFirmware         = "inspect_boot_firmware"
)

type argumentSpec struct {
	Types    []string
	Required bool
}

type operationSpec struct {
	ID             string
	PrivacyClass   string
	Arguments      map[string]argumentSpec
	DefaultTimeout int
	MaxTimeout     int
}

var evidenceOperations = map[string]operationSpec{
	OpReviewMissingSources: {
		ID:           OpReviewMissingSources,
		PrivacyClass: PrivacyMachineInventory,
		Arguments: map[string]argumentSpec{
			"check_id": {Types: []string{"string"}, Required: false},
		},
		DefaultTimeout: 30,
		MaxTimeout:     60,
	},
	OpIdentifyWindowsInstallation: {
		ID:           OpIdentifyWindowsInstallation,
		PrivacyClass: PrivacyMachineInventory,
		Arguments: map[string]argumentSpec{
			"root": {Types: []string{"string"}, Required: true},
		},
		DefaultTimeout: 30,
		MaxTimeout:     60,
	},
	OpSelectWindowsTarget: {
		ID:           OpSelectWindowsTarget,
		PrivacyClass: PrivacyMachineInventory,
		Arguments: map[string]argumentSpec{
			"root": {Types: []string{"string"}, Required: true},
		},
		DefaultTimeout: 15,
		MaxTimeout:     30,
	},
	OpInspectBCDEntries: {
		ID:           OpInspectBCDEntries,
		PrivacyClass: PrivacyBootConfig,
		Arguments: map[string]argumentSpec{
			"store_path": {Types: []string{"string"}, Required: false},
		},
		DefaultTimeout: 30,
		MaxTimeout:     60,
	},
	OpInspectStorageHealth: {
		ID:           OpInspectStorageHealth,
		PrivacyClass: PrivacyStorageHealth,
		Arguments: map[string]argumentSpec{
			"device_id": {Types: []string{"string", "integer"}, Required: false},
		},
		DefaultTimeout: 45,
		MaxTimeout:     90,
	},
	OpReviewBitLockerAccess: {
		ID:           OpReviewBitLockerAccess,
		PrivacyClass: PrivacyEncryptionStatus,
		Arguments: map[string]argumentSpec{
			"mount_point": {Types: []string{"string"}, Required: false},
		},
		DefaultTimeout: 30,
		MaxTimeout:     60,
	},
	OpInspectNetworkStatus: {
		ID:             OpInspectNetworkStatus,
		PrivacyClass:   PrivacyNetworkStatus,
		Arguments:      map[string]argumentSpec{},
		DefaultTimeout: 20,
		MaxTimeout:     45,
	},
	OpInspectPartitionLayout: {
		ID:           OpInspectPartitionLayout,
		PrivacyClass: PrivacyMachineInventory,
		Arguments: map[string]argumentSpec{
			"disk_number": {Types: []string{"integer"}, Required: false},
		},
		DefaultTimeout: 30,
		MaxTimeout:     60,
	},
	OpInspectBootFirmware: {
		ID:             OpInspectBootFirmware,
		PrivacyClass:   PrivacyBootConfig,
		Arguments:      map[string]argumentSpec{},
		DefaultTimeout: 15,
		MaxTimeout:     30,
	},
}

// ReadOnlyEvidenceOperations returns the closed allowlist in stable order.
func ReadOnlyEvidenceOperations() []string {
	ids := make([]string, 0, len(evidenceOperations))
	for id := range evidenceOperations {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// IsAllowedEvidenceOperation reports whether operation is in the closed allowlist.
func IsAllowedEvidenceOperation(operation string) bool {
	_, ok := evidenceOperations[operation]
	return ok
}

// ValidateEvidenceRequest checks operation membership, privacy class, arguments,
// timeout, and that argument values exist in the current diagnostic report.
func ValidateEvidenceRequest(request EvidenceRequest, report diagnostics.Report) error {
	if err := validateID("evidence request", request.ID); err != nil {
		return err
	}
	spec, ok := evidenceOperations[request.Operation]
	if !ok {
		return fmt.Errorf("evidence request %q uses unknown operation %q", request.ID, request.Operation)
	}
	if request.PrivacyClass != spec.PrivacyClass {
		return fmt.Errorf("evidence request %q privacy_class must be %q", request.ID, spec.PrivacyClass)
	}
	if err := boundedText("evidence reason", request.Reason, 2000); err != nil {
		return err
	}
	if err := boundedText("expected information", request.ExpectedInformation, 2000); err != nil {
		return err
	}
	if request.TimeoutSeconds <= 0 || request.TimeoutSeconds > spec.MaxTimeout {
		return fmt.Errorf("evidence request %q timeout_seconds must be between 1 and %d", request.ID, spec.MaxTimeout)
	}
	args := request.Arguments
	if args == nil {
		args = map[string]any{}
	}
	for name := range args {
		if _, allowed := spec.Arguments[name]; !allowed {
			return fmt.Errorf("evidence request %q has unsupported argument %q", request.ID, name)
		}
	}
	for name, argSpec := range spec.Arguments {
		value, present := args[name]
		if !present || value == nil {
			if argSpec.Required {
				return fmt.Errorf("evidence request %q missing required argument %q", request.ID, name)
			}
			continue
		}
		if !argumentTypeAllowed(value, argSpec.Types) {
			return fmt.Errorf("evidence request %q argument %q has invalid type", request.ID, name)
		}
		if str, ok := value.(string); ok {
			if err := boundedText("argument "+name, str, 512); err != nil {
				return err
			}
		}
		if err := argumentMatchesReport(name, value, report); err != nil {
			return fmt.Errorf("evidence request %q: %w", request.ID, err)
		}
	}
	return nil
}

// CanonicalRequestKey fingerprints an evidence request for duplicate detection.
func CanonicalRequestKey(request EvidenceRequest) string {
	args := request.Arguments
	if args == nil {
		args = map[string]any{}
	}
	names := make([]string, 0, len(args))
	for name := range args {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, 1+len(names))
	parts = append(parts, request.Operation)
	for _, name := range names {
		parts = append(parts, name+"="+stringifyArgument(args[name]))
	}
	return strings.Join(parts, "|")
}

func argumentTypeAllowed(value any, types []string) bool {
	actual := ""
	switch value.(type) {
	case string:
		actual = "string"
	case bool:
		actual = "boolean"
	case float64:
		// JSON numbers decode as float64.
		actual = "integer"
		if number, ok := value.(float64); ok && number != float64(int64(number)) {
			return false
		}
	case int:
		actual = "integer"
	case int64:
		actual = "integer"
	case nil:
		actual = "null"
	default:
		return false
	}
	for _, allowed := range types {
		if allowed == actual {
			return true
		}
	}
	return false
}

func stringifyArgument(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return fmt.Sprintf("%v", typed)
	}
}
