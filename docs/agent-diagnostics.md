# Diagnostic agent boundary

`effexorwinpe-agent.exe` is the first local layer of the technician assistant. In this milestone it runs an offline deterministic preflight over `diagnostic-report.schema.json` and writes a `diagnosis.schema.json` assessment.

It is intentionally useful without internet access, but it is not presented as the final AI diagnosis. Model-backed reasoning, the repair knowledge base, device-token policy, rate limits, and audit storage remain behind the dedicated EffexorWinPE gateway.

## Current flow

1. `effexorwinpe-collector.exe` creates a structured read-only report.
2. `effexorwinpe-agent.exe` validates the report version.
3. Conservative rules correlate missing sources, Windows installations, firmware/BCD visibility, storage health, reliability counters, and BitLocker access.
4. The result contains findings, confidence, evidence references, focused follow-up questions, and typed read-only next steps.
5. The technician receives an explicit limitation instead of a false `healthy` result when evidence is incomplete.

The executable never accepts or emits arbitrary command strings. Offline next steps are operation identifiers from a closed allowlist. They cannot write to the client system.

## Later online flow

The same assessment contract will carry the gateway result in `online_agent` mode. The backend may propose a broader typed operation, but the local policy layer must reject unknown operations, preview the exact effect, require confirmation for mutations, and append the result to an audit log.

The OpenAI API key never crosses the gateway boundary. A removable technician device token will be separately enrolled, revocable, rate-limited, and stored outside the immutable ISO.

## Deliberately provisional heuristics

Temperature and wear thresholds in the offline preflight are triage signals, not vendor-independent failure criteria. Controller reporting, device class, and manufacturer limits vary. The agent therefore assigns medium confidence and asks for vendor-specific verification rather than declaring the disk failed.
