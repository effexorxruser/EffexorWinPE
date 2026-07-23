package agentloop

import (
	"fmt"
	"strings"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/gateway"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
)

// PrivacyPolicy is the enforceable policy attached to each privacy_class.
type PrivacyPolicy struct {
	Class                      string
	AllowedFactKeys            map[string]struct{}
	RedactedFields             []string
	UploadAllowed              bool
	RequiresAdditionalApproval bool
}

var privacyPolicies = map[string]PrivacyPolicy{
	PrivacyMachineInventory: {
		Class:           PrivacyMachineInventory,
		AllowedFactKeys: nil, // deferred to operation schema
		RedactedFields:  []string{"hostname", "username", "sid"},
		UploadAllowed:   true,
	},
	PrivacyBootConfig: {
		Class:          PrivacyBootConfig,
		RedactedFields: []string{"hostname"},
		UploadAllowed:  true,
	},
	PrivacyStorageHealth: {
		Class:          PrivacyStorageHealth,
		RedactedFields: []string{"serial_number", "hostname"},
		UploadAllowed:  true,
	},
	PrivacyEncryptionStatus: {
		Class:                      PrivacyEncryptionStatus,
		RedactedFields:             []string{"recovery_key", "password", "protector_id", "hostname"},
		UploadAllowed:              false,
		RequiresAdditionalApproval: true,
	},
	PrivacyNetworkStatus: {
		Class:          PrivacyNetworkStatus,
		RedactedFields: []string{"mac_address", "hostname", "ip_address"},
		UploadAllowed:  true,
	},
}

// PrivacyPolicyFor returns the closed policy for a privacy class.
func PrivacyPolicyFor(class string) (PrivacyPolicy, error) {
	policy, ok := privacyPolicies[class]
	if !ok {
		return PrivacyPolicy{}, fmt.Errorf("unknown privacy_class %q", class)
	}
	return policy, nil
}

// MayUploadEvidence reports whether facts from this class may enter the next
// provider round without additional technician approval.
func MayUploadEvidence(class string) bool {
	policy, err := PrivacyPolicyFor(class)
	if err != nil {
		return false
	}
	return policy.UploadAllowed && !policy.RequiresAdditionalApproval
}

// NewSanitizedAgentContext reuses gateway sanitization so hostname, session
// events, and latest assessments never reach the provider.
func NewSanitizedAgentContext(report diagnostics.Report, sess session.Session) SanitizedAgentContext {
	sanitized := gateway.SanitizeDiagnosisRequest(gateway.DiagnosisRequest{
		DiagnosticReport:   report,
		Session:            sess,
		TechnicianApproved: true,
	})
	return SanitizedAgentContext{
		Report:  sanitized.DiagnosticReport,
		Session: sanitized.Session,
	}
}

// ApplyPrivacyRedactions removes policy-listed fields from evidence facts.
func ApplyPrivacyRedactions(facts map[string]any, class string) map[string]any {
	policy, err := PrivacyPolicyFor(class)
	if err != nil || facts == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(facts))
	redacted := map[string]struct{}{}
	for _, field := range policy.RedactedFields {
		redacted[strings.ToLower(field)] = struct{}{}
	}
	for key, value := range facts {
		if _, drop := redacted[strings.ToLower(key)]; drop {
			continue
		}
		out[key] = value
	}
	return out
}

// FilterUploadableEvidence keeps only evidence that privacy policy allows into
// the next provider round.
func FilterUploadableEvidence(payloads []EvidencePayload) []EvidencePayload {
	out := make([]EvidencePayload, 0, len(payloads))
	for _, payload := range payloads {
		class := payload.PrivacyClass
		if class == "" {
			if spec, ok := evidenceOperations[payload.Operation]; ok {
				class = spec.PrivacyClass
			}
		}
		if !MayUploadEvidence(class) {
			continue
		}
		out = append(out, payload)
	}
	return out
}
