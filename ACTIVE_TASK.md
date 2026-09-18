# Active Task

ID: CI-COMPAT-05
Goal: Enforce protobuf backward compatibility in pull-request CI
Scope: .github/workflows/ci.yml, buf.yaml, readiness checklist, CI evidence
Acceptance: Buf breaking check compares every pull request with its target branch
Validation: buf breaking against main; git diff --check
Commit: f53894f
