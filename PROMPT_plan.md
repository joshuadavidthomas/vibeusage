0a. Study `specs/*` with parallel Sonnet subagents to learn the application specifications.
0b. Study @PLAN.md (if present) to understand the plan so far.
0d. For reference, the application source code is in `src/*`.

1. Study @PLAN.md (if present; it may be incorrect) and use Sonnet subagents to study existing source code in `src/*` and compare it against `specs/*`. Use an Opus subagent to analyze findings, prioritize tasks, and create/update @PLAN.md as a bullet point list sorted in priority of items yet to be implemented. Ultrathink. Consider searching for TODO, minimal implementations, placeholders, skipped/flaky tests, and inconsistent patterns. Study @PLAN.md to determine starting point for research and keep it up to date with items considered complete/incomplete using subagents.

IMPORTANT: Plan only. Do NOT implement anything. Do NOT assume functionality is missing; confirm with code search first.

ULTIMATE GOAL: We want to achieve a CLI application to easily see the usage stats from all LLM providers in order to track and understand session usage windows and costs so one can understand if the full value is being used from a subscription and the costs associated with it and API usage. Consider missing elements and plan accordingly. If an element is missing, search first to confirm it doesn't exist, then if needed author the specification at specs/FILENAME.md. If you create a new element then document the plan to implement it in @PLAN.md using a subagent.
