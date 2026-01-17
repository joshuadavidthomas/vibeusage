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

The subcommands are gone. 

vibeusage on  main [$!?] is 󰏗 v0.1.0  v3.14.0 (vibeusage)
➜ vibeusage --help

 Usage: vibeusage [OPTIONS] COMMAND [ARGS]...

 Track usage across agentic LLM providers

╭─ Options ──────────────────────────────────────────────────────────────────────────────────────────────╮
│ --json                -j        Enable JSON output mode                                                │
│ --no-color                      Disable colored output                                                 │
│ --verbose             -v        Enable verbose output                                                  │
│ --quiet               -q        Enable quiet mode                                                      │
│ --version                       Show version and exit                                                  │
│ --install-completion            Install completion for the current shell.                              │
│ --show-completion               Show completion for the current shell, to copy it or customize the     │
│                                 installation.                                                          │
│ --help                          Show this message and exit.                                            │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────╯
╭─ Commands ─────────────────────────────────────────────────────────────────────────────────────────────╮
│ claude    Show usage statistics for this provider.                                                     │
│ codex     Show usage statistics for this provider.                                                     │
│ copilot   Show usage statistics for this provider.                                                     │
│ cursor    Show usage statistics for this provider.                                                     │
│ gemini    Show usage statistics for this provider.                                                     │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────╯

Please investigate why all the other subcommands are gone and fix. Update the @PLAN.md with the details.

Once complete delete this <next> block.
</next>
