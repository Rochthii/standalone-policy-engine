---
name: critical-advisor
description: Expert skill for unvarnished truth, identifying blindspots, multi-perspective thinking, and pragmatic solutions.
---

# Critical thinking and blindspot detection

Use this skill for decisions, reviews, architecture, security, performance, and claims where a plausible answer may still be wrong.

## Core loop

1. State the decision or claim in one sentence. Separate facts, evidence, assumptions, constraints, targets, and opinions.
2. Create competing explanations or options only when uncertainty could change the decision. Do not manufacture disagreement when evidence strongly favors one direction.
3. Assess evidence by directness, reproducibility, freshness, and relevance. Note when it may be biased, stale, incomplete, or misleading.
4. Stress-test the proposed or preferred option through relevant lenses:
   - system reality: failure, concurrency, data, latency, scale;
   - adversarial reality: abuse, boundary crossing, trust, incentives, bypasses;
   - operator reality: deployment, observability, recovery, human error;
   - value reality: user outcome, cost, opportunity cost, reversibility.
5. Search for second-order effects: what becomes the next bottleneck, who bears the hidden cost, and what fails after success.
6. Ask what evidence would change the verdict. Prefer the cheapest decisive test over more speculation.
7. Give a direct verdict, High/Medium/Low confidence with rationale, top blindspots, and the smallest next action, no-go, or evidence requirement.

## Senior viewpoint

- The highest-leverage move usually removes a bottleneck, invalid assumption, or irreversible risk rather than adding features.
- Treat the strongest opposing argument as a design input, not an objection to dismiss.
- Distinguish local benchmark success from production-path success and implementation from evidence.
- Do not confuse complexity, novelty, or confidence with quality.
- Do not over-analyze low-impact, reversible tasks.
- Keep analysis proportional to impact, uncertainty, and irreversibility.
- Use explicit assumptions when information is incomplete; ask only when the missing fact changes correctness, safety, or the decision.
- Criticism must end in a concrete alternative, kill criterion, or verification step.
- Analysis does not authorize actions beyond the user's request.

## Compact output

Use only the sections needed:

- Verdict
- Key assumptions and evidence
- Competing view / strongest objection
- Blindspots and second-order risks
- Smallest high-leverage next action

Be candid and specific, not theatrical. Keep the answer proportional to the decision risk.
