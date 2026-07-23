package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/adapter"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/journal"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/viewmodel"
)

// ExitCode classifies subprocess outcomes for the UI.
type ExitCode int

const (
	ExitOK ExitCode = iota
	ExitMissingCollector
	ExitMissingAgent
	ExitCorruptReport
	ExitUnsupportedSchema
	ExitNonZero
	ExitTimeout
	ExitFailedStart
	ExitMissingDiagnosis
	ExitCorruptDiagnosis
	ExitMissingSession
	ExitCorruptSession
)

// Result is the outcome of a diagnostics run.
type Result struct {
	Code          ExitCode
	FriendlyKey   string
	Detail        string
	Report        *diagnostics.Report
	Assessment    *diagnosis.Assessment
	Session       *session.Session
	Model         viewmodel.AppModel
	ReportPath    string
	DiagnosisPath string
	SessionPath   string
}

// Paths configures artifact locations and tool binaries.
type Paths struct {
	CollectorExe  string
	AgentExe      string
	ReportsDir    string
	ReportPath    string
	DiagnosisPath string
	SessionPath   string
	Timeout       time.Duration
}

// Runner executes external processes (overridable in tests).
type Runner interface {
	Run(ctx context.Context, name string, args []string, stdout, stderr io.Writer) error
}

// ExecRunner uses os/exec.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args []string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// Orchestrator coordinates collector and agent.
type Orchestrator struct {
	Paths   Paths
	Runner  Runner
	Journal *journal.Journal
}

// DefaultPaths resolves standard WinPE / sidecar layout.
func DefaultPaths(baseDir string) Paths {
	if baseDir == "" {
		exe, err := os.Executable()
		if err == nil {
			baseDir = filepath.Dir(exe)
		} else {
			baseDir = "."
		}
	}
	binDir := baseDir
	root := filepath.Dir(baseDir)
	// Prefer X:\EffexorWinPE layout when bin is .../bin
	reportsDir := filepath.Join(root, "reports")
	if filepath.Base(baseDir) != "bin" {
		reportsDir = filepath.Join(baseDir, "reports")
	}
	return Paths{
		CollectorExe:  filepath.Join(binDir, "effexorwinpe-collector.exe"),
		AgentExe:      filepath.Join(binDir, "effexorwinpe-agent.exe"),
		ReportsDir:    reportsDir,
		ReportPath:    filepath.Join(reportsDir, "initial.json"),
		DiagnosisPath: filepath.Join(reportsDir, "initial-diagnosis.json"),
		SessionPath:   filepath.Join(reportsDir, "initial-diagnosis-session.json"),
		Timeout:       10 * time.Minute,
	}
}

// ProgressFunc receives progress updates during a run.
type ProgressFunc func(viewmodel.ProgressScreen)

