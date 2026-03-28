---
kind: auth.password-rotation
category: auth
version: 1.0.0
description: Automated database credential rotation via CronJob.
ports:
  - name: upstream-dsn
    portType: dsn
    direction: input
    required: true
  - name: credential
    portType: credential
    direction: output
parameters:
  - name: rotationSchedule
    type: string
    default: "0 0 */7 * *"
    required: true
    description: Cron schedule for password rotation (default weekly).
  - name: passwordLength
    type: int
    default: "32"
    description: Generated password length.
  - name: secretName
    type: string
    default: ""
    description: Name of the Secret storing current credentials. Defaults to {cluster}-{name}-creds.
requires:
  - "engine.*"
provides:
  - credential
---

# auth.password-rotation

Runs a CronJob that periodically rotates database credentials. Stores the
current credentials in a Kubernetes Secret.

## Composition Notes

- **Must** wire `upstream-dsn` from a database engine block.
- Outputs `credential` for downstream consumers needing DB access.
- Uses a CronJob with a lightweight container to execute rotation.

## Kubernetes Resources Created

- CronJob: `{cluster}-{name}` with rotation script
- Secret: `{cluster}-{name}-creds` (current credentials)
- ConfigMap: `{cluster}-{name}-config` (rotation configuration)
- ServiceAccount: `{cluster}-{name}-sa`
- Role + RoleBinding: for Secret update permissions
