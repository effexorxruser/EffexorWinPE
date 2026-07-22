package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
)

const (
	JobQueued   = "queued"
	JobRunning  = "running"
	JobComplete = "complete"
	JobFailed   = "failed"
)

type Analyzer interface {
	Analyze(context.Context, DiagnosisRequest) (diagnosis.Assessment, error)
}

type AnalyzerFunc func(context.Context, DiagnosisRequest) (diagnosis.Assessment, error)

func (function AnalyzerFunc) Analyze(ctx context.Context, request DiagnosisRequest) (diagnosis.Assessment, error) {
	return function(ctx, request)
}

type ServerOptions struct {
	MaxJobs          int
	MaxJobsPerDevice int
	MaxConcurrent    int
	JobTTL           time.Duration
	AnalysisTimeout  time.Duration
	Logger           *log.Logger
	Now              func() time.Time
}

type Server struct {
	analyzer  Analyzer
	verifier  TokenVerifier
	options   ServerOptions
	semaphore chan struct{}

	mu   sync.Mutex
	jobs map[string]*diagnosisJob
}

type diagnosisJob struct {
	ID        string
	Owner     string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
	Request   DiagnosisRequest
	Result    diagnosis.Assessment
}

func NewServer(analyzer Analyzer, verifier TokenVerifier, options ServerOptions) (*Server, error) {
	if analyzer == nil {
		return nil, fmt.Errorf("gateway analyzer is required")
	}
	if len(verifier.hashes) == 0 {
		return nil, fmt.Errorf("gateway token verifier has no credentials")
	}
	if options.MaxJobs <= 0 {
		options.MaxJobs = 128
	}
	if options.MaxConcurrent <= 0 {
		options.MaxConcurrent = 2
	}
	if options.MaxJobsPerDevice <= 0 {
		options.MaxJobsPerDevice = 16
	}
	if options.JobTTL <= 0 {
		options.JobTTL = 30 * time.Minute
	}
	if options.AnalysisTimeout <= 0 {
		options.AnalysisTimeout = 2 * time.Minute
	}
	if options.Logger == nil {
		options.Logger = log.New(io.Discard, "", 0)
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Server{
		analyzer:  analyzer,
		verifier:  verifier,
		options:   options,
		semaphore: make(chan struct{}, options.MaxConcurrent),
		jobs:      map[string]*diagnosisJob{},
	}, nil
}

func (server *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", server.handleHealth)
	mux.HandleFunc("/rescue/v1/diagnoses", server.handleDiagnoses)
	mux.HandleFunc("/rescue/v1/diagnoses/", server.handleDiagnosis)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		mux.ServeHTTP(writer, request)
	})
}

func (server *Server) handleHealth(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (server *Server) handleDiagnoses(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/rescue/v1/diagnoses" {
		http.NotFound(writer, request)
		return
	}
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	owner, authorized := server.verifier.VerifyAuthorization(request.Header.Get("Authorization"))
	if !authorized {
		writeError(writer, http.StatusUnauthorized, "unauthorized", "Device credential is missing or invalid.")
		return
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(writer, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json.")
		return
	}

	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var payload DiagnosisRequest
	if err := decoder.Decode(&payload); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(writer, http.StatusRequestEntityTooLarge, "request_too_large", "Diagnostic context exceeds the allowed size.")
			return
		}
		writeError(writer, http.StatusBadRequest, "invalid_request", "Request does not match the gateway contract.")
		return
	}
	if err := requireJSONEOF(decoder); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid_request", "Request contains trailing JSON data.")
		return
	}
	if err := ValidateDiagnosisRequest(payload); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid_request", "Request does not match the gateway contract.")
		return
	}
	payload = SanitizeDiagnosisRequest(payload)
	job, ok := server.createJob(owner, payload)
	if !ok {
		writer.Header().Set("Retry-After", "30")
		writeError(writer, http.StatusTooManyRequests, "capacity_exceeded", "Gateway capacity is temporarily exhausted.")
		return
	}
	go server.runJob(job.ID)
	writeJSON(writer, http.StatusAccepted, map[string]string{"diagnosis_id": job.ID, "status": JobQueued})
}

