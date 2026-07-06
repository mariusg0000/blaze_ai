## Feature Description
Add a 300KB file size limit to the read_file tool, returning an actionable error with alternative tool suggestions when exceeded.

## Rationale And Implementation
Prevents the agent from accidentally reading very large files into LLM context, which wastes tokens and degrades response quality. The check uses os.Stat before reading, avoids loading oversized files, and guides the user to rg for targeted search or shell with head/tail for partial reads.

## Modified Files
- internal/tools/read_file.go: Added maxReadFileSize constant (300KB) and file-size check via os.Stat in Execute() before os.ReadFile.
- internal/tools/read_file_test.go: Added TestReadFileTooLarge for the size-limit error path.
