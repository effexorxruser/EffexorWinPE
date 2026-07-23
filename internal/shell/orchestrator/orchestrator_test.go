package orchestrator

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/shell/journal"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/mock"
)

type fakeRunner struct {
	fn func(ctx context.Context, name string, args []string, stdout, stderr io.Writer) error
}

func (f fakeRunner) Run(ctx context.Context, name string, args []string, stdout, stderr io.Writer) error {
	return f.fn(ctx, name, args, stdout, stderr)
}

func TestMissingCollectorMapping(t *testing.T) {
	dir := t.TempDir()
	o := &Orchestrator{
		Paths: Paths{
			CollectorExe: filepath.Join(dir, "missing-collector.exe"),
			AgentExe:     filepath.Join(dir, "missing-agent.exe"),
			ReportsDir:   filepath.Join(dir, "reports"),
			Timeout:      time.Second,
		},
		Journal: journal.New(""),
	}
	res := o.RunCollection(context.Background(), nil)
	if res.Code != ExitMissingCollector || res.FriendlyKey != "msg.collector_missing" {
		t.Fatalf("result = %+v", res)
	}
}

func TestMissingAgentMapping(t *testing.T) {
	dir := t.TempDir()
	collector := filepath.Join(dir, "effexorwinpe-collector.exe")
	if err := os.WriteFile(collector, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	o := &Orchestrator{
		Paths: Paths{
			CollectorExe: collector,
			AgentExe:     filepath.Join(dir, "missing-agent.exe"),
			ReportsDir:   filepath.Join(dir, "reports"),
			Timeout:      time.Second,
		},
	}
	res := o.RunCollection(context.Background(), nil)
	if res.Code != ExitMissingAgent || res.FriendlyKey != "msg.agent_missing" {
		t.Fatalf("result = %+v", res)
	}
}

func TestNonZeroExitMapping(t *testing.T) {
	dir := t.TempDir()
	collector := filepath.Join(dir, "collector.exe")
	agent := filepath.Join(dir, "agent.exe")
	for _, p := range []string{collector, agent} {
		if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	o := &Orchestrator{
		Paths: Paths{
			CollectorExe:  collector,
			AgentExe:      agent,
			ReportsDir:    filepath.Join(dir, "reports"),
			ReportPath:    filepath.Join(dir, "reports", "initial.json"),
			DiagnosisPath: filepath.Join(dir, "reports", "diagnosis.json"),
			SessionPath:   filepath.Join(dir, "reports", "session.json"),
			Timeout:       time.Second,
		},
		Runner: fakeRunner{fn: func(ctx context.Context, name string, args []string, stdout, stderr io.Writer) error {
			_, _ = stderr.Write([]byte("collector boom"))
			return &exec.ExitError{}
		}},
		Journal: journal.New(filepath.Join(dir, "journal.log")),
	}
	res := o.RunCollection(context.Background(), nil)
	if res.Code != ExitNonZero || res.FriendlyKey != "msg.process_failed" {
		t.Fatalf("result = %+v", res)
	}
}

func TestTimeoutMapping(t *testing.T) {
	dir := t.TempDir()
	collector := filepath.Join(dir, "collector.exe")
	agent := filepath.Join(dir, "agent.exe")
	for _, p := range []string{collector, agent} {
		if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	o := &Orchestrator{
		Paths: Paths{
			CollectorExe:  collector,
			AgentExe:      agent,
			ReportsDir:    filepath.Join(dir, "reports"),
			ReportPath:    filepath.Join(dir, "reports", "initial.json"),
			DiagnosisPath: filepath.Join(dir, "reports", "diagnosis.json"),
			SessionPath:   filepath.Join(dir, "reports", "session.json"),
			Timeout:       20 * time.Millisecond,
		},
		Runner: fakeRunner{fn: func(ctx context.Context, name string, args []string, stdout, stderr io.Writer) error {
			<-ctx.Done()
			return ctx.Err()
		}},
	}
	res := o.RunCollection(context.Background(), nil)
	if res.Code != ExitTimeout || res.FriendlyKey != "msg.process_hung" {
		t.Fatalf("result = %+v", res)
	}
}

func TestSuccessfulRunLoadsMockArtifacts(t *testing.T) {
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
	o := &Orchestrator{
		Paths: Paths{
			CollectorExe:  collector,
			AgentExe:      agent,
			ReportsDir:    filepath.Join(dir, "reports"),
			ReportPath:    reportPath,
			DiagnosisPath: diagPath,
			SessionPath:   sessPath,
			Timeout:       time.Second,
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
				return errors.New("unexpected binary")
			}
		}},
		Journal: journal.New(filepath.Join(dir, "j.log")),
	}
	res := o.RunCollection(context.Background(), nil)
	if res.Code != ExitOK {
		t.Fatalf("result = %+v", res)
	}
	if res.Report == nil || res.Assessment == nil || res.Session == nil {
		t.Fatalf("missing loaded artifacts: %+v", res)
	}
	if !res.Model.Overview.HasReport || !res.Model.Agent.HasAssessment {
		t.Fatalf("model incomplete: %+v", res.Model)
	}
}

func TestMissingDiagnosisMapping(t *testing.T) {
	dir := t.TempDir()
	collector := filepath.Join(dir, "collector.exe")
	agent := filepath.Join(dir, "agent.exe")
	for _, p := range []string{collector, agent} {
		if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	reportPath := filepath.Join(dir, "reports", "initial.json")
	o := &Orchestrator{
		Paths: Paths{
			CollectorExe:  collector,
			AgentExe:      agent,
			ReportsDir:    filepath.Join(dir, "reports"),
			ReportPath:    reportPath,
			DiagnosisPath: filepath.Join(dir, "reports", "diagnosis.json"),
			SessionPath:   filepath.Join(dir, "reports", "session.json"),
			Timeout:       time.Second,
		},
		Runner: fakeRunner{fn: func(ctx context.Context, name string, args []string, stdout, stderr io.Writer) error {
			if name == collector {
				return os.WriteFile(reportPath, mock.ReportJSON(), 0o644)
			}
			return nil // agent "succeeds" but writes nothing
		}},
	}
	res := o.RunCollection(context.Background(), nil)
	if res.Code != ExitMissingDiagnosis || res.FriendlyKey != "msg.diagnosis_missing" {
		t.Fatalf("result = %+v", res)
	}
}

func TestCorruptDiagnosisMapping(t *testing.T) {
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
	o := &Orchestrator{
		Paths: Paths{
			CollectorExe:  collector,
			AgentExe:      agent,
			ReportsDir:    filepath.Join(dir, "reports"),
			ReportPath:    reportPath,
			DiagnosisPath: diagPath,
			SessionPath:   filepath.Join(dir, "reports", "session.json"),
			Timeout:       time.Second,
		},
		Runner: fakeRunner{fn: func(ctx context.Context, name string, args []string, stdout, stderr io.Writer) error {
			if name == collector {
				return os.WriteFile(reportPath, mock.ReportJSON(), 0o644)
			}
			return os.WriteFile(diagPath, []byte(`{"not":"valid-diagnosis`), 0o644)
		}},
	}
	res := o.RunCollection(context.Background(), nil)
	if res.Code != ExitCorruptDiagnosis || res.FriendlyKey != "msg.diagnosis_corrupt" {
		t.Fatalf("result = %+v", res)
	}
}

func TestMissingSessionMapping(t *testing.T) {
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
	o := &Orchestrator{
		Paths: Paths{
			CollectorExe:  collector,
			AgentExe:      agent,
			ReportsDir:    filepath.Join(dir, "reports"),
			ReportPath:    reportPath,
			DiagnosisPath: diagPath,
			SessionPath:   filepath.Join(dir, "reports", "session.json"),
			Timeout:       time.Second,
		},
		Runner: fakeRunner{fn: func(ctx context.Context, name string, args []string, stdout, stderr io.Writer) error {
			if name == collector {
				return os.WriteFile(reportPath, mock.ReportJSON(), 0o644)
			}
			return os.WriteFile(diagPath, mock.DiagnosisJSON(), 0o644)
		}},
	}
	res := o.RunCollection(context.Background(), nil)
	if res.Code != ExitMissingSession || res.FriendlyKey != "msg.session_missing" {
		t.Fatalf("result = %+v", res)
	}
}

func TestCorruptSessionMapping(t *testing.T) {
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
	o := &Orchestrator{
		Paths: Paths{
			CollectorExe:  collector,
			AgentExe:      agent,
			ReportsDir:    filepath.Join(dir, "reports"),
			ReportPath:    reportPath,
			DiagnosisPath: diagPath,
			SessionPath:   sessPath,
			Timeout:       time.Second,
		},
		Runner: fakeRunner{fn: func(ctx context.Context, name string, args []string, stdout, stderr io.Writer) error {
			if name == collector {
				return os.WriteFile(reportPath, mock.ReportJSON(), 0o644)
			}
			if err := os.WriteFile(diagPath, mock.DiagnosisJSON(), 0o644); err != nil {
				return err
			}
			return os.WriteFile(sessPath, []byte(`{`), 0o644)
		}},
	}
	res := o.RunCollection(context.Background(), nil)
	if res.Code != ExitCorruptSession || res.FriendlyKey != "msg.session_corrupt" {
		t.Fatalf("result = %+v", res)
	}
}

func TestCorruptReportMapping(t *testing.T) {
	dir := t.TempDir()
	collector := filepath.Join(dir, "collector.exe")
	agent := filepath.Join(dir, "agent.exe")
	for _, p := range []string{collector, agent} {
		if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	reportPath := filepath.Join(dir, "reports", "initial.json")
	o := &Orchestrator{
		Paths: Paths{
			CollectorExe:  collector,
			AgentExe:      agent,
			ReportsDir:    filepath.Join(dir, "reports"),
			ReportPath:    reportPath,
			DiagnosisPath: filepath.Join(dir, "reports", "d.json"),
			SessionPath:   filepath.Join(dir, "reports", "s.json"),
			Timeout:       time.Second,
		},
		Runner: fakeRunner{fn: func(ctx context.Context, name string, args []string, stdout, stderr io.Writer) error {
			return os.WriteFile(reportPath, []byte(`{"schema_version":"9.9.9"}`), 0o644)
		}},
	}
	res := o.RunCollection(context.Background(), nil)
	if res.Code != ExitUnsupportedSchema || res.FriendlyKey != "msg.schema_unsupported" {
		t.Fatalf("result = %+v", res)
	}
}
