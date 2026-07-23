package orchestrator

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/journal"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/mock"
)

func TestClearRunArtifactsArchivesStaleFiles(t *testing.T) {
	dir := t.TempDir()
	report := filepath.Join(dir, "initial.json")
	diag := filepath.Join(dir, "initial-diagnosis.json")
	sess := filepath.Join(dir, "initial-diagnosis-session.json")
	for _, p := range []string{report, diag, sess} {
		if err := os.WriteFile(p, []byte(`{"old":true}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	o := &Orchestrator{Paths: Paths{ReportsDir: dir, ReportPath: report, DiagnosisPath: diag, SessionPath: sess}, Journal: journal.New("")}
	if err := o.clearRunArtifacts(); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{report, diag, sess} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("expected %s removed, err=%v", p, err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, "archive"))
	if err != nil || len(entries) != 3 {
		t.Fatalf("archive entries=%v err=%v", entries, err)
	}
}

func TestStaleReportRejected(t *testing.T) {
	dir := t.TempDir()
	collector := filepath.Join(dir, "collector.exe")
	agent := filepath.Join(dir, "agent.exe")
	for _, p := range []string{collector, agent} {
		if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	reportPath := filepath.Join(dir, "reports", "initial.json")
	_ = os.MkdirAll(filepath.Dir(reportPath), 0o755)
	o := &Orchestrator{
		Paths: Paths{
			CollectorExe: collector, AgentExe: agent,
			ReportsDir: filepath.Join(dir, "reports"), ReportPath: reportPath,
			DiagnosisPath: filepath.Join(dir, "reports", "d.json"),
			SessionPath:   filepath.Join(dir, "reports", "s.json"),
			Timeout:       time.Second,
		},
		Runner: fakeRunner{fn: func(ctx context.Context, name string, args []string, stdout, stderr io.Writer) error {
			if err := os.WriteFile(reportPath, mock.ReportJSON(), 0o644); err != nil {
				return err
			}
			past := time.Now().Add(-time.Hour)
			return os.Chtimes(reportPath, past, past)
		}},
	}
	res := o.RunCollection(context.Background(), nil)
	if res.Code != ExitStaleArtifact || res.FriendlyKey != "msg.stale_artifact" {
		t.Fatalf("result=%+v", res)
	}
}

func TestStaleDiagnosisRejected(t *testing.T) {
	dir := t.TempDir()
	collector := filepath.Join(dir, "c.exe")
	agent := filepath.Join(dir, "a.exe")
	for _, p := range []string{collector, agent} {
		_ = os.WriteFile(p, []byte("x"), 0o755)
	}
	reportPath := filepath.Join(dir, "reports", "initial.json")
	diagPath := filepath.Join(dir, "reports", "diagnosis.json")
	_ = os.MkdirAll(filepath.Dir(reportPath), 0o755)
	o := &Orchestrator{
		Paths: Paths{
			CollectorExe: collector, AgentExe: agent,
			ReportsDir: filepath.Join(dir, "reports"), ReportPath: reportPath,
			DiagnosisPath: diagPath, SessionPath: filepath.Join(dir, "reports", "s.json"),
			Timeout: time.Second,
		},
		Runner: fakeRunner{fn: func(ctx context.Context, name string, args []string, stdout, stderr io.Writer) error {
			if name == collector {
				return os.WriteFile(reportPath, mock.ReportJSON(), 0o644)
			}
			if err := os.WriteFile(diagPath, mock.DiagnosisJSON(), 0o644); err != nil {
				return err
			}
			past := time.Now().Add(-time.Hour)
			return os.Chtimes(diagPath, past, past)
		}},
	}
	res := o.RunCollection(context.Background(), nil)
	if res.Code != ExitStaleArtifact {
		t.Fatalf("result=%+v", res)
	}
}

func TestStaleSessionRejected(t *testing.T) {
	dir := t.TempDir()
	collector := filepath.Join(dir, "c.exe")
	agent := filepath.Join(dir, "a.exe")
	for _, p := range []string{collector, agent} {
		_ = os.WriteFile(p, []byte("x"), 0o755)
	}
	reportPath := filepath.Join(dir, "reports", "initial.json")
	diagPath := filepath.Join(dir, "reports", "diagnosis.json")
	sessPath := filepath.Join(dir, "reports", "session.json")
	_ = os.MkdirAll(filepath.Dir(reportPath), 0o755)
	o := &Orchestrator{
		Paths: Paths{
			CollectorExe: collector, AgentExe: agent,
			ReportsDir: filepath.Join(dir, "reports"), ReportPath: reportPath,
			DiagnosisPath: diagPath, SessionPath: sessPath, Timeout: time.Second,
		},
		Runner: fakeRunner{fn: func(ctx context.Context, name string, args []string, stdout, stderr io.Writer) error {
			if name == collector {
				return os.WriteFile(reportPath, mock.ReportJSON(), 0o644)
			}
			if err := os.WriteFile(diagPath, mock.DiagnosisJSON(), 0o644); err != nil {
				return err
			}
			if err := os.WriteFile(sessPath, mock.SessionJSON(), 0o644); err != nil {
				return err
			}
			past := time.Now().Add(-time.Hour)
			return os.Chtimes(sessPath, past, past)
		}},
	}
	res := o.RunCollection(context.Background(), nil)
	if res.Code != ExitStaleArtifact {
		t.Fatalf("result=%+v", res)
	}
}

func TestDiagnosisForeignReportID(t *testing.T) {
	a := diagnosis.Assessment{
		SchemaVersion: diagnosis.SchemaVersion,
		ReportID:      "other-report",
		GeneratedAt:   time.Now().UTC(),
		Mode:          diagnosis.ModeOfflinePreflight,
	}
	if err := validateAssessment(a, "mockreport00000001"); err == nil {
		t.Fatal("expected foreign report_id error")
	}
}

func TestSessionForeignReportID(t *testing.T) {
	s := session.Session{
		SchemaVersion: session.SchemaVersion,
		SessionID:     "session-1",
		ReportID:      "other",
	}
	if err := validateSession(s, "mockreport00000001"); err == nil {
		t.Fatal("expected foreign report_id error")
	}
}

func TestEmptyJSONObjectCorrupt(t *testing.T) {
	var a diagnosis.Assessment
	if err := decodeStrictJSONObject([]byte(`{}`), &a); err == nil {
		t.Fatal("expected empty object rejection")
	}
	var s session.Session
	if err := decodeStrictJSONObject([]byte(`{}`), &s); err == nil {
		t.Fatal("expected empty session rejection")
	}
}

func TestWrongDiagnosisSchemaVersion(t *testing.T) {
	a := diagnosis.Assessment{
		SchemaVersion: "9.9.9",
		ReportID:      "mockreport00000001",
		GeneratedAt:   time.Now().UTC(),
		Mode:          diagnosis.ModeOfflinePreflight,
	}
	if err := validateAssessment(a, "mockreport00000001"); err == nil {
		t.Fatal("expected schema error")
	}
}

func TestValidateAssessmentRequiresFields(t *testing.T) {
	a := diagnosis.Assessment{
		SchemaVersion: diagnosis.SchemaVersion,
		ReportID:      "mockreport00000001",
		Mode:          diagnosis.ModeOfflinePreflight,
	}
	if err := validateAssessment(a, "mockreport00000001"); err == nil {
		t.Fatal("expected zero generated_at error")
	}
	a.GeneratedAt = time.Now().UTC()
	a.Mode = "online_agent"
	if err := validateAssessment(a, "mockreport00000001"); err == nil {
		t.Fatal("expected mode error")
	}
}

func TestSessionLatestAssessmentForeignReportID(t *testing.T) {
	s := session.Session{
		SchemaVersion: session.SchemaVersion,
		SessionID:     "session-1",
		ReportID:      "mockreport00000001",
		LatestAssessment: &diagnosis.Assessment{
			SchemaVersion: diagnosis.SchemaVersion,
			ReportID:      "other",
		},
	}
	if err := validateSession(s, "mockreport00000001"); err == nil {
		t.Fatal("expected latest_assessment mismatch")
	}
}

func TestSuccessfulRunStillOK(t *testing.T) {
	dir := t.TempDir()
	collector := filepath.Join(dir, "collector.exe")
	agent := filepath.Join(dir, "agent.exe")
	for _, p := range []string{collector, agent} {
		if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	reportPath := filepath.Join(dir, "reports", "initial.json")
	diagPath := filepath.Join(dir, "reports", "diagnosis.json")
	sessPath := filepath.Join(dir, "reports", "session.json")
	// Plant stale files that must be archived away.
	_ = os.MkdirAll(filepath.Dir(reportPath), 0o755)
	_ = os.WriteFile(reportPath, []byte(`{"schema_version":"1.2.0"}`), 0o644)

	o := &Orchestrator{
		Paths: Paths{
			CollectorExe: collector, AgentExe: agent,
			ReportsDir: filepath.Join(dir, "reports"), ReportPath: reportPath,
			DiagnosisPath: diagPath, SessionPath: sessPath, Timeout: time.Second,
		},
		Runner: fakeRunner{fn: func(ctx context.Context, name string, args []string, stdout, stderr io.Writer) error {
			switch name {
			case collector:
				return os.WriteFile(reportPath, mock.ReportJSON(), 0o644)
			case agent:
				if err := os.WriteFile(diagPath, mock.DiagnosisJSON(), 0o644); err != nil {
					return err
				}
				return os.WriteFile(sessPath, mock.SessionJSON(), 0o644)
			default:
				return errors.New("unexpected")
			}
		}},
		Journal: journal.New(filepath.Join(dir, "j.log")),
	}
	res := o.RunCollection(context.Background(), nil)
	if res.Code != ExitOK {
		t.Fatalf("result=%+v", res)
	}
}
