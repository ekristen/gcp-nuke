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

You can run **gcp-nuke** with Docker by using a command like this:

## Source

To compile **gcp-nuke** from source you need a working [Golang](https://golang.org/doc/install) development environment and [goreleaser](https://goreleaser.com/install/).

**gcp-nuke** uses go modules and so the clone path should not matter. Then simply change directory into the clone and run:

```bash
goreleaser build --clean --snapshot --single-target
```

