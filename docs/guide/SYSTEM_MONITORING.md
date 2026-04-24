# System Monitoring with Mandau

Complete guide to monitoring remote systems using Mandau's built-in system commands.

## Overview

Mandau provides comprehensive system monitoring capabilities for remote agents without requiring SSH access. All commands execute securely through the mTLS-encrypted HostShell channel.

## Quick Commands

For fast system checks, use root-level shortcuts:

```bash
# Process list
mandau ps              # Default: ps aux
mandau ps aux          # Full format
mandau ps -ef          # Alternative format

# Disk usage
mandau df              # Default: df -h
mandau df -hi          # Inodes

# Memory usage
mandau free            # Default: free -h
mandau free -m         # Megabytes

# System uptime
mandau uptime          # Shows load average
```

## Comprehensive System Information

### mandau system info

Get a complete system overview in one command:

```bash
mandau system info
mandau system info agent-001
```

**Output includes:**
- 🖥️  System Overview: Hostname, OS, Kernel, Architecture
- 💻 CPU Information: Model, Cores
- 🧠 Memory Usage: Total, used, free, available
- 💾 Disk Usage: Filesystem sizes and mount points
- ⏱️  Uptime: Load averages
- 🌐 Network Interfaces: IP addresses and status

**Example:**

```bash
$ mandau system info

📊 System Information for agent: agent-insanserver

🖥️  System Overview:
Hostname: insanserver
OS: Ubuntu 24.04.2 LTS
Kernel: 6.11.0-26-generic
Architecture: x86_64

💻 CPU Information:
CPU: Intel(R) Xeon(R) CPU E3-1240 v5 @ 3.50GHz
Cores: 8

🧠 Memory Usage:
               total        used        free      shared  buff/cache   available
Mem:            23Gi       5.9Gi       675Mi       112Mi        17Gi        17Gi
Swap:          8.0Gi        41Mi       8.0Gi

💾 Disk Usage:
Filesystem                         Size  Used Avail Use% Mounted on
/dev/mapper/ubuntu--vg-ubuntu--lv  913G  258G  618G  30% /
/dev/sda2                          2.0G  204M  1.6G  12% /boot
/dev/sda1                          1.1G  6.2M  1.1G   1% /boot/efi

⏱️  Uptime:
 04:35:07 up 303 days, 17:23,  3 users,  load average: 2.84, 2.67, 2.20

🌐 Network Interfaces:
lo               UNKNOWN        127.0.0.1/8 ::1/128
eno1             UP             103.16.199.4/26
docker0          DOWN           172.17.0.1/16
```

## Process Management

### mandau system ps

List running processes with various options:

```bash
# Default (ps aux)
mandau system ps

# Custom flags
mandau system ps aux
mandau system ps -ef
mandau system ps -eo pid,ppid,cmd,%mem,%cpu --sort=-%mem
```

**Common use cases:**

```bash
# Top memory consumers
mandau ps -eo pid,ppid,cmd,%mem,%cpu --sort=-%mem | head -20

# Top CPU consumers
mandau ps -eo pid,ppid,cmd,%mem,%cpu --sort=-%cpu | head -20

# Find specific process
mandau ps aux | grep nginx

# Process tree
mandau ps -ef --forest
```

## Disk Management

### mandau system df

Report file system disk space usage:

```bash
# Human-readable (default)
mandau system df

# With inodes
mandau system df -hi

# Custom flags
mandau system df -h
mandau system df -m  # Megabytes
```

### mandau system du

Estimate file space usage for directories:

```bash
# Summary of directory
mandau system du /var/log

# Top-level directories only
mandau system du / -h --max-depth=1

# Custom path
mandau system du agent-001 /home -sh

# Multiple directories
mandau system du /var /tmp /home -sh
```

**Common use cases:**

```bash
# Find large directories
mandau system du / -h --max-depth=2 | sort -hr | head -20

# Check log sizes
mandau system du /var/log -sh

# Docker volumes
mandau system du /var/lib/docker/volumes -h --max-depth=2
```

## Memory Management

### mandau system free

Display memory usage:

```bash
# Human-readable (default)
mandau system free

# Megabytes
mandau system free -m

# Gigabytes
mandau system free -g

# Continuous monitoring
mandau shell  # Then run: watch free -h
```

**Understanding output:**

```
               total        used        free      shared  buff/cache   available
Mem:            23Gi       5.9Gi       675Mi       112Mi        17Gi        17Gi
Swap:          8.0Gi        41Mi       8.0Gi
```

- **total**: Total installed memory
- **used**: Memory in use by applications
- **free**: Unused memory
- **buff/cache**: Memory used by kernel buffers and cache
- **available**: Memory available for starting new applications

## User Activity

### mandau system who

Show who is currently logged on:

```bash
mandau system who
mandau system who agent-001
```

**Example output:**

```
its      tty1         2025-06-16 08:41
its      pts/0        2025-06-25 07:03 (tmux(3504810).%2)
fhadli   pts/6        2025-12-08 20:43 (103.121.180.136)
```

### mandau system last

Show listing of last logged in users:

```bash
# Last 10 (default)
mandau system last

# Last 20
mandau system last -20

# Custom query
mandau system last username
```

**Example output:**

```
bhangun  pts/1        182.10.98.215    Tue Apr 14 21:09 - 21:09  (00:00)
loka     pts/1        103.28.116.127   Tue Apr 14 13:09 - 14:03  (00:54)

wtmp begins Wed Mar  5 08:49:39 2025
```

