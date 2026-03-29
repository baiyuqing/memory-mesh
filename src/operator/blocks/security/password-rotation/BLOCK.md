---
kind: security.password-rotation
category: security
version: 1.0.0
description: "Credential rotation scaffold via CronJob (stub — generates and stores passwords in Secret, does not yet execute ALTER USER on upstream DB)."
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
  - "datastore.*"
provides:
  - credential
---

# security.password-rotation

Credential rotation scaffold. Deploys a CronJob that generates new passwords
and stores them in a Kubernetes Secret. **Stub**: does not yet execute
`ALTER USER` on the upstream database — the Secret is updated but the actual
DB credentials remain unchanged.

## Composition Notes

- **Must** wire `upstream-dsn` from a database engine block.
- Outputs `credential` for downstream consumers needing DB access.
- Uses a CronJob with a lightweight container to generate passwords.
- **Current limitation**: rotation script updates the Secret only; upstream DB password is not changed.

## Kubernetes Resources Created

- CronJob: `{cluster}-{name}` with rotation script
- Secret: `{cluster}-{name}-creds` (current credentials)
- ConfigMap: `{cluster}-{name}-config` (rotation configuration)
- ServiceAccount: `{cluster}-{name}-sa`
- Role + RoleBinding: for Secret update permissions
