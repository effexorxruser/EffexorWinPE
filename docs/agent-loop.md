# Online agent loop

The provider-neutral agent loop turns one approved diagnostic report and session into a bounded sequence of read-only evidence requests and a final online assessment. Model credentials and vendor APIs stay behind the EffexorWinPE gateway; this package defines the loop contract and policy, not a new client upload protocol.

## States

Every versioned result (`contracts/agent-result.schema.json`, schema `0.1.0`) uses exactly one state:

| State | Meaning |
| --- | --- |
| `completed` | Final `online_agent` assessment is present; no further evidence requests |
| `needs_more_evidence` | One or more typed read-only evidence requests must be collected locally |
| `blocked` | Policy stopped the loop (for example max rounds) without a completed assessment |
| `failed` | Provider or validation failure; the client system is unchanged |

## Provider surface

`RoundProvider` receives only a `SanitizedAgentContext` built with `gateway.SanitizeDiagnosisRequest`. Hostname, session events, and latest assessments never reach the provider. Prior evidence is filtered by privacy-class upload policy before the next round.

The provider returns a strict `ProviderProposal`. The loop does not invent missing `schema_version`, `report_id`, `generated_at`, `round`, `limitations`, `evidence_requests`, or `retrieved_sources`. Assessment `sources` / `source_refs` may cite only URLs from that turn's `retrieved_sources`.

## Evidence requests

Evidence requests are structured objects, never shell text:

- `operation` — closed read-only allowlist ID
- `arguments` — validated against the current report (roots, BCD stores, disks/devices, mount points, check IDs only; UNC and device namespaces are rejected; Windows paths compare case-insensitively after slash normalization)
- `reason` — why the observation is needed
- `expected_information` — what the next round expects to learn
- `privacy_class` — enforceable policy with allowed fact keys, redactions, `upload_allowed`, and `requires_additional_approval`
- `timeout_seconds` — applied with `context.WithTimeout` around local collection

Each operation has a closed fact schema (including safe arrays/objects where needed). Evidence refs are generated under `evidence.<operation>`; collector-supplied refs are ignored. Evidence that is not upload-allowed is audited as redacted and omitted from the next provider round.

Completed assessments are validated by the shared gateway policy (`ValidateOnlineAssessmentWithEvidence`).

## Loop limits

`internal/agentloop` enforces:

- at most three provider rounds
- rejection of identical repeated evidence requests (canonical operation + arguments)
- an overall loop timeout (default 90s)
- request and response size caps (1 MiB each by default)
- an audit timeline owned by the loop
- rejection of PowerShell, `cmd`, `diskpart`, download, and similar command text in model-authored fields

The loop never executes repair. A later local executor remains responsible for preview, confirmation, and mutation.

## Provider compatibility

The existing gateway `Analyzer` interface and OpenAI Responses provider remain the single-shot online path. The agent loop's `RoundProvider` stays separate so multi-step reasoning can be added without breaking `/rescue/v1/diagnoses`.
