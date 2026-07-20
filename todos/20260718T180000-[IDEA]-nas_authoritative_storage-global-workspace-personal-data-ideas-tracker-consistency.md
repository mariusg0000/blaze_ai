# IDEA: NAS-authoritative global workspaces

## WHAT MUST BE DONE
Explore making the NAS the authoritative storage location for selected global skill workspaces, including ideas, personal information, and personal tracking data.

## WHY IT MUST BE DONE
The desired behavior is that an entry created from any device reaches one common server location. A single authority reduces divergent copies and clarifies where backups and recovery belong.

## HOW IT MUST BE DONE
Evaluate a workspace abstraction that hides the transport from the skill, while requiring direct remote reads and writes. Concurrent updates, atomic operations, access failures, and workspace identity remain open design questions.
