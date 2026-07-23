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
	ExitStaleArtifact
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
	// Now is an optional clock override for tests.
	Now func() time.Time
}

func (o *Orchestrator) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
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
	if err := o.clearRunArtifacts(); err != nil {
		o.log("clear artifacts: %v", err)
		return Result{Code: ExitFailedStart, FriendlyKey: "msg.process_failed", Detail: err.Error()}
	}

	runStarted := o.now()

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

	if err := fileFreshnessError(o.Paths.ReportPath, runStarted); err != nil {
		o.log("stale report: %v", err)
		return Result{Code: ExitStaleArtifact, FriendlyKey: "msg.stale_artifact", Detail: err.Error(), ReportPath: o.Paths.ReportPath}
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
	if err := validateLoadedReport(report); err != nil {
		return Result{Code: ExitCorruptReport, FriendlyKey: "msg.report_corrupt", Detail: err.Error(), ReportPath: o.Paths.ReportPath}
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
	model := adapter.FromReport(report, false)
	if errors.Is(actx.Err(), context.DeadlineExceeded) {
		return Result{
			Code: ExitTimeout, FriendlyKey: "msg.process_hung", Detail: errBuf.String(),
			Report: &report, Model: model, ReportPath: o.Paths.ReportPath,
		}
	}
	if err != nil {
		return Result{
			Code: mapExecError(err), FriendlyKey: "msg.process_failed", Detail: err.Error() + "\n" + errBuf.String(),
			Report: &report, Model: model, ReportPath: o.Paths.ReportPath,
		}
	}

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

	fail := func(code ExitCode, key, detail string) Result {
		result.Code = code
		result.FriendlyKey = key
		result.Detail = detail
		result.Model.Progress = viewmodel.ProgressScreen{
			Phase: "failed", StatusKey: "status.failed", Percent: 100,
			Detail: key, FriendlyError: key, ShowJournalHint: true,
		}
		notify(result.Model.Progress)
		return result
	}

	if err := fileFreshnessError(o.Paths.DiagnosisPath, runStarted); err != nil {
		if os.IsNotExist(err) {
			o.log("diagnosis missing: %v", err)
			return fail(ExitMissingDiagnosis, "msg.diagnosis_missing", err.Error())
		}
		o.log("stale diagnosis: %v", err)
		return fail(ExitStaleArtifact, "msg.stale_artifact", err.Error())
	}
	diagBytes, err := os.ReadFile(o.Paths.DiagnosisPath)
	if err != nil {
		o.log("diagnosis missing: %v", err)
		return fail(ExitMissingDiagnosis, "msg.diagnosis_missing", err.Error())
	}
	var assessment diagnosis.Assessment
	if err := decodeStrictJSONObject(diagBytes, &assessment); err != nil {
		o.log("diagnosis corrupt: %v", err)
		return fail(ExitCorruptDiagnosis, "msg.diagnosis_corrupt", err.Error())
	}
	if err := validateAssessment(assessment, report.ReportID); err != nil {
		o.log("diagnosis invalid: %v", err)
		code := ExitCorruptDiagnosis
		key := "msg.diagnosis_corrupt"
		if strings.Contains(err.Error(), "unsupported diagnosis schema") {
			key = "msg.schema_unsupported"
			code = ExitUnsupportedSchema
		}
		return fail(code, key, err.Error())
	}
	result.Assessment = &assessment

	if err := fileFreshnessError(o.Paths.SessionPath, runStarted); err != nil {
		if os.IsNotExist(err) {
			o.log("session missing: %v", err)
			adapter.ApplyAssessment(&result.Model, assessment, nil)
			return fail(ExitMissingSession, "msg.session_missing", err.Error())
		}
		o.log("stale session: %v", err)
		adapter.ApplyAssessment(&result.Model, assessment, nil)
		return fail(ExitStaleArtifact, "msg.stale_artifact", err.Error())
	}
	sessBytes, err := os.ReadFile(o.Paths.SessionPath)
	if err != nil {
		o.log("session missing: %v", err)
		adapter.ApplyAssessment(&result.Model, assessment, nil)
		return fail(ExitMissingSession, "msg.session_missing", err.Error())
	}
	var sess session.Session
	if err := decodeStrictJSONObject(sessBytes, &sess); err != nil {
		o.log("session corrupt: %v", err)
		adapter.ApplyAssessment(&result.Model, assessment, nil)
		return fail(ExitCorruptSession, "msg.session_corrupt", err.Error())
	}
	if err := validateSession(sess, report.ReportID); err != nil {
		o.log("session invalid: %v", err)
		adapter.ApplyAssessment(&result.Model, assessment, nil)
		return fail(ExitCorruptSession, "msg.session_corrupt", err.Error())
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

// pulseProgress emits intermediate progress while a subprocess runs.
// The returned stop function waits until the progress goroutine exits.
func pulseProgress(ctx context.Context, notify ProgressFunc, phase, detail string, from, to int) func() {
	if notify == nil {
		return func() {}
	}
	done := make(chan struct{})
	var once sync.Once
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
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
	return func() {
		once.Do(func() { close(done) })
		wg.Wait()
	}
}

// Kept for callers that still decode permissive JSON elsewhere.
func decodeJSON(raw []byte, dest any) error {
	return json.Unmarshal(raw, dest)
}
