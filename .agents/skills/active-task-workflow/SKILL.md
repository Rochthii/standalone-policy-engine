---
name: active-task-workflow
description: Complete or resume the task in ACTIVE_TASK.md, batching related changes and verification without reopening finished work.
---

# Active Task Workflow

Use this skill when the repository root contains `ACTIVE_TASK.md` and the user
asks to execute or continue its task. It does not apply to planning, general
questions, or work outside that file's declared scope.

## Resume from the delta

- Use root `AGENTS.md`, the active task and its linked evidence as the entry
  points. Inspect branch/worktree changes; preserve unrelated edits rather than
  treating their presence alone as a blocker.
- Load required skills in full under the session's skill rules. During the same
  ongoing execution, reuse available unchanged context; reload instructions when
  required, changed or missing. Do not scan the proposal, history or all skills.
- Read only unresolved evidence rows and directly affected code/dependencies.
  A summary is a navigation aid, not proof that code or test inputs are unchanged.

## Finish one acceptance boundary

- Identify the remaining acceptance gaps before patching. Group checks sharing
  fixtures or a trust boundary into one implementation pass; keep their existing
  case IDs. Internal steps are not new project tasks or separate approval turns.
- Continue through implementation, relevant checks and evidence reconciliation
  while useful authorized work remains. Do not stop after each passing subcase
  to ask the user to say "continue".
- A newly found in-scope defect belongs to this task. Record unrelated findings
  briefly; do not expand scope. Pause for missing authority, a material scope
  decision or an external blocker; never weaken acceptance to finish sooner.

## Verify proportionally

- Run focused checks during repair, then the required consolidated boundary gate
  on the final relevant code/test/configuration state. Prefer one successful exit
  gate per batch, not a full gate after every case.
- Reuse a passing result only when its inputs and relevant environment are
  unchanged. On failure, diagnose and rerun the affected check first; rerun the
  gate if its inputs changed or its result is uncertain. Docs-only edits do not
  trigger Odoo/Docker tests unless they change executable inputs.
- Keep command, result, tested revision or dirty-worktree scope and environment
  in the existing evidence artifact. Do not paste full logs into active status.

## Checkpoint and close once

- `ACTIVE_TASK.md` is the compact resume pointer: goal/acceptance, remaining
  gaps, touched boundaries, evidence link, blocker and exact next action. Update
  it before a handoff/interruption, not after every command.
- Keep detailed evidence in one existing ledger. At closure or a material
  handoff, reconcile board/changelog once; update the current-state audit only
  when verified implementation claims change. Link instead of copying history.
- Acceptance completion and Git publication are separate: record VERIFIED with
  commit/push pending when appropriate. Advance only after acceptance passes and
  the next task is authorized; push is not an implicit prerequisite or permission.
- Commit/push only with explicit authorization and scoped staging. Final report:
  outcome/files, validation, unresolved risk, next action; publication if relevant.
