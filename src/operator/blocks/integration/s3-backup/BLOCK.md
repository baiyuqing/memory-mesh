---
kind: integration.s3-backup
category: integration
version: 1.0.0
description: Scheduled database backups to S3-compatible storage.
ports:
  - name: source-dsn
    portType: dsn
    direction: input
    required: true
parameters:
  - name: schedule
    type: string
    default: "0 2 * * *"
    required: true
    description: Cron expression for backup schedule.
  - name: bucket
    type: string
    required: true
    description: "S3 bucket path (e.g. s3://backups/cluster-name)."
  - name: retentionDays
    type: int
    default: "7"
    description: Number of days to retain backups.
  - name: endpoint
    type: string
    default: "http://localstack.localstack.svc:4566"
    description: S3 endpoint URL (for LocalStack or other S3-compatible services).
requires:
  - "datastore.*"
provides: []
---

# integration.s3-backup

Runs scheduled backups via a CronJob. Connects to the database using the
wired DSN and streams backups to an S3-compatible bucket.

## Composition Notes

- **Must** wire `source-dsn` from an engine block's `dsn` output.
- Uses `pg_dump` for PostgreSQL, `mysqldump` for MySQL (engine-aware).
- In local dev, point `endpoint` to LocalStack's S3 service.

## Kubernetes Resources Created

- CronJob: `{cluster}-{name}` running on the configured schedule
- Secret: `{cluster}-{name}-s3-credentials` with AWS credentials
- ConfigMap: `{cluster}-{name}-config` with backup parameters
