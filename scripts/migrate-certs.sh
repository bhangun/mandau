#!/bin/bash
# =============================================================================
# Mandau Certificate Migration Script
# =============================================================================
# Migrates certificates from legacy locations to the standard ~/.mandau/ directory
#
# Legacy locations:
#   - ./certs/ (relative to current directory)
#   - ~/mandau-certs/
#   - /etc/mandau/certs/
#
# New standard location:
#   - ~/.mandau/certs/
#   - ~/.mandau/ca/ (for CA materials)
# =============================================================================

set -euo pipefail

# Default values
DRY_RUN=false
BACKUP=true
FORCE=false
VERBOSE=false

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

print_verbose() {
    if [[ "$VERBOSE" == true ]]; then
        echo -e "${BLUE}[DEBUG]${NC} $1"
    fi
}

show_usage() {
    cat << 'EOF'
Usage: migrate-certs.sh [OPTIONS]

Migrates Mandau certificates from legacy locations to ~/.mandau/certs/

Options:
  --dry-run          Show what would be migrated without moving files
  --no-backup        Don't create backup of target directory
  --force            Overwrite existing files in target directory
  --verbose          Show detailed output
  --from DIR         Migrate from specific directory (auto-detects if not specified)
  --help             Show this help message

Legacy Locations Searched (in order):
  1. ./certs/ (current directory)
  2. ~/mandau-certs/
  3. /etc/mandau/certs/

Target Location:
  ~/.mandau/certs/

Examples:
  # Preview migration
  ./scripts/migrate-certs.sh --dry-run

  # Migrate from auto-detected location
  ./scripts/migrate-certs.sh

  # Migrate from specific location
  ./scripts/migrate-certs.sh --from ~/mandau-certs

  # Force migration (overwrite existing)
  ./scripts/migrate-certs.sh --force
EOF
    exit 0
}

# Parse arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --help|-h)
                show_usage
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --no-backup)
                BACKUP=false
                shift
                ;;
            --force)
                FORCE=true
                shift
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            --from)
                SOURCE_DIR="$2"
                shift 2
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

# Find source directory with certificates
find_source_dir() {
    local locations=(
        "./certs"
        "$HOME/mandau-certs"
        "/etc/mandau/certs"
    )

    # If specific directory was provided, check it first
    if [[ -n "${SOURCE_DIR:-}" ]]; then
        if [[ -d "$SOURCE_DIR" ]] && [[ -f "$SOURCE_DIR/ca.crt" ]]; then
            echo "$SOURCE_DIR"
            return 0
        else
            print_error "Source directory not found or missing ca.crt: $SOURCE_DIR"
            exit 1
        fi
    fi

    # Search legacy locations
    for loc in "${locations[@]}"; do
        if [[ -d "$loc" ]] && [[ -f "$loc/ca.crt" ]]; then
            echo "$loc"
            return 0
        fi
    done

    return 1
}

# Backup target directory
backup_target() {
    if [[ "$DRY_RUN" == true ]]; then
        print_info "[DRY RUN] Would backup target directory"
        return 0
    fi

    if [[ ! -d "$TARGET_DIR" ]]; then
        return 0
    fi

    local backup_dir="${TARGET_DIR}.backup.$(date +%Y%m%d_%H%M%S)"
    print_info "Backing up target directory to: $backup_dir"
    cp -r "$TARGET_DIR" "$backup_dir"
    print_success "Backup created: $backup_dir"
}

# Migrate certificates
migrate_certs() {
    local source_dir="$1"
    local target_dir="$2"
    local ca_target_dir="${target_dir%/*}/ca"  # ~/.mandau/ca

    # Create target directories
    if [[ "$DRY_RUN" == true ]]; then
        print_info "[DRY RUN] Would create directories:"
        print_info "  $target_dir"
        print_info "  $ca_target_dir"
        return 0
    fi

    mkdir -p "$target_dir"
    chmod 700 "$target_dir"
    
    mkdir -p "$ca_target_dir"
    chmod 700 "$ca_target_dir"

    # Define files to migrate
    local cert_files=(
        "ca.crt"
        "ca.key"
        "core.crt"
        "core.key"
        "agent.crt"
        "agent.key"
        "client.crt"
        "client.key"
    )

    local migrated=0
    local skipped=0
    local failed=0

    print_info "Migrating certificates from: $source_dir"
    print_info "To: $target_dir"
    echo ""

    for file in "${cert_files[@]}"; do
        local src="$source_dir/$file"
        local dst="$target_dir/$file"

        # Check if source file exists
        if [[ ! -f "$src" ]]; then
            print_verbose "Skipping $file (not found in source)"
            skipped=$((skipped + 1))
            continue
        fi

        # Check if target file exists
        if [[ -f "$dst" ]] && [[ "$FORCE" != true ]]; then
            print_warning "Skipping $file (already exists in target, use --force to overwrite)"
            skipped=$((skipped + 1))
            continue
        fi

        # Copy file
        if cp "$src" "$dst"; then
            # Set appropriate permissions
            if [[ "$file" == *.key ]]; then
                chmod 600 "$dst"
            else
                chmod 644 "$dst"
            fi
            print_success "Migrated: $file"
            migrated=$((migrated + 1))
        else
            print_error "Failed to migrate: $file"
            failed=$((failed + 1))
        fi
    done

    # Copy CA files to CA directory as well
    if [[ -f "$source_dir/ca.crt" ]]; then
        local ca_dst="$ca_target_dir/ca.crt"
        if [[ ! -f "$ca_dst" ]] || [[ "$FORCE" == true ]]; then
            cp "$source_dir/ca.crt" "$ca_dst"
            chmod 644 "$ca_dst"
            print_success "Migrated: ca.crt -> ~/.mandau/ca/"
        fi
    fi

    if [[ -f "$source_dir/ca.key" ]]; then
        local ca_key_dst="$ca_target_dir/ca.key"
        if [[ ! -f "$ca_key_dst" ]] || [[ "$FORCE" == true ]]; then
            cp "$source_dir/ca.key" "$ca_key_dst"
            chmod 600 "$ca_key_dst"
            print_success "Migrated: ca.key -> ~/.mandau/ca/"
        fi
    fi

    echo ""
    print_info "Migration Summary:"
    print_info "  Migrated: $migrated files"
    print_info "  Skipped: $skipped files"
    
    if [[ $failed -gt 0 ]]; then
        print_error "  Failed: $failed files"
        return 1
    fi

    return 0
}

