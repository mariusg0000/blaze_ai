## Feature Description
Added read_file and write_file tools for direct file I/O, and made replace_block support relative paths resolved against the work directory.

## Rationale And Implementation
These tools fill a gap in the native toolset — the agent previously needed shell commands for reading/writing files. Direct tools are faster and avoid shell overhead. The replace_block relative path improvement makes the tool consistent with how the new read_file/write_file tools handle paths, and removes the friction of requiring absolute paths in every agent call. Tools were added to the tool registry in runtime.go, and replace_block uses a workDir func to resolve relative paths at execution time.

## Modified Files
- internal/tools/read_file.go: new tool — reads a file from disk with optional workdir resolution for relative paths
- internal/tools/read_file_test.go: tests for absolute path, relative path, and missing file cases
- internal/tools/write_file.go: new tool — writes content to a file with workdir resolution for relative paths
- internal/tools/write_file_test.go: tests for absolute path, relative path, and create-directory-if-missing behavior
- internal/tools/replace_block.go: added relative path resolution and structured error messages using resolved path
- internal/tools/replace_block_test.go: added tests for relative path resolution and no-workdir error cases
- internal/runtime/runtime.go: registered NewReadFileTool and NewWriteFileTool in the tool registry
