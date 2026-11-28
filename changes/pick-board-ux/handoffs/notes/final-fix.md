## Commit

Fix empty-state and active-chip invariants in pick detail

Narrowing the pick detail to a category with no matching rows previously left the detail pane blank and dropped the active category chip from the filter bar because legacy single-tab fields and fallback logic masked the empty sections. Removing the dead legacy fields and retaining the narrowed category in tab generation ensures the empty state and active chip render consistently.
