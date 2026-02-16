# Installation Guide

## Quick Start

```bash
# 1. Build the binary
make build

# 2. Test it works
./claude-wrapper --help

# 3. Install system-wide
make install
```

## Detailed Installation Steps

### Prerequisites

1. **Install Go 1.22+**

   ```bash
   # Ubuntu/Debian
   sudo apt update
   sudo apt install golang-go
   
   # macOS
   brew install go
   
   # Or download from https://go.dev/dl/
   ```

2. **Verify Go installation**

   ```bash
   go version
   # Should output: go version go1.22.x ...
   ```

3. **Ensure Claude CLI is installed**

   ```bash
   which claude
   # Should output: /usr/local/bin/claude or similar
   ```

### Build from Source

1. **Clone or download the source files**

   ```bash
   # Create project directory
   mkdir -p ~/projects/claude-wrapper
   cd ~/projects/claude-wrapper
   
   # Copy all source files here:
   # - main.go
   # - main_test.go
   # - go.mod
   # - Makefile
   # - README.md
   ```

2. **Build the binary**

   ```bash
   make build
   # Creates ./claude-wrapper binary
   ```

3. **Run tests** (optional but recommended)

   ```bash
   make test
   ```

### Installation Options

#### Option 1: System-wide installation (Recommended)

```bash
# Install to /usr/local/bin
sudo make install

# Now 'claude-wrapper' is available everywhere
cd /any/git/repo
claude-wrapper [args]
```

#### Option 2: User-local installation

```bash
# Create local bin directory if needed
mkdir -p ~/bin

# Copy binary
cp claude-wrapper ~/bin/

# Add to PATH in ~/.bashrc or ~/.zshrc
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc

# Use it
cd /any/git/repo
claude-wrapper [args]
```

#### Option 3: Alias to 'claude'

```bash
# Build first
make build

# Install as claude-wrapper
sudo install -m 755 claude-wrapper /usr/local/bin/claude-wrapper

# Add alias to shell RC file
echo "alias claude='claude-wrapper'" >> ~/.bashrc
source ~/.bashrc

# Now just use 'claude' as normal
cd /any/git/repo
claude [args]
```

#### Option 4: Replace original claude

**Warning**: Only do this if you understand the implications

```bash
# Build first
make build

# Backup original claude
sudo cp /usr/local/bin/claude /usr/local/bin/claude-original

# Replace with wrapper
sudo install -m 755 claude-wrapper /usr/local/bin/claude

# Test it
cd /any/git/repo
claude [args]

# To restore original:
# sudo mv /usr/local/bin/claude-original /usr/local/bin/claude
```

### Verification

After installation, verify it works:

```bash
# Navigate to any git repository
cd /path/to/git/repo

# Run the wrapper
claude-wrapper --help

# Should execute claude normally
# Check for any error messages in the output
```

### First Run

The first time you run the wrapper in a repository:

1. It creates `~/.workspaces/{repo-name}/` directory
2. It reads `.git/info/exclude` for managed files
3. It copies any existing files from storage to working directory
4. It runs claude normally
5. It syncs files back to storage

### Troubleshooting Installation

#### "go: command not found"

Install Go first (see Prerequisites)

#### "Permission denied" during make install

Use sudo:
```bash
sudo make install
```

#### "claude: command not found" after installation

Check PATH:
```bash
echo $PATH | grep "/usr/local/bin"

# If not found, add to ~/.bashrc:
echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

#### Binary built but doesn't execute

Check permissions:
```bash
ls -l claude-wrapper
# Should show: -rwxr-xr-x

# If not executable:
chmod +x claude-wrapper
```

### Updating

To update to a newer version:

```bash
# Pull new source code
cd ~/projects/claude-wrapper

# Clean old build
make clean

# Build new version
make build

# Test it
make test

# Reinstall
sudo make install
```

### Uninstallation

To remove the wrapper:

```bash
# Remove binary
sudo rm /usr/local/bin/claude-wrapper

# Remove alias (if you added one)
# Edit ~/.bashrc and remove the alias line

# Optionally remove storage (THIS DELETES YOUR FILES)
# rm -rf ~/.workspaces

# Optionally remove source
# rm -rf ~/projects/claude-wrapper
```

## Post-Installation

### Configure your workflow

1. **Add files to .git/info/exclude**

   ```bash
   cd /your/repo
   echo "myfile.txt" >> .git/info/exclude
   ```

2. **Run claude through wrapper**

   ```bash
   claude-wrapper [args]
   ```

3. **Files in exclude are now managed per-branch**

   Switch branches and files automatically sync!

### Set up shell alias (optional)

For convenience, you can alias `claude` to `claude-wrapper`:

```bash
# Add to ~/.bashrc or ~/.zshrc
alias claude='claude-wrapper'

# Reload shell
source ~/.bashrc
```

## Advanced: Systemd Integration

See `claude-wrapper.service` for optional systemd service configuration (for future background sync features).

## Support

For issues:
1. Check `make test` passes
2. Verify claude works without wrapper
3. Check `~/.workspaces/{repo}/` permissions
4. Review error messages in output
