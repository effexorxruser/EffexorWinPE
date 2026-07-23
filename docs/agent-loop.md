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

## Evidence requests

Evidence requests are structured objects, never shell text:

- `operation` — closed read-only allowlist ID
- `arguments` — validated against the current report (roots, BCD stores, disks/devices, mount points, check IDs only; UNC and device namespaces are rejected)
- `reason` — why the observation is needed
- `expected_information` — what the next round expects to learn
- `privacy_class` — fixed per operation (`machine_inventory`, `boot_config`, `storage_health`, `encryption_status`, `network_status`)
- `timeout_seconds` — applied with `context.WithTimeout` around local collection

Completed assessments are validated by the shared gateway policy (`ValidateOnlineAssessmentWithEvidence`), including evidence/source refs, finding counts/severity/confidence/IDs, and the closed next-step catalog. Collected evidence refs may augment the report catalog; they do not invent a second validator.

## Loop limits

`internal/agentloop` enforces:

- at most three provider rounds
- rejection of identical repeated evidence requests (canonical operation + arguments)
- an overall loop timeout (default 90s)
- request and response size caps (1 MiB each by default)
- an audit timeline owned by the loop (`round_started`, `provider_proposed`, evidence events, terminal kinds)
- rejection of PowerShell, `cmd`, `diskpart`, download, and similar command text in model-authored fields

The loop never executes repair. A later local executor remains responsible for preview, confirmation, and mutation.

## Provider compatibility

The existing gateway `Analyzer` interface and OpenAI Responses provider remain the single-shot online path. The agent loop introduces a separate `RoundProvider` surface so multi-step reasoning can be added without breaking the current `/rescue/v1/diagnoses` contract or forcing OpenAI-specific tool calls into the WinPE client.
