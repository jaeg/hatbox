# treasure-chest

## Description
Treasure Chest is a scalable and self syncing CDN.  All files in the `contents` folder will be kept synchronized with other chests in the namespace.  When a new chest is introduced with no files it'll be automatically populated by the rest of the cluster.

## Runtime params
- cluster-name - name of cluster   
- chest-name - name of the chest   
- redis-address - address to redis server  
- redis-password - password for redis server   
- run-now - run registered scripts on this wart immediately
- host-port - port to host file server on
- health-port - port to host health server on

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

## Routes
GET /< filepath > 
- Returns the newest version of the requested file.  If the chest that got asked for the file either doesn't have it or is out of date it syncs the file.