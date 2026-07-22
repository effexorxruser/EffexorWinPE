package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/gateway"
)

var version = "dev"

func main() {
	listen := flag.String("listen", envOr("EFFEXORWINPE_LISTEN", "127.0.0.1:8080"), "HTTP listen address; terminate public TLS at a reverse proxy")
	deviceTokenHashes := flag.String("device-token-sha256-file", os.Getenv("EFFEXORWINPE_DEVICE_TOKEN_SHA256_FILE"), "file containing one SHA-256 device-token digest per line")
	apiKeyFile := flag.String("openai-api-key-file", os.Getenv("EFFEXORWINPE_OPENAI_API_KEY_FILE"), "file containing the server-side OpenAI API key")
	openAIBaseURL := flag.String("openai-base-url", envOr("EFFEXORWINPE_OPENAI_BASE_URL", "https://api.openai.com/v1"), "OpenAI Responses API base URL")
	model := flag.String("model", os.Getenv("EFFEXORWINPE_MODEL"), "OpenAI model used for diagnostic reasoning")
	reasoningEffort := flag.String("reasoning-effort", envOr("EFFEXORWINPE_REASONING_EFFORT", "low"), "Responses API reasoning effort: minimal, low, medium, or high")
	language := flag.String("language", envOr("EFFEXORWINPE_LANGUAGE", "ru"), "diagnostic output language")
	webSearch := flag.Bool("web-search", true, "allow server-side web search constrained to official domains")
	sourceDomainsFile := flag.String("source-domains-file", os.Getenv("EFFEXORWINPE_SOURCE_DOMAINS_FILE"), "optional replacement list of allowed official source domains")
	analysisTimeout := flag.Duration("analysis-timeout", 2*time.Minute, "maximum duration of one model analysis")
	jobTTL := flag.Duration("job-ttl", 30*time.Minute, "retention period for in-memory diagnosis jobs")
	maxJobs := flag.Int("max-jobs", 128, "maximum retained diagnosis jobs")
	maxJobsPerDevice := flag.Int("max-jobs-per-device", 16, "maximum retained jobs for one device token")
	maxConcurrent := flag.Int("max-concurrent", 2, "maximum concurrent model analyses")
	flag.Parse()

	if strings.TrimSpace(*deviceTokenHashes) == "" || strings.TrimSpace(*apiKeyFile) == "" || strings.TrimSpace(*model) == "" {
		log.Fatal("device-token-sha256-file, openai-api-key-file, and model are required")
	}
	verifier, err := gateway.LoadTokenVerifier(*deviceTokenHashes)
	if err != nil {
		log.Fatalf("load device-token digests: %v", err)
	}
	apiKey, err := gateway.LoadAPIKey(*apiKeyFile)
	if err != nil {
		log.Fatalf("load OpenAI API key: %v", err)
	}
	domains := gateway.DefaultOfficialSourceDomains
	if strings.TrimSpace(*sourceDomainsFile) != "" {
		domains, err = gateway.LoadSourceDomains(*sourceDomainsFile)
		if err != nil {
			log.Fatalf("load source domains: %v", err)
		}
	}
	provider, err := gateway.NewOpenAIResponsesProvider(*openAIBaseURL, apiKey, *model, domains, *webSearch)
	if err != nil {
		log.Fatalf("configure model provider: %v", err)
	}
	provider.ReasoningEffort = *reasoningEffort
	provider.Language = *language
	service, err := gateway.NewServer(provider, verifier, gateway.ServerOptions{
		MaxJobs:          *maxJobs,
		MaxJobsPerDevice: *maxJobsPerDevice,
		MaxConcurrent:    *maxConcurrent,
		JobTTL:           *jobTTL,
		AnalysisTimeout:  *analysisTimeout,
		Logger:           log.Default(),
	})
	if err != nil {
		log.Fatalf("configure gateway: %v", err)
	}

	httpServer := &http.Server{
		Addr:              *listen,
		Handler:           service.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-shutdownContext.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("gateway shutdown: %v", err)
		}
	}()

	log.Printf("EffexorWinPE gateway %s listening on %s", version, *listen)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(fmt.Errorf("serve gateway: %w", err))
	}
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
