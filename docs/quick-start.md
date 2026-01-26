# First Run

## First Configuration

First you need to create a config file for _gcp-nuke_. This is a minimal one:

```yaml
regions:
  - global
  - us-east1

blocklist:
  - production-12345

accounts:
  playground-12345: {} # gcp-nuke-example
```

## First Run (Dry Run)

With this config we can run _gcp-nuke_:

```bash
$ gcp-nuke nuke -c config/nuke-config.yaml
gcp-nuke - 1.0.0
Do you really want to nuke the project with the ID 'playground-12345'?
Do you want to continue? Enter project ID to continue.
> playground-12345

starting scan for resources

global - IAMServiceAccount - grafana@playground-12345.iam.gserviceaccount.com - [Description: "", ID: "1234567890123456789", Name: "grafana@playground-12345.iam.gserviceaccount.com"] - would remove
global - IAMServiceAccount - playground-filestore-backup@playground-12345.iam.gserviceaccount.com - [Description: "", ID: "1234567890123456789", Name: "playground-filestore-backup@playground-12345.iam.gserviceaccount.com"] - would remove
global - DNSManagedZone - sql-psa-goog - [CreationTime: "2026-01-25T12:26:03.208Z", DNSName: "sql-psa.goog.", Name: "sql-psa-goog", Visibility: "private"] - would remove
global - DNSManagedZone - example-com - [CreationTime: "2026-01-25T12:18:51.287Z", DNSName: "example.com.", Name: "example-com", Visibility: "public"] - would remove
global - IAMServiceAccountKey - 123456789012-compute@developer.gserviceaccount.com -> 1a2b3c4d5e6f7g8h9i0j1k2l3m4n5o6p - [Algorithm: "KEY_ALG_RSA_2048", Disabled: "false", ID: "1a2b3c4d5e6f7g8h9i0j1k2l3m4n5o6p", ManagedType: "SYSTEM_MANAGED", ServiceAccount: "Compute Engine default service account", ServiceAccountEmail: "123456789012-compute@developer.gserviceaccount.com", ServiceAccountID: "1234567890123456789"] - filtered: will not remove system managed key
us-east1 - ArtifactRegistryRepository - playground-repo - [Format: "DOCKER", FullName: "projects/playground-12345/locations/us-east1/repositories/playground-repo", Name: "playground-repo", label:goog-terraform-provisioned: "true"] - would remove
us-east1 - ComputeBackendService - gkegw1-tw1m-argocd-argocd-server-443-32wtkmg3e8op - [Name: "gkegw1-tw1m-argocd-argocd-server-443-32wtkmg3e8op"] - would remove
us-east1 - ComputeBackendService - gkegw1-tw1m-grafana-grafana-80-ura50snsg7it - [Name: "gkegw1-tw1m-grafana-grafana-80-ura50snsg7it"] - would remove
us-east1 - ComputeBackendService - gkegw1-tw1m-prometheus-prometheus-server-80-qxf2xtfecs7d - [Name: "gkegw1-tw1m-prometheus-prometheus-server-80-qxf2xtfecs7d"] - would remove
us-east1 - StorageBucket - playground-loki-logs - [MultiRegion: "false", Name: "playground-loki-logs", label:goog-terraform-provisioned: "true"] - would remove
us-east1 - KMSKey - playground-67890 - [Keyring: "playground-12345", Name: "playground-67890", State: "ENABLED"] - would remove
us-east1 - KMSKey - playground-12345 - [Keyring: "playground-12345", Name: "playground-12345", State: "DESTROYED"] - filtered: key is already destroyed
...
...
...
Scan complete: 333 total, 205 nukeable, 128 filtered.

The above resources would be deleted with the supplied configuration. Provide --no-dry-run to actually destroy resources.
```

As we see, _gcp-nuke_ only lists all found resources and exits. This is because the `--no-dry-run` flag is missing.

```yaml
regions:
  - global # Nuke global resources
  - us-east1 # Nuke resources in the us-east1 region

resource-types:
  excludes:
    - StorageBucketObject # Exclude Storage Bucket Objects

blocklist:
  - production-12345 # Never nuke this project

accounts: # i.e. Google Cloud projects
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

presets:
  common:
    filters:
      VPC:
        - default
```

## Second Run (No Dry Run)

!!! warning
This will officially remove resources from your gcp project. Make sure you really want to do this!

