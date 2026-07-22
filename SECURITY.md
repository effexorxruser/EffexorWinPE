# Security policy

## Secrets

Do not commit or embed:

- OpenAI API keys;
- VPN private keys or exported VPN profiles;
- production backend credentials;
- reusable bearer tokens;
- client reports, event logs, dumps, registry hives, or copied files.

The future rescue client will use a revocable, scope-limited device credential. Development configuration must use placeholders only.

## Diagnostic data

Diagnostic output may contain personal or commercially sensitive data. Collection must follow data minimization:

- exclude usernames, document paths, Wi-Fi profiles, browser data, and file contents by default;
- label potentially sensitive fields in the contract;
- show a local preview before upload;
- make local deletion and retention behavior explicit;
- redact serials and identifiers when they are not needed for diagnosis.

## Repair operations

Destructive or state-changing operations must never execute from an AI response directly. Each operation needs a typed implementation, validation, a human-readable preview, explicit technician confirmation, and an audit record.
