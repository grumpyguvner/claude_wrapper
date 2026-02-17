#!/bin/bash
# deploy.sh - Build and create beta/alpha pre-release
#
# Usage: ./scripts/deploy.sh [auto|patch|minor|major]

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib/version.sh"

BUMP_TYPE="${1:-auto}"
BRANCH=$(git rev-parse --abbrev-ref HEAD)

if [ "$BRANCH" = "main" ]; then
    validate_git_status "$BRANCH" "strict"
    echo -e "${BLUE}Running tests...${NC}"
    if ! go test ./... 2>&1; then
        echo -e "${RED}ERROR: Tests failed.${NC}"
        exit 1
    fi
    SUFFIX="beta"
else
    validate_git_status "$BRANCH" ""
    SUFFIX="alpha"
    BRANCH_SLUG=$(echo "$BRANCH" | sed 's/[^a-zA-Z0-9]/-/g' | tr '[:upper:]' '[:lower:]')
fi

# Calculate version
LATEST_TAG=$(get_latest_production_tag)
parse_version "$LATEST_TAG"
if [ "$BUMP_TYPE" = "auto" ]; then
    detect_bump_type "$LATEST_TAG"
else
    BUMP_REASON="manual override"
fi
apply_bump "$BUMP_TYPE"
NEW_VERSION="$VERSION_MAJOR.$VERSION_MINOR.$VERSION_PATCH"

# Create tag name
if [ "$SUFFIX" = "beta" ]; then
    PRERELEASE_NUM=$(get_next_prerelease_number "v${NEW_VERSION}-beta.*" "v${NEW_VERSION}-beta.")
    NEW_TAG="v${NEW_VERSION}-beta.${PRERELEASE_NUM}"
else
    PRERELEASE_NUM=$(get_next_prerelease_number "v${NEW_VERSION}-alpha.${BRANCH_SLUG}.*" "v${NEW_VERSION}-alpha.${BRANCH_SLUG}.")
    NEW_TAG="v${NEW_VERSION}-alpha.${BRANCH_SLUG}.${PRERELEASE_NUM}"
fi

echo ""
echo "Tag: $NEW_TAG  |  Bump: $BUMP_TYPE ($BUMP_REASON)"
echo ""
printf "Build and create pre-release? (y/N) "
read -r REPLY

case "$REPLY" in
    [Yy]*)
        echo -e "${BLUE}Building binaries...${NC}"
        make VERSION="$NEW_TAG" build-all

        create_and_push_tag "$NEW_TAG" "Pre-release $NEW_TAG" "$BRANCH"

        echo -e "${BLUE}Creating GitHub release...${NC}"
        gh release create "$NEW_TAG" \
            --title "$NEW_TAG" \
            --generate-notes \
            --prerelease \
            dist/claude-wrapper-linux-amd64 \
            scripts/install.sh

        echo -e "${GREEN}Pre-release $NEW_TAG created!${NC}"
        ;;
    *)
        echo "Cancelled."
        exit 1
        ;;
esac