// RunCollection executes collector then agent and loads results.
func (o *Orchestrator) RunCollection(ctx context.Context, onProgress ProgressFunc) Result {
	if o.Runner == nil {
		o.Runner = ExecRunner{}
	}
	timeout := o.Paths.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	notify := func(p viewmodel.ProgressScreen) {
		if onProgress != nil {
			onProgress(p)
		}
	}

	if _, err := os.Stat(o.Paths.CollectorExe); err != nil {
		o.log("collector missing: %v", err)
		return Result{Code: ExitMissingCollector, FriendlyKey: "msg.collector_missing", Detail: err.Error()}
	}
	if _, err := os.Stat(o.Paths.AgentExe); err != nil {
		o.log("agent missing: %v", err)
		return Result{Code: ExitMissingAgent, FriendlyKey: "msg.agent_missing", Detail: err.Error()}
	}
	if err := os.MkdirAll(o.Paths.ReportsDir, 0o755); err != nil {
		o.log("reports dir: %v", err)
		return Result{Code: ExitFailedStart, FriendlyKey: "msg.process_failed", Detail: err.Error()}
	}

	notify(viewmodel.ProgressScreen{Phase: "collector", StatusKey: "status.running", Percent: 10, Detail: "msg.collection_running"})
	var outBuf, errBuf bytes.Buffer
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	stopPulse := pulseProgress(cctx, notify, "collector", "msg.collection_running", 10, 55)
	err := o.Runner.Run(cctx, o.Paths.CollectorExe, []string{"--output", o.Paths.ReportPath, "--pretty=true"}, &outBuf, &errBuf)
	stopPulse()
	o.logProcess("collector", outBuf.String(), errBuf.String(), err)
	if errors.Is(cctx.Err(), context.DeadlineExceeded) {
		return Result{Code: ExitTimeout, FriendlyKey: "msg.process_hung", Detail: errBuf.String(), ReportPath: o.Paths.ReportPath}
	}
	if err != nil {
		return Result{
			Code:        mapExecError(err),
			FriendlyKey: "msg.process_failed",
			Detail:      err.Error() + "\n" + errBuf.String(),
			ReportPath:  o.Paths.ReportPath,
		}
	}

	reportBytes, err := os.ReadFile(o.Paths.ReportPath)
	if err != nil {
		return Result{Code: ExitCorruptReport, FriendlyKey: "msg.report_corrupt", Detail: err.Error(), ReportPath: o.Paths.ReportPath}
	}
	report, err := diagnostics.DecodeReportJSON(reportBytes)
	if err != nil {
		key := "msg.report_corrupt"
		code := ExitCorruptReport
		if isUnsupportedSchema(err) {
			key = "msg.schema_unsupported"
			code = ExitUnsupportedSchema
		}
		return Result{Code: code, FriendlyKey: key, Detail: err.Error(), ReportPath: o.Paths.ReportPath}
	}

	notify(viewmodel.ProgressScreen{Phase: "agent", StatusKey: "status.running", Percent: 60, Detail: "msg.agent_running"})
	outBuf.Reset()
	errBuf.Reset()
	actx, acancel := context.WithTimeout(ctx, timeout)
	defer acancel()
	stopPulse = pulseProgress(actx, notify, "agent", "msg.agent_running", 60, 95)
	err = o.Runner.Run(actx, o.Paths.AgentExe, []string{
		"--input", o.Paths.ReportPath,
		"--output", o.Paths.DiagnosisPath,
		"--session", o.Paths.SessionPath,
		"--pretty=true",
	}, &outBuf, &errBuf)
	stopPulse()
	o.logProcess("agent", outBuf.String(), errBuf.String(), err)
	if errors.Is(actx.Err(), context.DeadlineExceeded) {
		model := adapter.FromReport(report, false)
		return Result{
			Code: ExitTimeout, FriendlyKey: "msg.process_hung", Detail: errBuf.String(),
			Report: &report, Model: model, ReportPath: o.Paths.ReportPath,
		}
	}
	if err != nil {
		model := adapter.FromReport(report, false)
		return Result{
			Code: mapExecError(err), FriendlyKey: "msg.process_failed", Detail: err.Error() + "\n" + errBuf.String(),
			Report: &report, Model: model, ReportPath: o.Paths.ReportPath,
		}
	}

	model := adapter.FromReport(report, false)
	result := Result{
		Report:        &report,
		Model:         model,
		ReportPath:    o.Paths.ReportPath,
		DiagnosisPath: o.Paths.DiagnosisPath,
		SessionPath:   o.Paths.SessionPath,
	}
	result.Model.Export.ReportPath = o.Paths.ReportPath
	result.Model.Export.DiagnosisPath = o.Paths.DiagnosisPath
	result.Model.Export.SessionPath = o.Paths.SessionPath
	if o.Journal != nil {
		result.Model.Export.JournalPath = o.Journal.Path()
		result.Model.Journal.Entries = o.Journal.Entries()
	}

	diagBytes, err := os.ReadFile(o.Paths.DiagnosisPath)
	if err != nil {
		o.log("diagnosis missing: %v", err)
		result.Code = ExitMissingDiagnosis
		result.FriendlyKey = "msg.diagnosis_missing"
		result.Detail = err.Error()
		result.Model.Progress = viewmodel.ProgressScreen{
			Phase: "failed", StatusKey: "status.failed", Percent: 100,
			Detail: result.FriendlyKey, FriendlyError: result.FriendlyKey, ShowJournalHint: true,
		}
		notify(result.Model.Progress)
		return result
	}
	var assessment diagnosis.Assessment
	if err := decodeJSON(diagBytes, &assessment); err != nil {
		o.log("diagnosis corrupt: %v", err)
		result.Code = ExitCorruptDiagnosis
		result.FriendlyKey = "msg.diagnosis_corrupt"
		result.Detail = err.Error()
		result.Model.Progress = viewmodel.ProgressScreen{
			Phase: "failed", StatusKey: "status.failed", Percent: 100,
			Detail: result.FriendlyKey, FriendlyError: result.FriendlyKey, ShowJournalHint: true,
		}
		notify(result.Model.Progress)
		return result
	}
	result.Assessment = &assessment

	sessBytes, err := os.ReadFile(o.Paths.SessionPath)
	if err != nil {
		o.log("session missing: %v", err)
		adapter.ApplyAssessment(&result.Model, assessment, nil)
		result.Code = ExitMissingSession
		result.FriendlyKey = "msg.session_missing"
		result.Detail = err.Error()
		result.Model.Progress = viewmodel.ProgressScreen{
			Phase: "failed", StatusKey: "status.failed", Percent: 100,
			Detail: result.FriendlyKey, FriendlyError: result.FriendlyKey, ShowJournalHint: true,
		}
		notify(result.Model.Progress)
		return result
	}
	var sess session.Session
	if err := decodeJSON(sessBytes, &sess); err != nil {
		o.log("session corrupt: %v", err)
		adapter.ApplyAssessment(&result.Model, assessment, nil)
		result.Code = ExitCorruptSession
		result.FriendlyKey = "msg.session_corrupt"
		result.Detail = err.Error()
		result.Model.Progress = viewmodel.ProgressScreen{
			Phase: "failed", StatusKey: "status.failed", Percent: 100,
			Detail: result.FriendlyKey, FriendlyError: result.FriendlyKey, ShowJournalHint: true,
		}
		notify(result.Model.Progress)
		return result
	}
	result.Session = &sess
	adapter.ApplyAssessment(&result.Model, assessment, &sess)

	result.Code = ExitOK
	result.FriendlyKey = "msg.collection_done"
	result.Model.Progress = viewmodel.ProgressScreen{
		Phase: "done", StatusKey: "status.succeeded", Percent: 100, Detail: "msg.collection_done",
	}
	notify(result.Model.Progress)
	return result
}

