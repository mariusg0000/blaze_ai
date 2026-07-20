# IDEA: Device federation profiles

## WHAT MUST BE DONE
Explore optional per-device federation profiles that define how a device reaches shared skill data on the NAS. A device without a configured profile should not offer global skills as available storage.

## WHY IT MUST BE DONE
Global skill behavior depends on the device having a known and configured connection method. Making this explicit prevents silent local storage and makes missing infrastructure visible.

## HOW IT MUST BE DONE
Evaluate a profile containing a name, transport method, remote host, and shared root. Candidate methods discussed are SSH/SFTP, SSHFS, and Syncthing. Missing or unavailable configuration must produce an explicit error rather than a local fallback.
