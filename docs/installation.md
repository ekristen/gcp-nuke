# Install

## Install the pre-compiled binary

### Homebrew Tap (MacOS/Linux)

```console
brew install ekristen/tap/gcp-nuke
```

## Releases

You can download pre-compiled binaries from the [releases](https://github.com/ekristen/gcp-nuke/releases) page.

## Docker

Registries:

- [ghcr.io/ekristen/gcp-nuke](https://github.com/ekristen/gcp-nuke/pkgs/container/gcp-nuke)

### Using gcloud Default Credentials

Mount your local gcloud configuration into the container:

```bash
docker run --rm \
  -v ~/.config/gcloud:/home/gcp-nuke/.config/gcloud:ro \
  -v "$(pwd)/config.yaml:/config.yaml:ro" \
  ghcr.io/ekristen/gcp-nuke:v1.10.0 \
  run --config /config.yaml --project-id playground-12345
```

### Using Service Account Key File

Mount the service account JSON file and set `GOOGLE_APPLICATION_CREDENTIALS`:

```bash
docker run --rm \
  -v "$(pwd)/credentials.json:/credentials.json:ro" \
  -v "$(pwd)/config.yaml:/config.yaml:ro" \
  -e GOOGLE_APPLICATION_CREDENTIALS=/credentials.json \
  ghcr.io/ekristen/gcp-nuke:v1.10.0 \
  run --config /config.yaml --project-id playground-12345
```

### Using Service Account Key as JSON String

Pass credentials directly via environment variable without mounting files:

```bash
docker run --rm \
  -v "$(pwd)/config.yaml:/config.yaml:ro" \
  -e GOOGLE_APPLICATION_CREDENTIALS_JSON="$GOOGLE_APPLICATION_CREDENTIALS_JSON" \
  ghcr.io/ekristen/gcp-nuke:v1.10.0 \
  run --config /config.yaml --project-id playground-12345
```

## Source

To compile **gcp-nuke** from source you need a working [Golang](https://golang.org/doc/install) development environment and [goreleaser](https://goreleaser.com/install/).

**gcp-nuke** uses go modules and so the clone path should not matter. Then simply change directory into the clone and run:

```bash
goreleaser build --clean --snapshot --single-target
```
