---
name: gv-analyze
description: Analyze recurring review findings and speed levers across finished good-vibes runs. Use when the user asks for a review analysis, recurring review issues, or how to speed up the implement loop.
---

# Analyze review history

`good-vibes analyze` builds a deterministic digest from the finished run folders. It groups findings, measures where
time went, and applies the speed rules. You add the judgment: rename the clusters, pick one guard for each theme, and
weigh each lever against what it could cost.

## Steps

1. Run `good-vibes analyze --json`. Narrow it with `--since 30d`, `--change <slug>`, or `--runs N` only when the person
   asked for a smaller window. Do not pass `--full` unless you need every finding in one cluster; it makes the digest
   large.
2. Read `coverage` before giving any advice. Report how many runs hold findings, verify logs, and task timing, and name
   the files in `unreadable_files`. If the history is too thin for a useful pattern, say so plainly and stop at coverage
   and the fired levers.
3. Read `inventory` before proposing a guard. It lists `checks` (name, `run`, `paths`, `fix`, `gates`), `lenses` (name,
   title, `stages`, guidance, `short`, `error`), and `guards` (path, `present`). Open the files you cite when their
   contents decide the advice. A lens with an `error` is still listed; do not treat it as a working guard. The guard
   list cannot see raw git hooks, because a worktree's `.git` is a file; check `.git/hooks` or `core.hooksPath` yourself
   before proposing a hook.
4. Rename every cluster from its `quotes`, `top_paths`, `top_packages`, `categories`, and `reviewers`. The digest's
   `name` is only three tokens. Merge or split a cluster only when the evidence supports it, and keep the original `id`
   in the report.
5. For each recurring cluster, choose the cheapest guard that prevents the issue and says why. Work through the options
   in order and name the one you picked: a linter rule or config change, a new or tighter verify check, a test helper,
   one line in `AGENTS.md`, a repo lens, or a reviewer or worker prompt edit. Write the concrete snippet: the exact lint
   setting, the `[[implement.verify.checks]]` table, the helper signature, the instruction line, or the prompt sentence.
   A cluster with a high `repeated_next_round` is a worker-side problem, so prefer a worker prompt or a fixing check
   over a new reviewer rule.
6. Suggest a lens only when the cluster spans different changes, the concern applies to every future change, and the
   inventory has no lens that covers it. Name the narrowest `stages` from `proposal`, `design`, and `review`, draft a
   guidance body at or under `thresholds.lens_guidance_short_bytes`, and write the `[lenses.<name>]` table beside it.
   Use a lowercase slug, a unique title, and no built-in heading such as `Context`, `Decision`, `Risks`, or `Reviews`.
   Never suggest a lens for a one-off finding, a single-task repeat, or a worker round problem: a lens makes `check`
   fail every later change in its stage.
7. For each lever whose `status` is `fires`, state what could be lost and whether its evidence supports the move.
   `defer-check` shows the failures it caught, whether a phase or final gate already covers it, and how many failures
   reached the final gate. Prefer moving work later over removing it, and say when a check should stay early because it
   fails often. For `order-checks`, keep every fixing check before every reading check. For `coarser-review-cadence`,
   weigh the listed rejections and `later_repeats`. For `guard-cluster`, repeat that the saving is an estimate.
8. Rank the suggestions by estimated wall time saved, then by recurrence. Say which numbers are partial: reviewer time
   is unmeasured, and `other` mixes review time with overhead.
9. Write the report to `.good-vibes/analysis/<timestamp>.md`, using UTC in `YYYYMMDD-HHMMSS` form to match run folders,
   then print its path. Create the folder when it is missing.

## Report

```markdown
# Review analysis <timestamp>

## Coverage

- Window: <runs, since, change filter>
- Findings: <n> in <n> of <n> runs. Verify logs: <n> of <n>. Task timing: <n> of <n>.
- Unreadable: <files>

## Recurring themes

### cluster-01 — <renamed theme>

- Evidence: <count> findings in <runs> runs, <changes> changes, <tasks> tasks, <first> to <last>.
- Categories: <mix>. Reviewers: <mix>. Paths: <top paths>.
- Quotes: <shortest quotes with their files>.
- `repeated_next_round`: <n> of <count>. <What that means for the worker.>
- Guard: <cheapest guard and why>. <Concrete snippet.>
- Lens: <none, or the table and guidance draft.>

## Speed levers

### defer-check:test — fires

- Evidence: <share of verify time, gates, failures, final-gate coverage>.
- What could be lost: <failures it caught and where>.
- Advice: <move, keep, or order the check, and why>.

## Too thin to judge

<Coverage gaps, unreadable files, and rules that need more history.>
```

## Rules

- Cite a cluster id or lever id for every suggestion. Use `cluster-01`, `guard-cluster:cluster-01`, `defer-check:test`,
  or the exact id in the digest. No id, no suggestion.
- Never open `.good-vibes/runs/` or the logs. The digest and the cited guard files hold what you need.
- Do not propose a guard the inventory already has. If a configured check or lens nearly covers the issue, say how to
  tighten it instead.
- Lens suggestions are permanent gates. A lens must pass `check` on every later change in its stage, so suggest one only
  for a recurring, cross-change concern with no covering lens.
- Speed advice never trades quality silently. Every fired lever names its evidence and what moving the work could cost.
- This skill edits no config, lint file, or prompt. Write the report only. A person turns the advice into a change, or
  adds a suggested lens with `gv-configure`.
