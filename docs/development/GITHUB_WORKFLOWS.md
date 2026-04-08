# GitHub Workflows & Installation Improvements

## Overview
This document summarizes the improvements made to GitHub Actions workflows and installation documentation to make Mandau easy to install across all platforms via `curl -sSL`.

---

## ✅ Workflow Improvements

### 1. Test Workflow (`.github/workflows/test.yml`)

**Before:**
- Tested Go 1.21 and 1.22
- Used outdated `actions/setup-go@v4`
- No protoc installation
- Basic test execution
- No coverage reporting

**After:**
- ✅ Updated to **Go 1.24** (matches `go.mod`)
- ✅ Uses `actions/setup-go@v5` (latest)
- ✅ Installs `protobuf-compiler` for proto generation
- ✅ Runs tests for all new packages:
  - `pkg/auth/...`
  - `pkg/agent/queue/...`
  - `pkg/agent/operation/...`
  - `pkg/middleware/...`
  - `pkg/tlsutil/...`
- ✅ Generates coverage report
- ✅ Uploads to Codecov
- ✅ Better output with success messages

**Key Changes:**
```yaml
# Go version updated
go-version: [ '1.24' ]

# Added protoc installation
- name: Install protoc
  run: |
    sudo apt-get update
    sudo apt-get install -y protobuf-compiler

# Comprehensive test execution
- name: Test
  run: |
    go test -v -race -coverprofile=coverage.out \
      ./pkg/auth/... \
      ./pkg/agent/queue/... \
      ./pkg/agent/operation/... \
      ./pkg/middleware/... \
      ./pkg/tlsutil/...
    go tool cover -func=coverage.out | tail -1

# Coverage upload
- name: Upload coverage
  uses: codecov/codecov-action@v4
  if: always()
```

---

### 2. Release Workflow (`.github/workflows/release.yml`)

**Before:**
- Used Go 1.21
- Manual build steps for each platform
- No checksums generated
- Basic release notes
- Used `actions/setup-go@v4` and `softprops/action-gh-release@v1`

**After:**
- ✅ Updated to **Go 1.24**
- ✅ Uses `actions/setup-go@v5` and `softprops/action-gh-release@v2`
- ✅ **Automated build loop** for all platforms
- ✅ **SHA256 checksums** for all binaries
- ✅ **Combined SHA256SUMS.txt** file
- ✅ **Installation script verification** (syntax check)
- ✅ **Auto-generated release notes**
- ✅ **Comprehensive release body** with installation instructions

**Platform Matrix:**
```
linux/amd64    - Linux 64-bit (Intel/AMD)
linux/arm64    - Linux ARM64 (AWS Graviton, Raspberry Pi)
darwin/amd64   - macOS Intel
darwin/arm64   - macOS Apple Silicon (M1/M2/M3)
windows/amd64  - Windows 64-bit
```

**Checksums Generation:**
```bash
# Individual checksums
checksums/mandau-linux-amd64-v0.1.0.tar.gz.sha256
checksums/mandau-linux-arm64-v0.1.0.tar.gz.sha256
checksums/mandau-darwin-amd64-v0.1.0.tar.gz.sha256
checksums/mandau-darwin-arm64-v0.1.0.tar.gz.sha256
checksums/mandau-windows-amd64-v0.1.0.zip.sha256

# Combined file
dist/SHA256SUMS.txt
```

**Verify Downloads:**
```bash
# Download checksums
curl -fsSL https://github.com/bhangun/mandau/releases/download/v0.1.0/SHA256SUMS.txt -o SHA256SUMS.txt

# Verify your download
sha256sum -c SHA256SUMS.txt
```

**Installation Script Validation:**
```yaml
- name: Verify installation script
  run: |
    bash -n scripts/install.sh
    echo "✅ Installation script syntax valid"
```

---

## 📦 Installation Script

The installation script (`scripts/install.sh`) provides:

### Features
- ✅ **Auto-detection** of OS (Linux/macOS/Windows)
- ✅ **Auto-detection** of architecture (amd64/arm64)
- ✅ **Latest version** fetched from GitHub API
- ✅ **Certificate generation** (CA, Core, Agent, Client)
- ✅ **Configuration profiles** (dev, test, prod)
- ✅ **Systemd services** (Linux)
- ✅ **Stack directory** creation

### Usage

**Linux/macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | sudo bash
```

**What it does:**
1. Detects platform: `linux/amd64`, `darwin/arm64`, etc.
2. Fetches latest release from GitHub API
3. Downloads appropriate archive
4. Extracts binaries to `/usr/local/bin/`:
   - `mandau` (CLI)
   - `mandau-core` (server)
   - `mandau-agent` (host agent)
5. Generates TLS certificates in `~/mandau-certs/`
6. Creates configuration in `~/.mandau/`
7. Creates systemd service files (Linux)
8. Creates `~/mandau-stacks/` directory

---

## 📚 Documentation Updates

### 1. README.md

**Updated Quick Start:**
```markdown
### 1. Install (One Command)

**Linux/macOS:**
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | sudo bash

