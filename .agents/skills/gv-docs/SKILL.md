---
name: gv-docs
description: Write or update good-vibes living docs in docs/. Use when a change touches behavior, and when the user asks for documentation.
---

# Write living docs

`docs/` describes the system as it is now, not its history. A change updates the docs it invalidates.

## Principles

Six principles decide what belongs in a doc. Each names the findings that catch its common failure, and the judgment
only you can make.

1. **Explain what code cannot.** A doc says why, not what the code already says. `budget-lines` and `budget-spans` flag
   a doc that retells mechanics; move the mechanics to a reference doc or split the concept.
2. **Generate what can be.** A list of names, keys, or flags is a generator's job. `hand-table` flags a table of three
   or more names from one configured vocabulary, and a stale generated region is a drift finding; make repeated facts
   generated regions. Do not edit a region's body by hand.
3. **Little code, tested code.** `budget-fences` keeps a doc to the fence languages its type expects, and the fence
   checks run each sample. Prefer a sample that runs; mark one `good-vibes:unchecked` only when it cannot be made true.
   Then ask whether the sample earns its place at all.
4. **Stay true.** `names-path` and the vocabulary findings (`names-<vocab>`) catch a name that no longer exists, and
   `good-vibes check` reports a stale generated region and a stale map. The final reviewer reads the docs a diff leaves
   alone. After lint passes, judge whether the prose still matches the code.
5. **Easy to find.** `summary` keeps the one sentence the map shows, and the ADR map keeps history out of the living
   docs. Judge whether the title and summary use the words a reader would search for.
6. **One home per fact.** Every rule or contract lives in one living doc; other docs link to it instead of restating it.
   `covers-overlap` flags same-type docs or unlinked cross-type docs that cover the same paths. Keep `reference/` to
   external contracts (flags, schemas, outputs), not internal control flow.

## Doc types

The folders are configured in `good-vibes.toml`. The default split follows Diátaxis:

- `concepts/` — how a part of the system works, and why
- `guides/` — task-oriented steps for a person using the system
- `runbooks/` — operating procedures, including rollback
- `reference/` — exact behavior: flags, endpoints, schemas
- `decisions/` — one ADR per significant decision, written by `good-vibes done`

Three single files sit outside `docs/` and are read first: `README.md` is the project's front door, `ARCHITECTURE.md`
shows the system as diagrams, and `GLOSSARY.md` fixes the words the code and docs use. All three are configured under
`[docs.high_level.<name>]`.

## Docs vs. code comments

Keep each kind of explanation in the layer that owns it:

| Layer | What belongs here | Examples |
| --- | --- | --- |
| **Code comment** | Local *why* scoped to one function or file | Invariants, edge-case guards, ordering constraints, performance bounds |
| **Living doc** | Cross-cutting mental models and contracts | Package roles, workflows, runbooks, flags, schemas, external interfaces |
| **ADR** | Historical *why* | Alternatives considered, trade-offs, consequences |

When a surprising constraint comes from an ADR, add a one-line code comment pointing to the ADR instead of restating it.

## Steps

1. Read `ARCHITECTURE.md` and `GLOSSARY.md` before writing. A doc that contradicts the code map or renames a glossary
   term is wrong even if its own page reads well.

2. Find the docs the change invalidates. Run `good-vibes docs impact` (add `--base <ref>` to compare against an older
   commit) to list the docs whose `covers` entries the diff touched and that the diff did not edit. Open and judge each
   one. Docs without `covers` are the fallback: `docs/llms.txt` lists the living docs, and each type with its own map,
   such as `docs/decisions/llms.txt` for ADRs, lists its docs. Read the maps, compare them with the paths the change
   touches, and open each doc whose summary covers behavior the change alters.

3. Update each in the same change as the code. Read it with `good-vibes docs get <id>#<heading>` when you only need one
   section.

4. If no doc covers new behavior, add one in the right folder. Start from the type's template:

   ```sh
   good-vibes docs template reference --id config --title "Configuration"
   ```

5. Fill in the front matter. `summary` is one sentence, and it is what the docs map shows a reader searching for the
   doc.

