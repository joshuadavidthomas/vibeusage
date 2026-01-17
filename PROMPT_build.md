0a. Study `specs/*` with parallel Sonnet subagents to learn the application specifications.
0b. Study @PLAN.md.
0c. For reference, the application source code is in `src/*`.
0d. For reference, the test suite is in `tests/*`.

1. Your task is to implement functionality per the specifications using parallel subagents.
2. Follow @PLAN.md and choose the most important item to address. **PICK ONLY ONE ITEM.**
3. Before making changes, search the codebase (don't assume not implemented) using Sonnet subagents. You may use parallel Sonnet subagents for searches/reads and only 1 Sonnet subagent for build/tests. Use Opus subagents when complex reasoning is needed (debugging, architectural decisions).
4. After implementing functionality or resolving problems, run the tests for that unit of code that was improved. If functionality is missing then it's your job to add it as per the application specifications. Ultrathink.
5. Write quality tests for every implementation. Tests that mock everything or assert nothing don't count. Ask: "Would this fail if the code broke?"
6. When you discover issues, immediately update @PLAN.md with your findings using a subagent. When resolved, update and remove the item.
7. When the tests pass, update @PLAN.md, then `git add -A` then `git commit` with a message describing the changes. After the commit, `git push`.

99999. Important: When authoring documentation, capture the why — tests and implementation importance.
999999. Important: Single sources of truth, no migrations/adapters. If tests unrelated to your work fail, resolve them as part of the increment.
9999999. As soon as there are no build or test errors create a git tag. If there are no git tags start at 0.0.0 and increment patch by 1 for example 0.0.1  if 0.0.0 does not exist.
99999999. You may add extra logging if required to debug issues.
999999999. Keep @PLAN.md current with learnings using a subagent — future work depends on this to avoid duplicating efforts. Update especially after finishing your turn.
9999999999. When you learn something new about how to run the application, update @AGENTS.md using a subagent but keep it brief. For example if you run commands multiple times before learning the correct command then that file should be updated.
99999999999. For any bugs you notice, resolve them or document them in @PLAN.md using a subagent even if it is unrelated to the current piece of work.
999999999999. Implement functionality completely. Placeholders and stubs waste efforts and time redoing the same work.
9999999999999. When @PLAN.md becomes large periodically clean out the items that are completed from the file using a subagent.
99999999999999. If you find inconsistencies in the specs/* then use an Opus 4.5 subagent with 'ultrathink' requested to update the specs.
999999999999999. IMPORTANT: Keep @AGENTS.md operational only — status updates and progress notes belong in `PLAN.md`. A bloated AGENTS.md pollutes every future loop's context.
9999999999999999. Before ending: verify tests exist for modules you touched. No tests = add to @PLAN.md.

<next>
**IMPORTANT** This is your next task.

## Usage Display is NOT to Spec

The usage command output does not match spec `05-cli-interface.md`. There are TWO distinct formats that must be implemented correctly:

---

### Issue 1: Missing Provider Commands

Per spec lines 49-56, these top-level commands should exist but DO NOT:
- `vibeusage claude` → currently "No such command 'claude'"
- `vibeusage codex` → currently "No such command 'codex'"
- etc.

These should be **aliases** that behave identically to `vibeusage usage <provider>`.

---

### Issue 2: Single Provider View is Wrong

When showing ONE provider (e.g., `vibeusage claude` or `vibeusage usage claude`), the format should be:

```
Claude
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Session (5h)  ████████████░░░░░░░░ 58%    resets in 2h 15m

Weekly
  All Models  ████░░░░░░░░░░░░░░░░ 23%    resets in 4d 12h
  Opus        ██░░░░░░░░░░░░░░░░░░ 12%    resets in 4d 12h
  Sonnet      ██████░░░░░░░░░░░░░░ 31%    resets in 4d 12h

╭─ Overage ──────────────────────────────────────────────╮
│ Extra Usage: $5.50 / $100.00 USD                       │
╰────────────────────────────────────────────────────────╯
```

**Key features:**
- Title line with `━━━` separator (NO panel wrapper for the main content)
- Session period standalone, then **blank line**
- "Weekly" section **header** (bold)
- Model-specific periods are **indented** (`  All Models`, `  Sonnet`)
- Only overage uses a Panel
- Grid columns: period name | bar + percentage | reset time

See `ccusage_example.py` `ClaudeUsage.__rich_console__()` for a working reference implementation.

---

### Issue 3: Multi-Provider View Should Be Compact

When showing ALL providers (e.g., `vibeusage` or `vibeusage usage`), the format should be:

```
╭─ Claude ───────────────────────────────────────────────╮
│ Session (5h)  ████████████░░░░░░░░ 58%   resets in 2h  │
│ Weekly        ████░░░░░░░░░░░░░░░░ 23%   resets in 4d  │
╰────────────────────────────────────────────────────────╯
╭─ Codex ────────────────────────────────────────────────╮
│ Session       ██████████░░░░░░░░░░ 50%   resets in 3h  │
│ Weekly        ██░░░░░░░░░░░░░░░░░░ 12%   resets in 5d  │
╰────────────────────────────────────────────────────────╯
```

**Key features:**
- Uses Panel wrapper with provider name as title
- **COMPACT** - NO model-specific periods (skip where `period.model is not None`)
- Only shows main periods (Session, Weekly)
- Grid columns: period name | bar + percentage | reset time

See spec 05-cli-interface.md lines 386-398:
```python
for period in self.snapshot.periods:
    if period.model is None:  # Skip model-specific in compact view
```

---

### Current Output (WRONG)

```
╭────────────────────────────────────────────── ─ Claude ─ ──────────────────────────────────────────────╮
│ 5-hour session  █░░░░░░░░░░░░░░░░░░░ 6%    resets in 4h 1m                                             │
│ 7-day period    █████░░░░░░░░░░░░░░░ 28%  resets in 5d 16h                                             │
│ 7-day: Sonnet   ░░░░░░░░░░░░░░░░░░░░ 3%   resets in 6d 16h  ← SHOULD NOT APPEAR IN MULTI-PROVIDER VIEW │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

Problems:
1. Multi-provider view shows model-specific periods (7-day: Sonnet) - should be compact
2. Single provider view uses panels - should use title+separator format
3. No top-level provider commands exist

---

### Files to Modify

1. `src/vibeusage/cli/app.py` - Add provider command aliases
2. `src/vibeusage/cli/display.py` - Fix `ProviderPanel` to skip model-specific periods
3. `src/vibeusage/cli/commands/usage.py` - Fix `display_snapshot` for single provider format

### Reference

See `ccusage_example.py` for a working implementation of the single-provider format. Delete this file after the fix is complete.

After fixing, update PLAN.md and delete this `<next>` block and `ccusage_example.py`.
</next>
