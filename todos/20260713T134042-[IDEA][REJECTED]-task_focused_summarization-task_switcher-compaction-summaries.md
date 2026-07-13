# TODO: Reimplement task-focused summarization

## WHAT MUST BE DONE
Do not reimplement the removed asynchronous task-switch detector. Retain ordinary token compaction, boundary-safe pruning, summary storage, and synthetic summary injection as the supported context-management behavior.

## WHY IT MUST BE DONE
The previous TaskSwitcher implementation produced false positives, redundant cumulative summaries, extra latency and token cost, and significant protocol complexity without relevant practical benefit.

## HOW IT MUST BE DONE
Treat this item as rejected based on the documented removal. Any future reconsideration must first demonstrate a simpler design and measurable benefit over existing token compaction, with explicit validation of false positives, summary redundancy, latency, and cost.