6. Update the high-level docs in the same change. Add a domain term to `GLOSSARY.md` as you coin it (not internal helper
   or struct names). Edit `ARCHITECTURE.md` only when the change alters a module's responsibility, an invariant, or a
   cross-cutting concern, and only with an accepted change that lists `architecture` in `amends`. Edit `README.md` only
   when the change alters the project's top-level overview, positioning, or quickstart, and only with an accepted change
   that lists `readme` in `amends`.

7. Run `good-vibes docs lint`, then `good-vibes docs index` so every docs map matches. If a doc has generated regions,
   run `good-vibes docs generate` so their bodies match too.

## Generated regions

A generated region is a span whose body a command produces. Two HTML comments on their own lines mark it, so the markers
vanish when the doc renders:

```markdown
<!-- good-vibes:generated commands-setup -->
| Command | What it does |
...
<!-- good-vibes:end commands-setup -->
```

The name matches a `[docs.generated.<name>]` table in `good-vibes.toml`. Its `run` argv prints the body and runs from
the project root. Edit the prose around a region, never the body by hand: if the body is wrong, change its generator.
After changing code a generator reads, run `good-vibes docs generate <name>` and keep the diff.
`good-vibes docs generate` with no name rewrites every region, and `--check` reports drift without writing. The full
`good-vibes check` reports an out-of-date region as an error; `check --fast` skips it because a generator may compile
code. Keep generators deterministic: a date or an absolute path makes the region flap.

## Names in code spans

A backticked path is a promise that it exists today. Lint resolves it from the project root, or from the doc's folder
when it starts with `./` or `../`, and fails a changed doc when the path is missing. Put example snippets and run-time
paths in a fence, or list the doc under `docs.names.exclude` or the path under `docs.names.path_ignore` in
`good-vibes.toml`. Do not backtick a path that is only an illustration.

A span that matches a configured vocabulary must name something its list command prints. Fix the name, or move the
example into a fence when it is not a real name.

## Code fences

A fence is a code sample. A `[docs.fences.<lang>]` table in `good-vibes.toml` turns on the check for one language: its
`check` argv reads one sample on stdin and exits non-zero to fail it, and a `line N: reason` stderr line names sample
line N. The full `good-vibes check` runs each unique sample once and maps a failure back to the file line;
`check --fast` skips these checks. A language with no table is not checked, so `bash` and `sh` stay separate. Fences in
generated regions and decision docs never run.

When a sample is foreign on purpose, opt out with an HTML comment on the nearest non-blank line above the fence:

````markdown
<!-- good-vibes:unchecked a mise.toml, not good-vibes config -->

```toml
[tools]
```
````

The comment renders as nothing. It must give a reason and sit above a fence; lint reports a marker with no reason or no
fence below it. Mark a sample only when it cannot be made true. A marker that hides a real failure is a bad trade.

## Rules

- Every `concepts/` doc keeps a high-level Mermaid diagram in its Overview: the parts and how they relate. A missing
  Overview is a `section` finding, and an Overview with no Mermaid fence is a `diagram` finding. Add more in How it
  works when the flow needs them. Update the diagram in the same change as the behavior. Other doc types use diagrams
  only where they clarify.
- `README.md` is a Diátaxis front door and signpost, not a manual: orient the reader and route into `docs/` instead of
  duplicating concepts, guides, runbooks, or reference docs. Structure it as an inverted pyramid: lead above the fold
  with what the project is, why it matters, and a bare-bones quickstart before linking out to deeper docs. It needs no
  YAML front matter, and editing it requires an accepted change that lists `readme` in `amends`.
- `ARCHITECTURE.md` is a map, not an essay: each section leads with a diagram and prose only adds what the diagram
  cannot carry. Keep one idea per diagram and split the code map by layer.
- `GLOSSARY.md` holds one `##` heading per term: the definition, then an optional `_Avoid:_` line. Scope it to domain
  terms shared across code and docs, not internal helper or struct names. Adding a term is free. Never reword or remove
  an existing term without an accepted change that amends `glossary`.
- Update the doc in place. Do not append a changelog; history lives in git.
- Short paragraphs, one idea each. Code samples must run.
- If a doc describes behavior that no longer exists, delete or rewrite it. A stale doc is worse than no doc.
- A doc whose summary covers behavior the change altered belongs in the same change as the code, even when no task names
  it.
