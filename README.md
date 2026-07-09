# sfa-enduro-workflows

**sfa-enduro-workflows** provides one preprocessing workflow and two
poststorage workflow implementations for SFA. The worker always registers the
preprocessing workflow, then registers either `poststorage-apis` when APIS is
enabled or `poststorage-cantons` when APIS is disabled.

- [Configuration](#configuration)
- [Local environment](#local-environment)
- [Makefile](#makefile)
- [Available activities](#available-activities)

## Configuration

The worker needs to share the filesystem with Enduro's a3m or Archivematica
workers, connect to the same Temporal server, and be related to Enduro with the
correct namespace, task queue and workflow names.

### Worker configuration

An example configuration for the worker binary:

```toml
debug = false
verbosity = 0

[temporal]
address = "temporal-frontend.enduro-sdps:7233"
namespace = "default"

[worker]
maxConcurrentSessions = 1
taskQueue = "sfa-enduro"

[preprocessing]
workflowName = "preprocessing"
sharedPath = "/home/enduro/shared"

[preprocessing.bagCreate]
checksumAlgorithm = "md5"

[preprocessing.bagValidate]
cacheDir = ""
poolSize = 1

[preprocessing.fileFormat]
allowlistPath = "/home/enduro/.config/allowed_file_formats.csv"

[preprocessing.filevalidate.verapdf]
path = "/opt/verapdf/verapdf"

[poststorage]
workingDir = "/tmp"

[poststorage.apis]
workflowName = "poststorage-apis"

[poststorage.cantons]
workflowName = "poststorage-cantons"

[poststorage.cantons.bucket]
endpoint = "https://s3.example.com"
pathStyle = true
accessKey = "example-access-key"
secretKey = "example-secret-key"
region = "us-west-1"
bucket = "example-bucket"

[poststorage.amss]
baseURL = "http://ambox.enduro-sdps:64081"
username = "test"
key = "test"

[apis]
enabled = true
url = "http://apis-mock.enduro-sdps:8080"
timeout = "10s"
pollInterval = "1s"
token = "mock-token"

[apis.oidc]
enabled = false
providerURL = "http://keycloak:7470/realms/artefactual"
tokenURL = ""
clientID = "enduro-s2s"
clientSecret = "uSh7f2r4j2U5wA9d7mJ3xP6nQ8cT1vL0"
scopes = ""
audience = ""
tokenExpiryLeeway = "30s"
retryMaxAttempts = 3
retryInitialInterval = "500ms"
retryMaxInterval = "2s"
retryBackoffCoefficient = 2.0

```

When `apis.enabled = true`, the worker registers `preprocessing` and
`poststorage-apis`. When `apis.enabled = false`, the worker registers
`preprocessing` and `poststorage-cantons`; in that mode the
`poststorage.cantons.bucket` section is required. The example above uses the
generic S3-compatible bucket fields. Filesystem buckets can be configured with
a `url` such as `file:///home/enduro/cantons?create_dir=true`. Other bucket
settings are the same options documented in Enduro's
[bucket configuration options].

### Enduro

The child workflow sections for Enduro's configuration:

```toml
[[childWorkflows]]
type = "preprocessing"
namespace = "default"
taskQueue = "sfa-enduro"
workflowName = "preprocessing"
extract = true
sharedPath = "/home/enduro/preprocessing"

[[childWorkflows]]
type = "poststorage"
namespace = "default"
taskQueue = "sfa-enduro"
workflowName = "poststorage-apis"
```

For a deployment without APIS, configure Enduro to use the Cantons poststorage
workflow:

```toml
[[childWorkflows]]
type = "preprocessing"
namespace = "default"
taskQueue = "sfa-enduro"
workflowName = "preprocessing"
extract = true
sharedPath = "/home/enduro/preprocessing"

[[childWorkflows]]
type = "poststorage"
namespace = "default"
taskQueue = "sfa-enduro"
workflowName = "poststorage-cantons"
```

## Local environment

This project provides SFA child workflows for the Enduro development
environment. The supported development workflow is to run `tilt up` from the
Enduro repository and load this repository through Enduro's
`CHILD_WORKFLOW_PATHS` mechanism.

Bring up the Enduro environment by following the [Enduro development manual].

### Set up

The specific requirements for this project are:

- clone this repository as a sibling of the Enduro repository
- configure `CHILD_WORKFLOW_PATHS=../sfa-enduro-workflows`
- configure `MOUNT_PREPROCESSING_VOLUME=true`
- run `tilt up` from the Enduro repository

All other development workflow details, including `.tilt.env`, live updates,
starting, stopping, and clearing the environment, are documented in Enduro.
This repository can also provide local overrides through its own `.tilt.env`
file, including settings such as `TRIGGER_MODE_AUTO`.

The local APIS mock uses happy path terminal results by default. Set these
values in this repository's `.tilt.env` to exercise APIS conflict or failure
flows:

```bash
MOCK_ANALYSIS_RESULT=AlleNeu      # AlleNeu, AlleGleich, Konflikte, Fehler
MOCK_IMPORT_RESULT=Erfolgreich    # Erfolgreich, Fehler
```

When running the Cantons poststorage workflow in the development environment,
the filesystem bucket configuration writes bundles to `/home/enduro/cantons`
inside the worker pod. List the generated bundles and copy one to your working
directory with:

```bash
kubectl -n enduro-sdps exec -c sfa-enduro-worker sfa-enduro-worker-0 -- \
  ls -lh /home/enduro/cantons

kubectl -n enduro-sdps cp -c sfa-enduro-worker \
  sfa-enduro-worker-0:/home/enduro/cantons/<bundle>.zip \
  ./<bundle>.zip
```

### Requirements for development

While we run the services inside a Kubernetes cluster we recomend installing
Go and other tools locally to ease the development process.

- [Go] (1.26+)
- GNU [Make] and [GCC]

## Makefile

The Makefile provides developer utility scripts via command line `make` tasks.
Running `make` with no arguments (or `make help`) prints the help message.
Dependencies are downloaded automatically.

### Debug mode

The debug mode produces more output, including the commands executed. E.g.:

```shell
$ make env DBG_MAKEFILE=1
Makefile:10: ***** starting Makefile for goal(s) "env"
Makefile:11: ***** Fri 10 Nov 2023 11:16:16 AM CET
go env
GO111MODULE=''
GOARCH='amd64'
...
```

## Available activities

Most of the activities documented below belong to the preprocessing child
workflow.

* [Unbag SIP](#unbag-sip)
* [Identify SIP structure](#identify-sip-structure)
* [Validate SIP structure](#validate-sip-structure)
* [Validate SIP name](#validate-sip-name)
* [Verify SIP manifest](#verify-sip-manifest)
* [Verify SIP checksums](#verify-sip-checksums)
* [Validate SIP files](#validate-sip-files)
* [Validate logical metadata](#validate-logical-metadata)
* [Create premis.xml](#create-premisxml)
* [Restrucuture SIP](#restructure-sip)
* [Create identifiers.json](#create-identifiersjson)
* [Other activities](#other-activities)

### Unbag SIP

Extracts the contents of the bag.

Only runs if the SIP is a BagIt bag. If the SIP is not a bag, this activity will
not run.

#### Steps

* Check if SIP is a bag
* If yes, extract the contents of the bag for additional ingest processing
* Else, skip

#### Success critera

* Bag is successfully extracted

### Identify SIP structure

Determines the SIP type by analyzing the name and distinguishing features of
the package, based on eCH-0160 requirements and other internal policies.

Package types include:

* BornDigitalSIP
* DigitizedSIP
* BornDigitalAIP
* DigitizedAIP

#### Steps

* Base type is BornDigitalSIP; assume this is the SIP type unless other
  conditions are met
* Check if the package contains a `Prozess_Digitalisierung_PREMIS.xml` file
  * If yes, it is a Digitized package - either DigitizedSIP or DigitizedAIP
* Check if the package contains an additional directory
  * If yes, it is a migration AIP - either BornDigitalAIP or DigitizedAIP
* Compare check results and determine package type

#### Success criteria

* Package is successfully identified as one of the 4 supported types

### Validate SIP structure

Ensures that the SIP directory structure conforms to eCH-0160 specifications,
that no empty directories are included, and that there are no disallowed
characters used in file and directory names.

**Note**: Character restrictions for file and directory names are based on some
of the requirements of the tools used by
[Archivematica](https://www.archivematica.org) during preservation processing -
at present, the file name cleanup steps in Archivematica cannot be modified or
disabled without forking. To ensure that SFA package metadata matches the
content, this validation check ensures that no disallowed characters are
included in file or directory names that might be automatically changed once
received by Archivematica.

#### Steps

* Read SIP type from previous activity
* Check for presence of `content` and `header` directories
* Check all file and directory names for invalid characters
* Check for empty directories

#### Success critera

* Files and directories only contain valid characters
  * `A-Z`, `a-z`, `0-9`, or `-_.()`
* SIPs contain `content` and `header` directories
  * If content type is an AIP, it also contains an `additional` directory
* No empty directories are found

### Validate SIP name

Ensure that submitted SIPs use the required naming convention for the identified
package type.

#### Steps

* Read SIP type from previous activity
* Use regular expression to validate SIP name based on identified type

#### Success critera

* SIP follows expected naming convention for package type:
  * BornDigitalSIP: `SIP_[YYYYMMDD]_[delivering office]_[reference]`
  * DigitizedSIP: `SIP_[YYYYMMDD]_Vecteur_[delivering office]_[reference]`

### Verify SIP manifest

Checks if all files and directories listed in the metadata manifest match those
found in the SIP, and that no extra files or directories are found.

#### Steps

* Load SIP metadata manifest into memory
* Parse the manifest contents and return a list of files and directories
* Parse the SIP and return a list of files and directories
* Compare lists
* Return a list of any missing files found in the manifest but not the SIP
* Return a list of unexpected files found in the SIP but not the manifest

#### Success critera

* There is a matching file or directory for every entry found in the
  `metadata.xml` (or `UpdatedAreldaMetadata.xml`) manifest
* No unexpected files that are not listed in the manifest are found

### Verify SIP checksums

Confirms that the checksums included in the metadata manifest match those
calculated during validation.

#### Steps

* Check if a given file exists in the manifest
* If yes, calculate a checksum - else skip
* Compare calculated checksum to manifest checksum

#### Success critera

* A checksum calculated using the same algorithm as the one used in the metadata
  file returns the same value as the one included in the metadata manifest for
  each file listed

### Validate SIP files

Ensures that files included in the SIP are well-formed and match their format
specifications.

#### Steps

* For PDF/As, use [VeraPDF](https://github.com/veraPDF) to validate against the
  PDF/A specification
* Note: additional format validation checks will be added in the future

#### Success critera

* All files pass validation

### Validate logical metadata

Ensures that a logical metadata file is included for AIPs being migrated from
DIR and validates the file against a PREMIS schema file

**Note** : this activity uses some custom workflow code and a locally stored
copy of the PREMIS schema to run the general temporal activity
[xmlvalidate](https://github.com/artefactual-sdps/temporal-activities/blob/main/xmlvalidate/activity.go).

#### Steps

* Read package type from memory
* If package type is bornDigitalAIP or DigitizedAIP, check for XML file in
  `additional` directory
* If found, validate the XML file against a locally stored copy of the PREMIS
  schema; fail ingest if any errors are returned

#### Success critera

* Logical metadata file is found in the `additional` directory of the package
* Logical metadata file validates against PREMIS 3.x schema

### Create premis.xml

Generates a PREMIS XML file that captures ingest preservation actions performed
by Enduro as PREMIS events for inclusion in the resulting AIP METS file.

**NOTE**: This activity is broken up into 3 different activity files in
`/internal/activites`:

* `add_premis_agent.go`
* `add_premis_event.go`
* `add_premisobjects.go`

The XML output is then assembled via `/internal/premis/premis.go`.

#### Steps

* Review event details for all successful tasks
* Create premis.xml file in a new metadata directory
* Write PREMIS objects to file
* Write PREMIS events to file
* Write PREMIS agents to file

#### Success critera

* A `premis.xml` file is successfully generated with ingest events

### Restructure SIP

Reorganizes SIP directory structure into a Preservation Information Package
(PIP) that the preservation engine (Archivematica) can process.

#### Steps

* Check if `metadata` directory exists, else create a new `metadata` directory
* Move the `Prozess_Digitalisierung_PREMIS.xml` file to the `metadata` directory
* For AIPs, move the `UpdatedAreldaMetatdata.xml` and logical metadata files to
  the `metadata` directory
* Create an `objects` directory, and in that directory create a sub-directory
  with the SIP name
* Delete `xsd` directory and its contents from `header` directory
* Move `content` directory into the new `objects` directory
* Create a new `header` directory in objects
* Move the `metadata.xml` file into the new `header` directory
* Delete original top-level directories

#### Success critera

* XSD files are removed
* Restructured package now has `objects` and `metadata` directories immediately
  inside parent container
* All content for preservation is within the `objects` directory
* Enduro-generated PREMIS file is in the `metadata` directory
* For Digitized packages, `Prozess_Digitalisierung_PREMIS.xml` file is in the
  metadata directory

### Create identifiers.json

Extract original UUIDs from the SIP metadata file and add them to an
`identifiers.json` file added to the `metadata` directory of the package for
parsing by the preservation engine

#### Steps

* Parse SIP metadata file
* Extract persistent identifiers and write to memory
* Convert manifest file paths to the restructured PIP file paths
* Exclude any files in the manifest that aren't found in the PIP
* Using extracted identifiers, generate an `identifiers.json` file that conforms
  to Archivematica's expectations
* Move generated file to package `metadata` directory

#### Success critera

* An `identifiers.json` file is added to the `metadata` directory of the package
* UUIDs present in the original SIP metadata are maintained and used by the
  preservation engine during preservation processing

### Other activities

The preprocessing child workflow that invokes the activities listed above (see the
[preprocessing.go](https://github.com/artefactual-sdps/sfa-enduro-workflows/blob/main/internal/workflows/preprocessing.go)
file) also uses a number of other more general Enduro
[temporal activites](https://github.com/artefactual-sdps/temporal-activities), including:

* `archiveextract`
* `bagcreate`
* `bagvalidate`
* `ffvalidate`
* `xmlvalidate`

The `poststorage-apis` workflow downloads the AIP METS file from Archivematica
Storage Service and submits it to APIS. The `poststorage-cantons` workflow
downloads the AIP METS file and the selected Arelda metadata file from
Archivematica Storage Service, combines them, creates a ZIP bundle, and deposits
the bundle in the configured bucket.

[bucket configuration options]: https://enduro.readthedocs.io/admin-manual/configuration/#bucket-configuration-options
[Enduro development manual]: https://enduro.readthedocs.io/dev-manual/devel/
[go]: https://go.dev/doc/install
[make]: https://www.gnu.org/software/make/
[gcc]: https://gcc.gnu.org/
