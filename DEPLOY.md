# Mandau Deploy Container Command

Deploy Docker images from your local machine to remote agents safely and efficiently.

## Overview

The `mandau deploy container` command streams Docker images directly from local to remote without storing intermediary tar files, supporting large images and maintaining zero-disk overhead.

**Features:**
- 🔄 **Streaming transfer**: No temporary files; pipes docker save → docker load
- ✅ **Verification**: Optional image verification post-load
- 🚀 **Auto-launch**: Optional immediate container startup with custom config
- 🏷️ **Flexible tagging**: Tag images with custom names
- 🔍 **Dry-run support**: Preview commands before execution
- 📝 **Full docker-run control**: Ports, env vars, volumes, custom args

## Basic Usage

### Simple deployment (image only)

```bash
mandau deploy container my-app:latest my-app:v1.0
```

This transfers the local image `my-app:latest` to the remote agent and tags it as `my-app:v1.0`.

### Deploy and start container

```bash
mandau deploy container my-app:latest my-app:v1.0 --up-remote
```

### Deploy with container name

```bash
mandau deploy container my-app:latest my-app:v1.0 \
  --up-remote \
  --name my-app-prod
```

## Advanced Usage

### Deploy with ports, environment variables, and volumes

```bash
mandau deploy container my-app:latest my-app:v1.0 \
  --up-remote \
  --name my-app \
  --port 8080:8080 \
  --port 9000:9000 \
  --env DB_HOST=db.example.com \
  --env DB_PORT=5432 \
  --volume /data:/app/data \
  --volume /logs:/app/logs
```

### Verify image after deployment

```bash
mandau deploy container my-app:latest my-app:v1.0 \
  --verify
```

This will confirm the image exists on the remote agent after loading.

### Dry-run to preview commands

```bash
mandau deploy container my-app:latest my-app:v1.0 \
  --up-remote \
  --name my-app \
  --port 8080:8080 \
  --dry-run
```

Output shows what would be executed without making changes:
```
📦 Deploying image 'my-app:latest' to agent agent-1 as 'my-app:v1.0'
🔍 DRY RUN MODE - No changes will be made
📋 Would run: docker run -d --name my-app -p 8080:8080 my-app:v1.0
```

### Target specific agent

```bash
mandau deploy container my-app:latest my-app:v1.0 \
  --up-remote \
  --agent agent-2
```

## Full Command Reference

```
mandau deploy container [local-image] [remote-image] [flags]

Arguments:
  local-image    Docker image tag on local machine (e.g., my-app:latest)
  remote-image   Tag name for the image on remote agent (e.g., my-app:v1.0)

Flags:
  -p, --port stringSlice           Publish port(s): -p host:container (repeatable)
  -e, --env stringSlice            Set env var(s): -e KEY=VALUE (repeatable)
  -v, --volume stringSlice         Mount volume(s): -v /host:/container (repeatable)
      --docker-run-args stringSlice Additional docker run arguments
  -n, --name string                Container name for the running container
      --up-remote                  Start the container on remote after loading image
      --verify                     Verify image exists on remote after loading
      --dry-run                    Show what would be done without making changes
  -a, --agent string               Target agent ID (auto-selected if not specified)
```

## How It Works

1. **Streaming Transfer**
   - Starts local `docker save <image>` process
   - Opens remote HostShell session
   - Pipes output directly: `docker save` → remote `docker load`
   - No intermediate storage on local or remote

2. **Image Tagging**
   - After load, image is tagged with the remote-image name
   - Original image name is preserved during transfer

3. **Optional Container Launch**
   - If `--up-remote` is set, starts a container immediately
   - Supports custom ports, env vars, volumes, and arbitrary docker args

4. **Verification** (optional)
   - `docker image inspect <image>` on remote confirms image exists
   - Catches failed loads or tagging issues early

## Use Cases

### CI/CD Pipeline
Deploy build artifacts directly from CI to production agents:
```bash
mandau deploy container myapp:$GIT_COMMIT myapp:prod \
  --up-remote \
  --verify \
  --port 8080:8080 \
  --env ENVIRONMENT=production
```

### Blue-Green Deployments
Test new image before cutting traffic:
```bash
# Deploy new version to staging
mandau deploy container myapp:v2 myapp:staging --up-remote --name myapp-staging

# After verification, switch to production
mandau deploy container myapp:v2 myapp:prod --up-remote --name myapp-prod
```

### Multi-Environment Rollout
```bash
# Staging
mandau deploy container myapp:latest myapp:staging --agent staging-1 --up-remote

# Production (multiple agents)
mandau deploy container myapp:latest myapp:prod --agent prod-1 --up-remote
mandau deploy container myapp:latest myapp:prod --agent prod-2 --up-remote
mandau deploy container myapp:latest myapp:prod --agent prod-3 --up-remote
```

### Development & Testing
```bash
# Quick test without starting container
mandau deploy container dev-image:test dev-image:verify --verify

# Preview what would run
mandau deploy container dev-image:latest dev-image:prod \
  --port 3000:3000 \
  --env DEBUG=true \
  --dry-run
```

## Performance & Safety

### Performance
- **No disk I/O**: Streaming transfer avoids temporary files
- **Efficient for large images**: Handles multi-GB images without buffering
- **Minimal network overhead**: Direct pipe reduces copies

### Safety
- **Atomic operations**: Image only tagged after successful load
- **Dry-run mode**: Preview commands before execution
- **Verification available**: Confirm image exists post-load
- **Clear error reporting**: Distinguishes local vs. remote errors

## Troubleshooting

### "docker save failed"
Ensure the local image exists:
```bash
docker images | grep my-app:latest
```

### "remote shell error" during transfer
Check network connectivity to the agent:
```bash
mandau agent list
```

### Image not found on remote after load
Enable `--verify` to catch this early:
```bash
mandau deploy container my-app:latest my-app:v1.0 --verify
```

### Container fails to start with "image not found"
The image may not have been tagged. Check:
```bash
mandau docker images
```

And explicitly tag it:
```bash
mandau docker tag my-app:latest my-app:v1.0
```

## Integration with Other Mandau Commands

### View deployed containers
```bash
mandau docker ps
# or
mandau docker list
```

### View all images on remote
```bash
mandau docker images
```

### View container logs
```bash
mandau docker logs <container-id>
```

### Execute commands inside container
```bash
mandau docker exec <container-id> <command>
```

## Notes

- Images transferred with their original name and tag preserved
- The `--docker-run-args` flag allows arbitrary docker run options for advanced use cases
- Verification uses `docker image inspect`, which is fast and non-intrusive
- If `--up-remote` is not set, the image is only transferred and tagged; no container is started
