
## How to update the certificates manually
scp <USER>@<IP_ADDRESS>:~/.mandau/certs/{ca.crt,client.crt,client.key} ~/.mandau/certs/





## Secure Environment Variables

### Setting up secure environment variables

1. **Set a variable manually:**
```bash
mandau env set DB_PASSWORD=secret123
```

2. **Import from a .env file:**
```bash
mandau env import prod.env
```

3. **List stored variables:**
```bash
mandau env list
```

### Using env vars with apply

Include specific variables from the secure store:
```bash
mandau apply docker-compose.yaml --env DB_PASSWORD,API_KEY
```

Include additional .env files:
```bash
mandau apply docker-compose.yaml --env-file prod.env
```

Combine both:
```bash
mandau apply docker-compose.yaml --env DB_PASSWORD --env-file prod.env
```

### Troubleshooting env issues

If variables are not being passed:
1. Check stored variables: `mandau env list`
2. Verify the keys match exactly (case-sensitive)
3. Ensure the store is initialized (may need MANDAU_ENV_MASTER_PASSWORD set)





## When the agent is not registering itself
Run these commands on your remote server (<REMOTE_SERVER_HOST>):

1. Check the Agent's status
It's probably stuck in a "restarting" loop or just failing silently:

```bash
sudo systemctl status mandau-agent
```

2. Update the Agent's service file
We need to change -server localhost:3443 to -server localhost:9444 in the agent's service definition.

```bash
sudo sed -i 's/-server localhost:3443/-server localhost:9444/g' /etc/systemd/system/mandau-agent.service
```

3. Reload and Restart the Agent
```bash
sudo systemctl daemon-reload
sudo systemctl restart mandau-agent
```

4. Verify from your Mac
After a few seconds, the agent should register itself. Try listing from your local again:

```bash
mandau agent list
```
Once you see agent-<REMOTE_SERVER_HOST> in that list, your entire stack is officially stabilized!



## Error: rpc error: code = Unavailable desc = connection error: desc = "transport: Error while dialing: dial tcp [IP_ADDRESS]:9444: connect: connection refused"
```bash
 % mandau agent list
Loaded configuration from ~/.mandau/config.yaml
Using auto-discovered certificates from ~/.mandau/certs
Error: rpc error: code = Unavailable desc = connection error: desc = "transport: Error while dialing: dial tcp [IP_ADDRESS]:9444: connect: connection refused"
```

If connection refused, we need to check if the service actually successfully started on 9444 and if the port is open to the outside world.

Please run these 3 commands on your remote server (<REMOTE_SERVER_HOST>):
1. Check if the service is actually running on 9444
It's possible the restart failed or it couldn't bind to the new port:

```bash
sudo systemctl status mandau-core
sudo netstat -tulpn | grep 9444
```

2. Check the logs for errors
If the status is "failed", the logs will tell us why (e.g., another process already on 9444):

```bash
sudo journalctl -u mandau-core -n 20
```

3. Open the port in the firewall
Even if the service is running, the server might be blocking port 9444:

```bash
sudo ufw allow 9444/tcp
```
Also, check from your Mac:
You can quickly check if the port is "visible" from your Mac using this command:

```bash
nc -zv [IP_ADDRESS] 9444
```
If it says "succeeded", then the network is fine.
If it says "refused" or "timeout", then the issue is definitely on the server (firewall or service stopped).




## How to update the binary manually

## 1. Upload the new binary to a temporary location
scp bin/mandau-core-linux <USER>@<IP_ADDRESS>:/tmp/mandau-core

## 2. Move it to the system binary path and restart the service
ssh <USER>@<IP_ADDRESS> "sudo mv /tmp/mandau-core /usr/local/bin/mandau-core && sudo systemctl restart mandau-core"

## 3. Check the logs to confirm it's now listening on 9444
ssh <USER>@<IP_ADDRESS> "sudo journalctl -u mandau-core -n 5"

## 4. Install and Restart (with terminal for password)
```bash
ssh -t <USER>@<IP_ADDRESS> "sudo mv /tmp/mandau-core /usr/local/bin/mandau-core && \
                            sudo mv /tmp/mandau-agent /usr/local/bin/mandau-agent && \
                            sudo systemctl restart mandau-core && \
                            sudo systemctl restart mandau-agent"
```
Once you do this, the server will be running the new real-time container logic, and your local CLI will have the correct certificates to talk to it!




# Resolce old mandau binary

Ran command: `./bin/mandau container list agent-remote`
Ran command: `# 1. Rebuild specifically for Mac (Darwin)
PATH=$PATH:/opt/homebrew/bin GOOS=darwin GOARCH=arm64 make build

# 2. Install it to your system path
sudo cp bin/mandau /usr/local/bin/mandau
`
Ran command: `mandau container list agent-remote
`

It's persistent! If you're still seeing that stub message, it means your terminal is running a **different** `mandau` binary that's hiding in your system (likely at a higher priority in your `$PATH` than `/usr/local/bin`).

Let's find exactly which one is being executed. Please run this command:

```bash
which -a mandau
```

This will list **all** versions of Mandau installed on your system. 

Once we see the list, you can either:
1. Delete the "ghost" versions.
2. Or, run the new one directly using the full path:
   ```bash
   /usr/local/bin/mandau container list agent-remote
   ```

Try the full path command first to confirm we've definitely got the right logic working!




Edited pom.xml
Ran command: `# Path was bin/mandau after the last build
sudo cp bin/mandau /usr/local/bin/mandau
`
Ran command: `mandau container list agent-insanserver `
Ran command: `ls -l bin/mandau /usr/local/bin/mandau && which mandau && mandau version`
Viewed container.go:70-147
Viewed container.go:1-60

It looks like the `bin/mandau` file you copied was likely the **Linux version** (from when we cross-compiled for the server), or it was from an older build. This is why you're still seeing the "Note: This would call..." stub message.

Since you're on a Mac, we need to explicitly rebuild the binary for **macOS** so that it includes the new gRPC logic.

Please run these commands to rebuild and install the correct Darwin version:

```bash
# 1. Rebuild specifically for Mac (Darwin)
PATH=$PATH:/opt/homebrew/bin GOOS=darwin GOARCH=arm64 make build

# 2. Install it to your system path
sudo cp bin/mandau /usr/local/bin/mandau
```

After that, try the command again:
```bash
mandau container list agent-insanserver
```

This will ensure the binary you're running matches the code changes we made!