#!/bin/bash
# =============================================================================
# Mandau Deployment Script
# =============================================================================
# Automates version bumping, tagging, building, and pushing to GitHub
#
# Usage:
#   ./deploy.sh [VERSION] [OPTIONS]
#
# Examples:
#   ./deploy.sh 0.0.17                    # Deploy version 0.0.17
#   ./deploy.sh 0.0.17 --dry-run         # Show what would be done
#   ./deploy.sh 0.0.17 --skip-tests      # Skip tests
#   ./deploy.sh 0.0.17 --skip-build      # Skip build step
# =============================================================================

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

print_info() {
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

print_step() {
    echo ""
    echo -e "${PURPLE}[STEP]${NC} $1"
    echo "─────────────────────────────────────────────────────"
}

# Default values
VERSION=""
DRY_RUN=false
SKIP_TESTS=false
SKIP_BUILD=false
SKIP_PUSH=false
REMOTE="origin"
BRANCH="main"

show_usage() {
    cat << 'EOF'
Usage: ./deploy.sh VERSION [OPTIONS]

Automate Mandau deployment: version bump, tag, build, and push.

Required:
  VERSION                     Version number (e.g., 0.0.17)

Options:
  --dry-run                   Show what would be done without executing
  --skip-tests                Skip running tests
  --skip-build                Skip building binaries
  --skip-push                 Skip pushing to remote (tag only)
  --remote REMOTE             Remote name (default: origin)
  --branch BRANCH             Branch name (default: main)
  --help                      Show this help message

Examples:
  # Deploy version 0.0.17
  ./deploy.sh 0.0.17

  # Preview deployment
  ./deploy.sh 0.0.17 --dry-run

  # Deploy without tests
  ./deploy.sh 0.0.17 --skip-tests

  # Tag only, no push
  ./deploy.sh 0.0.17 --skip-push
EOF
    exit 0
}

# Parse arguments
parse_args() {
    if [[ $# -eq 0 ]]; then
        print_error "VERSION is required"
        echo ""
        show_usage
    fi

    VERSION="$1"
    shift

    # Validate version format
    if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        print_error "Invalid version format: $VERSION"
        print_info "Version must be in format: MAJOR.MINOR.PATCH (e.g., 0.0.17)"
        exit 1
    fi

    while [[ $# -gt 0 ]]; do
        case $1 in
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --skip-tests)
                SKIP_TESTS=true
                shift
                ;;
            --skip-build)
                SKIP_BUILD=true
                shift
                ;;
            --skip-push)
                SKIP_PUSH=true
                shift
                ;;
            --remote)
                REMOTE="$2"
                shift 2
                ;;
            --branch)
                BRANCH="$2"
                shift 2
                ;;
            --help|-h)
                show_usage
                ;;
            -*)
                print_error "Unknown option: $1"
                show_usage
                ;;
            *)
                print_error "Unexpected argument: $1"
                show_usage
                ;;
        esac
    done
}

# Validate environment
validate_environment() {
    print_step "Validating Environment"

    # Check if we're in a git repository
    if ! git rev-parse --git-dir &> /dev/null; then
        print_error "Not a git repository"
        exit 1
    fi

    # Check if we're on the correct branch
    current_branch=$(git branch --show-current)
    if [[ "$current_branch" != "$BRANCH" ]]; then
        print_warning "Not on branch '$BRANCH' (currently on '$current_branch')"
        if [[ "$DRY_RUN" != true ]]; then
            read -p "Switch to branch '$BRANCH'? (y/N): " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                git checkout "$BRANCH"
            else
                print_error "Deployment cancelled"
                exit 1
            fi
        fi
    fi

    # Check if working directory is clean
    if ! git diff --quiet 2>/dev/null || ! git diff --cached --quiet 2>/dev/null; then
        print_warning "Working directory has uncommitted changes"
        if [[ "$DRY_RUN" != true ]]; then
            read -p "Continue anyway? (y/N): " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                print_error "Deployment cancelled"
                exit 1
            fi
        fi
    fi

    # Check required tools
    local required_tools=("git" "go")
    for tool in "${required_tools[@]}"; do
        if ! command -v "$tool" &> /dev/null; then
            print_error "'$tool' is required but not installed"
            exit 1
        fi
    done

    print_success "Environment validation passed"
}

# Run tests
run_tests() {
    if [[ "$SKIP_TESTS" == true ]]; then
        print_info "Skipping tests (--skip-tests)"
        return 0
    fi

    print_step "Running Tests"

    # Navigate to mandau directory
    cd "$(dirname "$0")"

    print_info "Running Go tests..."
    if ! go test ./... 2>&1; then
        print_error "Tests failed"
        exit 1
    fi

    print_info "Running linter..."
    if command -v golangci-lint &> /dev/null; then
        if ! golangci-lint run 2>&1; then
            print_warning "Linter found issues (continuing anyway)"
        fi
    else
        print_info "golangci-lint not installed, skipping"
    fi

    print_success "All tests passed"
}

