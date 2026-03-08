# Cloud SQL Instance

## Details

- **Type:** `CloudSQLInstance`
- **Scope:** project

## Properties

- **`CreationDate`**: The time when the instance was created
- **`DatabaseVersion`**: The database engine type and version
- **`Labels`**: The user-defined labels associated with this Cloud SQL instance
- **`Name`**: Name of the Cloud SQL instance
- **`State`**: The current serving state of the Cloud SQL instance
## Depends On

!!! Experimental Feature
    This is an **experimental** feature, please read more about it here <>. This feature attempts to remove all resources in one resource type before moving onto the dependent resource type

- [Cloud SQL Backup](cloud-sql-backup.md)
## Settings

- `DisableDeletionProtection`
