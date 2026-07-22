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

func TestOpenAIResponsesProviderUsesStructuredOutputWebSearchAndSanitizedContext(t *testing.T) {
	request := testDiagnosisRequest(t)
	const sourceURL = "https://learn.microsoft.com/windows-hardware/drivers/"
	modelResult := modelAssessment{
		Summary: diagnosis.Summary{Headline: "Нужна проверка драйвера", HighestSeverity: diagnosis.SeverityWarning, FindingCount: 99},
		Findings: []diagnosis.Finding{{
			ID:           "driver.check-required",
			Title:        "Требуется проверка драйвера",
			Severity:     diagnosis.SeverityWarning,
			Confidence:   diagnosis.ConfidenceMedium,
			Rationale:    "Состояние нужно сопоставить с официальной документацией.",
			EvidenceRefs: []string{"checks[0].status"},
			SourceRefs:   []string{sourceURL + "#section"},
		}},
		Questions: []diagnosis.Question{},
		NextSteps: []diagnosis.NextStep{{
			ID:                   "review-sources",
			Title:                "Проверить источник",
			Operation:            "review_missing_sources",
			Risk:                 diagnosis.RiskReadOnly,
			RequiresConfirmation: false,
			Rationale:            "Нужна дополнительная проверка.",
		}},
		Limitations: []string{"Диагноз ограничен доступными данными."},
	}
	structured, err := json.Marshal(modelResult)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, httpRequest *http.Request) {
		if httpRequest.URL.Path != "/v1/responses" {
			t.Errorf("request path = %q", httpRequest.URL.Path)
		}
		if httpRequest.Header.Get("Authorization") != "Bearer server-api-key" {
			t.Error("missing OpenAI authorization")
		}
		var payload map[string]any
		if err := json.NewDecoder(httpRequest.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if stored, ok := payload["store"].(bool); !ok || stored {
			t.Error("OpenAI request did not disable storage")
		}
		input, _ := payload["input"].(string)
		if strings.Contains(input, "client-laptop") {
			t.Error("hostname leaked into model context")
		}
		if !strings.Contains(input, "storage.disks[0].health_status") {
			t.Error("evidence catalog is missing from model context")
		}
		tools, ok := payload["tools"].([]any)
		if !ok || len(tools) != 1 {
			t.Error("web search tool is missing")
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"status": "completed",
			"output": []any{
				map[string]any{
					"type": "web_search_call",
					"action": map[string]any{"sources": []any{
						map[string]any{"type": "url", "title": "Microsoft drivers", "url": sourceURL},
						map[string]any{"type": "url", "title": "Untrusted", "url": "https://example.invalid/advice"},
					}},
				},
				map[string]any{
					"type": "message",
					"content": []any{map[string]any{
						"type": "output_text",
						"text": string(structured),
						"annotations": []any{map[string]any{
							"type": "url_citation", "title": "Microsoft drivers", "url": sourceURL,
						}},
					}},
				},
			},
		})
	}))
	defer server.Close()

	provider, err := NewOpenAIResponsesProvider(server.URL+"/v1", "server-api-key", "test-model", []string{"microsoft.com"}, true)
	if err != nil {
		t.Fatalf("NewOpenAIResponsesProvider() error = %v", err)
	}
	provider.HTTPClient = server.Client()
	provider.Now = func() time.Time { return time.Unix(200, 0) }
	result, err := provider.Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Summary.FindingCount != 1 || len(result.Sources) != 1 {
		t.Fatalf("unexpected grounded result: %+v", result)
	}
	if result.Sources[0].Domain != "learn.microsoft.com" || result.Findings[0].SourceRefs[0] != sourceURL {
		t.Fatalf("source canonicalization failed: %+v", result)
	}
}

