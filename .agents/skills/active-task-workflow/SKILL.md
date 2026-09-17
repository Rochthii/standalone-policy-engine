---
name: active-task-workflow
description: Execute one repository task from ACTIVE_TASK.md with a bounded scope, stated validation, evidence, and an authorized commit/push.
---

# Active Task Workflow

Use this skill when the repository root contains `ACTIVE_TASK.md` and the user
asks to execute or continue its task. It does not apply to planning, general
questions, or work outside that file's declared scope.

1. Read `AGENTS.md` and `ACTIVE_TASK.md`, then inspect the branch and working
   tree. If unrelated changes are present, report the blocker before staging or
   committing anything.
2. Read only the files named by `Scope`, make the smallest change that meets
   `Acceptance`, and run the command in `Validation`.
3. Record evidence only when validation supports the claim. Keep the task
   active when validation fails or a required external gate is unavailable.
4. Commit and push only when the user task explicitly authorizes them. Report
   the resulting commit hash; do not edit `Commit: pending` to self-reference
   that same commit.
5. Replace `ACTIVE_TASK.md` with a new task only after the previous task has
   been successfully pushed, or after the user explicitly changes the task.

Final report is limited to: changed, validation, commit, and blocker.
