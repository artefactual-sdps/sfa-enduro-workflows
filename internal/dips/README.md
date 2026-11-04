# DIPs

The [`sfa-dips` service](../../cmd/sfa-dips/main.go) runs an HTTP API and a
Temporal worker for the `create-dip` workflow in one process. It uses ACTApro
document metadata and AIPs in an Archivematica Storage Service (AMSS) to prepare
Dissemination Information Packages (DIPs), and stores their details in a
database.

This README is intended for developers integrating with the DIP API,
administrators configuring and operating the service, and the ACTApro and SFA
teams reviewing the integration. It documents API behavior, workflow steps,
failure conditions, metadata parsing, and service configuration.

## Table of contents

- [API](#api)
- [Creation workflow](#creation-workflow)
- [Configuration](#configuration)

## API

See the [OpenAPI specification](api/gen/http/openapi3.yaml) for request and
response schemas.

- `POST /dips`: accepts an ACTApro `docKey` and returns the DIP generation ID
  in the `id` field. It returns the newest DIP that is queued, in progress, or
  cached and still available. If no eligible DIP is available a new DIP is
  generated. Set `ignoreCache: true` to force the creation of a new DIP instead of
  using a cached DIP. If a DIP generation is queued or in-progress then
  `ignoreCache: true` is ignored and the previous DIP UUID is returned, to prevent
  multiple generations of the same DIP over a short time span.
- `GET /dips/{id}`: returns the recorded DIP creation status, timestamps,
  error message and object key, when available. The optional `expires_at` field
  gives the date and time when the cached DIP expires.
- `GET /livez`: checks that the API server is running and able to respond to
  requests (is "alive").

For `POST /dips`, invalid requests return `400 Bad Request`, authentication
failures return `401 Unauthorized`, and errors accessing the database or
starting the workflow return `500 Internal Server Error`.

A `202 Accepted` response returns a DIP UUID, which may identify a cached or new
DIP; if a new DIP is created processing continues asynchronously. Use the
returned DIP ID with `GET /dips/{id}` to check progress and any subsequent
processing failure. The returned ID identifies the DIP; the Temporal workflow
has a separate ID. Several POST requests can return the same DIP ID.

If a cached DIP ID is returned, the expiry time of the cache may be extended but
never shortened. A DIP GET request or ZIP file download does not extend the cache
expiry.

## Creation workflow

The [`create-dip` workflow](workflows/create_dip.go) runs these activities in
order. Each activity uses its own retry policy. Steps 4–7 also run as a group
with up to three attempts per session, independently of whether an individual
activity error is retryable. Other failure handling is described below.

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
   check the export progress. A missing export ID fails the current export
   attempt.
5. `poll-actapro-export-status`: polls the status of the ACTApro document export
   started in the previous step until it is `COMPLETED`, `FAILED` or `CANCELED`.
   Downloading proceeds only after a successful export. A `FAILED` or
   `CANCELED` status fails the current export attempt and includes any available
   export logs in its error. The next group attempt requests a fresh export.
6. `download-actapro-export`: saves the ACTApro metadata export to the local DIP
   working directory (`<workingDir>/<DIP UUID>/metadata.xml`).
7. `xml-validate`: validates `metadata.xml` against the XML Schema Definition
   (XSD) `arelda.xsd` in the configured eCH-0160 v1.1 schema directory (`xsdDir`).
   Reported validation failures, activity errors from steps 4–7, and failed or
   canceled exports repeat those steps, requesting a fresh export each time.
   All share a limit of three export attempts per session. After the third
   unsuccessful attempt, the workflow fails with the final error.
8. `fetch-aip-file` (each AIP): downloads the METS file into the working
   directory. All METS file downloads are attempted before reporting combined
   errors.
9. `parse-dip-metadata`: builds separate content and header file lists and
   matches content files to AIPs as described below. Parsing or matching errors
   fail both the activity and the workflow, without retrying the activity. An
   empty content or header file list also fails the workflow.
10. `prepare-dip`: creates `header` under
    `<workingDir>/<DIP UUID>/DIP_<DIP UUID>`. For declared files under
    `header/xsd`, it finds their sources at the same relative paths under the
    configured schema directory (`xsdDir`) and copies them to their DIP paths,
    creating parent directories as needed, then validates the copied files'
    checksums. After checking all schemas, it reports missing files from the
    configured schema directory (`xsdDir`), checksum validation or copy failures,
    and all unexpected header files outside `header/xsd` together. These errors
    fail both the activity and the workflow, without retrying the activity. On
    success, it moves the export into `header/metadata.xml`.
11. `fetch-aip-file` (each content file): downloads from the matched AIP using
    `AIPPath`, which already includes the AIP directory name, into
    `<DIP directory>/<DIPPath>`, creating missing parent directories. Each
    downloaded file is validated using the checksum details from the ACTApro
    metadata. Download or checksum validation errors cause the file to be
    downloaded again, up to 3 attempts in total.
12. `archive-zip`: creates `DIP_<DIP UUID>.zip` alongside the DIP directory.
13. `bucket-upload`: uploads the ZIP to the configured bucket under its filename,
    using the object key `DIP_<DIP UUID>.zip`. An upload failure fails the workflow.
14. `remove-paths`: removes the DIP working directory on session exit, including
    on failure. Cleanup errors are logged without failing the workflow.
15. `update-dip`: records completion time and `stored` or `failed`, with the
    object key or error message. A successful DIP generation also records an
    expiry date when retention is finite.
16. After successful DIP creation and the final update, wait until the DIP
    expires. Before deletion, `update-dip` checks the stored expiry date in case
    a POST request extended retention. If it has changed, the workflow waits
    until the new expiry date. A DIP without an expiry date is retained
    indefinitely. Once expired, the DIP is marked `deleted` and its object key
    is cleared. Then `bucket-delete` removes the uploaded ZIP. Deletion errors
    fail the workflow without changing the DIP's `deleted` status.

For all ACTApro activities, a potentially transient API error response
(`409 Conflict`, `423 Locked`, `500 Internal Server Error`) causes the activity
to retry according to its retry policy. A permanent error response
(`400 Bad Request`, `403 Forbidden`, `404 Not Found`) stop that activity's
retries. Within steps 4–7, they still allow a fresh group attempt; outside the
export group, they fail the workflow.

During steps 4–7, the activity's own retries run before another export is
requested. For example, if downloading an export fails with a retryable error
on all three download activity attempts, the next export attempt starts from
`create-actapro-export`. Thus an activity can run up to nine times across three
export attempts in one session. The workflow waits 10 seconds between export
attempts.

Whenever the workflow fails, no further retrieval or export steps run, but the
final update to record the failure is still attempted.

Export creation and polling, downloads, validation, parsing, DIP preparation,
ZIP creation and upload run in a Temporal session so they share one worker's
local files. Worker loss can restart the session from the export creation. If
the session cannot be created or recovered, the workflow fails.

Cleanup and the final database update are attempted even after workflow
cancellation. If the final update fails, the database may retain an earlier
status. In that case, any uploaded DIP archive is deleted immediately. Failed
DIP generation also triggers deletion if an upload was attempted. Cancellation
stops the retention wait and deletion for a successfully completed DIP.

Successful upload sets the DIP's object key to `DIP_<DIP UUID>.zip` and
completes with `stored`. Session cleanup removes the local ZIP. The uploaded
archive remains in the configured bucket until its expiry date, which later
POST requests may extend.

### Metadata parsing and file paths

[`parse-dip-metadata`](activities/parse_metadata.go) combines the ACTApro XML
export with the downloaded AIP METS files:

1. Read `inhaltsverzeichnis` and recursively collect `datei` elements under
   `content` and `header` into `ContentFiles` and `HeaderFiles`, respectively.
   Other top-level folders are excluded.
2. For each file, store the `datei/@id` as `DateiID`, `pruefsumme` as
   `Checksum`, and `pruefalgorithmus` as `ChecksumAlgorithm`. The checksums are
   not verified against the file contents by this activity.
3. Build `DIPPath`, the file's destination path within the DIP, by joining the
   folder `name` values from `content` or `header` down to the file's `name` in
   the ACTApro export.
4. For content files, search the METS files in AIP order for PREMIS `object`
   elements. Match an `objectIdentifier` whose `objectIdentifierType` is
   exactly `local` and whose `objectIdentifierValue` exactly equals the file's
   `DateiID`. Header files do not require AIP matches; `prepare-dip` supplies
   schemas locally and rejects other header files.
5. Take the matched object's PREMIS `originalName`, remove a leading
   `%transferDirectory%`, and store the result as `AIPPath` together with the
   source `AIPUUID`. Objects with an empty resulting path are skipped. The first
   match wins; later objects or AIPs cannot replace it.

For example, a file with ID `_one` and DIP path `content/dossier/report.pdf`
can match a PREMIS local identifier `_one` with original name
`%transferDirectory%data/objects/report.pdf`. Its source is then
`data/objects/report.pdf` in the matched AIP, while its DIP destination remains
`content/dossier/report.pdf`.

Every content file must match a PREMIS object; unmatched files cause the
activity to fail with an error listing each missing file ID and its DIP path. If
the PREMIS XML document is unreadable or malformed the workflow will also fail
with an error. Each file is matched to one, and only one, PREMIS object —
subsequent matches are ignored.

## Configuration

The `sfa-dips.toml` file configures the API, Temporal worker, database, external
services, and DIP storage. The service searches for it in the current directory,
`$HOME/.config/`, and `/etc`, in that order. To use a different location, pass
`--config /etc/sfa-dips/sfa-dips.toml` when starting the service.

Environment variables override file values. Prefix the configuration key with
`SFA_DIPS_`, use uppercase, and replace periods with underscores. For example,
`temporal.taskQueue` becomes `SFA_DIPS_TEMPORAL_TASKQUEUE`, and
`persistence.dsn` becomes `SFA_DIPS_PERSISTENCE_DSN`.

### Application logging

These top-level settings control application events written to standard error.
HTTP request logging is configured separately under [API logs](#api-logs).

**Example configuration**:

```toml
logFormat = "json"
verbosity = 0
```

- `logFormat`: Application log encoding. Accepted values are `"json"` (the
  default) for structured output and `"text"` for human-readable, colorized
  output.
- `verbosity`: Application log verbosity, from `0` to `128`. The default is `0`,
  which logs only the most important informational messages. Higher values
  include more detail; `2` is used in development and `128` includes all log
  events. Errors are always logged.

### DIP preparation and retention

These settings are also top-level keys. Place them before any TOML section
headers, such as `[api]` or `[temporal]`.

**Example configuration**:

```toml
workingDir = "/var/lib/sfa-dips/work"
xsdDir = "/usr/local/share/sfa-dips/eCH-0160-1.1"
retentionPeriod = "24h"
```

- `workingDir`: Directory for temporary DIP files. Defaults to the operating
  system's temporary directory. The service must be able to create and remove
  files here.
- `xsdDir`: Required directory containing `arelda.xsd` and its supporting
  eCH-0160 schemas. Schema files and their relative paths must match those
  referenced in the ACTApro metadata export.
- `retentionPeriod`: How long uploaded DIP archives are retained after
  successful creation. Must not be negative. The default, `"0"`, retains them
  indefinitely. A finite duration enables automatic archive deletion. See
  [API](#api) for how cache reuse affects retention.

### Temporal configuration

All fields in this section are required.

**Example configuration**:

```toml
[temporal]
address = "localhost:7233"
namespace = "default"
taskQueue = "sfa-dips"
maxConcurrentSessions = 4
```

- `address`: Temporal frontend host and port.
- `namespace`: Temporal namespace in which DIP workflows run.
- `taskQueue`: Queue used for DIP workflows and activities.
- `maxConcurrentSessions`: Maximum simultaneous Temporal sessions per worker
  process. Must be greater than zero.

### API server

**Example configuration**:

```toml
[api]
listen = "127.0.0.1:8080"
corsOrigin = "https://archives.example.com"
```

- `listen`: TCP address in `host:port` form. Defaults to `"127.0.0.1:8080"`.
- `corsOrigin`: Allowed origin for Cross-Origin Resource Sharing (CORS), used
  in the `Access-Control-Allow-Origin` response header. Default to the same
  value as the `listen` setting when empty, disallowing cross-origin requests.
  Use the requesting web application's origin, or `"*"` to allow any origin.

#### Enable API authentication

API authentication uses OpenID Connect (OIDC).

**Example configuration**:

```toml
[api.auth]
enabled = true
```

- `enabled`: Enables access token verification for DIP endpoints. Defaults to
  `false`. When `true`, at least one `[[api.auth.oidc]]` entry is required.

#### OIDC authentication providers

Add one TOML array-of-tables entry per provider and client audience. Multiple
entries can use the same provider with different client IDs.

**Example configuration**:

```toml
[[api.auth.oidc]]
providerURL = "https://idp.example.com/realms/sfa"
clientID = "sfa-dips"
skipEmailVerifiedCheck = false

[[api.auth.oidc]]
providerURL = "https://idp.example.com/realms/sfa"
clientID = "sfa-dips-s2s"
skipEmailVerifiedCheck = true
```

- `providerURL`: OIDC issuer URL used to discover the provider and verify
  tokens. Required for every entry when API authentication is enabled.
- `clientID`: Expected access token audience. Must be included in the token's
  `aud` claim. Required for every entry when API authentication is enabled.
- `skipEmailVerifiedCheck`: Defaults to `false`, requiring the
  `email_verified` claim to be present and `true` in the access token. Set to
  `true` to skip this check.

#### API logs

API logs record HTTP request and response information independently of the
application logger.

**Example configuration**:

```toml
[api.log]
path = "/var/log/sfa-dips/api.log"
level = "info"
format = "json"
```

- `path`: Log file path, or `"stdout"` / `"stderr"` (case-insensitive) for
  console output. Defaults to an empty string, which disables API logging.
  Files are created if needed and appended to; their parent directory must
  already exist and be writable.
- `level`: Case-insensitive log level. `"debug"` logs all responses, including
  OPTIONS; `"info"` (the default) excludes OPTIONS; `"warn"` logs only 4xx and
  5xx responses, except 429; `"error"` logs only 5xx responses.
- `format`: Log encoding, either `"json"` (the default) for line-delimited
  structured data or `"text"` for human-readable `key=value` pairs.

### Database connection

MySQL is required. Although the driver configuration accepts `"sqlite3"`,
SQLite is unsupported because DIP persistence requires `SELECT ... FOR UPDATE`
locking.

**Example configuration**:

```toml
[persistence]
driver = "mysql"
dsn = "sfa_dips:example-password@tcp(mysql.internal.example.com:3306)/sfa_dips"
migrate = true
```

- `driver`: Required database driver. Set to `"mysql"`.
- `dsn`: Required MySQL data source name. Use
  `user:password@tcp(host:port)/database`.
- `migrate`: Applies bundled MySQL schema migrations at startup when `true`.
  Defaults to `false`, in which case the schema must already be up to date.
  The database itself must exist before startup.

### ACTApro connection

Use [OIDC authentication](#actapro-oidc-authentication) for ACTApro in production.

**Example configuration**:

```toml
[actapro]
url = "https://actapro.example.com/ACTApro-connector"
timeout = "60s"
pollInterval = "30s"
```

- `url`: Required ACTApro connector base URL.
- `timeout`: HTTP client timeout for ACTApro requests, including export
  downloads. The `timeout` must not be negative and defaults to `"10s"`. A value
  of `"0"` will cause the client to wait indefinitely for a response.
- `pollInterval`: Interval between checks of an ACTApro export's status.
  Defaults to `"30s"` and must be greater than zero.
- `token`: Static bearer token for testing and development with mock or local
  ACTApro services only. Leave unset in production.

#### ACTApro OIDC authentication

ACTApro uses the OAuth2 resource owner password credentials grant. These
settings configure outbound authentication to ACTApro, separately from the
providers used to authenticate incoming DIP API requests.

**Example configuration**:

```toml
[actapro.oidc]
enabled = true
providerURL = ""
tokenURL = "https://actapro.example.com/AuthService-ACTApro/oauth2/token"
clientID = "sfa-dips"
clientSecret = "example-client-secret"
username = "svc-sfa-dips"
password = "example-password"
scopes = []
tokenExpiryLeeway = "30s"
retryMaxAttempts = 3
retryInitialInterval = "500ms"
retryMaxInterval = "2s"
retryBackoffCoefficient = 2.0
```

- `enabled`: Enables the ACTApro OIDC token provider. Defaults to `false`.
- `providerURL`: OIDC issuer URL used to discover the token endpoint. Required
  when OIDC is enabled and `tokenURL` is empty. Defaults to an empty string and
  is ignored when `tokenURL` is set.
- `tokenURL`: Explicit token endpoint URL. Defaults to an empty string, in
  which case the endpoint is discovered from `providerURL`.
- `clientID`: OAuth2 client ID. Required when OIDC is enabled.
- `clientSecret`: OAuth2 client secret. Defaults to an empty string for public
  clients; provide it for confidential clients.
- `username`: Resource owner's ACTApro login name. Required when OIDC is
  enabled.
- `password`: Resource owner's ACTApro password. Required when OIDC is enabled.
- `scopes`: Optional list of scopes to request, such as `["openid"]`. Defaults
  to an empty list. Environment variable values use a comma-separated list.
- `tokenExpiryLeeway`: Time before token expiry at which the provider obtains a
  fresh token. Defaults to `"30s"` and must be positive.
- `retryMaxAttempts`: Maximum token endpoint attempts per refresh or password
  login, including the initial attempt. Defaults to `3`; use `1` to disable
  retries.
- `retryInitialInterval`: Initial delay between retries after a transient
  token request failure. Defaults to `"500ms"` and must be positive.
- `retryMaxInterval`: Maximum retry delay. Defaults to `"2s"`, must be
  positive, and must be at least `retryInitialInterval`.
- `retryBackoffCoefficient`: Multiplier used to increase the retry delay.
  Defaults to `2.0` and must be at least `1.0`.

For token expiry leeway and the four retry settings, an omitted or zero value
selects the default. These retries apply to token requests; the workflow's
activity and export retries are described under
[Creation workflow](#creation-workflow).

### Archivematica Storage Service connection

All three fields are required.

**Example configuration**:

```toml
[amss]
baseURL = "https://storage.example.com"
username = "sfa-dips"
key = "example-api-key"
```

- `baseURL`: Archivematica Storage Service base URL, including the scheme and
  port if needed.
- `username`: AMSS user used to authenticate API requests.
- `key`: API key belonging to that user.

### DIP storage bucket

S3-compatible destination for completed DIP archives. The service needs
permission to write and delete objects. Configure either an S3 `url` or the
S3-compatible fields; at least `url` or `endpoint` must be set.

**Example S3-compatible field configuration**:

```toml
[bucket]
url = ""
endpoint = "https://s3.example.com"
bucket = "sfa-dips"
accessKey = "example-access-key"
secretKey = "example-secret-key"
token = ""
profile = ""
region = "eu-central-1"
pathStyle = true
```

- `url`: S3 bucket connection URL using the `s3://` scheme. Defaults to an
  empty string. When set, it takes precedence over all S3-compatible fields
  below.
- `endpoint`: S3-compatible API endpoint, including the scheme and optional
  port. Required when `url` is empty.
- `bucket`: Target S3 bucket name. Set this when using the S3-compatible
  fields. Defaults to an empty string; URL configurations include the bucket
  name in `url` instead.
- `accessKey`: S3 access key ID. Defaults to an empty string. Supply this and
  `secretKey` when using the S3-compatible fields.
- `secretKey`: S3 secret access key. Defaults to an empty string.
- `token`: Optional S3 session token for temporary credentials. Defaults to an
  empty string.
- `profile`: Optional AWS shared configuration profile name. Defaults to an
  empty string. The field-based connection uses the explicit `accessKey`,
  `secretKey`, and `token` values for credentials, even when a profile is set.
- `region`: S3 signing region, for example `"eu-central-1"`. Defaults to an empty
  string, allowing the AWS SDK to resolve it from its configuration.
- `pathStyle`: Uses path-style S3 addressing when `true`, for S3-compatible
  services that require it. Defaults to `false`.

**Example S3 URL configuration**:

```toml
[bucket]
url = "s3://sfa-dips-production?region=eu-central-1"
```

The URL form uses the Go CDK S3 opener and its AWS credential configuration;
the separate S3-compatible credential fields are ignored.
