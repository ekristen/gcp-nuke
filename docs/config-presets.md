# Presets

It might be the case that some filters are the same across multiple projects. This especially could happen, if
provisioning tools like Terraform are used or if IAM resources follow the same pattern.

For this case *gcp-nuke* supports presets of filters, that can applied on multiple projects.

`Presets` are defined globally. They can then be referenced in the `accounts` section of the configuration.

A preset configuration could look like this:

```yaml
presets:
  common:
    filters:
      IAMRole:
        - custom-role
      VPCNetwork:
        - default
```

A project referencing a preset would then look something like this:

```yaml
accounts:
  playground-12345:
    presets:
      - common
```

Putting it all together it would look something like this:

```yaml
blocklist:
  - production-12345

regions:
  - global
  - us-east1

accounts:
  playground-12345:
    presets:
      - common

presets:
  common:
    filters:
      IAMRole:
        - custom-role
      VPCNetwork:
        - default
```