# Mandau Deploy Commands - Quick Reference

## The Problem
Deploying Docker images from local to remote machines traditionally requires:
- Temporary storage of large tar files
- High disk I/O overhead
- Network retransmits on failures
- Complex rollback procedures

## The Solution
**Zero-disk streaming deployment** - pipes docker save → docker load directly

## Commands at a Glance

### Deploy an Image
```bash
mandau deploy container <local-image> <remote-image> [flags]
```

| Command | Purpose |
|---------|---------|
| `mandau deploy container app:latest app:v1` | Transfer image only |
| `mandau deploy container app:latest app:v1 --up-remote` | Transfer and start |
| `mandau deploy container app:latest app:v1 --verify` | Transfer + verify |
| `mandau deploy container app:latest app:v1 --dry-run` | Preview only |

### Monitor Deployments
```bash
mandau deploy status [image-name]     # View containers
mandau deploy rollback <container>    # Stop & remove
```

## Practical Examples

### 1. Simple Transfer
```bash
mandau deploy container myapp:latest myapp:prod
```

### 2. Transfer + Launch
```bash
mandau deploy container myapp:latest myapp:prod \
  --up-remote --name myapp-prod
```

### 3. Production Setup
```bash
mandau deploy container myapp:latest myapp:prod \
  --up-remote \
  --name myapp-prod \
  -p 8080:8080 \
  -p 9090:9090 \
  -e DB_HOST=db.prod \
  -e LOG_LEVEL=info \
  -v /data:/app/data \
  --verify
```

### 4. Multi-Agent Deployment
```bash
for agent in prod-1 prod-2 prod-3; do
  mandau deploy container myapp:latest myapp:prod \
    --agent "$agent" --up-remote --verify
done
```

### 5. Blue-Green Switch
```bash
# Deploy green
mandau deploy container app:v2 app:green --up-remote

# After testing, switch to blue
mandau deploy rollback app-blue
mandau docker rename app-green app-blue
```

## Flag Reference

### Container Configuration
| Flag | Use Case |
|------|----------|
| `-n, --name` | Container name |
| `-p, --port` | Port mapping (can repeat) |
| `-e, --env` | Environment var (can repeat) |
| `-v, --volume` | Volume mount (can repeat) |

### Deployment Options
| Flag | Use Case |
|------|----------|
| `--up-remote` | Start container immediately |
| `--verify` | Check image after transfer |
| `--dry-run` | Preview without executing |
| `-a, --agent` | Target specific agent |

### Examples with Flags
```bash
# Named container
mandau deploy container app:latest app:prod --name myapp

# Multiple ports
mandau deploy container app:latest app:prod -p 8080:8080 -p 9000:9000

# Multiple env vars
mandau deploy container app:latest app:prod \
  -e DB=prod -e LOG=debug -e CACHE=redis

# Multiple volumes
mandau deploy container app:latest app:prod \
  -v /data:/app/data -v /logs:/app/logs

# Combined
mandau deploy container app:latest app:prod \
  --up-remote --name app -p 8080:8080 \
  -e ENV=prod -v /data:/app/data --verify
```

## Status & Rollback

### View Running Containers
```bash
mandau deploy status                  # All containers
mandau deploy status myapp            # Filter by image
```

### Stop & Remove Container
```bash
mandau deploy rollback myapp-prod     # With confirmation
mandau deploy rollback myapp-prod -f  # Force (no prompt)
```

## Integration with Docker Commands

After deploying, use standard docker commands:
```bash
mandau docker ps                      # List containers
mandau docker images                  # List images
mandau docker logs <id>               # View logs
mandau docker exec <id> <cmd>         # Execute command
mandau docker stop/restart <id>       # Control container
```

## Workflow Examples

### Staging → Production
```bash
# Test in staging
mandau deploy container app:latest app:staging \
  --agent staging-1 --up-remote

# Deploy to production
mandau deploy container app:latest app:prod \
  --agent prod-1 --up-remote --verify
```

### Rolling Update
```bash
# For each instance:
mandau deploy rollback app-old        # Remove old
mandau deploy container app:v2 app:prod \
  --up-remote --name app --port 8080:8080 --verify
```

### Quick Testing
```bash
# Preview without executing
mandau deploy container app:latest app:test \
  --up-remote --port 3000:3000 \
  -e DEBUG=true --dry-run

# Execute after review
mandau deploy container app:latest app:test \
  --up-remote --port 3000:3000 -e DEBUG=true
```

## Performance Tips

- **Large images**: Use `--verify` to catch issues early
- **Multiple agents**: Use script or parallel deployment
- **Rollback ready**: Tag previous versions to enable quick rollback
- **Test locally**: Run `docker run` locally before remote deploy

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "docker save failed" | Check local image: `docker images \| grep <image>` |
| "host shell error" | Check agent: `mandau agent list` |
| "image not found" | Check remote: `mandau docker images` |
| Container won't start | View logs: `mandau docker logs <id>` |

## Key Benefits

✅ **Zero Disk**: No temporary files  
✅ **Large Images**: Works with 2GB+ images  
✅ **Fast**: Direct streaming, no extra copies  
✅ **Safe**: Failed transfers leave no artifacts  
✅ **Flexible**: Full docker-run configuration  
✅ **Verified**: Optional image verification  
✅ **Reversible**: Easy rollback capability  
✅ **Scriptable**: Works in CI/CD pipelines  

## Documentation

- **DEPLOY.md** - Detailed usage guide
- **DEPLOY-GUIDE.md** - Comprehensive reference
- **DEPLOY-SUMMARY.md** - Implementation details
- **scripts/** - Ready-to-use examples

## Try It Now

```bash
# Simple test
mandau deploy container nginx:latest nginx:test --dry-run

# With verification
mandau deploy container nginx:latest nginx:test --verify

# Full deployment
mandau deploy container nginx:latest nginx:prod \
  --up-remote --name webserver \
  -p 80:80 --verify
```

---

**Need help?** Run `mandau deploy container --help` for full command reference.
