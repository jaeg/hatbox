# treasure-chest

## Runtime params
- cluster-name - name of cluster   
- chest-name - name of the chest   
- redis-address - address to redis server  
- redis-password - password for redis server   
- run-now - run registered scripts on this wart immediately

## Getting dependencies
Requires a version of go that supports go.mod
- go get

## Get up and running
- Build it
  - `make build`
- Build and run it
  - `make run`
- You can get started using an example config as such
  -  `./bin/chest --config chest.config`
- Or you can pass in through runtime params  
  - `./bin/chest --redis-address=<address> --redis-password=<password> --chest-name=chest`
- Or run through a docker container
  - `docker run jaeg/treasure-chest:latest --redis-address=<address> --redis-password=<password> --chest-name=chest`
