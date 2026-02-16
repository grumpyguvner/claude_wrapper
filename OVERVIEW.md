# Claude Wrapper - Architecture Overview

## Project Summary

A Go wrapper for the Claude CLI that provides branch-specific personal file management. Files listed in `.git/info/exclude` are synced to/from `~/.workspaces/{repo}/` per branch.

## Core Files

- **main.go** - Implementation, zero external dependencies
- **main_test.go** - Unit tests for utility functions
- **go.mod** - Go module definition (Go 1.22+)
- **Makefile** - Build, test, install automation

## Architecture

```
main()
  └─ run()
      ├─ loadConfig()                 # Git detection & configuration
      ├─ syncIn()                     # Storage -> Working directory
      ├─ execClaude()                 # Run actual claude command
      ├─ syncOut()                    # Working directory -> Storage
      └─ cleanupDeletedBranches()     # Housekeeping
```

### Error Handling Strategy

1. **Git errors** (not in repo, detached HEAD): Pass through to claude directly
2. **Sync errors**: Fail with clear message, claude does not run
3. **Cleanup errors**: Log warning, do not fail the operation
4. **Claude exit code**: Preserved via `exec.ExitError` unwrapping

### Storage Layout

```
~/.workspaces/{repo}/              # Default branch (backwards compatible)
  ├── file1
  ├── file2
  └── branches/                    # Feature branches (URL-encoded names)
      ├── feature%2Flogin/
      │   └── file1
      └── old-feature/
          └── .deleted_at          # Timestamp marker
```

### File Management

Files are managed by adding them to `.git/info/exclude`. The wrapper:
- On **sync in**: copies files from storage into the working tree and ensures they're in the exclude file
- On **sync out**: reads the exclude file to determine which files to copy back to storage
- Glob patterns, comments, and non-existent files in the exclude file are ignored
- Symlinks are skipped with a warning

### Branch Name Encoding

Branch names are URL-encoded for storage paths using `url.PathEscape`. This handles `/` in branch names (e.g., `feature/login` -> `feature%2Flogin`) and avoids filesystem path collisions. Decoding uses `url.PathUnescape` during cleanup.

## Test Coverage

Unit tests cover utility functions:
- Item filtering (deletion markers, branches directory)
- Exclude file parsing (comments, patterns, non-existent files)
- Adding items to exclude (deduplication)
- File copying (permissions, content)
- Directory copying (recursive, structure)
- Directory listing (including non-existent)

The orchestration functions (`run`, `syncIn`, `syncOut`, `cleanupDeletedBranches`) are not unit tested as they depend on git state and filesystem side effects.

## Design Decisions

- **Zero dependencies**: Only Go stdlib. Git is invoked via `exec.Command`.
- **`.git/info/exclude` as source of truth**: This is a standard git mechanism for local-only ignores. The wrapper reuses it rather than inventing a new config format.
- **URL-encoded branch names**: Avoids filesystem issues with `/` in branch names while remaining fully reversible.
- **7-day grace period for cleanup**: Prevents accidental data loss if a branch is temporarily deleted.
- **Symlinks skipped**: Copying symlinks could silently change semantics (following vs preserving). Skipping with a warning is the safe default.
