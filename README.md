# gcp-nuke

[![license](https://img.shields.io/github/license/ekristen/gcp-nuke.svg)](https://github.com/ekristen/gcp-nuke/blob/main/LICENSE)
[![release](https://img.shields.io/github/release/ekristen/gcp-nuke.svg)](https://github.com/ekristen/gcp-nuke/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/ekristen/gcp-nuke)](https://goreportcard.com/report/github.com/ekristen/gcp-nuke)
[![Maintainability](https://api.codeclimate.com/v1/badges/51b67f545bfb93ecab2f/maintainability)](https://codeclimate.com/github/ekristen/gcp-nuke/maintainability)

**This is potentially very destructive! Use at your own risk!**

**Status:** Beta. Tool is stable, but could experience odd behaviors with some resources.

## Overview

Remove all resources from a GCP Project.

**gcp-nuke** is in beta, but it is likely that not all GCP resources are covered by it. Be encouraged to add missing
resources and create a Pull Request or to create an [Issue](https://github.com/ekristen/gcp-nuke/issues/new).

## Quick Start (Docker)

**1. Create config file** (`nuke-config.yaml`):

```yaml
regions:
  - global # Nuke global resources
  - us-east1 # Nuke resources in the us-east1 region

blocklist:
  - production-12345 # Never nuke this project

accounts:
  playground-12345: {} # Nuke all resources in the playground-12345 project
```

**2. Authenticate:**

```bash
gcloud auth application-default login
```

**3. Dry run (preview what will be deleted):**

```bash
docker run --rm -it \
  -v "${HOME}/.config/gcloud:/root/.config/gcloud:ro" \
  -v "$(pwd)/nuke-config.yaml:/nuke-config.yaml:ro" \
  ghcr.io/ekristen/gcp-nuke:v1.12.0 run \
  --config /nuke-config.yaml \
  --disable-deletion-protection \
  --project-id playground-12345
```

**4. Nuke everything (DESTRUCTIVE):**

```bash
docker run --rm -it \
  -v "${HOME}/.config/gcloud:/root/.config/gcloud:ro" \
  -v "$(pwd)/nuke-config.yaml:/nuke-config.yaml:ro" \
  ghcr.io/ekristen/gcp-nuke:v1.12.0 run \
  --config /nuke-config.yaml \
  --project-id playground-12345 \
  --disable-deletion-protection \
  --no-dry-run
```

## Documentation

All documentation is in the [docs/](docs) directory and is built using [Material for Mkdocs](https://squidfunk.github.io/mkdocs-material/).

It is hosted at [https://ekristen.github.io/gcp-nuke/](https://ekristen.github.io/gcp-nuke/).

## Attribution, License, and Copyright

This tool was written using [libnuke](https://github.com/ekristen/libnuke) at it's core. It shares similarities and commonalities with [aws-nuke](https://github.com/ekristen/aws-nuke)
and [azure-nuke](https://github.com/ekristen/azure-nuke). These tools would not have been possible without the hard work
that came before me on the original tool by the team and contributors over at [rebuy-de](https://github.com/rebuy-de) and their original work
on [rebuy-de/aws-nuke](https://github.com/rebuy-de/aws-nuke).

This tool is licensed under the MIT license as well. See the [LICENSE](LICENSE) file for more information. Reference
was made to [dshelley66/gcp-nuke](https://github.com/dshelley66/gcp-nuke) during the creation of this tool therefore I
included them in the license copyright although no direct code was used.

## Authentication

Authentication uses [Application Default Credentials (ADC)](https://cloud.google.com/docs/authentication/application-default-credentials). The following methods are supported:

### gcloud CLI (Recommended for local development)

```bash
gcloud auth application-default login
```

### Service Account Key (File Path)

```bash
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account-key.json
```

### Service Account Key (JSON String)

For CI/CD pipelines and containerized environments where you want to pass credentials directly without creating a file:

```bash
export GOOGLE_APPLICATION_CREDENTIALS_JSON='{"type":"service_account","project_id":"...","private_key":"..."}'
```

If both `GOOGLE_APPLICATION_CREDENTIALS` and `GOOGLE_APPLICATION_CREDENTIALS_JSON` are set, `GOOGLE_APPLICATION_CREDENTIALS_JSON` takes precedence.

### Workload Identity (GKE, Cloud Run, etc.)

When running on GCP infrastructure, credentials are automatically provided via the attached service account.

## Configuring

The entire configuration of the tool is done via a single YAML file.

### Example Configuration

**Note:** you must add at least one entry to the blocklist.

```yaml
regions:
  - global # Nuke global resources
  - us-east1 # Nuke resources in the us-east1 region

resource-types:
  excludes:
    - StorageBucketObject # Exclude Storage Bucket Objects

blocklist:
  - production-12345 # Never nuke this project

accounts: # i.e. Google Cloud projects to nuke
  playground-12345:
    presets:
      - common
    filters:
      # Protect specific service accounts by email
      IAMServiceAccount:
        - 'custom-service-account@playground-12345.iam.gserviceaccount.com'

      # Protect service account keys by service account email
      IAMServiceAccountKey:
        - property: ServiceAccountEmail
          value: 'custom-service-account@playground-12345.iam.gserviceaccount.com'

      # Protect a DNS zone from deletion
      DNSManagedZone:
        - 'my-dns-zone'

      # Protect IAM policy bindings for specific users
      IAMPolicyBinding:
        - property: Member
          value: 'user:admin@example.com'

      # Delete DNS records only in a specific zone
      DNSRecordSet:
        - property: Zone
          value: 'my-dns-zone'
          invert: true

      # Protect secrets with name containing "prod"
      SecretManagerSecret:
        - property: Name
          type: contains
          value: 'prod'

      # Protect KMS keys with prefix
      KMSKey:
        - property: Name
          type: glob
          value: 'prod-*'

presets:
  common:
    filters:
      VPCNetwork:
        - default
```
