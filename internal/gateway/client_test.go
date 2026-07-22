package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
)

func TestDiagnoseRequiresHTTPSAndApproval(t *testing.T) {
	client := Client{}
	_, err := client.Diagnose(context.Background(), "http://example.test", "secret", DiagnosisRequest{TechnicianApproved: true})
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("Diagnose() error = %v, want HTTPS rejection", err)
	}
	_, err = client.Diagnose(context.Background(), "https://example.test", "secret", DiagnosisRequest{})
	if err == nil || !strings.Contains(err.Error(), "approval") {
		t.Fatalf("Diagnose() error = %v, want approval rejection", err)
	}
}

func TestSafeDiagnosisIDRejectsPathTraversal(t *testing.T) {
	for _, value := range []string{"../diagnosis", "a/b", "", "..", "diagnosis-1", "diagnosis-0000000000000000000000000000000G"} {
		if safeDiagnosisID(value) {
			t.Fatalf("safeDiagnosisID(%q) = true, want false", value)
		}
	}
	if !safeDiagnosisID("diagnosis-00000000000000000000000000000001") {
		t.Fatal("safeDiagnosisID() rejected a valid gateway id")
	}
}

func TestDiagnoseSubmitsApprovedContextAndPollsForAssessment(t *testing.T) {
	const diagnosisID = "diagnosis-00000000000000000000000000000001"
	polls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer device-secret" {
			t.Error("missing device authorization")
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/rescue/v1/diagnoses":
			var submitted DiagnosisRequest
			if err := json.NewDecoder(request.Body).Decode(&submitted); err != nil {
				t.Errorf("decode request: %v", err)
			}
			if !submitted.TechnicianApproved {
				t.Error("request was not marked approved")
			}
			writer.WriteHeader(http.StatusAccepted)
			_, _ = writer.Write([]byte(`{"diagnosis_id":"` + diagnosisID + `","status":"queued"}`))
		case request.Method == http.MethodGet && request.URL.Path == "/rescue/v1/diagnoses/"+diagnosisID:
			polls++
			writer.Header().Set("Content-Type", "application/json")
			if polls == 1 {
				_, _ = writer.Write([]byte(`{"diagnosis_id":"diagnosis-1","status":"running"}`))
				return
			}
			_ = json.NewEncoder(writer).Encode(diagnosis.Assessment{
				SchemaVersion: diagnosis.SchemaVersion,
				ReportID:      "report-1",
				GeneratedAt:   time.Unix(100, 0).UTC(),
				Mode:          diagnosis.ModeOnlineAgent,
				Summary:       diagnosis.Summary{Headline: "Online assessment", HighestSeverity: diagnosis.SeverityInfo},
				Findings:      []diagnosis.Finding{},
				Questions:     []diagnosis.Question{},
				NextSteps:     []diagnosis.NextStep{},
				Limitations:   []string{"Test result"},
				Sources:       []diagnosis.Source{},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := Client{HTTPClient: server.Client(), PollInterval: time.Millisecond}
	result, err := client.Diagnose(context.Background(), server.URL+"/rescue/v1", "device-secret", testDiagnosisRequest(t))
	if err != nil {
		t.Fatalf("Diagnose() error = %v", err)
	}
	if result.Mode != diagnosis.ModeOnlineAgent || polls != 2 {
		t.Fatalf("unexpected result or poll count: result=%+v polls=%d", result, polls)
	}
}

func TestDiagnoseRejectsUnsafeGatewayAssessmentLocally(t *testing.T) {
	const diagnosisID = "diagnosis-00000000000000000000000000000002"
	request := testDiagnosisRequest(t)
	unsafe := testOnlineAssessment(request)
	unsafe.NextSteps[0].Operation = "run_powershell"
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, httpRequest *http.Request) {
		switch httpRequest.Method {
		case http.MethodPost:
			writer.WriteHeader(http.StatusAccepted)
			_, _ = writer.Write([]byte(`{"diagnosis_id":"` + diagnosisID + `","status":"queued"}`))
		case http.MethodGet:
			_ = json.NewEncoder(writer).Encode(unsafe)
		default:
			http.NotFound(writer, httpRequest)
		}
	}))
	defer server.Close()

	client := Client{HTTPClient: server.Client(), PollInterval: time.Millisecond}
	_, err := client.Diagnose(context.Background(), server.URL+"/rescue/v1", "device-secret", request)
	if err == nil || !strings.Contains(err.Error(), "unknown operation") {
		t.Fatalf("Diagnose() error = %v, want local operation rejection", err)
	}
}
