#!/usr/bin/env bash
set -euo pipefail

REPO="grumpyguvner/claude_wrapper"
BINARY="claude-wrapper"

# Get version from argument or prompt
VERSION="${1:-}"
if [ -z "$VERSION" ]; then
    echo -n "Version (e.g. v0.1.0): "
    read -r VERSION
fi

# Validate version format
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "error: version must match vX.Y.Z (e.g. v0.1.0)" >&2
    exit 1
fi

# Check for uncommitted changes
if [ -n "$(git status --porcelain)" ]; then
    echo "error: working directory is not clean" >&2
    echo "commit or stash changes before releasing" >&2
    exit 1
fi

# Check gh is authenticated
if ! gh auth status >/dev/null 2>&1; then
    echo "error: gh is not authenticated. Run 'gh auth login'" >&2
    exit 1
fi

# Check tag doesn't already exist
if git rev-parse "$VERSION" >/dev/null 2>&1; then
    echo "error: tag $VERSION already exists" >&2
    exit 1
fi

# Run tests
echo "Running tests..."
go test -race ./...

# Build linux/amd64
echo "Building ${BINARY} linux/amd64..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "${BINARY}" .

# Tag
echo "Tagging ${VERSION}..."
git tag -a "$VERSION" -m "Release ${VERSION}"
git push origin "$VERSION"

# Create release and upload asset
echo "Creating GitHub release..."
gh release create "$VERSION" \
    --repo "$REPO" \
    --title "$VERSION" \
    --generate-notes \
    "${BINARY}"

echo "Released ${VERSION}"
echo "https://github.com/${REPO}/releases/tag/${VERSION}"
