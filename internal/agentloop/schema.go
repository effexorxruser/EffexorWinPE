package agentloop

import (
	"fmt"
	"sort"
	"strings"
)

type factKind string

const (
	factString      factKind = "string"
	factInteger     factKind = "integer"
	factBoolean     factKind = "boolean"
	factNull        factKind = "null"
	factStringArray factKind = "string_array"
	factObjectArray factKind = "object_array"
)

type factFieldSpec struct {
	Kinds      []factKind
	Required   bool
	ObjectKeys map[string]factFieldSpec
}

// operationFactSchemas define the closed fact shapes accepted per operation.
var operationFactSchemas = map[string]map[string]factFieldSpec{
	OpInspectNetworkStatus: {
		"adapters": {
			Kinds:    []factKind{factObjectArray},
			Required: true,
			ObjectKeys: map[string]factFieldSpec{
				"name":        {Kinds: []factKind{factString}, Required: true},
				"status":      {Kinds: []factKind{factString}, Required: true},
				"status_code": {Kinds: []factKind{factInteger, factNull}, Required: false},
			},
		},
		"dhcp_state": {Kinds: []factKind{factString, factNull}, Required: false},
	},
	OpInspectBCDEntries: {
		"stores": {
			Kinds:    []factKind{factObjectArray},
			Required: true,
			ObjectKeys: map[string]factFieldSpec{
				"path": {Kinds: []factKind{factString}, Required: true},
				"kind": {Kinds: []factKind{factString}, Required: true},
			},
		},
		"default_object_visible": {Kinds: []factKind{factBoolean, factNull}, Required: false},
	},
	OpInspectPartitionLayout: {
		"partitions": {
			Kinds:    []factKind{factObjectArray},
			Required: true,
			ObjectKeys: map[string]factFieldSpec{
				"disk_number":      {Kinds: []factKind{factInteger}, Required: true},
				"partition_number": {Kinds: []factKind{factInteger}, Required: true},
				"size_bytes":       {Kinds: []factKind{factInteger}, Required: true},
				"type":             {Kinds: []factKind{factString, factNull}, Required: false},
				"drive_letter":     {Kinds: []factKind{factString, factNull}, Required: false},
			},
		},
	},
	OpInspectStorageHealth: {
		"device_id":           {Kinds: []factKind{factString, factInteger}, Required: false},
		"health_status":       {Kinds: []factKind{factString}, Required: true},
		"temperature_celsius": {Kinds: []factKind{factInteger, factNull}, Required: false},
		"read_errors_total":   {Kinds: []factKind{factInteger, factNull}, Required: false},
		"write_errors_total":  {Kinds: []factKind{factInteger, factNull}, Required: false},
		"power_on_hours":      {Kinds: []factKind{factInteger, factNull}, Required: false},
	},
	OpReviewBitLockerAccess: {
		"inventory_status": {Kinds: []factKind{factString}, Required: true},
		"volumes": {
			Kinds:    []factKind{factObjectArray},
			Required: false,
			ObjectKeys: map[string]factFieldSpec{
				"mount_point":       {Kinds: []factKind{factString}, Required: true},
				"protection_status": {Kinds: []factKind{factString, factNull}, Required: false},
				"lock_status":       {Kinds: []factKind{factString, factNull}, Required: false},
				"encryption_method": {Kinds: []factKind{factString, factNull}, Required: false},
			},
		},
	},
	OpInspectBootFirmware: {
		"firmware_mode": {Kinds: []factKind{factString}, Required: true},
		"secure_boot":   {Kinds: []factKind{factBoolean, factNull}, Required: false},
	},
	OpReviewMissingSources: {
		"missing_count": {Kinds: []factKind{factInteger}, Required: true},
		"check_ids":     {Kinds: []factKind{factStringArray}, Required: false},
	},
	OpIdentifyWindowsInstallation: {
		"root":         {Kinds: []factKind{factString}, Required: true},
		"product_name": {Kinds: []factKind{factString, factNull}, Required: false},
		"build":        {Kinds: []factKind{factString, factNull}, Required: false},
	},
	OpSelectWindowsTarget: {
		"root":     {Kinds: []factKind{factString}, Required: true},
		"selected": {Kinds: []factKind{factBoolean}, Required: true},
	},
}

// ValidateOperationFacts checks facts against the closed per-operation schema
// and privacy redaction list (redacted keys must not appear).
func ValidateOperationFacts(operation string, facts map[string]any) error {
	schema, ok := operationFactSchemas[operation]
	if !ok {
		return fmt.Errorf("operation %q has no evidence fact schema", operation)
	}
	spec, ok := evidenceOperations[operation]
	if !ok {
		return fmt.Errorf("unknown operation %q", operation)
	}
	policy, err := PrivacyPolicyFor(spec.PrivacyClass)
	if err != nil {
		return err
	}
	redacted := map[string]struct{}{}
	for _, field := range policy.RedactedFields {
		redacted[strings.ToLower(field)] = struct{}{}
	}
	if facts == nil {
		return fmt.Errorf("facts must be present")
	}
	if len(facts) > maxEvidenceFacts {
		return fmt.Errorf("facts exceed %d entries", maxEvidenceFacts)
	}
	for key := range facts {
		if _, drop := redacted[strings.ToLower(key)]; drop {
			return fmt.Errorf("fact key %q is redacted for privacy_class %q", key, spec.PrivacyClass)
		}
		if _, allowed := schema[key]; !allowed {
			return fmt.Errorf("fact key %q is not allowed for operation %q", key, operation)
		}
	}
	for key, field := range schema {
		value, present := facts[key]
		if !present || value == nil {
			if field.Required && !kindAllowsNull(field.Kinds) {
				return fmt.Errorf("missing required fact %q", key)
			}
			if present && value == nil && !kindAllowsNull(field.Kinds) {
				return fmt.Errorf("fact %q cannot be null", key)
			}
			continue
		}
		if err := validateFactValue(key, value, field); err != nil {
			return err
		}
	}
	return nil
}