## Network Monitoring

### mandau system netstat

Show network statistics:

```bash
# Listening ports (default)
mandau system netstat

# All connections
mandau system netstat -a

# TCP only
mandau system netstat -t

# With process info
mandau system netstat -tulpn
```

**Note:** Uses `ss` command if `netstat` is not available.

**Example output:**

```
Netid State  Recv-Q Send-Q Local Address:Port  Peer Address:PortProcess
tcp   LISTEN 0      511    0.0.0.0:80         0.0.0.0:*
tcp   LISTEN 0      4096   0.0.0.0:22         0.0.0.0:*
tcp   LISTEN 0      200    127.0.0.1:5432     0.0.0.0:*
tcp   LISTEN 0      4096   0.0.0.0:3005       0.0.0.0:*
```

## Log Management

### mandau system logs

Tail log files on remote agents:

```bash
# Default syslog
mandau system logs

# Specific log file
mandau system logs /var/log/nginx/access.log

# Follow mode (interactive)
mandau system logs /var/log/syslog -f

# Last N lines
mandau system logs /var/log/syslog -n 100

# Multiple files
mandau system logs /var/log/nginx/error.log -n 50
```

**Common log files:**

```bash
# System logs
mandau system logs /var/log/syslog
mandau system logs /var/log/kern.log

# Application logs
mandau system logs /var/log/nginx/access.log
mandau system logs /var/log/postgresql/postgresql.log

# Docker logs (if not using mandau docker logs)
mandau system logs /var/lib/docker/containers/*/container.log
```

## Interactive Tools

### mandau system top

Interactive process viewer:

```bash
mandau system top
mandau system top agent-001
```

**Features:**
- Real-time process monitoring
- CPU and memory usage
- Interactive controls
- Process management

### mandau system htop

Advanced interactive process viewer:

```bash
mandau system htop
```

**Requirements:**
- `htop` must be installed on the remote agent
- Install: `apt-get install htop` or `yum install htop`

## Monitoring Workflows

### Daily Health Check

```bash
# Quick overview
mandau system info

# Resource usage
mandau df -h
mandau free -h
mandau uptime

# Check critical services
mandau ps aux | grep nginx
mandau ps aux | grep postgres
```

### Performance Investigation

```bash
# Top resource consumers
mandau ps -eo pid,ppid,cmd,%mem,%cpu --sort=-%cpu | head -20

# Disk I/O hotspots
mandau system du /var -h --max-depth=2 | sort -hr | head -10

# Network connections
mandau system netstat -tulpn

# Memory details
mandau system free -h
```

### Security Audit

```bash
# Check logged-in users
mandau system who

# Recent login history
mandau system last -50

# Listening services
mandau system netstat -tulpn

# Running processes
mandau ps aux
```

### Capacity Planning

```bash
# Disk usage trends
mandau system du / -h --max-depth=2 | sort -hr

# Memory utilization
mandau system free -m

# CPU load
mandau uptime

# Process count
mandau ps aux | wc -l
```

## Automation Examples

### Monitoring Script

Create a monitoring script:

```bash
#!/bin/bash
# check-system.sh

AGENT=${1:-""}

echo "=== System Health Check ==="
echo ""

echo "📊 System Info:"
mandau system info $AGENT | head -20
echo ""

echo "💾 Disk Usage:"
mandau system df $AGENT
echo ""

echo "🧠 Memory:"
mandau system free $AGENT
echo ""

echo "⏱️  Load:"
mandau uptime $AGENT
echo ""

echo "🔌 Network:"
mandau system netstat $AGENT -tulpn | head -20
```

### Alert Thresholds

Check for issues:

```bash
# Check disk usage > 80%
mandau df | awk 'NR>1 {if ($5+0 > 80) print "WARNING: " $6 " is " $5 " full"}'

# Check load average
mandau uptime | awk -F'load average:' '{print $2}' | awk -F',' '{if ($1+0 > 4) print "HIGH LOAD: " $1}'

# Check memory usage
mandau free | awk 'NR==2 {if ($3/$2*100 > 80) print "HIGH MEMORY: " $3/$2*100 "% used"}'
```

## Comparison with Traditional Tools

| Traditional | Mandau Equivalent | Notes |
|-------------|-------------------|-------|
| `ssh host ps aux` | `mandau ps aux` | Faster, no SSH setup needed |
| `ssh host df -h` | `mandau df -h` | Encrypted via mTLS |
| `ssh host free -m` | `mandau free -m` | Same output |
| `ssh host top` | `mandau system top` | Interactive support |
| `ssh host tail -f /var/log/syslog` | `mandau system logs /var/log/syslog -f` | Stream logs |
| `ssh host` | `mandau shell` | Full shell access |

## Security Considerations

- All commands execute through mTLS-encrypted channels
- No SSH keys to manage
- Certificate-based authentication
- Audit logging of all commands (when enabled)
- Per-agent access control via RBAC

## Troubleshooting

### Command Timeout

If a command times out:
```bash
# Try with shell for long-running commands
mandau shell
# Then run: du -sh /var
```

### Permission Denied

Some commands may require elevated privileges:
```bash
# Use shell for sudo commands
mandau shell
sudo du -sh /root
```

### Command Not Found

If a command is not available:
```bash
# Install via shell
mandau shell
sudo apt-get install htop
```

## See Also

- [CLI Reference Guide](CLI_REFERENCE.md)
- [Quick Start Guide](QUICKSTART.md)
- [Shell Access Guide](#shell-access)
