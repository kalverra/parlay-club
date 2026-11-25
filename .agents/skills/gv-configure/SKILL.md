---
name: gv-configure
description: Configure good-vibes for a repository. Use for good-vibes setup, editing the project config, choosing verify and final-gate checks, or adding repo lenses.
---

# Configure good-vibes

Write only the project configuration this repository needs. The full key reference is `docs/reference/config.md` in the
good-vibes repository.

```mermaid
flowchart TD
  READ[Read project config and merged origins] --> INSPECT[Inspect manifests, scripts, tools, and CI]
  INSPECT --> EVIDENCE{Commands and ownership clear?}
  EVIDENCE -->|no| ASK[Ask the person or report the unknown]
  EVIDENCE -->|yes| LENS[Ask whether the repo wants lenses]
  LENS -->|yes| ADDLENS[Write lens tables and guidance files]
  LENS -->|no| WRITE[Write minimal project overrides]
  ADDLENS --> WRITE
  WRITE --> FAST[Fast per-turn verify checks]
  WRITE --> FINAL[Slow or thorough final-gate checks]
  FAST --> VALIDATE[config show and verify]
  FINAL --> VALIDATE
  VALIDATE -->|failure| FIX[Fix config and rerun]
  VALIDATE -->|pass| REPORT[Report settings and checks]
```

## Steps

1. Read `good-vibes.toml` and run `good-vibes config show --origin`. The origin view says which layer supplied each
   value. Preserve unrelated settings and comments, and edit only the project config. Do not copy user, local,
   environment, or profile values into it, and do not flatten those layers.
2. Inspect the repository before choosing commands. Read manifests, lockfiles, package scripts, tool configuration, and
   CI workflows. Find the commands the project already uses for lint, format, test, type-check, and build. Do not assume
   a language, and do not copy a starter or reference example until you have checked it against the repository. Every
   active `run` must be a command this project can execute, never a placeholder such as `./path/to`.
3. Ask the person whether this repo has a concern every change must answer, such as a threat model, a compatibility
   story, or a migration and rollback plan. Lenses are opt-in and none ship built in. Ask once, and do not invent one on
   their behalf: a lens makes `check` fail every proposal or design in its stage until the section holds prose, and a
   review-stage lens rides in every reviewer prompt. When they name concerns, add a `[lenses.<name>]` table for each,
   with a unique `title`, a `guidance` path, and `stages` (omit it for all three). Use a lowercase slug for the name,
   and keep the title off every built-in heading such as `Context`, `Decision`, `Risks`, or `Reviews`. A lens applies
   only to changes that have not passed its stage yet, so say so when the repo has work in flight. Write each guidance
   file short and concrete, because it is the comment the author fills and the rule the reviewer applies. It must be
   non-empty, must not contain `-->`, and its front matter is dropped. Put it outside the docs tree, such as
   `lenses/<name>.md`; a file under `docs/` must also pass `docs lint`, so give it a configured doc type and front
   matter.
4. Write only intentional overrides. Keep the per-turn `[implement.verify]` gate fast and deterministic: commands that
   finish quickly, need no network, and give stable results. Put broad suites, race detectors, expensive builds, and
   other slow or thorough commands in `[implement.final.verify]`. Do not turn on an empty gate or add a check that
   repeats another.
5. Give every check a unique `name` and a full `run`. Add `quick` only when a useful fast form exists for the worker. It
   is guidance, so it may contain a location the worker narrows, but it must otherwise be a real command. Add `paths`
   only when they state the real dependency boundary, including every input, output, config file, and fixture. Omitted
   or empty `paths` mean always run; that is safer than a guessed filter.
6. Set `fix = true` on every check that may write. Keep fixing checks before reading checks, because fixers run first in
   config order. A fixing check's `paths` must cover every file it reads and every file it may add, change, or delete.
   Do not mark a read-only check as a fixer.
7. Validate the result:
   - `good-vibes config show` must load the merged config. It also rejects a bad lens name, title, or stage.
   - Run the per-turn checks with `good-vibes verify`. Use `good-vibes verify --exec -- <command>` when you need a
     final-only command in the run environment.
   - Exercise a final-only command directly when no run owns the worktree.
   - Run `good-vibes check` when you added a lens. It loads every guidance file and reports one that is missing, empty,
     or holds `-->`.
   - Never report a command as tested when you did not run it.

Report the settings you wrote, which checks run per turn versus at the final gate, and each lens with its stages. Leave
a real project failure visible instead of weakening or deleting a check to make it pass.
