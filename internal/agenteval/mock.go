package agenteval

import (
	"context"
	"fmt"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/agentloop"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
)

// MockProvider returns predetermined round proposals without calling a model API.
type MockProvider struct {
	rounds    []agentloop.ProviderProposal
	index     int
	requested []string
	lastInput agentloop.RoundInput
}

func NewMockProvider(rounds []agentloop.ProviderProposal) *MockProvider {
	copied := make([]agentloop.ProviderProposal, len(rounds))
	copy(copied, rounds)
	return &MockProvider{rounds: copied}
}

func (provider *MockProvider) Propose(_ context.Context, input agentloop.RoundInput) (agentloop.ProviderProposal, error) {
	provider.lastInput = input
	if provider.index >= len(provider.rounds) {
		return agentloop.ProviderProposal{}, fmt.Errorf("mock provider has no scripted round %d", input.Round)
	}
	proposal := provider.rounds[provider.index]
	provider.index++
	if proposal.EvidenceRequests == nil {
		return agentloop.ProviderProposal{}, fmt.Errorf("scripted proposal missing evidence_requests")
	}
	if proposal.RetrievedSources == nil {
		proposal.RetrievedSources = []diagnosis.Source{}
	}
	for _, request := range proposal.EvidenceRequests {
		provider.requested = append(provider.requested, request.Operation)
	}
	return proposal, nil
}

func (provider *MockProvider) RequestedOperations() []string {
	return append([]string(nil), provider.requested...)
}

func (provider *MockProvider) LastInput() agentloop.RoundInput {
	return provider.lastInput
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
	if payload.RequestID == "" || payload.Operation == "" || payload.CollectedAt.IsZero() || payload.Facts == nil {
		return agentloop.EvidencePayload{}, fmt.Errorf("fixture evidence payload for %q is incomplete", key)
	}
	return payload, nil
}