**Windows (PowerShell):**
Invoke-WebRequest -Uri "https://github.com/bhangun/mandau/releases/latest/download/install.sh" -OutFile "install.sh"
bash install.sh
```

**Removed:**
- Redundant installation options section
- Old version references (v0.0.6)
- Confusing multi-option layout

**Simplified to:**
1. Install (curl command)
2. Generate certificates (if needed)
3. Build from source (for development)
4. Post-installation setup

### 2. INSTALL.md (New)

**Comprehensive installation guide** with:

- **Platform-specific instructions**:
  - Ubuntu/Debian Linux
  - RHEL/CentOS/Fedora Linux
  - macOS (Homebrew)
  - macOS (Apple Silicon)
  - Windows (WSL2)
  - Windows (Native)

- **Installation methods**:
  - Quick install (curl)
  - From release binaries
  - From source

- **Post-installation setup**:
  - Verification steps
  - Certificate generation
  - Service startup options (systemd vs manual)
  - Web dashboard access

- **Configuration**:
  - Environment variables
  - Configuration files
  - Example configs

- **Uninstallation**:
  - Linux/macOS steps
  - Windows steps

- **Troubleshooting**:
  - Installation failures
  - Certificate errors
  - Port conflicts
  - Permission issues

---

## 🔐 Security Improvements

### Checksums for Verification

**Why it matters:**
- Ensures binary integrity
- Prevents tampered downloads
- Verifies complete transfer

**How to use:**
```bash
# Download binary and checksum
curl -fsSL https://github.com/bhangun/mandau/releases/download/v0.1.0/mandau-linux-amd64-v0.1.0.tar.gz -o mandau.tar.gz
curl -fsSL https://github.com/bhangun/mandau/releases/download/v0.1.0/SHA256SUMS.txt -o SHA256SUMS.txt

# Verify
sha256sum -c SHA256SUMS.txt
# Output: mandau-linux-amd64-v0.1.0.tar.gz: OK
```

### Installation Script Syntax Validation

The release workflow now validates the installation script syntax before publishing:

```yaml
- name: Verify installation script
  run: |
    bash -n scripts/install.sh
    echo "✅ Installation script syntax valid"
```

This prevents broken installation scripts from being released.

---

## 🚀 Release Process

### Creating a Release

1. **Tag the release:**
   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```

2. **GitHub Actions automatically:**
   - ✅ Builds binaries for all 5 platforms
   - ✅ Generates SHA256 checksums
   - ✅ Validates installation script
   - ✅ Creates GitHub Release with:
     - All binary archives
     - Checksum files
     - Installation script
     - Auto-generated release notes
     - Comprehensive installation guide

3. **Users can then:**
   ```bash
   # One-command install
   curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | sudo bash

   # Or download from releases and verify checksums
   curl -fsSL https://github.com/bhangun/mandau/releases/download/v0.1.0/SHA256SUMS.txt -o SHA256SUMS.txt
   sha256sum -c SHA256SUMS.txt
   ```

---

## 📊 Workflow Comparison

| Feature | Before | After |
|---------|--------|-------|
| Go Version | 1.21 | **1.24** ✅ |
| Setup Go Action | v4 | **v5** ✅ |
| Protoc Install | ❌ | **✅** |
| Test Coverage | Basic | **Codecov upload** ✅ |
| Checksums | ❌ | **SHA256SUMS.txt** ✅ |
| Release Notes | Manual | **Auto-generated** ✅ |
| Script Validation | ❌ | **Syntax check** ✅ |
| Build Process | Manual per platform | **Automated loop** ✅ |
| Release Body | Basic | **Comprehensive guide** ✅ |
| Installation Docs | Scattered | **Centralized INSTALL.md** ✅ |

---

## 🎯 Benefits

### For Users
1. **One-command install** - `curl | sudo bash`
2. **Verified downloads** - SHA256 checksums
3. **Platform auto-detection** - No manual selection needed
4. **Complete setup** - Certificates, config, services all created
5. **Clear documentation** - Platform-specific guides

### For Developers
1. **Automated releases** - Just tag and push
2. **Checksums automatic** - No manual generation
3. **Better CI/CD** - Go 1.24, coverage, protoc
4. **Release notes auto** - From git log
5. **Script validation** - Catch errors before release

### For Security
1. **Binary verification** - SHA256 checksums
2. **Script validation** - Syntax checking
3. **Secure defaults** - TLS 1.3, mTLS
4. **Audit trail** - All steps logged in workflow

---

## 📝 Files Modified/Created

### Modified (3)
- `.github/workflows/test.yml` - Updated to Go 1.24, added tests, coverage
- `.github/workflows/release.yml` - Automated builds, checksums, better releases
- `README.md` - Simplified quick start, added curl command

### Created (1)
- `INSTALL.md` - Comprehensive installation guide with platform-specific instructions

---

## 🧪 Testing the Workflows

### Test Workflow
The test workflow runs on every push/PR to `main`:
```bash
# Trigger by pushing to a branch
git push origin feature-branch

# Or create a PR
gh pr create --title "Test PR" --body "Testing workflow"
```

### Release Workflow
To test the release workflow:
```bash
# Create a test tag
git tag v0.0.0-test
git push origin v0.0.0-test

# Delete after testing
git push --delete origin v0.0.0-test
git tag -d v0.0.0-test
```

---

## 🎉 Summary

All GitHub workflows are now:
- ✅ **Updated to Go 1.24**
- ✅ **Automated and efficient**
- ✅ **Secure with checksums**
- ✅ **Well-documented**
- ✅ **Easy to use via curl**

Users can now install Mandau on any platform with a single command:

```bash
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | sudo bash
```

And verify their installation:

```bash
mandau --help
mandau-core --version
mandau-agent --version
```

**Production-ready CI/CD with secure, easy installation across all platforms!** 🗡️
