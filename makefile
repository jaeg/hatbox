# Go parameters
 GOCMD=go
 GOBUILD=$(GOCMD) build
 GOCLEAN=$(GOCMD) clean
 GOTEST=$(GOCMD) test
 GOGET=$(GOCMD) get
 BINARY_NAME=chest
 BINARY_UNIX=$(BINARY_NAME)_unix

 all: test build
 build:
				 $(GOBUILD) -o ./bin/$(BINARY_NAME) -v
 build-linux:
				CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o ./bin/$(BINARY_UNIX) -v
 test:
				 $(GOTEST) -v ./...
 clean:
				 $(GOCLEAN)
				 rm -f ./bin/$(BINARY_NAME)
 run: build
	./bin/$(BINARY_NAME) --config chest1.config
image: build-linux
	docker build ./ -t jaeg/treasure-chest:latest
	docker tag jaeg/treasure-chest:latest jaeg/treasure-chest:$(shell git describe --abbrev=0 --tags)-$(shell git rev-parse --short HEAD)
publish:
	docker push jaeg/treasure-chest:latest
	docker push jaeg/treasure-chest:$(shell git describe --abbrev=0 --tags)-$(shell git rev-parse --short HEAD)
release:
	docker tag jaeg/treasure-chest:$(shell git describe --abbrev=0 --tags)-$(shell git rev-parse --short HEAD) jaeg/treasure-chest:$(shell git describe --abbrev=0 --tags)
	docker push jaeg/treasure-chest:$(shell git describe --abbrev=0 --tags)
	docker push jaeg/treasure-chest:latest