func TestOpenAIResponsesProviderRejectsModelOperationOutsideAllowlist(t *testing.T) {
	request := testDiagnosisRequest(t)
	modelResult := modelAssessment{
		Summary: diagnosis.Summary{Headline: "Unsafe", HighestSeverity: diagnosis.SeverityWarning, FindingCount: 1},
		Findings: []diagnosis.Finding{{
			ID: "unsafe.finding", Title: "Unsafe", Severity: diagnosis.SeverityWarning, Confidence: diagnosis.ConfidenceLow,
			Rationale: "Test", EvidenceRefs: []string{"checks[0].status"}, SourceRefs: []string{},
		}},
		Questions: []diagnosis.Question{},
		NextSteps: []diagnosis.NextStep{{
			ID: "unsafe-step", Title: "Unsafe", Operation: "run_powershell", Risk: diagnosis.RiskReadOnly,
			RequiresConfirmation: false, Rationale: "Test",
		}},
		Limitations: []string{"Test"},
	}
	structured, _ := json.Marshal(modelResult)
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"status": "completed",
			"output": []any{map[string]any{
				"type": "message",
				"content": []any{map[string]any{
					"type": "output_text",
					"text": string(structured),
				}},
			}},
		})
	}))
	defer server.Close()
	provider, err := NewOpenAIResponsesProvider(server.URL+"/v1", "server-api-key", "test-model", nil, false)
	if err != nil {
		t.Fatalf("NewOpenAIResponsesProvider() error = %v", err)
	}
	provider.HTTPClient = server.Client()
	if _, err := provider.Analyze(context.Background(), request); err == nil || !strings.Contains(err.Error(), "unknown operation") {
		t.Fatalf("Analyze() error = %v, want operation rejection", err)
	}
}

func TestOpenAIResponsesProviderDoesNotExposeProviderErrorBody(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":"server-api-key leaked"}`))
	}))
	defer server.Close()
	provider, err := NewOpenAIResponsesProvider(server.URL+"/v1", "server-api-key", "test-model", nil, false)
	if err != nil {
		t.Fatalf("NewOpenAIResponsesProvider() error = %v", err)
	}
	provider.HTTPClient = server.Client()
	_, err = provider.Analyze(context.Background(), testDiagnosisRequest(t))
	if err == nil || strings.Contains(err.Error(), "server-api-key leaked") {
		t.Fatalf("Analyze() error = %v", err)
	}
}

func TestOpenAIResponsesProviderRejectsTrailingStructuredOutput(t *testing.T) {
	request := testDiagnosisRequest(t)
	structured, _ := json.Marshal(modelAssessment{
		Summary:     diagnosis.Summary{Headline: "Test", HighestSeverity: diagnosis.SeverityInfo},
		Findings:    []diagnosis.Finding{},
		Questions:   []diagnosis.Question{},
		NextSteps:   []diagnosis.NextStep{},
		Limitations: []string{"Test limitation"},
	})
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"status": "completed",
			"output": []any{map[string]any{
				"type": "message",
				"content": []any{map[string]any{
					"type": "output_text",
					"text": string(structured) + `{}`,
				}},
			}},
		})
	}))
	defer server.Close()
	provider, err := NewOpenAIResponsesProvider(server.URL+"/v1", "server-api-key", "test-model", nil, false)
	if err != nil {
		t.Fatalf("NewOpenAIResponsesProvider() error = %v", err)
	}
	provider.HTTPClient = server.Client()
	if _, err := provider.Analyze(context.Background(), request); err == nil || !strings.Contains(err.Error(), "trailing data") {
		t.Fatalf("Analyze() error = %v, want trailing data rejection", err)
	}
}

func TestNormalizeDomainsRejectsURLsAndAllowsSubdomains(t *testing.T) {
	if _, err := normalizeDomains([]string{"https://microsoft.com"}); err == nil {
		t.Fatal("normalizeDomains() accepted a URL")
	}
	if !domainAllowed("learn.microsoft.com", []string{"microsoft.com"}) {
		t.Fatal("domainAllowed() rejected an official subdomain")
	}
}
