package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/effexorxruser/EffexorWinPE/internal/shell/i18n"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/journal"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/mock"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/orchestrator"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/viewmodel"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/winui"
)

var version = "dev"

func main() {
	mockMode := flag.Bool("mock", false, "load embedded mock diagnostic data (desktop development)")
	locale := flag.String("locale", i18n.Default, "UI locale (ru-RU default, en-US fallback)")
	baseDir := flag.String("base-dir", "", "directory containing collector/agent binaries (default: executable directory)")
	reportsDir := flag.String("reports-dir", "", "override reports directory")
	reportPath := flag.String("report", "", "load an existing diagnostic report JSON")
	kiosk := flag.Bool("kiosk", true, "start in fullscreen/kiosk mode (Esc or UI control restores windowed)")
	windowed := flag.Bool("windowed", false, "start in windowed mode (overrides --kiosk)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("effexorwinpe-shell %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return
	}

	bundle, err := i18n.New(*locale)
	if err != nil {
		fmt.Fprintf(os.Stderr, "localization: %v\n", err)
		os.Exit(1)
	}

	paths := orchestrator.DefaultPaths(*baseDir)
	if *reportsDir != "" {
		paths.ReportsDir = *reportsDir
		paths.ReportPath = filepath.Join(*reportsDir, "initial.json")
		paths.DiagnosisPath = filepath.Join(*reportsDir, "initial-diagnosis.json")
		paths.SessionPath = filepath.Join(*reportsDir, "initial-diagnosis-session.json")
	}
	if err := os.MkdirAll(paths.ReportsDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "reports dir: %v\n", err)
	}
	jpath := filepath.Join(paths.ReportsDir, "shell-journal.log")
	j := journal.New(jpath)
	j.Append("effexorwinpe-shell %s starting locale=%s mock=%v", version, bundle.Locale(), *mockMode)

	var model viewmodel.AppModel
	switch {
	case *mockMode:
		model, err = mock.AppModel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "mock data: %v\n", err)
			os.Exit(1)
		}
		model.Export.JournalPath = jpath
		model.Journal.Entries = append(model.Journal.Entries, j.Entries()...)
	case *reportPath != "":
		model, err = orchestrator.LoadReportFile(*reportPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load report: %v\n", err)
			os.Exit(1)
		}
		model.Export.JournalPath = jpath
	default:
		model = viewmodel.AppModel{
			Progress: viewmodel.ProgressScreen{Phase: "idle", StatusKey: "status.idle"},
			Export: viewmodel.ExportScreen{
				ReportPath:    paths.ReportPath,
				DiagnosisPath: paths.DiagnosisPath,
				SessionPath:   paths.SessionPath,
				JournalPath:   jpath,
			},
		}
		// Load existing report if present from a previous run.
		if _, err := os.Stat(paths.ReportPath); err == nil {
			if loaded, err := orchestrator.LoadReportFile(paths.ReportPath); err == nil {
				model = loaded
				model.Export.ReportPath = paths.ReportPath
				model.Export.DiagnosisPath = paths.DiagnosisPath
				model.Export.SessionPath = paths.SessionPath
				model.Export.JournalPath = jpath
			}
		}
	}

	orch := &orchestrator.Orchestrator{Paths: paths, Journal: j}
	cfg := winui.Config{
		Bundle:       bundle,
		Model:        model,
		Orchestrator: orch,
		Journal:      j,
		Mock:         *mockMode,
		Kiosk:        *kiosk && !*windowed,
	}
	if err := winui.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
