package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
)

// clearRunArtifacts archives existing diagnostic outputs so a new run cannot
// accidentally reuse stale files from a previous collector/agent execution.
func (o *Orchestrator) clearRunArtifacts() error {
	paths := []string{o.Paths.ReportPath, o.Paths.DiagnosisPath, o.Paths.SessionPath}
	archiveDir := filepath.Join(o.Paths.ReportsDir, "archive")
	stamp := time.Now().UTC().Format("20060102T150405Z")
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if info.IsDir() {
			continue
		}
		if err := os.MkdirAll(archiveDir, 0o755); err != nil {
			return err
		}
		dest := filepath.Join(archiveDir, stamp+"-"+filepath.Base(path))
		if err := os.Rename(path, dest); err != nil {
			// Fall back to remove if cross-device rename fails.
			if rmErr := os.Remove(path); rmErr != nil {
				return fmt.Errorf("archive %s: rename: %v; remove: %w", path, err, rmErr)
			}
			o.log("removed stale artifact %s (archive rename failed: %v)", path, err)
			continue
		}
		o.log("archived stale artifact %s -> %s", path, dest)
	}
	return nil
}

func fileFreshnessError(path string, notBefore time.Time) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	// Allow small clock skew / filesystem timestamp rounding.
	if info.ModTime().Before(notBefore.Add(-2 * time.Second)) {
		return fmt.Errorf("stale artifact %s: mtime %s is before run start %s", path, info.ModTime().UTC().Format(time.RFC3339Nano), notBefore.UTC().Format(time.RFC3339Nano))
	}
	return nil
}

func decodeStrictJSONObject(raw []byte, dest any) error {
	trim := strings.TrimSpace(string(raw))
	if trim == "" {
		return fmt.Errorf("empty json")
	}
	if trim == "{}" {
		return fmt.Errorf("empty json object")
	}
	dec := json.NewDecoder(strings.NewReader(trim))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		// Retry without DisallowUnknownFields for forward-compatible assessment/session fields,
		// but keep the empty-object rejection above.
		if err2 := json.Unmarshal(raw, dest); err2 != nil {
			return err2
		}
	}
	return nil
}

func validateAssessment(assessment diagnosis.Assessment, reportID string) error {
	if strings.TrimSpace(assessment.SchemaVersion) == "" &&
		strings.TrimSpace(assessment.ReportID) == "" &&
		assessment.GeneratedAt.IsZero() &&
		strings.TrimSpace(assessment.Mode) == "" {
		return fmt.Errorf("empty diagnosis object")
	}
	if assessment.SchemaVersion != diagnosis.SchemaVersion {
		return fmt.Errorf("unsupported diagnosis schema %q; expected %q", assessment.SchemaVersion, diagnosis.SchemaVersion)
	}
	if assessment.ReportID != reportID {
		return fmt.Errorf("diagnosis report_id %q does not match report %q", assessment.ReportID, reportID)
	}
	if assessment.Mode != diagnosis.ModeOfflinePreflight {
		return fmt.Errorf("unexpected diagnosis mode %q; expected %q", assessment.Mode, diagnosis.ModeOfflinePreflight)
	}
	if assessment.GeneratedAt.IsZero() {
		return fmt.Errorf("diagnosis generated_at is zero")
	}
	return nil
}

func validateSession(sess session.Session, reportID string) error {
	if strings.TrimSpace(sess.SchemaVersion) == "" &&
		strings.TrimSpace(sess.SessionID) == "" &&
		strings.TrimSpace(sess.ReportID) == "" {
		return fmt.Errorf("empty session object")
	}
	if err := sess.Validate(reportID); err != nil {
		return err
	}
	if sess.LatestAssessment != nil {
		if sess.LatestAssessment.ReportID != reportID {
			return fmt.Errorf("session latest_assessment report_id %q does not match report %q", sess.LatestAssessment.ReportID, reportID)
		}
		if sess.LatestAssessment.SchemaVersion != "" && sess.LatestAssessment.SchemaVersion != diagnosis.SchemaVersion {
			return fmt.Errorf("session latest_assessment schema %q; expected %q", sess.LatestAssessment.SchemaVersion, diagnosis.SchemaVersion)
		}
	}
	return nil
}

func validateLoadedReport(report diagnostics.Report) error {
	if strings.TrimSpace(report.ReportID) == "" {
		return fmt.Errorf("report_id is required")
	}
	if report.SchemaVersion != diagnostics.SchemaVersion {
		return fmt.Errorf("unsupported diagnostic schema %q", report.SchemaVersion)
	}
	if report.CollectedAt.IsZero() {
		return fmt.Errorf("collected_at is zero")
	}
	return nil
}
