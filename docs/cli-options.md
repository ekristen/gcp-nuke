# Options

This is not a comprehensive list of options, but rather a list of features that I think are worth highlighting.

## Authentication Environment Variables

gcp-nuke supports the following environment variables for authentication:

| Variable | Description |
|----------|-------------|
| `GOOGLE_APPLICATION_CREDENTIALS` | Path to a service account JSON key file |
| `GOOGLE_APPLICATION_CREDENTIALS_JSON` | Service account JSON key as a string (takes precedence) |

The `GOOGLE_APPLICATION_CREDENTIALS_JSON` variable is useful for CI/CD pipelines and containerized environments where you want to pass credentials directly without creating a file on disk.

Example:
```bash
export GOOGLE_APPLICATION_CREDENTIALS_JSON='{"type":"service_account","project_id":"...","private_key":"..."}'
gcp-nuke run --config config.yaml --project-id playground-12345
```

## Wait on Dependencies

`--wait-on-dependencies` will wait for dependent resources to be deleted before deleting resources that depend on them. This is useful when resources have dependencies on each other (e.g., a VPC network cannot be deleted until all subnets are deleted first).

## Skip Prompts

`--no-prompt` will skip the prompt to verify you want to run the command. This is useful if you are running in a CI/CD environment.
`--prompt-delay` will set the delay before the command runs. This is useful if you want to give yourself time to cancel the command.

## Logging

- `--log-level` will set the log level. This is useful if you want to see more or less information in the logs.
- `--log-caller` will log the caller (aka line number and file). This is useful if you are debugging.
- `--log-disable-color` will disable log coloring. This is useful if you are running in an environment that does not support color.
- `--log-full-timestamp` will force log output to always show full timestamp. This is useful if you want to see the full timestamp in the logs.