# Verify migrated certificates
verify_certs() {
    local target_dir="$1"

    print_info "Verifying migrated certificates..."
    echo ""

    local errors=0

    # Verify core cert
    if [[ -f "$target_dir/core.crt" ]]; then
        if openssl verify -CAfile "$target_dir/ca.crt" "$target_dir/core.crt" &>/dev/null; then
            print_success "Core certificate: Valid"
        else
            print_error "Core certificate: Invalid"
            errors=$((errors + 1))
        fi
    fi

    # Verify agent cert
    if [[ -f "$target_dir/agent.crt" ]]; then
        if openssl verify -CAfile "$target_dir/ca.crt" "$target_dir/agent.crt" &>/dev/null; then
            print_success "Agent certificate: Valid"
        else
            print_error "Agent certificate: Invalid"
            errors=$((errors + 1))
        fi
    fi

    # Verify client cert
    if [[ -f "$target_dir/client.crt" ]]; then
        if openssl verify -CAfile "$target_dir/ca.crt" "$target_dir/client.crt" &>/dev/null; then
            print_success "Client certificate: Valid"
        else
            print_error "Client certificate: Invalid"
            errors=$((errors + 1))
        fi
    fi

    echo ""
    return $errors
}

# Print migration summary
print_summary() {
    local target_dir="$1"

    echo ""
    print_success "Certificate Migration Complete!"
    echo ""
    echo "┌─────────────────────────────────────────────────────────────┐"
    echo "│ Migration Summary                                           │"
    echo "├─────────────────────────────────────────────────────────────┤"
    echo "│ Target Directory: $target_dir                             │"
    echo "│ CA Directory: ${target_dir%/*}/ca                          │"
    echo "│                                                             │"
    echo "│ Files:                                                      │"
    ls -la "$target_dir" 2>/dev/null | tail -n +2 | while read -r line; do
        echo "│  $line"
    done
    echo "│                                                             │"
    echo "│ Next Steps:                                                 │"
    echo "│ 1. Verify certificates: mandau cert verify                  │"
    echo "│ 2. Check status: mandau cert status                         │"
    echo "│ 3. Test CLI: mandau agent list                              │"
    echo "│                                                             │"
    echo "│ Optional:                                                   │"
    echo "│ - Delete old certificates after verification                │"
    echo "│ - Update scripts to use new location                        │"
    echo "└─────────────────────────────────────────────────────────────┘"
    echo ""
}

# Main execution
main() {
    parse_args "$@"

    print_info "Mandau Certificate Migration"
    echo ""

    # Find source directory
    SOURCE_DIR=$(find_source_dir) || {
        print_error "No certificates found in legacy locations"
        print_info "Searched:"
        print_info "  - ./certs/"
        print_info "  - ~/mandau-certs/"
        print_info "  - /etc/mandau/certs/"
        print_info ""
        print_info "If you have certificates in a custom location, use --from DIR"
        exit 1
    }

    # Set target directory
    TARGET_DIR="$HOME/.mandau/certs"

    print_info "Source: $SOURCE_DIR"
    print_info "Target: $TARGET_DIR"
    echo ""

    # Check if target already has certificates
    if [[ -d "$TARGET_DIR" ]] && [[ -f "$TARGET_DIR/ca.crt" ]]; then
        print_warning "Target directory already contains certificates"
        if [[ "$FORCE" != true ]] && [[ "$DRY_RUN" != true ]]; then
            read -p "Continue and overwrite? (y/N): " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                print_info "Migration cancelled"
                exit 0
            fi
        fi
    fi

    # Backup target if needed
    if [[ "$BACKUP" == true ]]; then
        backup_target
    fi

    # Migrate certificates
    if migrate_certs "$SOURCE_DIR" "$TARGET_DIR"; then
        # Verify certificates
        verify_certs "$TARGET_DIR"

        # Print summary
        print_summary "$TARGET_DIR"
    else
        print_error "Migration failed"
        exit 1
    fi
}

main "$@"
