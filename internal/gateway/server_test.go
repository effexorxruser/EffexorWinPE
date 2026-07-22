package gateway

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
)

func testTokenVerifier(t *testing.T, tokens ...string) TokenVerifier {
	t.Helper()
	var hashes strings.Builder
	for _, token := range tokens {
		digest := sha256.Sum256([]byte(token))
		hashes.WriteString(hex.EncodeToString(digest[:]))
		hashes.WriteByte('\n')
	}
	verifier, err := ParseTokenVerifier(strings.NewReader(hashes.String()))
	if err != nil {
		t.Fatalf("ParseTokenVerifier() error = %v", err)
	}
	return verifier
}

func TestGatewayServerAuthenticatesQueuesSanitizesAndReturnsAssessment(t *testing.T) {
	request := testDiagnosisRequest(t)
	analyzer := AnalyzerFunc(func(_ context.Context, received DiagnosisRequest) (diagnosis.Assessment, error) {
		if received.DiagnosticReport.Environment.Hostname != "" || received.DiagnosticReport.Privacy.ContainsPersonalData || len(received.Session.Events) != 0 {
			t.Error("gateway passed excluded report or session metadata to analyzer")
		}
		return testOnlineAssessment(received), nil
	})
	service, err := NewServer(analyzer, testTokenVerifier(t, "device-secret"), ServerOptions{AnalysisTimeout: time.Second})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	tlsServer := httptest.NewTLSServer(service.Handler())
	defer tlsServer.Close()

	client := Client{HTTPClient: tlsServer.Client(), PollInterval: time.Millisecond}
	result, err := client.Diagnose(context.Background(), tlsServer.URL+"/rescue/v1", "device-secret", request)
	if err != nil {
		t.Fatalf("Client.Diagnose() error = %v", err)
	}
	if result.Mode != diagnosis.ModeOnlineAgent || result.ReportID != request.DiagnosticReport.ReportID {
		t.Fatalf("unexpected assessment: %+v", result)
	}
}

func TestGatewayServerHidesJobsFromOtherDeviceTokens(t *testing.T) {
	request := testDiagnosisRequest(t)
	block := make(chan struct{})
	analyzer := AnalyzerFunc(func(_ context.Context, received DiagnosisRequest) (diagnosis.Assessment, error) {
		<-block
		return testOnlineAssessment(received), nil
	})
	service, err := NewServer(analyzer, testTokenVerifier(t, "device-one", "device-two"), ServerOptions{})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()
	defer close(block)

	payload, _ := json.Marshal(request)
	post, _ := http.NewRequest(http.MethodPost, server.URL+"/rescue/v1/diagnoses", bytes.NewReader(payload))
	post.Header.Set("Authorization", "Bearer device-one")
	post.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(post)
	if err != nil {
		t.Fatalf("POST error = %v", err)
	}
	defer response.Body.Close()
	var receipt struct {
		DiagnosisID string `json:"diagnosis_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&receipt); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	get, _ := http.NewRequest(http.MethodGet, server.URL+"/rescue/v1/diagnoses/"+receipt.DiagnosisID, nil)
	get.Header.Set("Authorization", "Bearer device-two")
	otherResponse, err := http.DefaultClient.Do(get)
	if err != nil {
		t.Fatalf("GET error = %v", err)
	}
	defer otherResponse.Body.Close()
	if otherResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("other device status = %d, want 404", otherResponse.StatusCode)
	}
}

func TestGatewayServerReturnsGenericFailureForInvalidModelResult(t *testing.T) {
	request := testDiagnosisRequest(t)
	analyzer := AnalyzerFunc(func(_ context.Context, received DiagnosisRequest) (diagnosis.Assessment, error) {
		result := testOnlineAssessment(received)
		result.NextSteps[0].Operation = "run_powershell"
		return result, nil
	})
	service, err := NewServer(analyzer, testTokenVerifier(t, "device-secret"), ServerOptions{})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()

	payload, _ := json.Marshal(request)
	post, _ := http.NewRequest(http.MethodPost, server.URL+"/rescue/v1/diagnoses", bytes.NewReader(payload))
	post.Header.Set("Authorization", "Bearer device-secret")
	post.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(post)
	if err != nil {
		t.Fatalf("POST error = %v", err)
	}
	var receipt struct {
		DiagnosisID string `json:"diagnosis_id"`
	}
	_ = json.NewDecoder(response.Body).Decode(&receipt)
	response.Body.Close()

	var body map[string]any
	for attempts := 0; attempts < 100; attempts++ {
		get, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/rescue/v1/diagnoses/%s", server.URL, receipt.DiagnosisID), nil)
		get.Header.Set("Authorization", "Bearer device-secret")
		result, err := http.DefaultClient.Do(get)
		if err != nil {
			t.Fatalf("GET error = %v", err)
		}
		body = map[string]any{}
		_ = json.NewDecoder(result.Body).Decode(&body)
		result.Body.Close()
		if body["status"] == JobFailed {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if body["status"] != JobFailed {
		t.Fatalf("job did not fail: %+v", body)
	}
	encoded, _ := json.Marshal(body)
	if strings.Contains(string(encoded), "run_powershell") {
		t.Fatalf("gateway exposed internal validation details: %s", encoded)
	}
}

func TestGatewayServerRejectsMissingCredential(t *testing.T) {
	service, err := NewServer(AnalyzerFunc(func(_ context.Context, request DiagnosisRequest) (diagnosis.Assessment, error) {
		return testOnlineAssessment(request), nil
	}), testTokenVerifier(t, "device-secret"), ServerOptions{})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/rescue/v1/diagnoses", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	service.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}

func TestGatewayServerBoundsJobsPerDevice(t *testing.T) {
	block := make(chan struct{})
	analyzer := AnalyzerFunc(func(_ context.Context, request DiagnosisRequest) (diagnosis.Assessment, error) {
		<-block
		return testOnlineAssessment(request), nil
	})
	service, err := NewServer(analyzer, testTokenVerifier(t, "device-secret"), ServerOptions{MaxJobs: 8, MaxJobsPerDevice: 1})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()
	defer close(block)
	payload, _ := json.Marshal(testDiagnosisRequest(t))
	for attempt := 0; attempt < 2; attempt++ {
		request, _ := http.NewRequest(http.MethodPost, server.URL+"/rescue/v1/diagnoses", bytes.NewReader(payload))
		request.Header.Set("Authorization", "Bearer device-secret")
		request.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("POST error = %v", err)
		}
		response.Body.Close()
		want := http.StatusAccepted
		if attempt == 1 {
			want = http.StatusTooManyRequests
		}
		if response.StatusCode != want {
			t.Fatalf("attempt %d status = %d, want %d", attempt, response.StatusCode, want)
		}
	}
}
