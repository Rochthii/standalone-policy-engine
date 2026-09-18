# External Audit Archive — 2029 Design Decision

## Status and decision

This is the selected target design for the graduation-thesis deployment; it is
not external WORM evidence and does not close G6. The current PDP remains a
prototype until the rehearsal below succeeds against an independently
administered AWS account.

**Selected provider:** Amazon S3 Object Lock, **COMPLIANCE** mode, in a
dedicated archive AWS account. The archive stores the already encrypted,
redacted audit payloads produced by `internal/audit`; PostgreSQL remains the
operational query store, not the WORM store.

The selection fits the ERP purchasing/audit scenario in the thesis proposal:
the archive needs object-version retention, a non-bypassable protection mode
and a reproducible Go/AWS integration. S3 Compliance mode prevents every
principal, including the account root user, from overwriting or deleting a
protected version or shortening its retention period. Governance mode is only
allowed in the disposable rehearsal bucket because a specially authorized user
can bypass it. Object Lock is version based, so an uploader must use a fresh,
immutable object key for every archive segment and must never rely on a
delete marker as proof of deletion.

The 7-year retention below is a project policy for the 2029 ERP thesis, not a
statement of Vietnamese or customer-specific legal compliance. Before a real
deployment, the data controller and counsel must approve the jurisdiction,
data residency, retention interval and legal-hold authority.

## Target resources and controls

| Resource | Mandatory configuration | Retention |
|---|---|---:|
| `pdp-audit-archive-<environment>-<suffix>` | General-purpose S3 bucket; versioning and Object Lock enabled; Object Ownership `BucketOwnerEnforced`; Block Public Access; SSE-KMS; default Object Lock `COMPLIANCE` | 7 years |
| `pdp-audit-evidence-<environment>-<suffix>` | Separate general-purpose S3 bucket with the same hardening; receives CloudTrail object-level data events with log-file validation, Object Lock configuration evidence and signed rehearsal manifests | 9 years |
| Archive KMS key | Separate customer-managed key; publisher can encrypt only; decryption is granted only through the approved evidence-retrieval role; key deletion is prohibited until the last protected object and evidence record have expired | at least 9 years |

The archive account is separate from the PDP workload account. The PDP archive
publisher receives only `s3:PutObject` for its assigned prefix plus the minimum
KMS encryption permissions. It receives no `DeleteObject`, `DeleteObjectVersion`,
`PutObjectRetention`, `PutObjectLegalHold`, `BypassGovernanceRetention`, bucket
policy, lifecycle, versioning or KMS-key administration permission. Bucket
policy denies public access and those destructive/control actions to every
workload role.

`ArchiveComplianceOfficer` is a separately administered human-break-glass role.
It may set or release a documented legal hold only under an approved case ID;
it cannot shorten Compliance retention. Archive infrastructure changes require
two-person review. The workload account cannot assume either role.

## Archive and deletion evidence flow

1. The archive worker reads only committed PostgreSQL audit rows, serializes
   encrypted entries into an NDJSON segment and writes a unique key of the
   form `tenant=<id>/date=YYYY-MM-DD/<ULID>.ndjson`.
2. It writes a manifest with segment SHA-256, audit-ID range, row count,
   S3 version ID, Object Lock mode and retain-until timestamp. The manifest is
   itself archived as a separate immutable object.
3. CloudTrail must record S3 object-level `PutObject`, `DeleteObject`,
   `DeleteObjectVersion`, `PutObjectLockRetention` and legal-hold operations
   for both buckets. Deliver those logs and digest files to the independent
   evidence bucket; do not use S3 server-access logging because Object Lock
   buckets cannot be its destination.
4. After seven years and only without a legal hold, the controlled disposal
   job records a signed destruction manifest in the 9-year evidence bucket,
   then requests deletion by exact key and version ID. CloudTrail's resulting
   object-level event and the retained manifest are the deletion evidence.
   A lifecycle rule must never delete a protected version or evidence object.

The design deliberately does not make a simple delete request. With Object
Lock, a simple delete can create a delete marker while leaving a protected
version intact; the rehearsal must operate by version ID.

## Required external rehearsal

Use a disposable isolated AWS account and a distinct short-retention bucket
first. Never test destructive controls against the 7-year archive bucket.

1. Provision the two target-shaped buckets and roles through reviewed IaC.
   Confirm Object Lock, versioning, default `COMPLIANCE` retention, public
   access block, ownership controls, KMS policy, CloudTrail selectors and the
   7/9-year values by API—not screenshots.
2. Upload one encrypted sample segment through the restricted publisher role.
   Fetch its version-specific Object Lock retention and verify the mode and
   retain-until timestamp.
3. Attempt `DeleteObject` with that exact version ID using both publisher and
   archive-administrator credentials. Both attempts must fail with `403` while
   the object is under Compliance retention; record the command, request IDs,
   version ID and CloudTrail data event.
4. Verify the evidence bucket contains the corresponding `PutObject` and
   failed deletion event plus the CloudTrail digest files. Store the redacted
   command output, IaC revision, AWS account aliases, region, UTC timestamps
   and object/version identifiers in a committed evidence record.
5. Re-run with the production retention values in a non-destructive config
   assertion. The G6 checkbox may become `[x]` only when an automated release
   job requires this rehearsal and preserves its external evidence artifact.

## 2029 preflight

Revalidate this decision against the then-current provider documentation,
pricing, region/data-residency commitments and thesis environment before
provisioning. If AWS is unavailable or conflicts with the approved data
residency, Azure Immutable Blob Storage is the first substitute to assess;
it provides locked time-based WORM, but the implementation and rehearsal must
be redone rather than relabeling this evidence.

## Primary references

- [Amazon S3 Object Lock](https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-lock.html)
- [Configure Amazon S3 Object Lock](https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-lock-configure.html)
- [Amazon S3 CloudTrail events](https://docs.aws.amazon.com/AmazonS3/latest/userguide/cloudtrail-logging-s3-info.html)
- [AWS CloudTrail data events](https://docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-events.html)
