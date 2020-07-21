# hatbox

## Description
Hatbox is a scalable and self syncing CDN.  All files in the `contents` folder will be kept synchronized with other hatboxs in the namespace.  When a new hatbox is introduced with no files it'll be automatically populated by the rest of the cluster.

## Runtime params
- cluster-name - name of cluster   
- hatbox-name - name of the hatbox   
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
  -  `./bin/hatbox --config hatbox.config`
- Or you can pass in through runtime params  
  - `./bin/hatbox --redis-address=<address> --redis-password=<password> --hatbox-name=hatbox`
- Or run through a docker container
  - `docker run jaeg/hatbox:latest --redis-address=<address> --redis-password=<password> --hatbox-name=hatbox`

## Routes
GET /< filepath > 
- Returns the newest version of the requested file.  If the hatbox that got asked for the file either doesn't have it or is out of date it syncs the file.