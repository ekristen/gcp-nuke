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

## Usage

**Note:** all cli flags can also be expressed as environment variables.

**By default, no destructive actions will be taken.**

### Example - Dry Run only

```bash
gcp-nuke run \
  --config test-config.yaml \
  --project-id playground-12345 
```

### Example - No Dry Run (DESTRUCTIVE)

To actually destroy you must add the `--no-dry-run` cli parameter.

```bash
gcp-nuke run \
  --config=test-config.yaml \
  --project-id playground-12345 \
  --no-dry-run
```

## Authentication

Authentication is only supported via a Service Account either by Key or via Workload Identity. 

### Service Account Key

```bash
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account-key.json
```

### Federated Token (Kubernetes)

**coming soon**

## Configuring

The entire configuration of the tool is done via a single YAML file.

### Example Configuration

**Note:** you must add at least one entry to the blocklist.

```yaml
regions:
  - global
  - eastus

blocklist:
  - 00001111-2222-3333-4444-555566667777

accounts: # i.e. projects but due to the commonality of libnuke, it's accounts here universally between tools
  playground-12345:
    presets:
      - common
    filters:
      IAMRole:
        - property: Name
          type: contains
          value: CustomRole
      IAMServiceAccount:
        - property: Name
          type: contains
          value: custom-service-account

presets:
  common:
    filters:
      VPC:
        - default
```