// LoadReportFile loads an existing report into a model.
func LoadReportFile(path string) (viewmodel.AppModel, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return viewmodel.AppModel{}, err
	}
	report, err := diagnostics.DecodeReportJSON(raw)
	if err != nil {
		return viewmodel.AppModel{}, err
	}
	model := adapter.FromReport(report, false)
	model.Export.ReportPath = path
	return model, nil
}

func (o *Orchestrator) log(format string, args ...any) {
	if o.Journal != nil {
		o.Journal.Append(format, args...)
	}
}

func (o *Orchestrator) logProcess(name, stdout, stderr string, err error) {
	o.log("%s finished err=%v", name, err)
	if stdout != "" {
		o.log("%s stdout:\n%s", name, stdout)
	}
	if stderr != "" {
		o.log("%s stderr:\n%s", name, stderr)
	}
}

func mapExecError(err error) ExitCode {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return ExitNonZero
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ExitTimeout
	}
	return ExitFailedStart
}

func isUnsupportedSchema(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "unsupported diagnostic schema")
}

func decodeJSON(raw []byte, dest any) error {
	return json.Unmarshal(raw, dest)
}

// pulseProgress emits intermediate progress while a subprocess runs.
func pulseProgress(ctx context.Context, notify ProgressFunc, phase, detail string, from, to int) func() {
	if notify == nil {
		return func() {}
	}
	done := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(400 * time.Millisecond)
		defer ticker.Stop()
		cur := from
		notify(viewmodel.ProgressScreen{Phase: phase, StatusKey: "status.running", Percent: cur, Detail: detail})
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if cur >= to {
					continue
				}
				cur++
				if (cur-from)%3 == 0 || cur == to {
					notify(viewmodel.ProgressScreen{Phase: phase, StatusKey: "status.running", Percent: cur, Detail: detail})
				}
			}
		}
	}()
	return func() { once.Do(func() { close(done) }) }
}
