# Security policy

## Secrets

Do not commit or embed:

- OpenAI API keys;
- VPN private keys or exported VPN profiles;
- production backend credentials;
- reusable bearer tokens;
- client reports, event logs, dumps, registry hives, or copied files.

The rescue client uses a removable raw device token; the gateway stores only configured SHA-256 digests. Development configuration must use placeholders only.

## Diagnostic data

Diagnostic output may contain personal or commercially sensitive data. Collection must follow data minimization:

- exclude usernames, document paths, Wi-Fi profiles, browser data, and file contents by default;
- label potentially sensitive fields in the contract;
- show a local preview before upload;
- make local deletion and retention behavior explicit;
- redact serials and identifiers when they are not needed for diagnosis.

The MVP gateway removes the hostname and prior derived assessments again before analysis, sends only report facts plus technician symptoms/answers to the provider, disables provider-side response storage, keeps jobs in memory only, and discards original request context after completion. Technician-entered symptom text is still potentially personal data and requires explicit review before upload.

## Model boundary

Treat model output and retrieved pages as untrusted input:

- accept only evidence paths present in the approved report or session;
- accept only source URLs returned by the configured retrieval provider;
- restrict retrieval to reviewed official domains;
- accept only closed read-only operation identifiers in the MVP;
- never return provider error bodies or arbitrary commands to the client.

## Repair operations

Destructive or state-changing operations must never execute from an AI response directly. Each operation needs a typed implementation, validation, a human-readable preview, explicit technician confirmation, and an audit record.
