package agenteval

import (
	"context"
	"fmt"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/agentloop"
)

// MockProvider returns predetermined round results without calling a model API.
type MockProvider struct {
	rounds    []agentloop.Result
	index     int
	requested []string
}

func NewMockProvider(rounds []agentloop.Result) *MockProvider {
	copied := make([]agentloop.Result, len(rounds))
	copy(copied, rounds)
	return &MockProvider{rounds: copied}
}

func (provider *MockProvider) Propose(_ context.Context, input agentloop.RoundInput) (agentloop.Result, error) {
	if provider.index >= len(provider.rounds) {
		return agentloop.Result{}, fmt.Errorf("mock provider has no scripted round %d", input.Round)
	}
	result := provider.rounds[provider.index]
	provider.index++
	if result.EvidenceRequests == nil {
		result.EvidenceRequests = []agentloop.EvidenceRequest{}
	}
	for _, request := range result.EvidenceRequests {
		provider.requested = append(provider.requested, request.Operation)
	}
	return result, nil
}

func (provider *MockProvider) RequestedOperations() []string {
	return append([]string(nil), provider.requested...)
}

// CatalogCollector returns fixture-supplied evidence for allowlisted operations.
type CatalogCollector struct {
	catalog map[string]agentloop.EvidencePayload
	now     time.Time
}

func NewCatalogCollector(catalog map[string]agentloop.EvidencePayload, now time.Time) CatalogCollector {
	copied := make(map[string]agentloop.EvidencePayload, len(catalog))
	for key, value := range catalog {
		copied[key] = value
	}
	return CatalogCollector{catalog: copied, now: now}
}

func (collector CatalogCollector) Collect(_ context.Context, request agentloop.EvidenceRequest) (agentloop.EvidencePayload, error) {
	key := agentloop.CanonicalRequestKey(request)
	payload, ok := collector.catalog[key]
	if !ok {
		payload, ok = collector.catalog[request.Operation]
	}
	if !ok {
		return agentloop.EvidencePayload{}, fmt.Errorf("no fixture evidence for %q", key)
	}
	if payload.RequestID == "" {
		payload.RequestID = request.ID
	}
	if payload.Operation == "" {
		payload.Operation = request.Operation
	}
	if payload.CollectedAt.IsZero() {
		payload.CollectedAt = collector.now.UTC()
	}
	if payload.Facts == nil {
		payload.Facts = map[string]any{}
	}
	if payload.EvidenceRefs == nil {
		payload.EvidenceRefs = []string{}
	}
	return payload, nil
}
