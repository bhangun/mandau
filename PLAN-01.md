Next plan for better UX.

## Phase 1: Installation
- install with below command, and enhance with print statements of every step (with color):
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | sudo bash
- this should include:
$ mandau cert gen

- Check if docker is installed, if not install it.
- Check if docker compose is installed, if not install it.
- Check if mandau is installed, if not install it.
- Check if mandau agent is installed, if not install it.
- Check if mandau service core & agent are running, if not start them.
- Check if mandau agent is connected to core, if not connect it.


## Phase 2: Client side installation
- install with below command, and enhance with print statements of every step (with color):
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | bash
- client just need to do:
$ mandau connect <server_ip>:<port> (port is optional, default is 9444)
- this should include:
    > setup local config.yaml with the IP <server_ip> and port <port> by default 9444
    > ssh to <server_ip> and copy the client certs to ~/.mandau/certs/ on the client side.
    or auto
    scp <USER>@<IP_ADDRESS>:~/.mandau/certs/{ca.crt,client.crt,client.key} ~/.mandau/certs/


## Phase 3: Operational
### general
- setup default agent to use, if not connected, prompt user to connect. with command below, and save(update) to config.yaml:
$ mandau agent default <agent_id>
- Or by default used the first agent in the list. and auto save to config.yaml 
- since agent default is set, all the commands will be executed on the default agent.

### Docker operational
- Change current sub-command "container" / "mandau container" to "mandau docker" 
- wrap all docker commands with mandau commands, so user can use mandau command as prefix to docker commands. like below:
$ mandau docker ps
$ mandau docker images

- For non default agent, use below command:
$ mandau -a <agent_id> docker ps
$ mandau -a <agent_id> docker images

-  and so on for all docker commands.




