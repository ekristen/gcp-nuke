# Logging Bucket

## Details

- **Type:** `LoggingBucket`
- **Scope:** project

## Properties

- **`Description`**: Description of the log bucket
- **`LifecycleState`**: Lifecycle state of the log bucket
- **`Location`**: Location the log bucket resides in
- **`Locked`**: Whether the retention period is locked
- **`Name`**: Name of the log bucket
- **`RetentionDays`**: Number of days log entries are retained
## Depends On

!!! Experimental Feature
    This is an **experimental** feature, please read more about it here <>. This feature attempts to remove all resources in one resource type before moving onto the dependent resource type

- [Logging Sink](logging-sink.md)