func kindAllowsNull(kinds []factKind) bool {
	for _, kind := range kinds {
		if kind == factNull {
			return true
		}
	}
	return false
}

func validateFactValue(key string, value any, field factFieldSpec) error {
	switch typed := value.(type) {
	case string:
		if !hasKind(field.Kinds, factString) {
			return fmt.Errorf("fact %q must not be a string", key)
		}
		if err := boundedText("fact "+key, typed, 2000); err != nil {
			return err
		}
		return RejectCommandText("fact "+key, typed)
	case bool:
		if !hasKind(field.Kinds, factBoolean) {
			return fmt.Errorf("fact %q must not be a boolean", key)
		}
	case float64, int, int64:
		if !hasKind(field.Kinds, factInteger) {
			return fmt.Errorf("fact %q must not be an integer", key)
		}
		if number, ok := value.(float64); ok && number != float64(int64(number)) {
			return fmt.Errorf("fact %q must be an integer", key)
		}
	case []any:
		if hasKind(field.Kinds, factStringArray) {
			for index, item := range typed {
				str, ok := item.(string)
				if !ok {
					return fmt.Errorf("fact %q[%d] must be a string", key, index)
				}
				if err := boundedText(fmt.Sprintf("fact %s[%d]", key, index), str, 512); err != nil {
					return err
				}
			}
			return nil
		}
		if hasKind(field.Kinds, factObjectArray) {
			if len(typed) > 64 {
				return fmt.Errorf("fact %q array exceeds 64 items", key)
			}
			for index, item := range typed {
				object, ok := item.(map[string]any)
				if !ok {
					return fmt.Errorf("fact %q[%d] must be an object", key, index)
				}
				if err := validateObjectItem(fmt.Sprintf("%s[%d]", key, index), object, field.ObjectKeys); err != nil {
					return err
				}
			}
			return nil
		}
		return fmt.Errorf("fact %q must not be an array", key)
	case nil:
		if !kindAllowsNull(field.Kinds) {
			return fmt.Errorf("fact %q cannot be null", key)
		}
	default:
		return fmt.Errorf("fact %q has unsupported type", key)
	}
	return nil
}

func validateObjectItem(path string, object map[string]any, keys map[string]factFieldSpec) error {
	for key := range object {
		if _, ok := keys[key]; !ok {
			return fmt.Errorf("fact %s.%s is not allowed", path, key)
		}
	}
	for key, field := range keys {
		value, present := object[key]
		if !present || value == nil {
			if field.Required && !kindAllowsNull(field.Kinds) {
				return fmt.Errorf("fact %s.%s is required", path, key)
			}
			continue
		}
		if err := validateFactValue(path+"."+key, value, field); err != nil {
			return err
		}
	}
	return nil
}

func hasKind(kinds []factKind, want factKind) bool {
	for _, kind := range kinds {
		if kind == want {
			return true
		}
	}
	return false
}

// GenerateEvidenceRefs builds deterministic refs from operation + facts.
// Collector-supplied refs are never trusted.
func GenerateEvidenceRefs(operation string, facts map[string]any) ([]string, error) {
	if !IsAllowedEvidenceOperation(operation) {
		return nil, fmt.Errorf("unknown operation %q", operation)
	}
	prefix := "evidence." + operation
	refs := make([]string, 0, 16)
	keys := make([]string, 0, len(facts))
	for key := range facts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := facts[key]
		switch typed := value.(type) {
		case []any:
			for index, item := range typed {
				base := fmt.Sprintf("%s.%s[%d]", prefix, key, index)
				if object, ok := item.(map[string]any); ok {
					subkeys := make([]string, 0, len(object))
					for sub := range object {
						subkeys = append(subkeys, sub)
					}
					sort.Strings(subkeys)
					for _, sub := range subkeys {
						refs = append(refs, base+"."+sub)
					}
				} else {
					refs = append(refs, base)
				}
			}
		default:
			refs = append(refs, prefix+"."+key)
		}
	}
	if len(refs) == 0 {
		refs = append(refs, prefix)
	}
	if len(refs) > maxEvidenceRefCount {
		return nil, fmt.Errorf("generated evidence refs exceed %d entries", maxEvidenceRefCount)
	}
	for _, ref := range refs {
		if !strings.HasPrefix(ref, prefix) {
			return nil, fmt.Errorf("generated evidence ref escaped operation namespace")
		}
	}
	return refs, nil
}
