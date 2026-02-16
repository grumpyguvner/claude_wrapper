# Claude Wrapper

> **This is a personal tool that I built for my own use.** You are welcome to use it, fork it, or learn from it, but you do so entirely at your own risk. I make no guarantees about its behaviour and I do not provide support.

A Go wrapper for the Claude CLI that manages branch-specific personal files in Git repositories.

## What It Does

You add files to `.git/info/exclude` to tell the wrapper which files are personal to you (notes, scratch files, local configs, etc.). The wrapper copies those files to/from a storage directory (`~/.workspaces/{repo}/`) before and after each `claude` invocation, keeping them separate per branch.

## Managing Files

The wrapper manages files listed in `.git/info/exclude`. To have a file managed:

```bash
# Add a file to git's local exclude (not committed, not shared)
echo "my-notes.md" >> .git/info/exclude

# Run claude through the wrapper - the file is now synced per-branch
claude-wrapper [args]
```

Glob patterns (e.g., `*.log`) are ignored. Only literal file/directory names are synced.

When you switch branches, the wrapper restores the files from that branch's storage. New branches inherit files from the default branch.

## Storage Structure

```
~/.workspaces/
  └── {repo}/                    # Default branch files
      ├── file1
      ├── file2
      └── branches/              # Branch-specific storage
          ├── feature%2Flogin/   # URL-encoded branch names
          │   ├── file1
          │   └── file2
          └── old-feature/
              └── .deleted_at    # Deletion marker (unix timestamp)
```

Branch names containing `/` are URL-encoded in storage paths (e.g., `feature/login` becomes `feature%2Flogin`).

## Requirements

- Go 1.22 or later
- Git
- Claude CLI installed

## Building

```bash
make build     # Build binary
make test      # Run tests
make install   # Build and install to /usr/local/bin
make clean     # Clean build artifacts
make lint      # Run go vet and gofmt
```

## Installation

### Quick install (Linux amd64)

```bash
curl -fsSL https://raw.githubusercontent.com/grumpyguvner/claude_wrapper/main/install.sh | bash
```

### From source

```bash
git clone https://github.com/grumpyguvner/claude_wrapper.git
cd claude_wrapper
make install
```

### Create alias (optional)

```bash
# Add to .bashrc/.zshrc to use 'claude' instead of 'claude-wrapper'
alias claude='claude-wrapper'
```

## How It Works

### Self-update (release builds only)

On startup, release builds check GitHub for a newer version. If found, the binary downloads the update, replaces itself, and restarts with the same arguments. A `CLAUDE_WRAPPER_UPDATED` environment variable prevents repeated restarts. Dev builds (`go build` without ldflags) skip this entirely.

To disable auto-update, set `CLAUDE_WRAPPER_UPDATED=1` before running the wrapper.

### Sync In (before claude runs)

1. Detects git repo, current branch, default branch
2. Initializes branch storage if needed (copies from default branch)
3. Copies files from storage to working directory
4. Adds each file to `.git/info/exclude`

### Sync Out (after claude runs)

1. Reads `.git/info/exclude` for managed file names
2. Copies those files back to storage
3. Removes files from storage that are no longer in the exclude file

### Cleanup (after sync)

1. Scans stored branches against actual git branches
2. Creates a `.deleted_at` timestamp marker for branches no longer in git
3. Removes branch storage 7 days after the marker was created

## Error Handling

- **Not in git repo**: Passes through directly to claude
- **Detached HEAD**: Passes through directly to claude
- **Sync errors**: Fail with a clear message (claude does not run)
- **Cleanup errors**: Logged as warnings, do not fail the main operation
- **Update errors**: Logged as warnings, do not prevent normal operation
- **Claude exit code**: Preserved and propagated to the caller

## Configuration

All automatic via git:

- **Repository**: `git rev-parse --show-toplevel`
- **Current branch**: `git branch --show-current`
- **Default branch**: `git symbolic-ref refs/remotes/origin/HEAD` (falls back to `main`)
- **Storage base**: `~/.workspaces/{repo-name}/`

## Releasing

```bash
make release VERSION=v0.1.0
```

This runs tests, builds a linux/amd64 binary with the version tag embedded (enabling automatic self-updates), tags the commit, and creates a GitHub release with the binary attached.

## Project Structure

```
.
├── main.go           # Implementation
├── main_test.go      # Unit tests
├── go.mod            # Go module definition
├── Makefile          # Build automation
├── install.sh        # curl-friendly installer
├── script/
│   └── release.sh    # Release automation
├── README.md         # This file
├── INSTALL.md        # Installation guide
└── OVERVIEW.md       # Architecture overview
```

## Troubleshooting

### Files not syncing
```bash
# Check which files are in the exclude list
cat .git/info/exclude

# Check storage location
ls -la ~/.workspaces/$(basename $(git rev-parse --show-toplevel))
```

### Branch cleanup not working
```bash
# Check branches directory
ls -la ~/.workspaces/$(basename $(git rev-parse --show-toplevel))/branches/

# Check deletion markers
find ~/.workspaces -name .deleted_at
```

## Disclaimer

This software is provided as-is with no warranty. Use at your own risk.

## License

MIT
