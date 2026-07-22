# EffexorWinPE gateway MVP

The gateway is the only component that talks to a model provider. WinPE and the portable Windows client send an explicitly approved report and diagnostic session to a narrow asynchronous API; provider credentials never enter the ISO.

## Implemented flow

1. The client submits `diagnostic_report`, `session`, and `technician_approved=true` over HTTPS.
2. The gateway authenticates a revocable device token by comparing its SHA-256 digest.
3. The hostname and any prior derived assessment are removed again at the server boundary; the provider receives report facts plus technician symptoms/answers, not session IDs or the service event log.
4. The OpenAI Responses provider receives the approved evidence, a closed evidence-path catalog, and a strict output schema.
5. Optional web search is restricted to configured official vendor domains. The request sets `store=false`.
6. The gateway accepts only known evidence paths, URLs actually returned by retrieval, and read-only operation identifiers.
7. The client polls until it receives an `online_agent` assessment or a generic failed state.
8. Original report/session context is discarded from memory when analysis finishes; the result expires with the in-memory job TTL.

The implementation follows the official [Responses API](https://developers.openai.com/api/docs/guides/migrate-to-responses), [web search](https://developers.openai.com/api/docs/guides/tools-web-search), and [Structured Outputs](https://developers.openai.com/api/docs/guides/structured-outputs) contracts. The model provider sits behind a Go interface so another provider can be added without changing the WinPE client protocol.

## Create credentials

Generate one random raw token for the technician USB and store only its digest on the server:

```bash
umask 077
openssl rand -hex 32 > device-token.txt
tr -d '\r\n' < device-token.txt | sha256sum | cut -d' ' -f1 > deploy/gateway/secrets/device-token-sha256.txt
```

Copy `device-token.txt` to protected removable storage after deployment, then remove the build-host copy. Put the provider key in `deploy/gateway/secrets/openai-api-key.txt`; never place either raw secret in Git, a Docker image, an ISO, or a screenshot.

## Run behind HTTPS

Set a current model explicitly and start the example Compose service:

```bash
export EFFEXORWINPE_MODEL='your-supported-model'
docker compose -f deploy/gateway/compose.gateway.example.yml up -d --build
curl http://127.0.0.1:8080/healthz
```

The container intentionally serves internal HTTP and binds the host loopback interface. Terminate public TLS at Nginx or another reverse proxy:

```nginx
location /rescue/v1/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto https;
    client_max_body_size 8m;
}
```

The client gateway URL is then `https://your-host.example/rescue/v1`. Do not expose port `8080` publicly and do not weaken the client's HTTPS requirement.

## Current limitations

- Jobs and results are memory-only and disappear on restart.
- Enrollment is manual: create/revoke tokens by editing the digest file and restarting the service.
- The operation catalog is read-only; no repair is executed from a model response.
- Web retrieval is bounded to the configured domains and can miss a small OEM or component vendor until its official domain is reviewed and added.
- Live provider behavior still needs an end-to-end test with a real API key after review of the exact report being uploaded.
