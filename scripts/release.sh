#!/bin/bash
# release.sh - Build and create production release
#
# Usage: ./scripts/release.sh [auto|patch|minor|major]

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib/version.sh"

BUMP_TYPE="${1:-auto}"
BRANCH=$(git rev-parse --abbrev-ref HEAD)

if [ "$BRANCH" != "main" ]; then
    echo -e "${RED}ERROR: Release must be from main branch${NC}"
    exit 1
fi

validate_git_status "$BRANCH" "strict"

echo -e "${BLUE}Running tests...${NC}"
if ! go test ./... 2>&1; then
    echo -e "${RED}ERROR: Tests failed.${NC}"
    exit 1
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
NEW_TAG="v$NEW_VERSION"

# Check for beta testing
BETA_EXISTS=$(git tag -l "v${NEW_VERSION}-beta.*" | head -1)
if [ -z "$BETA_EXISTS" ]; then
    echo -e "${YELLOW}WARNING: No beta release found for v${NEW_VERSION}${NC}"
    printf "Continue? (y/N) "
    read -r SKIP
    [[ ! "$SKIP" =~ ^[Yy]$ ]] && exit 1
fi

echo ""
echo "Tag: $NEW_TAG  |  Bump: $BUMP_TYPE ($BUMP_REASON)"
echo ""
printf "Build and create production release? (y/N) "
read -r REPLY

case "$REPLY" in
    [Yy]*)
        echo -e "${BLUE}Building binaries...${NC}"
        make VERSION="$NEW_TAG" build-all

        create_and_push_tag "$NEW_TAG" "Release $NEW_TAG" ""

        echo -e "${BLUE}Creating GitHub release...${NC}"
        gh release create "$NEW_TAG" \
            --title "$NEW_TAG" \
            --generate-notes \
            dist/claude-wrapper-linux-amd64 \
            scripts/install.sh

        echo -e "${GREEN}Production release $NEW_TAG created!${NC}"
        ;;
    *)
        echo "Cancelled."
        exit 1
        ;;
esac
