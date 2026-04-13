# Mandau Deploy Commands

Complete Docker deployment solution integrated with Mandau CLI. Transfer images from local machines to remote agents and manage their lifecycle.

## Quick Start

### Deploy a single image

```bash
# Transfer image only
mandau deploy container myapp:latest myapp:v1.0

# Transfer and start container
mandau deploy container myapp:latest myapp:v1.0 --up-remote

# Transfer, start, and name container
mandau deploy container myapp:latest myapp:v1.0 \
  --up-remote \
  --name myapp-prod
```

### Deployment with full configuration

```bash
mandau deploy container myapp:latest myapp:prod \
  --up-remote \
  --name myapp-prod \
  --port 8080:8080 \
  --port 9090:9090 \
  --env DATABASE_URL=postgres://db:5432 \
  --env LOG_LEVEL=info \
  --volume /data:/app/data \
  --volume /config:/etc/myapp \
  --verify
```

## Commands Overview

### 1. Deploy Container

Transfer a local Docker image to a remote agent and optionally start it.

```bash
mandau deploy container [local-image] [remote-image] [flags]
```

**Features:**
- Zero-disk streaming transfer (no temporary files)
- Optional automatic container startup
- Custom ports, environment variables, volumes
- Image verification post-load
- Dry-run mode for planning

**Flags:**

| Flag | Short | Description | Example |
|------|-------|-------------|---------|
| `--up-remote` | | Start container after image load | |
| `--name` | `-n` | Container name | `-n myapp` |
| `--port` | `-p` | Port mapping (repeatable) | `-p 8080:8080 -p 9000:9000` |
| `--env` | `-e` | Environment variable (repeatable) | `-e DB_HOST=db -e DEBUG=true` |
| `--volume` | `-v` | Volume mount (repeatable) | `-v /data:/app/data` |
| `--docker-run-args` | | Additional docker run options | `--docker-run-args --restart=always` |
| `--verify` | | Verify image after load | |
| `--dry-run` | | Preview without executing | |
| `--agent` | `-a` | Target agent ID | `-a prod-1` |

**Examples:**

```bash
# Minimal: transfer image only
mandau deploy container app:latest app:v1.0

# Start container with defaults
mandau deploy container app:latest app:prod --up-remote

# Full production setup
mandau deploy container app:latest app:prod \
  --up-remote \
  --name myapp-prod \
  --port 8080:8080 \
  --env ENVIRONMENT=production \
  --env DOMAIN=example.com \
  --volume /logs:/app/logs \
  --verify

# Preview before execution
mandau deploy container app:latest app:prod \
  --up-remote --name app \
  --port 8080:8080 \
  --dry-run

# Target specific agent
mandau deploy container app:latest app:prod \
  --agent prod-1 \
  --up-remote
```

### 2. Deployment Status

View containers and images deployed on a remote agent.

```bash
mandau deploy status [image-name]
```

**Examples:**

```bash
# View all containers
mandau deploy status

# View containers for specific image
mandau deploy status myapp

# On specific agent
mandau deploy status myapp --agent prod-1
```

**Output:**

```
📊 Deployment Status (Agent: prod-1)

CONTAINER           IMAGE                          STATE                STATUS
myapp-prod          myapp:prod                     running              Up 2 hours
api-server          api:latest                     running              Up 1 hour

2 container(s) running
```

### 3. Deployment Rollback

Stop and remove a deployed container.

```bash
mandau deploy rollback [container-name] [flags]
```

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Skip confirmation prompt |
| `--agent` | `-a` | Target agent ID |

**Examples:**

```bash
# Rollback with confirmation
mandau deploy rollback myapp-prod

# Force rollback without confirmation
mandau deploy rollback myapp-prod -f

# On specific agent
mandau deploy rollback myapp-prod --agent prod-1 -f
```

## Advanced Workflows

### Blue-Green Deployment

Deploy new version alongside current, test, then switch:

```bash
# 1. Deploy green (new version)
mandau deploy container myapp:v2 myapp:green \
  --up-remote --name myapp-green -p 8081:8080

# 2. Test green version
curl http://agent:8081/health

# 3. If OK, switch: stop blue, keep green
mandau deploy rollback myapp-blue
mandau docker rename myapp-green myapp-blue

# 4. If issues, rollback: stop green, restart blue
mandau deploy rollback myapp-green
mandau deploy container myapp:v1 myapp:blue \
  --up-remote --name myapp-blue -p 8080:8080
```

Or use the provided script:

```bash
./scripts/deploy-blue-green.sh myapp:v2
```

### Multi-Agent Deployment

Deploy to multiple agents sequentially or in parallel:

```bash
# Sequential deployment with verification
for agent in prod-1 prod-2 prod-3; do
  mandau deploy container myapp:latest myapp:prod \
    --agent "$agent" \
    --up-remote \
    --name myapp \
    --port 8080:8080 \
    --verify
done
```

Or use the multi-agent script:

```bash
./scripts/deploy-multi-agent.sh
```

### CI/CD Integration

Deploy automatically from build pipelines:

