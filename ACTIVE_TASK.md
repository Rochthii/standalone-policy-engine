# Active Task

ID: OPS-DEPLOY-06
Goal: Harden PDP and Control Plane container runtime privileges
Scope: PDP/Control Dockerfiles, Kubernetes deployments, container validation, deployment evidence
Acceptance: runtime uses non-root, read-only root filesystems, dropped capabilities and explicit writable mounts
Validation: Docker build/inspect, Compose config, kubectl client dry-run, git diff --check
Commit: pending
