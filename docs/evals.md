# Agent loop evals

Reproducible fixtures exercise the multi-step diagnostic agent loop without calling a live model provider.

## Layout

- Fixtures: `internal/agenteval/testdata/fixtures/*.json`
- Harness: `internal/agenteval`
- Deterministic mock provider: scripted `provider_rounds` plus an optional `evidence_catalog`

Each fixture is anonymized (example OEM/model names only) and declares:

- `expected.final_state`
- `expected.finding_ids`
- `expected.forbidden_claims` — substrings that must not appear in the marshaled result
- `expected.required_evidence_refs`
- `expected.allowed_operation_ids`

## Scenarios

The minimum set covers:

1. `healthy`
2. `failing-hdd`
3. `missing-smart`
4. `bitlocker-unavailable`
5. `multiple-windows`
6. `bcd-mismatch`
7. `no-dhcp`
8. `dual-boot`
9. `insufficient-evidence`
10. `corrupt-windows`

## Running

```bash
go test ./internal/agenteval -count=1
```

Scenario fixtures use harness name `scenario-eval`. Policy regressions use the separate `TestPolicyRegressionHarness` (`policy-regression`).

Optional machine-readable report path:

```bash
EFFEXORWINPE_EVAL_OUT=/tmp/effexorwinpe-eval.json go test ./internal/agenteval -count=1 -run TestEvalFixtures
```

The harness writes JSON with `schema_version`, `harness`, pass/fail counts, per-case finding IDs, operations seen, audit kinds, and failure strings. Fixtures require exact `finding_ids`, `final_round`, and optional `failure_code` / `block_code`.

## Regenerating fixtures

```bash
go run internal/agenteval/testdata/genfixtures/main.go
```

Keep provider rounds free of shell/PowerShell text and outside the closed evidence allowlist. Prefer low-confidence language when SMART, BitLocker, or other providers are missing—never treat absence as proof of health.