func (server *Server) handleDiagnosis(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	owner, authorized := server.verifier.VerifyAuthorization(request.Header.Get("Authorization"))
	if !authorized {
		writeError(writer, http.StatusUnauthorized, "unauthorized", "Device credential is missing or invalid.")
		return
	}
	id := strings.TrimPrefix(request.URL.Path, "/rescue/v1/diagnoses/")
	if id == request.URL.Path || !safeDiagnosisID(id) {
		http.NotFound(writer, request)
		return
	}
	job, ok := server.lookupJob(id, owner)
	if !ok {
		http.NotFound(writer, request)
		return
	}
	switch job.Status {
	case JobQueued, JobRunning:
		writer.Header().Set("Retry-After", "2")
		writeJSON(writer, http.StatusOK, map[string]string{"diagnosis_id": job.ID, "status": job.Status})
	case JobComplete:
		writeJSON(writer, http.StatusOK, job.Result)
	case JobFailed:
		writeJSON(writer, http.StatusOK, map[string]any{
			"diagnosis_id": job.ID,
			"status":       JobFailed,
			"error": map[string]string{
				"code":    "analysis_failed",
				"message": "Analysis failed without changing the client system.",
			},
		})
	default:
		writeError(writer, http.StatusInternalServerError, "invalid_state", "Gateway job state is invalid.")
	}
}

func (server *Server) createJob(owner string, request DiagnosisRequest) (diagnosisJob, bool) {
	now := server.options.Now().UTC()
	server.mu.Lock()
	defer server.mu.Unlock()
	server.pruneLocked(now)
	if len(server.jobs) >= server.options.MaxJobs {
		return diagnosisJob{}, false
	}
	deviceJobs := 0
	for _, existing := range server.jobs {
		if existing.Owner == owner {
			deviceJobs++
		}
	}
	if deviceJobs >= server.options.MaxJobsPerDevice {
		return diagnosisJob{}, false
	}
	id, err := newDiagnosisID()
	if err != nil {
		server.options.Logger.Printf("create diagnosis id: %v", err)
		return diagnosisJob{}, false
	}
	job := &diagnosisJob{
		ID:        id,
		Owner:     owner,
		Status:    JobQueued,
		CreatedAt: now,
		UpdatedAt: now,
		Request:   request,
	}
	server.jobs[id] = job
	return *job, true
}

func (server *Server) runJob(id string) {
	server.semaphore <- struct{}{}
	defer func() { <-server.semaphore }()

	request, ok := server.markRunning(id)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), server.options.AnalysisTimeout)
	result, err := server.analyzer.Analyze(ctx, request)
	cancel()
	if err == nil {
		err = ValidateOnlineAssessment(result, request)
	}
	server.finishJob(id, result, err)
}

func (server *Server) markRunning(id string) (DiagnosisRequest, bool) {
	server.mu.Lock()
	defer server.mu.Unlock()
	job, ok := server.jobs[id]
	if !ok || job.Status != JobQueued {
		return DiagnosisRequest{}, false
	}
	job.Status = JobRunning
	job.UpdatedAt = server.options.Now().UTC()
	return job.Request, true
}

func (server *Server) finishJob(id string, result diagnosis.Assessment, jobError error) {
	server.mu.Lock()
	defer server.mu.Unlock()
	job, ok := server.jobs[id]
	if !ok {
		return
	}
	job.UpdatedAt = server.options.Now().UTC()
	job.Request = DiagnosisRequest{}
	if jobError != nil {
		job.Status = JobFailed
		server.options.Logger.Printf("diagnosis %s failed: %v", id, jobError)
		return
	}
	job.Status = JobComplete
	job.Result = result
}

func (server *Server) lookupJob(id, owner string) (diagnosisJob, bool) {
	now := server.options.Now().UTC()
	server.mu.Lock()
	defer server.mu.Unlock()
	server.pruneLocked(now)
	job, ok := server.jobs[id]
	if !ok || job.Owner != owner {
		return diagnosisJob{}, false
	}
	return *job, true
}

func (server *Server) pruneLocked(now time.Time) {
	for id, job := range server.jobs {
		if now.Sub(job.CreatedAt) > server.options.JobTTL {
			delete(server.jobs, id)
		}
	}
}

func newDiagnosisID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return "diagnosis-" + hex.EncodeToString(value[:]), nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, code, message string) {
	writeJSON(writer, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
