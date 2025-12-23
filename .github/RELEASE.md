# GitHub Actions Workflows

This directory contains GitHub Actions workflows for the Mandau project.

## Workflows

### Build and Test (`test.yml`)
- Runs on every push to `main` branch and on pull requests
- Tests the code with multiple Go versions (1.21, 1.22)
- Builds the binaries and runs tests
- Verifies certificates generation

### Release (`release.yml`)
- Triggers when a new tag is pushed (e.g., `v1.0.0`)
- Builds static binaries for multiple platforms:
  - Linux AMD64/ARM64
  - macOS AMD64/ARM64
  - Windows AMD64
- Creates a GitHub release with the binaries
- Packages binaries in compressed archives

### Docker (`docker.yml`)
- Triggers when a new tag is pushed or release is published
- Builds Docker images for both core and agent
- Pushes images to GitHub Container Registry (ghcr.io)
- Tags images with the release version

### Draft Release (`draft-release.yml`)
- Manual workflow to create a draft release
- Generates changelog based on commit history
- Creates a draft release that can be reviewed before publishing

## Release Process

To create a new release:

1. Create and push a new tag:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. The `release.yml` workflow will automatically:
   - Build binaries for all supported platforms
   - Create a GitHub release
   - Upload the binaries as release assets

3. The `docker.yml` workflow will automatically:
   - Build Docker images
   - Push them to ghcr.io

## Container Images

Published images are available at:
- `ghcr.io/bhangun/mandau-core:tag`
- `ghcr.io/bhangun/mandau-agent:tag`