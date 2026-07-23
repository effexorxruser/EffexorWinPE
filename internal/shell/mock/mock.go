package mock

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/adapter"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/viewmodel"
)

//go:embed report.json
var reportJSON []byte

//go:embed diagnosis.json
var diagnosisJSON []byte

//go:embed session.json
var sessionJSON []byte

// Report returns the embedded mock diagnostic report.
func Report() (diagnostics.Report, error) {
	return diagnostics.DecodeReportJSON(reportJSON)
}

// Diagnosis returns the embedded mock assessment.
func Diagnosis() (diagnosis.Assessment, error) {
	var a diagnosis.Assessment
	if err := json.Unmarshal(diagnosisJSON, &a); err != nil {
		return diagnosis.Assessment{}, fmt.Errorf("decode mock diagnosis: %w", err)
	}
	return a, nil
}

// Session returns the embedded mock session.
func Session() (session.Session, error) {
	var s session.Session
	if err := json.Unmarshal(sessionJSON, &s); err != nil {
		return session.Session{}, fmt.Errorf("decode mock session: %w", err)
	}
	return s, nil
}

// AppModel builds a complete mock presentation model.
func AppModel() (viewmodel.AppModel, error) {
	report, err := Report()
	if err != nil {
		return viewmodel.AppModel{}, err
	}
	model := adapter.FromReport(report, true)
	assessment, err := Diagnosis()
	if err != nil {
		return viewmodel.AppModel{}, err
	}
	sess, err := Session()
	if err != nil {
		return viewmodel.AppModel{}, err
	}
	adapter.ApplyAssessment(&model, assessment, &sess)
	model.Export = viewmodel.ExportScreen{
		ReportPath:    "(mock) report.json",
		DiagnosisPath: "(mock) diagnosis.json",
		SessionPath:   "(mock) session.json",
		JournalPath:   "(mock) journal.log",
	}
	model.Journal = viewmodel.JournalScreen{
		Entries: []string{
			"[mock] Loaded embedded diagnostic report schema 1.3.0",
			"[mock] Loaded offline preflight diagnosis",
			"[mock] Session ready for export demonstration",
		},
	}
	model.Progress = viewmodel.ProgressScreen{
		Phase:     "done",
		StatusKey: "status.succeeded",
		Percent:   100,
		Detail:    "msg.collection_done",
	}
	return model, nil
}

// ReportJSON returns raw mock report bytes.
func ReportJSON() []byte { return append([]byte(nil), reportJSON...) }

// DiagnosisJSON returns raw mock diagnosis bytes.
func DiagnosisJSON() []byte { return append([]byte(nil), diagnosisJSON...) }

// SessionJSON returns raw mock session bytes.
func SessionJSON() []byte { return append([]byte(nil), sessionJSON...) }
