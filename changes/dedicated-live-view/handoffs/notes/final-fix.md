## Commit

Reload slip after live refresh and drop stray binary

Manual live refreshes can grade a slip from locked to graded, so rendering the panel with the pre-refresh slip kept
`data-live-mode="true"` and prevented the browser from transitioning to the graded view. Removing the stray `server`
binary also keeps compiled build artifacts out of the repository.
