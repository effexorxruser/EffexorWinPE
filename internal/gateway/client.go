package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
)

const (
	maxRequestBytes  = 8 << 20
	maxResponseBytes = 4 << 20
)

type DiagnosisRequest struct {
	DiagnosticReport   diagnostics.Report `json:"diagnostic_report"`
	Session            session.Session    `json:"session"`
	TechnicianApproved bool               `json:"technician_approved"`
}

type Client struct {
	HTTPClient   *http.Client
	PollInterval time.Duration
}

func LoadToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) > 4096 {
		return "", fmt.Errorf("device token file is unexpectedly large")
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("device token file is empty")
	}
	return token, nil
}

func (c Client) Diagnose(ctx context.Context, baseURL, token string, request DiagnosisRequest) (diagnosis.Assessment, error) {
	if !request.TechnicianApproved {
		return diagnosis.Assessment{}, fmt.Errorf("technician approval is required before upload")
	}
	if strings.TrimSpace(token) == "" || strings.ContainsAny(token, "\r\n") {
		return diagnosis.Assessment{}, fmt.Errorf("device token is invalid")
	}
	endpoint, err := parseEndpoint(baseURL)
	if err != nil {
		return diagnosis.Assessment{}, err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return diagnosis.Assessment{}, fmt.Errorf("encode diagnosis request: %w", err)
	}
	if len(payload) > maxRequestBytes {
		return diagnosis.Assessment{}, fmt.Errorf("diagnosis request exceeds %d bytes", maxRequestBytes)
	}

	diagnosesURL := *endpoint
	diagnosesURL.Path = path.Join(endpoint.Path, "diagnoses")
	response, err := c.do(ctx, http.MethodPost, diagnosesURL.String(), token, payload)
	if err != nil {
		return diagnosis.Assessment{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		return diagnosis.Assessment{}, responseError(response)
	}
	var receipt struct {
		DiagnosisID string `json:"diagnosis_id"`
		Status      string `json:"status"`
	}
	if err := decodeLimited(response.Body, &receipt); err != nil {
		return diagnosis.Assessment{}, fmt.Errorf("decode gateway receipt: %w", err)
	}
	if receipt.DiagnosisID == "" || receipt.Status != "queued" {
		return diagnosis.Assessment{}, fmt.Errorf("gateway returned an invalid diagnosis receipt")
	}
	if !safeDiagnosisID(receipt.DiagnosisID) {
		return diagnosis.Assessment{}, fmt.Errorf("gateway returned an unsafe diagnosis id")
	}

	pollURL := *endpoint
	pollURL.Path = path.Join(endpoint.Path, "diagnoses", url.PathEscape(receipt.DiagnosisID))
	interval := c.PollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	for {
		select {
		case <-ctx.Done():
			return diagnosis.Assessment{}, fmt.Errorf("wait for gateway diagnosis: %w", ctx.Err())
		case <-time.After(interval):
		}
		assessment, ready, err := c.poll(ctx, pollURL.String(), token)
		if err != nil {
			return diagnosis.Assessment{}, err
		}
		if ready {
			if assessment.Mode != diagnosis.ModeOnlineAgent {
				return diagnosis.Assessment{}, fmt.Errorf("gateway result has unexpected mode %q", assessment.Mode)
			}
			return assessment, nil
		}
	}
}

func (c Client) poll(ctx context.Context, endpoint, token string) (diagnosis.Assessment, bool, error) {
	response, err := c.do(ctx, http.MethodGet, endpoint, token, nil)
	if err != nil {
		return diagnosis.Assessment{}, false, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return diagnosis.Assessment{}, false, responseError(response)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return diagnosis.Assessment{}, false, fmt.Errorf("read gateway response: %w", err)
	}
	if len(data) > maxResponseBytes {
		return diagnosis.Assessment{}, false, fmt.Errorf("gateway response exceeds %d bytes", maxResponseBytes)
	}
	var state struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return diagnosis.Assessment{}, false, fmt.Errorf("decode gateway response: %w", err)
	}
	if state.Status == "queued" || state.Status == "running" {
		return diagnosis.Assessment{}, false, nil
	}
	if state.Status == "failed" {
		return diagnosis.Assessment{}, false, fmt.Errorf("gateway analysis failed")
	}
	var assessment diagnosis.Assessment
	if err := json.Unmarshal(data, &assessment); err != nil {
		return diagnosis.Assessment{}, false, fmt.Errorf("decode gateway assessment: %w", err)
	}
	if assessment.SchemaVersion != diagnosis.SchemaVersion {
		return diagnosis.Assessment{}, false, fmt.Errorf("unsupported gateway diagnosis schema %q", assessment.SchemaVersion)
	}
	return assessment, true, nil
}

func (c Client) do(ctx context.Context, method, endpoint, token string, payload []byte) (*http.Response, error) {
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{CheckRedirect: rejectRedirect}
	}
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("create gateway request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "EffexorWinPE-Agent")
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("gateway request failed: %w", err)
	}
	return response, nil
}

func rejectRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

func parseEndpoint(raw string) (*url.URL, error) {
	endpoint, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("parse gateway URL: %w", err)
	}
	if endpoint.Scheme != "https" || endpoint.Host == "" {
		return nil, fmt.Errorf("gateway URL must be an absolute HTTPS URL")
	}
	if endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, fmt.Errorf("gateway URL must not contain credentials, query parameters, or fragments")
	}
	return endpoint, nil
}

func decodeLimited(reader io.Reader, value any) error {
	data, err := io.ReadAll(io.LimitReader(reader, maxResponseBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxResponseBytes {
		return fmt.Errorf("response exceeds %d bytes", maxResponseBytes)
	}
	return json.Unmarshal(data, value)
}

func responseError(response *http.Response) error {
	return fmt.Errorf("gateway returned HTTP %d", response.StatusCode)
}

func safeDiagnosisID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return value != "." && value != ".."
}
