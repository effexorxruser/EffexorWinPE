# Gateway secrets

This directory is deny-listed by `.gitignore` and `.dockerignore`. Keep only local deployment files here:

- `device-token-sha256.txt`: one SHA-256 digest of a raw technician token per line;
- `openai-api-key.txt`: the server-side OpenAI API key.

The raw device token belongs on removable technician storage, not beside its server digest and never inside the ISO.
