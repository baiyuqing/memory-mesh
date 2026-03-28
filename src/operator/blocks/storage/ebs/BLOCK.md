---
kind: storage.ebs
category: storage
version: 1.0.0
description: AWS EBS storage provisioner. Uses StorageClass with EBS CSI driver (simulated via LocalStack for local dev).
ports:
  - name: pvc-spec
    portType: pvc-spec
    direction: output
parameters:
  - name: size
    type: string
    default: "10Gi"
    required: true
    description: Volume size.
  - name: volumeType
    type: string
    default: "gp3"
    description: "EBS volume type: gp2, gp3, io1, io2."
  - name: iops
    type: int
    default: "3000"
    description: Provisioned IOPS (for gp3/io1/io2).
  - name: encrypted
    type: string
    default: "true"
    description: Enable EBS encryption.
  - name: storageClass
    type: string
    default: "ebs-sc"
    description: Kubernetes StorageClass name.
requires: []
provides:
  - pvc-spec
---

# storage.ebs

Provisions AWS EBS volumes via a StorageClass. Drop-in replacement for
`storage.local-pv` when you need durable, cloud-backed storage.

## Composition Notes

- Directly substitutable for `storage.local-pv` — engine blocks consume
  `pvc-spec` identically.
- Creates a StorageClass resource referencing the EBS CSI driver.
- In local dev with LocalStack, uses simulated EBS.

## Kubernetes Resources Created

- StorageClass: `{cluster}-{name}-sc` with EBS CSI provisioner
