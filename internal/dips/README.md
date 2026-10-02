# DIPs

The [`sfa-dips` service](../../cmd/sfa-dips/main.go) runs an HTTP API and a
Temporal worker for the `create-dip` workflow in one process. It uses ACTApro
document metadata and AIPs in an Archivematica Storage Service (AMSS) to prepare
Dissemination Information Packages (DIPs), and stores their details in a
database.

This document is a shared reference for the teams involved in the DIP
integration, particularly ACTApro and SFA. It explains the workflow steps,
failure conditions, and metadata parsing so these teams can review and confirm
the initial implementation approach.

## API

See the [OpenAPI specification](api/gen/http/openapi3.yaml) for request and
response schemas.

- `POST /dips`: accepts an ACTApro `docKey`, creates a database record for the
  DIP with status `queued`, starts the DIP creation workflow, then returns the
  unique DIP ID in the `id` field.
- `GET /dips/{id}`: returns the recorded DIP creation status, timestamps,
  error message and object key, when available.
- `GET /livez`: checks that the API server is running and able to respond to
  requests (is "alive").

For `POST /dips`, invalid requests return `400 Bad Request`, authentication
failures return `401 Unauthorized`, and errors creating the database record or
starting the workflow return `500 Internal Server Error`.

A `202 Accepted` response confirms that the workflow start was accepted;
processing continues asynchronously. Use the returned DIP ID with
`GET /dips/{id}` to check progress and any subsequent processing failure. The
returned ID identifies the DIP; the Temporal workflow has a separate ID.

## Creation workflow

The [`create-dip` workflow](workflows/create_dip.go) runs these activities in
order. Only retryable errors are retried, according to each activity's retry
policy. Processing stops on a non-retryable error or when the allowed attempts
are exhausted, with the aggregation and cleanup exceptions below.

1. `update-dip`: sets the DIP status to `in progress` and sets the DIP creation
   start time.
2. `get-actapro-document`: fetches an ACTApro document by its `docKey` that
   lists the AIPs that contain files to be exported to the DIP, then collects
   the unique, non-empty `AIP_ID` UUIDs from the `AIP_ID_Gp` groups. One or more
   invalid AIP UUIDs will cause the entire `create-dip` workflow to fail, with
   no retries, and the invalid IDs will be reported in the DIP error message. If
   the document contains no AIP UUIDs, the workflow will also fail immediately,
   with no retries.
3. `get-aip-path` (each AIP): gets the AIP's current AMSS path. Missing packages
   or paths fail the activity. All AIPs are attempted before reporting combined
   errors.
4. `create-actapro-export`: starts an ACTApro export of a metadata document to
   be included in the DIP. The export response returns an export ID used to
   check the export progress. A missing export ID fails the workflow and returns
   an error.
5. `poll-actapro-export-status`: polls the status of the ACTApro document export
   started in the previous step until it is `COMPLETED`, `FAILED` or `CANCELED`.
   The workflow continues only after a successful export — otherwise it fails
   and returns the export logs, if they are available.
6. `download-actapro-export`: saves the ACTApro metadata export to the local DIP
   working directory (`<workingDir>/<DIP UUID>/metadata.xml`).
7. `xml-validate`: validates `metadata.xml` against the XML Schema Definition
   (XSD) configured at `xsdPath`. Both activity errors and reported validation
   failures cause the workflow to fail.
8. `fetch-aip-file` (each AIP): downloads the METS file into the working
   directory. All METS file downloads are attempted before reporting combined
   errors.
9. `parse-dip-metadata`: builds the file list and matches files to AIPs as
   described below. Parsing or matching errors fail both the activity and the
   workflow, without retrying the activity. An empty file list also fails the
   workflow.
10. `prepare-dip`: creates `<workingDir>/<DIP UUID>/DIP_<DIP UUID>`, copies all
    `.xsd` files from the configured schema's directory into `header/xsd`, and
    moves the export into `header/metadata.xml`.
11. `fetch-aip-file` (each content file): downloads from the matched AIP using
    `AIPPath`, which already includes the AIP directory name, into
    `<DIP directory>/<DIPPath>`, creating missing parent directories. A failed
    download stops processing once retries are exhausted.
12. `archive-zip`: creates `DIP_<DIP UUID>.zip` alongside the DIP directory.
13. `bucket-upload`: uploads the ZIP to the configured bucket under its filename
    and stores the returned object key. An upload failure fails the workflow.
14. `remove-paths`: removes the DIP working directory on session exit, including
    on failure. Cleanup errors are logged without failing the workflow.
15. `update-dip`: records completion time and `done` or `failed`, with the
    object key or error message.

For all ACTApro activities, a potentially transient API error response
(`409 Conflict`, `423 Locked`, `500 Internal Server Error`) causes the activity
to retry according to its retry policy. A permanent error response
(`400 Bad Request`, `403 Forbidden`, `404 Not Found`) fails the workflow
immediately.

Whenever the workflow fails, no further retrieval or export steps run, but the
final update to record the failure is still attempted.

Downloads, validation, parsing, DIP preparation, ZIP creation and upload run in
a Temporal session so they share one worker's local files. Worker loss can
restart the session from the export download. If the session cannot be created
or recovered, the workflow fails.

Cleanup and the final database update are attempted even after workflow
cancellation. If the final update fails, the database may retain an earlier
status.

Successful upload sets the DIP's object key to `DIP_<DIP UUID>.zip` and
completes with `done`. Session cleanup removes the local ZIP; the uploaded
archive remains in the configured bucket.

### Metadata parsing and file paths

[`parse-dip-metadata`](activities/parse_metadata.go) combines the ACTApro XML
export with the downloaded AIP METS files:

1. Read `inhaltsverzeichnis` and select its top-level `ordner` elements whose
   `name` is `content`. Recursively collect their `datei` elements; other
   top-level folders, including `header`, are excluded.
2. For each file, store the `datei/@id` as `DateiID`, `pruefsumme` as
   `Checksum`, and `pruefalgorithmus` as `ChecksumAlgorithm`. The checksums are
   not verified against the file contents by this activity.
3. Build `DIPPath`, the file's destination path within the DIP, by joining the
   folder `name` values from `content` down to the file's `name` in the ACTApro
   export.
4. Search the METS files in AIP order for PREMIS `object` elements. Match an
   `objectIdentifier` whose `objectIdentifierType` is exactly `local` and whose
   `objectIdentifierValue` exactly equals the file's `DateiID`.
5. Take the matched object's PREMIS `originalName`, remove a leading
   `%transferDirectory%`, and store the result as `AIPPath` together with the
   source `AIPUUID`. Objects with an empty resulting path are skipped. The first
   match wins; later objects or AIPs cannot replace it.

For example, a file with ID `_one` and DIP path `content/dossier/report.pdf`
can match a PREMIS local identifier `_one` with original name
`%transferDirectory%data/objects/report.pdf`. Its source is then
`data/objects/report.pdf` in the matched AIP, while its DIP destination remains
`content/dossier/report.pdf`.

Every selected file must match a PREMIS object; unmatched files cause the
activity to fail with an error listing each missing file ID and its DIP path. If
the PREMIS XML document is unreadable or malformed the workflow will also fail
with an error. Each file is matched to one, and only one, PREMIS object —
subsequent matches are ignored.