```bash
# In your CI/CD (GitHub Actions, GitLab CI, etc.)
mandau deploy container myapp:$GIT_COMMIT myapp:$ENVIRONMENT \
  --agent "$TARGET_AGENT" \
  --up-remote \
  --verify \
  --name "myapp-$ENVIRONMENT" \
  --port 8080:8080 \
  --env "GIT_COMMIT=$GIT_COMMIT" \
  --env "BUILD_ID=$CI_BUILD_ID"
```

Or use the CI/CD script:

```bash
./scripts/deploy-cicd.sh
```

### Rolling Updates

Update containers one at a time:

```bash
# Get current containers
mandau deploy status myapp

# For each instance:
# 1. Deploy new version to a temp name
mandau deploy container myapp:v2 myapp:v2-temp \
  --up-remote --name "myapp-temp-1" -p 8090:8080

# 2. Verify it works
curl http://agent:8090/health

# 3. Remove old version, start new version at main port
mandau deploy rollback myapp
mandau deploy container myapp:v2 myapp:v2 \
  --up-remote --name myapp -p 8080:8080

# 4. Remove temp
mandau deploy rollback myapp-temp-1
```

## How It Works

### Streaming Transfer (Zero-Disk)

The deploy container command uses a novel streaming approach:

1. **Local machine**: Starts `docker save <image>` process
2. **Remote connection**: Opens bidirectional shell session to agent
3. **Streaming pipe**: Pipes local `docker save` output directly into remote `docker load`
4. **No buffers**: Image is never written to disk (local or remote)

**Advantages:**
- ✅ No disk space requirements for large images
- ✅ Faster transfers (no extra I/O)
- ✅ Safe: if transfer fails, nothing is left behind
- ✅ Supports images larger than available disk

### Image Tagging

After successful load, the image is tagged with the desired name:

```bash
# If you deploy: myapp:latest → myapp:prod
# The image retains all layers and is tagged as myapp:prod
```

### Container Launch

When `--up-remote` is set, a container is started with:
- Custom name (--name)
- Port mappings (--port)
- Environment variables (--env)
- Volume mounts (--volume)
- Additional docker run args

### Verification

Optional `--verify` flag runs `docker image inspect` to confirm:
- Image exists on remote
- Image is loadable
- No corrupted layers

## Troubleshooting

### "docker save failed"
- Ensure image exists locally: `docker images | grep <image>`
- Ensure Docker is running: `docker ps`
- Check local network: `ping agent-address`

### "host shell error" during transfer
- Check agent connectivity: `mandau agent list`
- Verify agent status is "online"
- Check network between local machine and agent

### "image not found" after deploy
- Enable `--verify` to catch early: `mandau deploy container img img:tag --verify`
- Check image on remote: `mandau docker images`
- Check for naming issues: image names are case-sensitive

### Container fails to start
- Check image exists: `mandau docker images`
- Review docker run command: `mandau deploy container img img:tag --up-remote --dry-run`
- Check agent resources: `mandau docker stats`
- View container logs: `mandau docker logs <container-id>`

### Transfer is slow
- Check network bandwidth: `mandau docker run -d speedtest/speedtest`
- Reduce image size: optimize Dockerfile, remove unnecessary files
- Consider pushing to registry instead for very large images

## Integration with Mandau Ecosystem

The deploy commands integrate with existing Mandau features:

```bash
# After deploying, manage with docker commands
mandau docker ps                      # View containers
mandau docker images                  # View images
mandau docker logs <container-id>     # Stream logs
mandau docker exec <id> <command>     # Execute in container
mandau docker stop/start/restart      # Container control

# Or use fs commands for config management
mandau fs cp config.yaml /etc/myapp/config.yaml
```

## Examples

See the `scripts/` directory for ready-to-use examples:

- `deploy-multi-agent.sh` - Deploy across multiple agents with confirmation
- `deploy-blue-green.sh` - Blue-green deployment with traffic switchover
- `deploy-cicd.sh` - CI/CD pipeline integration

## Performance Notes

- **Typical transfer times**: 100 MB/min over 100 Mbps network
- **Zero disk overhead**: No temporary files during transfer
- **Safe failures**: Transfer interruptions leave no artifacts
- **Large images**: Tested with 2GB+ images without issues

## Security

- **TLS encryption**: All data transferred over encrypted gRPC channels
- **Agent authentication**: Automatic certificate-based auth
- **Image verification**: Optional post-load verification to catch corruption
- **Dry-run mode**: Plan changes before executing

## Related Commands

```bash
mandau docker ps                  # List containers
mandau docker images              # List images
mandau docker inspect <id>        # Container details
mandau docker logs <id>           # Container logs
mandau docker exec <id> cmd       # Execute in container
mandau docker stop/start          # Container control
mandau docker stats               # Container resource usage

mandau fs cp file remote:/path    # Copy config files
mandau fs fetch /path file        # Download files from agent
mandau shell                       # Interactive shell on agent
```

## See Also

- [DEPLOY.md](./DEPLOY.md) - Detailed deployment guide
- [README.md](./README.md) - General Mandau documentation
- [docker.go](./cmd/mandau-cli/docker.go) - Docker command wrapper
