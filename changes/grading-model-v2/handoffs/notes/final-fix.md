## Commit

Fix v2 indexing, eval leakage, and shared helpers

Final review caught a leg-index mismatch in slip Script Fit explanations when ungraded legs precede correlated legs, two walk-forward evaluation bugs in the offline Python pipeline (held-out Anytime TD base-rate leakage and mismatched SGP outcome lines), duplicated normal and sharp-lookup helpers across packages, and a stale highlight description in the slips concept doc. Fixing these ensures accurate script attribution, leak-free walk-forward artifacts, shared cross-package math and sharp-lookup seams, and up-to-date docs before landing.
