# APIS Mock Server

Mock API service generated from `../../apis/openapi3.json` using `ogen`.

This mock server was developed with AI assistance using Codex GPT-5.4. The
generated code still comes from the shared OpenAPI spec, and the behavior
lives in `internal/mock/server.go`.

## Intent

This service is a development and testing mock for APIS.

It gives preprocessing-sfa a real HTTP endpoint that can run locally and in
Tilt/Kubernetes so the worker can exercise the APIS integration against an
actual server process.

It is not a production-compatible APIS implementation. It only models the
parts of APIS that preprocessing-sfa currently needs:

- creating import tasks
- polling analysis status
- cancelling conflicted tasks
- starting import runs
- polling final import status

## Behavior source

The transport layer for this mock is generated from the shared APIS OpenAPI
spec:

- `../../apis/openapi3.json`

The mocked lifecycle and status behavior in `internal/mock/server.go` is based
on:

- issue `#173`: preprocessing APIS integration flow
- issue `#174`: poststorage APIS integration flow

In practice that means the mock follows the current expectations discussed in
those issues for how `ImportTasks` analysis moves through its states, how
metadata conflicts are surfaced, how cancellation works, and how import
completion is reported back through `GET /api/ImportTasks/{id}/status`.

## Why `ogen` for this spec

- The spec is OpenAPI `3.0.1`, and the mock needs both generated transport
  types and generated server plumbing from that shared contract.
- `ogen` generated a clean, compilable Go server for this document as-is,
  including the request/response unions and security hooks used by the mock.
- The generated layer stays small and mechanical, while endpoint behavior stays
  in normal Go code in `internal/mock/server.go`.

Options considered and rejected:

- `oapi-codegen`:
  with this spec, strict-server generation produced broken output around the
  response variants for endpoints that expose multiple content types such as
  `text/plain` and `application/json`. That made it a poor fit for a mock that
  should regenerate cleanly from the shared spec.
- Hand-written server types and routes:
  this would have given full control, but it would also duplicate the contract
  already described in the OpenAPI document and make it much easier for the
  mock to drift away from the client and the spec over time.
- A fake layered on top of the generated client only:
  that would help with unit tests, but not with local development where we want
  a real HTTP service inside Tilt/Kubernetes that other processes can talk to.

## Implementation Shape

The mock is intentionally small:

- one in-memory task record per `POST /api/ImportTasks`
- one optional import run per task
- `GET /api/ImportTasks/{id}/status` is the only endpoint that advances state
- terminal analysis and import results are configured with environment
  variables

That shape keeps the implementation easy to read and easy to tweak for local
development, while still matching the workflow semantics described in the APIS
integration issues.

## What This Mock Is For

- local development against a running HTTP API
- Tilt/Kubernetes environment testing
- manual workflow testing for APIS-related preprocessing/poststorage
- lightweight client integration testing

## What This Mock Is Not For

- validating exact APIS production parity
- full contract certification beyond the generated transport layer
- realistic authn/authz behavior
- persistence or long-lived state across restarts
- simulating every status detail or auxiliary field exposed by APIS

## Project layout

- `../../apis/openapi3.json`: source OpenAPI spec (shared)
- `internal/gen/*.go`: generated server/types/router/security
- `internal/mock/server.go`: mock behavior customization
- `cmd/mockapi/main.go`: executable entrypoint

## Regenerate after spec changes

```bash
(cd ../.. && make gen-apis-mock)
```

This regenerates all files in `internal/gen` from `../../apis/openapi3.json`.

## Run locally

```bash
go run ./cmd/mockapi
```

Server listens on `:8080` by default.

The generated security model expects a bearer token on all endpoints. The
health endpoint returns a minimal `OK` response.

## Build binary

```bash
go build -o bin/mockapi ./cmd/mockapi
```

## Docker

```bash
docker build -t apis-mock:latest .
docker run --rm -p 8080:8080 apis-mock:latest
docker run --rm -p 8080:8080 \
  -e MOCK_ANALYSIS_RESULT=Konflikte \
  -e MOCK_IMPORT_RESULT=Fehler \
  apis-mock:latest
```

## Mock tuning switches

- `MOCK_AUTH_TOKEN=some-token` -> expected bearer token (default: `mock-token`)
- `PORT=9090` -> change listening port
- `MOCK_ANALYSIS_RESULT=Konflikte` -> terminal `analysisResult` assigned to
  new import tasks (fallback if unset: `AlleNeu`; valid values: `AlleNeu`,
  `AlleGleich`, `Konflikte`, `Fehler`)
- `MOCK_IMPORT_RESULT=Fehler` -> terminal `importResult` assigned to import
  runs (fallback if unset: `Erfolgreich`; valid values: `Erfolgreich`,
  `Fehler`)

The configured result values apply to every task and import run created during
the mock server process.

The result variables are also read by the Tilt-managed Kubernetes Deployment.
Add them to the repo-local `.tilt.env` to change the mock behavior in a Tilt
environment:

```bash
MOCK_ANALYSIS_RESULT=Konflikte
MOCK_IMPORT_RESULT=Fehler
```

## What Is Intentionally Simplified

- all state is in memory and is lost on restart
- task and import statuses advance in a short deterministic sequence when polled
- auth is a single bearer-token equality check against `MOCK_AUTH_TOKEN`
- `/api/Healthz` returns a minimal `OK` response
- percentages and document counts are omitted because the current integration
  only cares about statuses and terminal results
- only the APIS behaviors currently needed by preprocessing-sfa are modeled

## Notes on behavior

- `POST /api/ImportTasks` creates in-memory task IDs like `task-000001`.
- `GET /api/ImportTasks/{id}/status` expects a task ID that was previously
  created by `POST /api/ImportTasks`.
- `GET /api/ImportTasks/{id}/status` is the primary lifecycle endpoint and
  advances both analysis and import state.
- The shared spec only models `200` for that status endpoint. This mock still
  fails fast for unknown task IDs instead of inventing a task, because that is
  more useful during development.
- `POST /api/ImportTasks/{id}/importRuns` creates in-memory run IDs like
  `run-000001`.
- `GET /api/ImportTasks/{id}/importRuns/{runId}/status` mirrors the current
  import state but does not drive it.
- The mock only supports one import run per task because that is enough for
  the current integration work.
- Successful analysis results that can move on to import are `AlleNeu` and
  `AlleGleich`.
- Analysis results `Konflikte` and `Fehler` stop the flow before import,
  matching the current workflow expectations.
- Cancelling a task after analysis keeps the completed `analysisResult` visible.

## Example Flows

Create a task:

```bash
curl \
  -H 'Authorization: Bearer mock-token' \
  -F 'file=@metadata.xml' \
  -F 'sipType=BornDigitalSIP' \
  -F 'username=archivist@example.com' \
  http://localhost:8080/api/ImportTasks
```

Run with a metadata conflict result:

```bash
MOCK_ANALYSIS_RESULT=Konflikte go run ./cmd/mockapi
```

Poll task status:

```bash
curl \
  -H 'Authorization: Bearer mock-token' \
  http://localhost:8080/api/ImportTasks/task-000001/status
```

Run with a failed import result:

```bash
MOCK_IMPORT_RESULT=Fehler go run ./cmd/mockapi
```
