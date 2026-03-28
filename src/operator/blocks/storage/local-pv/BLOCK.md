---
kind: storage.local-pv
category: storage
version: 1.0.0
description: Local PersistentVolume storage for database data. Ephemeral — data does not survive node loss.
ports:
  - name: pvc-spec
    portType: pvc-spec
    direction: output
parameters:
  - name: size
    type: string
    default: "1Gi"
    required: true
    description: Storage size (e.g. 1Gi, 10Gi).
  - name: storageClass
    type: string
    default: "local-path"
    description: Kubernetes StorageClass name.
requires: []
provides:
  - pvc-spec
---

# storage.local-pv

Provides local PersistentVolumeClaim specs for database engines. Uses the
cluster's default local-path provisioner (available in k3d out of the box).

## Composition Notes

- This is the simplest storage block. Suitable for local dev and testing.
- Data does **not** survive node deletion — acceptable for dev environments.
- Wire `pvc-spec` output to any engine block's `storage` input.

## Kubernetes Resources Created

- PersistentVolumeClaim template (embedded in the engine's StatefulSet volumeClaimTemplates)
