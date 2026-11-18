---
id: architecture
title: Architecture
type: architecture
summary: The system's parts, their boundaries, and the rules that hold across them.
tags: [architecture]
---

# Architecture

<!--
  The whole system on one page, as diagrams. Keep each diagram to one idea and label nodes with modules, not files or
  functions, so a rename inside a package does not break the doc. Each section opens with a diagram. Prose only adds
  what a diagram cannot carry: the reason behind a boundary, or a link down to a concept doc.
-->

## Overview

<!-- Caption: the main flow end to end, as a person and an agent move through it. -->

```mermaid
flowchart LR
  PERSON[person] --> ENTRY[entry point]
  ENTRY --> WORK[work]
  WORK --> RECORD[record]
```

## Code map

<!--
  Caption: packages by layer. Split the map when one diagram would carry more than one idea. Arrows show who calls
  whom; a dashed arrow or note says what a layer must never do.
-->

```mermaid
flowchart TD
  CLI[command line and TUI] --> FLOW[flow and checks]
  FLOW --> ORCH[orchestration and adapters]
  ORCH --> STORE[storage and git]
```

## Invariants

<!-- Caption: the lifecycles and ownership rules. Each "never" or "always" is an edge label or a note. -->

```mermaid
stateDiagram-v2
  [*] --> Draft
  Draft --> Active: a person approves
  Active --> Done: the work lands
  Done --> [*]
```

## Cross-cutting concerns

<!--
  Caption: one small diagram per concern: config layers, harness adapters, git and worktree safety, and the like. Add
  an H3 with its own diagram for each concern this project has.
-->

```mermaid
flowchart TD
  DEFAULTS[embedded defaults] --> USER[user config]
  USER --> PROJECT[project config]
  PROJECT --> ENV[environment]
  ENV --> FLAGS[command flags]
```
