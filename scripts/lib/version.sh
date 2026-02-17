#!/bin/bash
# version.sh - Shared version calculation and git utilities for deploy/release scripts

# Colors
: "${RED:='\033[0;31m'}"
: "${GREEN:='\033[0;32m'}"
: "${YELLOW:='\033[0;33m'}"
: "${BLUE:='\033[0;34m'}"
: "${NC:='\033[0m'}"

# Validate git working directory is clean and synced
validate_git_status() {
    local branch="$1"
    local strict_mode="$2"

    if [ -n "$(git status --porcelain)" ]; then
        echo -e "${RED}ERROR: You have uncommitted changes.${NC}"
        git status --short
        exit 1
    fi

    git fetch origin "$branch" 2>/dev/null || true
    git fetch --tags 2>/dev/null || true

    local behind ahead
    behind=$(git rev-list HEAD..origin/"$branch" --count 2>/dev/null || echo "0")
    ahead=$(git rev-list origin/"$branch"..HEAD --count 2>/dev/null || echo "0")

    if [ "$behind" -gt 0 ]; then
        echo -e "${RED}ERROR: Your branch is behind origin/$branch by $behind commit(s).${NC}"
        echo "Please pull before proceeding: git pull origin $branch"
        exit 1
    fi

    if [ "$ahead" -gt 0 ]; then
        if [ "$strict_mode" = "strict" ]; then
            echo -e "${RED}ERROR: Your branch is ahead of origin/$branch by $ahead commit(s).${NC}"
            echo "All commits must reach $branch via pull requests."
            exit 1
        else
            echo "Note: You have $ahead unpushed commit(s). These will be pushed with the tag."
        fi
    fi
}

# Get the latest production tag (excludes beta and alpha tags)
get_latest_production_tag() {
    local tag
    tag=$(git tag -l 'v*' | grep -v '\-beta' | grep -v '\-alpha' | sort -V | tail -1 2>/dev/null || echo "v0.0.0")
    if [ -z "$tag" ]; then
        tag="v0.0.0"
    fi
    echo "$tag"
}

# Parse version into components — sets VERSION_MAJOR, VERSION_MINOR, VERSION_PATCH
parse_version() {
    local tag="$1"
    local version="${tag#v}"
    VERSION_MAJOR=$(echo "$version" | cut -d. -f1)
    VERSION_MINOR=$(echo "$version" | cut -d. -f2)
    VERSION_PATCH=$(echo "$version" | cut -d. -f3 | cut -d- -f1)
}

# Auto-detect version bump type from conventional commits
detect_bump_type() {
    local base_tag="$1"
    local commits

    commits=$(git log "$base_tag"..HEAD --pretty=format:"%s" 2>/dev/null || git log --pretty=format:"%s")

    if echo "$commits" | grep -qE "^[a-z]+!:|BREAKING CHANGE"; then
        BUMP_TYPE="major"
        BUMP_REASON="breaking change detected"
    elif echo "$commits" | grep -qE "^feat(\(.+\))?:"; then
        BUMP_TYPE="minor"
        BUMP_REASON="feat: commit detected"
    else
        BUMP_TYPE="patch"
        BUMP_REASON="fix/chore/other commits"
    fi
}

# Apply version bump to VERSION_MAJOR, VERSION_MINOR, VERSION_PATCH
apply_bump() {
    local bump_type="$1"
    case "$bump_type" in
        major)
            VERSION_MAJOR=$((VERSION_MAJOR + 1))
            VERSION_MINOR=0
            VERSION_PATCH=0
            ;;
        minor)
            VERSION_MINOR=$((VERSION_MINOR + 1))
            VERSION_PATCH=0
            ;;
        *)
            VERSION_PATCH=$((VERSION_PATCH + 1))
            ;;
    esac
}

# Get the next pre-release number for a given tag pattern
get_next_prerelease_number() {
    local pattern="$1"
    local prefix="$2"
    local latest num

    latest=$(git tag -l "$pattern" | sort -V | tail -1)
    if [ -n "$latest" ]; then
        num=$(echo "$latest" | sed "s/$prefix//")
        echo $((num + 1))
    else
        echo "1"
    fi
}

# Create and push a tag
create_and_push_tag() {
    local tag="$1"
    local message="$2"
    local branch="${3:-}"

    git tag -a "$tag" -m "$message"

    if [ -n "$branch" ] && [ "$branch" != "main" ]; then
        git push origin "$branch"
    fi

    git push origin "$tag"
}
