# KMS Key

!!! warning "Renamed"
    `KMSKey` has been renamed to [`KMSKeyVersion`](kms-key-version.md), because it has always
    operated on a Cloud KMS *key version* rather than on the key itself.

    `KMSKey` still resolves as a deprecated alias, so existing configuration keeps working, but it
    should be updated to `KMSKeyVersion`.

Cloud KMS resources are now covered by three resource types, which must be removed in this order:

- [KMS Key Version](kms-key-version.md) — destroys, then deletes, key versions
- [KMS Crypto Key](kms-crypto-key.md) — deletes keys once all their versions are gone
- [KMS Key Ring](kms-key-ring.md) — deletes key rings once all their keys are gone
