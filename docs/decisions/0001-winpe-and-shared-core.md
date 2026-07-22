# ADR 0001: WinPE client with a shared agent core

- Status: accepted
- Date: 2026-07-22

## Context

The rescue environment needs reliable offline Windows diagnostics and optional AI assistance. Embedding a general agent and provider API key into distributable boot media would create credential, update, cost-control, and safety problems.

## Decision

Use Microsoft WinPE as the minimal Windows repair environment. Keep collection and approved typed execution local. Send technician-reviewed reports through a narrow HTTPS gateway to the existing shared ANP agent core. Store provider credentials only on the backend.

Wi-Fi and a full VPN client are not MVP dependencies. Wired Ethernet, phone USB tethering when supported, or an external VPN router are preferred. A separate portable full-Windows environment may be added later for software that requires ordinary Windows services and drivers.

## Consequences

- A lost image does not expose the OpenAI key.
- Device access can be revoked centrally.
- The rescue client remains useful offline.
- Backend availability is required only for AI features.
- The client/backend contract must be versioned and privacy reviewed.
