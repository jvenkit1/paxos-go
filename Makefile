# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod download

dep:
	$(GOMOD)

build:
	$(GOBUILD) -o bin/paxos ./cmd/paxos

test:
	$(GOTEST) -timeout 20s -v ./...
