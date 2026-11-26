---
name: gv-docs-prune
description: Audit and compact living docs across the repository to remove duplication, stray root docs, and local code-retelling. Use when docs feel bloated or after docs lint surfaces overlap or root-doc warnings.
---

# Prune living docs

Run a whole-tree audit to keep living docs concise, non-duplicative, and in the right layer. Work inside a normal change
so every edit and deletion is reviewed.

## Audit steps

1. Run `good-vibes docs lint --all` to collect `covers-overlap`, `root-doc`, `budget-*`, and staleness findings across
   every doc.
2. Read `docs/llms.txt` and `GLOSSARY.md` to see the full map of living docs and domain terms.
3. Inspect `covers-overlap` findings:
   - Same-type overlaps usually mean two docs own the same rules. Pick one canonical home, merge unique facts into it,
     and link to it from the other doc (or delete the redundant doc).
   - Cross-type overlaps (such as `concept` + `reference` on the same package) are fine when each keeps its own lane:
     link them in `related` front matter and move misplaced details into the right doc.
4. Inspect `root-doc` findings: move stray top-level `*.md` manuals under `docs/`, register intentional high-level docs
   under `[docs.high_level]`, or add non-living files to `docs.lint.root_doc_ignore`.
5. Inspect `budget` overrides and long docs:
   - Move local function-level invariants, edge-case guards, ordering constraints, and constants out of `docs/` into
     code comments next to the code they protect.
   - Replace restated cross-doc rules with links to the owning doc.
   - Drop or tighten stale `budget` overrides once the doc fits within default limits.
6. Check `GLOSSARY.md` for internal helper or struct names that are not shared domain vocabulary, and check
   `ARCHITECTURE.md` and `README.md` for prose that duplicates `docs/`.
7. Run `good-vibes docs lint --all`, `good-vibes docs index`, and `good-vibes docs generate` (when generators exist) to
   verify the tree is clean and every map matches.

## High-level doc guards

Never remove or reword existing `GLOSSARY.md` terms, or edit `ARCHITECTURE.md` or `README.md`, outside an accepted
change that lists `glossary`, `architecture`, or `readme` in `amends`.
