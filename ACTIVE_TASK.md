# Active Task

ID: AUD-ARCHIVE-03
Goal: Rehearse immutable archive retention and deletion evidence on isolated AWS
Scope: approved AWS archive account, Terraform apply, CloudTrail evidence
Acceptance: Object Lock deletion by version ID returns 403; retained CloudTrail evidence is recorded
Validation: approved AWS rehearsal command; evidence artifact; git diff --check
Commit: 6d674a5
