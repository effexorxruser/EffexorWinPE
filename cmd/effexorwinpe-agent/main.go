package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/gateway"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
	"github.com/effexorxruser/EffexorWinPE/internal/triage"
)

var version = "dev"

type repeatedFlag []string

func (values *repeatedFlag) String() string {
	return strings.Join(*values, ", ")
}

func (values *repeatedFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func main() {
	input := flag.String("input", "diagnostic-report.json", "path to a diagnostic report, or - for stdin")
	output := flag.String("output", "diagnosis.json", "path for the offline assessment, or - for stdout")
	sessionPath := flag.String("session", "", "path to create or resume a diagnostic session")
	interactive := flag.Bool("interactive", false, "prompt for symptoms and unanswered follow-up questions")
	pretty := flag.Bool("pretty", true, "indent JSON output")
	gatewayURL := flag.String("gateway-url", "", "HTTPS base URL for the optional EffexorWinPE gateway")
	deviceTokenFile := flag.String("device-token-file", "", "path to a removable device-token file")
	approveUpload := flag.Bool("approve-upload", false, "confirm that the technician reviewed and approved this upload")
	onlineOutput := flag.String("online-output", "online-diagnosis.json", "path for a completed online assessment")
	gatewayTimeout := flag.Duration("gateway-timeout", 90*time.Second, "maximum time to wait for an online assessment")
	var symptoms repeatedFlag
	var answers repeatedFlag
	flag.Var(&symptoms, "symptom", "technician-observed symptom; may be supplied more than once")
	flag.Var(&answers, "answer", "follow-up answer in question-id=value form; may be supplied more than once")
	flag.Parse()

	if *interactive && *input == "-" {
		exitError("--interactive cannot share stdin with --input -")
	}
	report, err := readReport(*input)
	if err != nil {
		exitError("read diagnostic report: %v", err)
	}
	offlineAssessment, err := triage.Analyze(report, time.Now())
	if err != nil {
		exitError("analyze diagnostic report: %v", err)
	}

	resolvedSessionPath := *sessionPath
	if resolvedSessionPath == "" {
		resolvedSessionPath = defaultSessionPath(*output)
	}
	diagnosticSession, err := openSession(resolvedSessionPath, report.ReportID, time.Now())
	if err != nil {
		exitError("open diagnostic session: %v", err)
	}
	for _, symptom := range symptoms {
		if err := diagnosticSession.AddSymptom(symptom, time.Now()); err != nil {
			exitError("record symptom: %v", err)
		}
	}
	for _, raw := range answers {
		if err := recordAnswer(&diagnosticSession, offlineAssessment, raw, time.Now()); err != nil {
			exitError("record answer: %v", err)
		}
	}
	if *interactive {
		if err := collectInteractive(&diagnosticSession, offlineAssessment, os.Stdin, os.Stdout); err != nil {
			exitError("interactive input: %v", err)
		}
	}
	offlineAssessment.Questions = unansweredQuestions(diagnosticSession, offlineAssessment.Questions)
	diagnosticSession.SetAssessment(offlineAssessment, session.EventOfflineAssessmentCreated, time.Now())

	if err := writeJSON(*output, offlineAssessment, *pretty); err != nil {
		exitError("write diagnosis: %v", err)
	}
	if resolvedSessionPath != "" {
		if err := session.Write(resolvedSessionPath, diagnosticSession); err != nil {
			exitError("write diagnostic session: %v", err)
		}
	}

	if *gatewayURL != "" {
		if resolvedSessionPath == "" {
			exitError("online diagnosis requires a persisted --session path")
		}
		if !*approveUpload {
			exitError("upload blocked: review the report and session, then repeat with --approve-upload")
		}
		if *deviceTokenFile == "" {
			exitError("online diagnosis requires --device-token-file")
		}
		token, err := gateway.LoadToken(*deviceTokenFile)
		if err != nil {
			exitError("read device token: %v", err)
		}
		diagnosticSession.AddEvent(session.EventGatewaySubmitted, "approved", time.Now())
		if err := session.Write(resolvedSessionPath, diagnosticSession); err != nil {
			exitError("write diagnostic session: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), *gatewayTimeout)
		onlineAssessment, err := (gateway.Client{}).Diagnose(ctx, *gatewayURL, token, gateway.DiagnosisRequest{
			DiagnosticReport:   report,
			Session:            diagnosticSession,
			TechnicianApproved: true,
		})
		cancel()
		if err != nil {
			exitError("online diagnosis: %v", err)
		}
		if onlineAssessment.ReportID != report.ReportID {
			exitError("online diagnosis belongs to report %q, expected %q", onlineAssessment.ReportID, report.ReportID)
		}
		diagnosticSession.SetAssessment(onlineAssessment, session.EventOnlineAssessmentReceived, time.Now())
		if err := session.Write(resolvedSessionPath, diagnosticSession); err != nil {
			exitError("write diagnostic session: %v", err)
		}
		if err := writeJSON(*onlineOutput, onlineAssessment, *pretty); err != nil {
			exitError("write online diagnosis: %v", err)
		}
	}

	if *output != "-" {
		fmt.Printf("EffexorWinPE agent %s wrote offline preflight to %s\n", version, *output)
		if resolvedSessionPath != "" {
			fmt.Printf("Diagnostic session: %s\n", resolvedSessionPath)
		}
	}
}

func readReport(path string) (diagnostics.Report, error) {
	var reader io.Reader
	if path == "-" {
		reader = os.Stdin
	} else {
		file, err := os.Open(path)
		if err != nil {
			return diagnostics.Report{}, err
		}
		defer file.Close()
		reader = file
	}
	var report diagnostics.Report
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return diagnostics.Report{}, err
	}
	return report, nil
}

func openSession(path, reportID string, now time.Time) (session.Session, error) {
	if path != "" {
		value, err := session.Load(path)
		if err == nil {
			if err := value.Validate(reportID); err != nil {
				return session.Session{}, err
			}
			return value, nil
		}
		if !os.IsNotExist(err) {
			return session.Session{}, err
		}
	}
	return session.New(reportID, now)
}

func recordAnswer(value *session.Session, assessment diagnosis.Assessment, raw string, now time.Time) error {
	questionID, answer, found := strings.Cut(raw, "=")
	if !found {
		return fmt.Errorf("answer %q must use question-id=value", raw)
	}
	question, found := findQuestion(assessment, strings.TrimSpace(questionID))
	if !found {
		return fmt.Errorf("assessment does not contain question %q", strings.TrimSpace(questionID))
	}
	return value.RecordAnswer(question.ID, normalizeAnswer(answer), question.AnswerType, now)
}

func collectInteractive(value *session.Session, assessment diagnosis.Assessment, input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	buffer := make([]byte, 4096)
	scanner.Buffer(buffer, 64*1024)
	fmt.Fprint(output, "Observed symptom (leave blank to skip): ")
	if scanner.Scan() {
		if text := strings.TrimSpace(scanner.Text()); text != "" {
			if err := value.AddSymptom(text, time.Now()); err != nil {
				return err
			}
		}
	} else if err := scanner.Err(); err != nil {
		return err
	}
	for _, question := range assessment.Questions {
		if value.HasAnswer(question.ID) {
			continue
		}
		fmt.Fprintf(output, "\n%s\nWhy: %s\n", question.Prompt, question.Reason)
		if question.AnswerType == diagnosis.AnswerYesNo {
			fmt.Fprint(output, "Answer [yes/no/unknown, blank to skip]: ")
		} else {
			fmt.Fprint(output, "Answer [blank to skip]: ")
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}
			break
		}
		answer := normalizeAnswer(scanner.Text())
		if answer == "" {
			continue
		}
		if err := value.RecordAnswer(question.ID, answer, question.AnswerType, time.Now()); err != nil {
			return err
		}
	}
	return nil
}

func findQuestion(assessment diagnosis.Assessment, id string) (diagnosis.Question, bool) {
	for _, question := range assessment.Questions {
		if question.ID == id {
			return question, true
		}
	}
	return diagnosis.Question{}, false
}

func unansweredQuestions(value session.Session, questions []diagnosis.Question) []diagnosis.Question {
	pending := make([]diagnosis.Question, 0, len(questions))
	for _, question := range questions {
		if !value.HasAnswer(question.ID) {
			pending = append(pending, question)
		}
	}
	return pending
}

func normalizeAnswer(value string) string {
	value = strings.TrimSpace(value)
	switch strings.ToLower(value) {
	case "да":
		return "yes"
	case "нет":
		return "no"
	case "не знаю", "неизвестно":
		return "unknown"
	default:
		return value
	}
}

func defaultSessionPath(output string) string {
	if output == "" || output == "-" {
		return ""
	}
	extension := filepath.Ext(output)
	name := strings.TrimSuffix(filepath.Base(output), extension)
	return filepath.Join(filepath.Dir(output), name+"-session.json")
}

func writeJSON(path string, value any, pretty bool) error {
	var data []byte
	var err error
	if pretty {
		data, err = json.MarshalIndent(value, "", "  ")
	} else {
		data, err = json.Marshal(value)
	}
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if path == "-" {
		_, err = os.Stdout.Write(data)
		return err
	}
	if directory := filepath.Dir(path); directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o600)
}

func exitError(format string, values ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", values...)
	os.Exit(1)
}
