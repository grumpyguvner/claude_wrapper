# Claude Wrapper - Go Implementation

A lightweight wrapper for the Claude CLI that manages branch-specific personal files in Git repositories.

## Features

- **Branch-aware file storage**: Automatically manages different file sets per branch
- **Backwards compatible**: Default branch uses repository root for existing workflows
- **Automatic cleanup**: Deletes branch storage 7 days after branch deletion
- **Zero dependencies**: Uses only Go standard library
- **Fast**: Binary execution with minimal overhead
- **Robust error handling**: Proper error propagation and logging

## Storage Structure

```
~/.workspaces/
  └── {repo}/                    # Default branch files (backwards compatible)
      ├── file1
      ├── file2
      └── branches/              # Branch-specific storage
          ├── feature-branch/
          │   ├── file1
          │   └── file2
          └── bugfix-branch/
              └── .deleted_at    # Deletion marker (unix timestamp)
```

## Requirements

- Go 1.22 or later
- Git repository
- Claude CLI installed

## Building

```bash
# Build binary
make build

# Run tests
make test

# Build and install to /usr/local/bin
make install

# Clean build artifacts
make clean

# Lint code
make lint
```

## Installation

### Quick Install

```bash
curl -fsSL https://github.com/grumpyguvner/claude_wrapper/releases/latest/download/install.sh | bash
```

This downloads the latest release to `~/.local/bin/claude-wrapper` and adds `alias claude='claude-wrapper'` to your shell config. Restart your shell or `source ~/.bashrc` to activate.

### Option 1: Install to /usr/local/bin

```bash
make install
```

Then use `claude-wrapper` instead of `claude` in your git repositories.

### Option 2: Create alias

```bash
# Build the binary
make build

# Move to your preferred location
mv claude-wrapper ~/bin/

# Add to your shell RC file (.bashrc, .zshrc, etc.)
alias claude='claude-wrapper'
```

### Option 3: Rename and replace

```bash
# Build the binary
make build

# Rename original claude (if you want to keep it)
sudo mv /usr/local/bin/claude /usr/local/bin/claude-original

# Install wrapper as claude
sudo install -m 755 claude-wrapper /usr/local/bin/claude
```

## Usage

Once installed, use it exactly like the regular Claude CLI:

```bash
# In any git repository
claude [arguments]

# The wrapper automatically:
# 1. Detects current branch
# 2. Syncs appropriate files based on branch
# 3. Runs claude with your arguments
# 4. Syncs changes back to storage
# 5. Performs cleanup of deleted branches
```

## How It Works

### Sync In (Before Claude runs)

1. Checks if you're in a git repository
2. Determines current branch
3. Initializes branch storage if needed (copies from default branch)
4. Copies files from storage to working directory
5. Updates `.git/info/exclude` to ignore managed files

### Sync Out (After Claude runs)

1. Reads `.git/info/exclude` to find managed files
2. Copies managed files back to storage
3. Removes files from storage that are no longer in exclude file

### Cleanup (After sync)

1. Scans `branches/` directory for stored branches
2. Checks if branch still exists in git
3. Creates deletion marker for missing branches
4. Removes branch storage after 7 days

## Configuration

Configuration is automatic via git commands:

- **Repository**: Detected from `git rev-parse --show-toplevel`
- **Current branch**: Detected from `git branch --show-current`
- **Default branch**: Detected from `git symbolic-ref refs/remotes/origin/HEAD`
- **Storage base**: `~/.workspaces/{repo-name}/`

## Testing

```bash
# Run all tests
make test

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test -run TestFilterItems
```

## Error Handling

The wrapper handles various error conditions gracefully:

- **Not in git repo**: Passes through directly to claude
- **Detached HEAD**: Passes through directly to claude
- **Storage errors**: Logged but don't prevent claude execution
- **Cleanup errors**: Logged but don't fail the main operation

## Logging

The wrapper uses standard Go logging:

```bash
# Normal operation: minimal output
claude [arguments]

# Cleanup warnings are logged but don't interrupt workflow
```

## Development

### Project Structure

```
.
├── main.go           # Main implementation
├── main_test.go      # Unit tests
├── go.mod            # Go module definition
├── Makefile          # Build automation
└── README.md         # This file
```

### Code Organization

- `main()`: Entry point and argument passing
- `run()`: Main orchestration logic
- `loadConfig()`: Configuration detection
- `syncIn()`: Storage → Working directory
- `syncOut()`: Working directory → Storage
- `cleanupDeletedBranches()`: Branch cleanup logic
- Helper functions: File operations, git interaction

### Adding Features

1. Write tests first (TDD approach)
2. Implement feature
3. Run tests: `make test`
4. Lint: `make lint`
5. Build: `make build`

## Comparison with Bash Version

| Feature | Bash | Go |
|---------|------|-----|
| Dependencies | bash, git, coreutils | git only |
| Performance | ~50-100ms overhead | ~5-10ms overhead |
| Error handling | Basic | Robust |
| Testing | Manual | Automated |
| Debugging | echo/set -x | Standard logging |
| Binary size | N/A | ~2MB |
| Maintainability | Shell complexity | Structured code |

## Troubleshooting

### Wrapper not found
```bash
# Check installation
which claude-wrapper

# Rebuild and reinstall
make clean && make install
```

### Files not syncing
```bash
# Check .git/info/exclude file
cat .git/info/exclude

# Check storage location
ls -la ~/.workspaces/$(basename $(git rev-parse --show-toplevel))
```

### Branch cleanup not working
```bash
# Check branches directory
ls -la ~/.workspaces/$(basename $(git rev-parse --show-toplevel))/branches/

# Check deletion markers
find ~/.workspaces -name .deleted_at -exec cat {} \;
```

## Migration from Bash Version

No migration needed! The Go version uses the same storage structure:

- Default branch files in `~/.workspaces/{repo}/` work immediately
- Existing branch directories should be moved to `branches/` subdirectory if you were using the multi-branch bash version

## License

MIT or similar - adjust as needed for your use case.

## Future Enhancements

Possible improvements aligned with infrastructure preferences:

- [ ] Systemd service for background sync
- [ ] Systemd timer for periodic cleanup
- [ ] Structured logging with journald integration
- [ ] Config file support (optional TOML/YAML)
- [ ] Metrics endpoint for monitoring
- [ ] Remote storage backend (S3/MinIO)
