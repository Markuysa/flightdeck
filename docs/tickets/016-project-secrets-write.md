---
id: 16
title: Project secrets — write endpoint and register-with-tokens
role: dev
depends: [8, 9, 13]
status: done
---
Close the gap where a project's routine/GitHub tokens can only be set by stopping the server
and calling Go. Add the write side of secrets to the API and the UI, keeping the read side's
guarantee: tokens go in, never come back out.

## Acceptance criteria
- [ ] `POST /api/projects` accepts optional `routine_token` and `github_token`; on create they
      are stored via `registry.SetSecrets`, never echoed in the response.
- [ ] `PUT /api/projects/{id}/secrets` sets/updates a project's tokens (204); `GET
      /api/projects/{id}/secrets` returns only booleans (`routine_token_set`,
      `github_token_set`), never the values.
- [ ] No endpoint ever returns a token value; the secret-redaction e2e/handler test covers the
      new routes.
- [ ] The register dialog has optional token fields; a project card offers a way to set/update
      tokens later. Token inputs are `type=password`, never logged, never kept in state longer
      than the request.
- [ ] lint + tests pass (Go and UI).

## Handoff

The write side of secrets is now in the API and UI; the read guarantee holds — tokens go in,
never come back out.

**API (auth-protected):**
- `POST /api/projects` now accepts optional `routine_token` / `github_token`; when present they
  are stored via `registry.SetSecrets` after the project is added. The response is still the
  bare `core.Project` — no token echoed.
- `PUT /api/projects/{id}/secrets` — body `{routine_token?, github_token?}`. **Read-current →
  overlay non-empty fields → write**, so you can rotate one token without resending the other;
  an empty/omitted field is left unchanged (never blanked). `204`, no body. `404` unknown project.
- `GET /api/projects/{id}/secrets` — returns **only** `{routine_token_set, github_token_set}`
  booleans (true when the stored value is non-empty). Never the values.
- The api `ProjectRegistry` interface gained `Secrets`/`SetSecrets` (`*registry.Store` already
  implements them); the fake registry mirrors it. The secret-redaction test now also exercises
  `POST /api/projects` (with tokens), `PUT .../secrets` and `GET .../secrets`.

**UI:**
- `RegisterProjectDialog` has optional Routine token / GitHub token fields (`type=password`,
  `autoComplete=off`), sent in `createProject` only when non-empty.
- `ManageTokensDialog` (opened from a key button on `ProjectCard`) sets/updates tokens via
  `PUT .../secrets` and shows each token's set/not-set state from `GET .../secrets` — never a
  value. Same dialog conventions (conditional mount, Escape/backdrop, inline errors).
- `lib/api.ts`: `createProject` passes tokens; new `getSecretsStatus(id)` and `setSecrets(id, body)`.
  `lib/types.ts`: `CreateProjectRequest` gains optional tokens; `SecretsStatus` / `SetSecretsRequest`.

Tokens are never logged, never persisted in UI state beyond submit, never returned. Test tokens
are fake/short. This closes the "no way to set tokens through the running server" gap (US-7).