# Update version in all source files
update_version() {
    print_step "Updating Version to $VERSION"

    # Navigate to mandau directory
    cd "$(dirname "$0")"

    # Version files to update
    local version_files=(
        "cmd/mandau-core/main.go"
        "cmd/mandau-agent/main.go"
        "cmd/mandau-cli/main.go"
    )

    for file in "${version_files[@]}"; do
        if [[ ! -f "$file" ]]; then
            print_warning "File not found: $file"
            continue
        fi

        # Update version using sed
        if [[ "$DRY_RUN" == true ]]; then
            print_info "[DRY RUN] Would update version in $file to $VERSION"
        else
            # Use sed to replace version (macOS and Linux compatible)
            if [[ "$OSTYPE" == "darwin"* ]]; then
                sed -i '' "s/version = \"[0-9]\+\.[0-9]\+\.[0-9]\+\"/version = \"$VERSION\"/" "$file"
            else
                sed -i "s/version = \"[0-9]\+\.[0-9]\+\.[0-9]\+\"/version = \"$VERSION\"/" "$file"
            fi
            print_success "Updated version in $file"
        fi
    done

    # Also update Makefile if it has a version variable
    if [[ -f "Makefile" ]]; then
        if grep -q "VERSION" Makefile; then
            if [[ "$DRY_RUN" == true ]]; then
                print_info "[DRY RUN] Would update version in Makefile to $VERSION"
            else
                if [[ "$OSTYPE" == "darwin"* ]]; then
                    sed -i '' "s/VERSION ?= [0-9]\+\.[0-9]\+\.[0-9]\+/VERSION ?= $VERSION/" Makefile 2>/dev/null || true
                else
                    sed -i "s/VERSION ?= [0-9]\+\.[0-9]\+\.[0-9]\+/VERSION ?= $VERSION/" Makefile 2>/dev/null || true
                fi
                print_success "Updated version in Makefile"
            fi
        fi
    fi
}

# Commit changes
commit_changes() {
    if [[ "$DRY_RUN" == true ]]; then
        print_info "[DRY RUN] Would commit version bump to $VERSION"
        return 0
    fi

    print_step "Committing Changes"

    # Add changed files
    git add cmd/mandau-core/main.go cmd/mandau-agent/main.go cmd/mandau-cli/main.go Makefile 2>/dev/null || true

    # Check if there are changes to commit
    if git diff --cached --quiet 2>/dev/null; then
        print_info "No changes to commit"
        return 0
    fi

    # Commit
    git commit -m "chore: bump version to $VERSION"
    print_success "Committed version bump"
}

# Create git tag
create_tag() {
    if [[ "$DRY_RUN" == true ]]; then
        print_info "[DRY RUN] Would create tag v$VERSION"
        return 0
    fi

    print_step "Creating Git Tag v$VERSION"

    # Check if tag already exists
    if git tag -l "v$VERSION" | grep -q "v$VERSION"; then
        print_error "Tag v$VERSION already exists"
        read -p "Delete and recreate? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            git tag -d "v$VERSION" 2>/dev/null || true
            print_info "Deleted existing tag"
        else
            print_error "Deployment cancelled"
            exit 1
        fi
    fi

    # Create tag
    git tag -a "v$VERSION" -m "Release v$VERSION"
    print_success "Created tag v$VERSION"
}

# Build binaries
build_binaries() {
    if [[ "$SKIP_BUILD" == true ]]; then
        print_info "Skipping build (--skip-build)"
        return 0
    fi

    print_step "Building Binaries (v$VERSION)"

    # Navigate to mandau directory
    cd "$(dirname "$0")"

    # Create build directory
    mkdir -p bin

    # Build mandau-core
    print_info "Building mandau-core..."
    if ! go build -ldflags "-X main.version=$VERSION" -o bin/mandau-core ./cmd/mandau-core 2>&1; then
        print_error "Failed to build mandau-core"
        exit 1
    fi

    # Build mandau-agent
    print_info "Building mandau-agent..."
    if ! go build -ldflags "-X main.version=$VERSION" -o bin/mandau-agent ./cmd/mandau-agent 2>&1; then
        print_error "Failed to build mandau-agent"
        exit 1
    fi

    # Build mandau CLI
    print_info "Building mandau CLI..."
    if ! go build -ldflags "-X main.version=$VERSION" -o bin/mandau ./cmd/mandau-cli 2>&1; then
        print_error "Failed to build mandau CLI"
        exit 1
    fi

    # Make binaries executable
    chmod +x bin/mandau-core bin/mandau-agent bin/mandau

    # Verify binaries
    print_info "Verifying binaries..."
    ./bin/mandau --version 2>/dev/null || print_warning "mandau --version check failed"
    ./bin/mandau-core --version 2>/dev/null || print_warning "mandau-core --version check failed"
    ./bin/mandau-agent --version 2>/dev/null || print_warning "mandau-agent --version check failed"

    print_success "Binaries built successfully"
}

