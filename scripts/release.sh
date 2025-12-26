#!/bin/bash

# Mandau Release Script
# Automates the process of creating a new release with version bumping, tagging, and pushing

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

# Function to get current version from go.mod
get_current_version() {
    if [ -f "go.mod" ]; then
        local version=$(grep "^module" go.mod | head -1 | awk '{print $2}')
        echo "$version"
    else
        print_error "go.mod file not found"
        exit 1
    fi
}

# Function to validate version format
validate_version() {
    local version=$1
    if [[ $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        return 0
    else
        return 1
    fi
}

# Function to get next version
get_next_version() {
    local current_version=$1
    local version_part=${current_version#v}  # Remove 'v' prefix
    local major=$(echo $version_part | cut -d. -f1)
    local minor=$(echo $version_part | cut -d. -f2)
    local patch=$(echo $version_part | cut -d. -f3)
    
    # Increment patch version
    local new_patch=$((patch + 1))
    
    echo "v${major}.${minor}.${new_patch}"
}

# Function to update version in main.go files
update_version_in_files() {
    local new_version=$1
    
    # Update version in all main.go files that have version variable
    for main_file in cmd/*/main.go; do
        if [ -f "$main_file" ]; then
            # Update the version assignment if it exists
            sed -i.bak "s/\(version = \"\)[^\"]*\(\"\)/\1${new_version#v}\2/" "$main_file" 2>/dev/null || true
            rm -f "$main_file.bak" 2>/dev/null || true
        fi
    done
    
    print_status "Updated version in main.go files to ${new_version#v}"
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [version]"
    echo "  version - Optional version tag (e.g., v1.0.0). If not provided, will auto-increment patch version."
    echo ""
    echo "Examples:"
    echo "  $0                           # Auto-increment patch version"
    echo "  $0 v1.2.3                   # Create release with specific version"
    echo ""
    echo "Requirements:"
    echo "  - Git repository must be clean (no uncommitted changes)"
    echo "  - Git remote 'origin' must be configured"
    echo "  - You must be on the main branch"
}

# Check if help is requested
if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    show_usage
    exit 0
fi

# Check if on main branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ] && [ "$CURRENT_BRANCH" != "master" ]; then
    print_error "You must be on the main or master branch to create a release"
    exit 1
fi

# Check if repository is clean
if [ -n "$(git status --porcelain)" ]; then
    print_error "Repository has uncommitted changes. Please commit or stash them first."
    print_status "Run 'git status' to see the changes."
    exit 1
fi

# Get current version
CURRENT_VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
print_status "Current version: $CURRENT_VERSION"

# Determine new version
if [ -n "$1" ]; then
    NEW_VERSION="$1"
    if ! validate_version "$NEW_VERSION"; then
        print_error "Invalid version format. Use format like v1.0.0"
        exit 1
    fi
else
    NEW_VERSION=$(get_next_version "$CURRENT_VERSION")
    print_status "Auto-generating next version: $NEW_VERSION"
fi

# Confirm release
print_status "Creating release: $CURRENT_VERSION -> $NEW_VERSION"
read -p "Continue with release creation? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    print_status "Release creation cancelled."
    exit 0
fi

# Update version in files
update_version_in_files "$NEW_VERSION"

# Commit version changes
print_status "Committing version changes..."
git add .
git commit -m "Bump version to $NEW_VERSION" -m "Automated release commit"

# Create and push tag
print_status "Creating tag: $NEW_VERSION"
git tag "$NEW_VERSION"

print_status "Pushing changes and tag to remote..."
git push origin "$(git branch --show-current)"
git push origin "$NEW_VERSION"

print_success "Release $NEW_VERSION has been created and pushed!"
print_status "The GitHub Actions release workflow should now build and publish the binaries."
print_status "Check the Actions tab in GitHub for build status: https://github.com/bhangun/mandau/actions"

# Optional: Open GitHub releases page
print_status "You can view the release at: https://github.com/bhangun/mandau/releases/tag/$NEW_VERSION"