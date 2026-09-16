# ACTApro client

This package uses an [ogen](https://github.com/ogen-go/ogen) client generated from
the ACTApro OpenAPI specification. The specification cannot be published in this
repository. Download it locally and correct its error response media types and
creation date schemas before regenerating the client.

## Generate the client

Run the commands below from the repository root.

1. Download the OpenAPI specification from your ACTApro deployment's API
   documentation. Save it as `actapro/openapi3.json`, creating the directory if
   necessary. The root `actapro/` directory is ignored by Git; keep both the
   downloaded and transformed specifications there.

2. Create a copy at `actapro/openapi3.local.json`. In this copy, change the
   `content` media-type key for error responses (HTTP 4xx and 5xx) from
   `application/hal+json` to `application/json`, preserving the associated schemas.
   These entries are under `paths.<path>.<method>.responses.<status>.content`.
   Leave responses already using `application/json` and successful responses
   unchanged, including the binary export's `application/octet-stream` response.

   ACTApro sends errors as `application/json`. Without this correction, the
   generated client expects `application/hal+json` and reports a content-type
   mismatch instead of decoding the API error response.

3. In the local copy, remove `format: date-time` from the `crdate` property of
   `components.schemas.MassOperationInfoDTO` and
   `components.schemas.MassOperationLogDTO`, keeping `type: string`.

   ACTApro returns creation dates without a timezone, such as
   `2026-09-15T09:15:10`. The generated RFC3339 decoder rejects these values.
   Keeping them as strings preserves the server's value without assuming a
   timezone.

4. Generate the client with the project's ogen version, configured in
   [`.bine.json`](../../.bine.json), pointing it to the transformed local spec:

   ```sh
   go tool bine run ogen \
     --config internal/actapro/gen/ogen.yml \
     --target internal/actapro/gen \
     --package gen \
     --clean \
     actapro/openapi3.local.json
   ```

   Bine installs the configured ogen version if needed. Commit the generated
   client changes under `internal/actapro/gen/`; keep the specifications local.

## Generated operations

The filters in [`gen/ogen.yml`](gen/ogen.yml) generate a client for only these five
operations:

| Client method | HTTP endpoint |
| --- | --- |
| `GetDocument` | `GET /document/{key}` |
| `CreateMassOperationExport` | `POST /massoperation/export` |
| `GetExportFile` | `GET /massoperation/export/binary` |
| `GetMassOperationInfo` | `GET /massoperation/{id}/info` |
| `GetMassOperationLogs` | `GET /massoperation/{id}/logs` |

Update the filters if additional operations are needed.
