# IDEA: Pluggable federation transport choices

## WHAT MUST BE DONE
Compare possible mechanisms for sharing global skill data and definitions: SSH/SFTP, SSHFS, Syncthing, and Git.

## WHY IT MUST BE DONE
Each mechanism has different consistency, availability, and operational characteristics. The right choice may differ between authoritative personal data and mostly read-only skill definitions.

## HOW IT MUST BE DONE
Treat SSH/SFTP as the simplest initial candidate for direct NAS-authoritative data. Evaluate SSHFS mount safety, Syncthing conflict behavior, and Git's suitability for versioned definitions. Do not allow a missing mount or connection to become an unnoticed local write.
