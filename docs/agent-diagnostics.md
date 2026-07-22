# Diagnostic agent boundary

`effexorwinpe-agent.exe` is the first local layer of the technician assistant. It runs an offline deterministic preflight over `diagnostic-report.schema.json`, writes a `diagnosis.schema.json` assessment, and creates or resumes a `diagnostic-session.schema.json` session.

It is intentionally useful without internet access, but it is not presented as the final AI diagnosis. Model-backed reasoning, device-token policy, official-source retrieval, and result validation remain behind the dedicated EffexorWinPE gateway.

## Current flow

1. `effexorwinpe-collector.exe` creates a structured read-only report.
2. `effexorwinpe-agent.exe` validates the report version.
3. Conservative rules correlate missing sources, Windows installations, firmware/BCD visibility, storage health, reliability counters, and BitLocker access.
4. The result contains findings, confidence, evidence references, focused follow-up questions, and typed read-only next steps.
5. The technician may record observed symptoms and typed answers interactively or with repeatable CLI flags; answered questions are removed from the next pending assessment.
6. The session keeps a compact timeline and latest assessment without duplicating symptom text into its event log.
7. The technician receives an explicit limitation instead of a false `healthy` result when evidence is incomplete.

The executable never accepts or emits arbitrary command strings. Offline next steps are operation identifiers from a closed allowlist. They cannot write to the client system.

## Optional online flow

The client implements the narrow asynchronous gateway contract: submit an approved report and session, poll for an evidence-backed result, reject plaintext HTTP, bound request and response sizes, and save an `online_agent` assessment into the session history. The repository now contains the matching server; it remains outside the image and is deployed separately behind HTTPS.

Supplying a gateway URL is not sufficient to upload. The technician must also provide a removable device-token file and `--approve-upload`. This makes online submission an explicit action after reviewing the report and free-text session context.

The MVP gateway accepts only the same closed read-only operation catalog as the offline preflight. A later repair executor must remain a separate local policy layer that rejects unknown operations, previews exact effects, requires confirmation for mutations, and appends results to an audit log.

The OpenAI API key never crosses the gateway boundary. A removable technician device token will be separately enrolled, revocable, rate-limited, and stored outside the immutable ISO.

## Deliberately provisional heuristics

Temperature and wear thresholds in the offline preflight are triage signals, not vendor-independent failure criteria. Controller reporting, device class, and manufacturer limits vary. The agent therefore assigns medium confidence and asks for vendor-specific verification rather than declaring the disk failed.
