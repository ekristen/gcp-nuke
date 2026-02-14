# Cloud SQL Backup

## Details

- **Type:** `CloudSQLBackup`
- **Scope:** project

## Properties

- **`BackupKind`**: Kind of backup (SNAPSHOT, PHYSICAL)
- **`EndTime`**: End time of the backup
- **`ID`**: Backup run ID
- **`Instance`**: Name of the Cloud SQL instance
- **`Location`**: Location of the backup
- **`StartTime`**: Start time of the backup
- **`Status`**: Status of the backup (SUCCESSFUL, FAILED, etc.)
- **`Type`**: Type of backup (AUTOMATED, ON_DEMAND)

## Usage

Filter by backup type:
```yaml
filters:
  CloudSQLBackup:
    - property: Type
      value: ON_DEMAND  # Keep on-demand backups, delete automated ones
```

Filter by instance:
```yaml
filters:
  CloudSQLBackup:
    - property: Instance
      value: my-production-db  # Keep backups for this instance
```
