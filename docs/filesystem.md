# Remote Filesystem Management Guide

Mandau provides a comprehensive set of tools to manage files and directories on any connected remote agent directly from your local CLI.

## Commands Overview

The filesystem commands are grouped under the `fs` category:

| Command | Usage | Description |
|---------|-------|-------------|
| **ls** | `mandau fs ls [path]` | List files and directories on the agent. |
| **cp** | `mandau fs cp [local] [remote]` | Upload a local file to the remote agent. |
| **fetch** | `mandau fs fetch [remote] [local]` | Download a remote file to your local machine. |
| **cat** | `mandau fs cat [path]` | Print the contents of a remote text file to stdout. |
| **mv** | `mandau fs mv [src] [dest]` | Move or rename a file/directory on the agent. |
| **rm** | `mandau fs rm [path]` | Delete a file or directory on the agent. |
| **mkdir**| `mandau fs mkdir [path]` | Create a directory on the remote agent. |

## Usage Examples

### 1. Exploring the Remote Host
By default, `ls` starts at the root `/`:
```bash
mandau fs ls /var/log/nginx
```

### 2. File Transfers
**Upload (Local to Remote):**
```bash
mandau fs cp ./my-site.conf my-agent:/etc/nginx/sites-available/site.conf
```

**Download (Remote to Local):**
```bash
mandau fs fetch my-agent:/var/log/syslog ./server-syslog.log
```

### 3. Viewing File Contents
Quickly inspect configuration or log files without downloading them:
```bash
mandau fs cat /etc/hosts
```

### 4. Moving and Renaming
```bash
# Rename a file
mandau fs mv /tmp/temp.txt /tmp/permanent.txt

# Move to another directory
mandau fs mv /tmp/app.log /var/log/app.log
```

### 5. Cleaning Up
Remove files or directories. For directories, use the `-r` (recursive) flag:
```bash
mandau fs rm /tmp/stale-file.txt
mandau fs rm /tmp/old-projects -r
```

## Security & Path Resolution

- **Permissions**: File operations are performed with the permissions of the `mandau-agent` process on the remote host.
- **Path Resolution**: 
  - Absolute paths (starting with `/`) are used exactly as provided.
  - Relative paths (in future updates) will be resolved relative to the agent's configured stack root.
- **Safety**: The Mandau CLI uses mTLS for all filesystem operations, ensuring that your data is encrypted during transit between your machine and the remote agent.