# Push to remote
push_to_remote() {
    if [[ "$SKIP_PUSH" == true ]]; then
        print_info "Skipping push (--skip-push)"
        return 0
    fi

    if [[ "$DRY_RUN" == true ]]; then
        print_info "[DRY RUN] Would push to $REMOTE/$BRANCH and tag v$VERSION"
        return 0
    fi

    print_step "Pushing to $REMOTE"

    # Push branch
    print_info "Pushing branch $BRANCH..."
    if ! git push "$REMOTE" "$BRANCH" 2>&1; then
        print_error "Failed to push branch"
        exit 1
    fi

    # Push tag
    print_info "Pushing tag v$VERSION..."
    if ! git push "$REMOTE" "v$VERSION" 2>&1; then
        print_error "Failed to push tag"
        exit 1
    fi

    print_success "Pushed to $REMOTE/$BRANCH and tag v$VERSION"
}

# Build release artifacts (optional)
build_release_artifacts() {
    print_step "Building Release Artifacts"

    if [[ "$DRY_RUN" == true ]]; then
        print_info "[DRY RUN] Would build release artifacts for v$VERSION"
        return 0
    fi

    # Create release directory
    local release_dir="releases/v$VERSION"
    mkdir -p "$release_dir"

    # Copy binaries
    cp bin/mandau-core "$release_dir/" 2>/dev/null || true
    cp bin/mandau-agent "$release_dir/" 2>/dev/null || true
    cp bin/mandau "$release_dir/" 2>/dev/null || true

    # Create checksums
    print_info "Creating checksums..."
    cd "$release_dir"
    if command -v sha256sum &> /dev/null; then
        sha256sum * > checksums.sha256
    elif command -v shasum &> /dev/null; then
        shasum -a 256 * > checksums.sha256
    fi
    cd - > /dev/null

    print_success "Release artifacts created in $release_dir"
}

# Print deployment summary
print_summary() {
    echo ""
    echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  Deployment Summary${NC}"
    echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  ${BLUE}Version:${NC}      $VERSION"
    echo -e "  ${BLUE}Tag:${NC}          v$VERSION"
    echo -e "  ${BLUE}Branch:${NC}       $BRANCH"
    echo -e "  ${BLUE}Remote:${NC}       $REMOTE"
    echo ""

    if [[ "$DRY_RUN" == true ]]; then
        echo -e "  ${YELLOW}⚠  DRY RUN - No changes were made${NC}"
    else
        echo -e "  ${GREEN}✓ Version updated${NC}"
        echo -e "  ${GREEN}✓ Changes committed${NC}"
        echo -e "  ${GREEN}✓ Tag created: v$VERSION${NC}"

        if [[ "$SKIP_BUILD" != true ]]; then
            echo -e "  ${GREEN}✓ Binaries built${NC}"
        fi

        if [[ "$SKIP_PUSH" != true ]]; then
            echo -e "  ${GREEN}✓ Pushed to $REMOTE${NC}"
        fi
    fi

    echo ""
    echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
    echo ""

    if [[ "$DRY_RUN" != true ]] && [[ "$SKIP_PUSH" != true ]]; then
        print_info "Next steps:"
        print_info "  1. Verify deployment on GitHub: https://github.com/bhangun/mandau/releases/tag/v$VERSION"
        print_info "  2. Test the new version: ./bin/mandau --version"
        print_info "  3. Create release notes on GitHub if needed"
        echo ""
    fi
}

# Main execution
main() {
    parse_args "$@"

    echo ""
    echo -e "${BLUE}╔═══════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║${NC}  ${GREEN}Mandau Deployment Script${NC}                          ${BLUE}║${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "  ${BLUE}Version:${NC}      $VERSION"
    echo -e "  ${BLUE}Dry Run:${NC}      $DRY_RUN"
    echo -e "  ${BLUE}Skip Tests:${NC}   $SKIP_TESTS"
    echo -e "  ${BLUE}Skip Build:${NC}   $SKIP_BUILD"
    echo -e "  ${BLUE}Skip Push:${NC}    $SKIP_PUSH"
    echo ""

    # Execute deployment steps
    validate_environment
    run_tests
    update_version
    commit_changes
    create_tag
    build_binaries
    build_release_artifacts
    push_to_remote
    print_summary
}

main "$@"