```bash
$ gcp-nuke nuke -c config/nuke-config.yaml --no-dry-run
gcp-nuke - 1.0.0
Do you really want to nuke the project with the ID 'playground-12345'?
Do you want to continue? Enter project ID to continue.
> playground-12345

starting scan for resources

global - IAMServiceAccount - grafana@playground-12345.iam.gserviceaccount.com - [Description: "", ID: "1234567890123456789", Name: "grafana@playground-12345.iam.gserviceaccount.com"] - would remove
global - IAMServiceAccount - playground-filestore-backup@playground-12345.iam.gserviceaccount.com - [Description: "", ID: "1234567890123456789", Name: "playground-filestore-backup@playground-12345.iam.gserviceaccount.com"] - would remove
global - DNSManagedZone - sql-psa-goog - [CreationTime: "2026-01-25T12:26:03.208Z", DNSName: "sql-psa.goog.", Name: "sql-psa-goog", Visibility: "private"] - would remove
global - DNSManagedZone - example-com - [CreationTime: "2026-01-25T12:18:51.287Z", DNSName: "example.com.", Name: "example-com", Visibility: "public"] - would remove
global - IAMServiceAccountKey - 123456789012-compute@developer.gserviceaccount.com -> 1a2b3c4d5e6f7g8h9i0j1k2l3m4n5o6p - [Algorithm: "KEY_ALG_RSA_2048", Disabled: "false", ID: "1a2b3c4d5e6f7g8h9i0j1k2l3m4n5o6p", ManagedType: "SYSTEM_MANAGED", ServiceAccount: "Compute Engine default service account", ServiceAccountEmail: "123456789012-compute@developer.gserviceaccount.com", ServiceAccountID: "1234567890123456789"] - filtered: will not remove system managed key
us-east1 - ArtifactRegistryRepository - playground-repo - [Format: "DOCKER", FullName: "projects/playground-12345/locations/us-east1/repositories/playground-repo", Name: "playground-repo", label:goog-terraform-provisioned: "true"] - would remove
us-east1 - ComputeBackendService - gkegw1-tw1m-argocd-argocd-server-443-32wtkmg3e8op - [Name: "gkegw1-tw1m-argocd-argocd-server-443-32wtkmg3e8op"] - would remove
us-east1 - ComputeBackendService - gkegw1-tw1m-grafana-grafana-80-ura50snsg7it - [Name: "gkegw1-tw1m-grafana-grafana-80-ura50snsg7it"] - would remove
us-east1 - ComputeBackendService - gkegw1-tw1m-prometheus-prometheus-server-80-qxf2xtfecs7d - [Name: "gkegw1-tw1m-prometheus-prometheus-server-80-qxf2xtfecs7d"] - would remove
us-east1 - StorageBucket - playground-loki-logs - [MultiRegion: "false", Name: "playground-loki-logs", label:goog-terraform-provisioned: "true"] - would remove
us-east1 - KMSKey - playground-67890 - [Keyring: "playground-12345", Name: "playground-67890", State: "ENABLED"] - would remove
us-east1 - KMSKey - playground-12345 - [Keyring: "playground-12345", Name: "playground-12345", State: "DESTROYED"] - filtered: key is already destroyed
...
...
...
Scan complete: 333 total, 205 nukeable, 128 filtered.

Do you really want to nuke these resources on the project with the ID 'playground-12345'?
Do you want to continue? Enter project ID to continue.
> playground-12345

global - DNSManagedZone - sql-psa-goog - triggered remove
global - DNSManagedZone - example-com - triggered remove
us-east1 - ArtifactRegistryRepository - playground-repo - triggered remove
us-east1 - ComputeBackendService - gkegw1-tw1m-argocd-argocd-server-443-32wtkmg3e8op - triggered remove
us-east1 - ComputeBackendService - gkegw1-tw1m-grafana-grafana-80-ura50snsg7it - triggered remove
us-east1 - ComputeBackendService - gkegw1-tw1m-prometheus-prometheus-server-80-qxf2xtfecs7d - triggered remove
us-east1 - StorageBucket - playground-loki-logs - triggered remove
...
...
...

Removal requested: 205 waiting, 0 failed, 128 skipped, 0 finished

global - DNSManagedZone - sql-psa-goog - removed
global - DNSManagedZone - example-com - waiting
us-east1 - ArtifactRegistryRepository - playground-repo - removed
us-east1 - ComputeBackendService - gkegw1-tw1m-argocd-argocd-server-443-32wtkmg3e8op - removed
us-east1 - ComputeBackendService - gkegw1-tw1m-grafana-grafana-80-ura50snsg7it - removed
us-east1 - ComputeBackendService - gkegw1-tw1m-prometheus-prometheus-server-80-qxf2xtfecs7d - removed
us-east1 - StorageBucket - playground-loki-logs - removed
...
...
...

Removal requested: 12 waiting, 0 failed, 128 skipped, 193 finished

--- truncating long output ---
```

As you see _gcp-nuke_ now tries to delete all resources which aren't filtered. This results in API errors which can be ignored.
These errors are shown at the end of the _gcp-nuke_ run, if they keep to appear.

_gcp-nuke_ retries deleting all resources until all specified ones are deleted or until there are only resources
with errors left.
