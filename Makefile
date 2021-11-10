PROJECTNAME := pdns-api
VERSION := $(shell cat VERSION)
BUILD := $(shell git rev-parse --short HEAD)
SHELL := /bin/bash
GITHUB := github.com/mixanemca

# Make is verbose in Linux. Make it silent.
MAKEFLAGS += --silent

# Use linker flags to provide version/build settings
LDFLAGS=-ldflags "-s -w -X=main.version=$(VERSION) -X=main.build=$(BUILD)"

.PHONY: build run test clean help

all: build

## build: Compile the binary.
build: clean
	@mkdir -p bin
	@go build $(LDFLAGS) -o bin/$(PROJECTNAME) cmd/$(PROJECTNAME)/main.go

## run: Run the go run command.
run: test
	@go run cmd/$(PROJECTNAME)/main.go

## test: Run the go test command.
test:
	@go test -v ./...

## clean: Cleanup binary.
clean: clean-docker clean-deb
	-@rm -f bin/$(PROJECTNAME)

## clean-deb: Cleanup deb package.
clean-deb:
	-@rm -f bin/*.{buildinfo,changes,deb}

## clean-docker: Cleanup Docker container and image.
clean-docker:
	-@docker rm -f $(PROJECTNAME)-package &>/dev/null
	-@docker rmi $(PROJECTNAME)-builder:$(VERSION) &>/dev/null

## deb: Build deb package in Docker
deb: clean-deb clean-docker
	@docker build -t $(PROJECTNAME)-builder:$(VERSION) -f Dockerfile .
	@docker run --rm -ti -v $(PWD)/bin:/tmp/packages --name $(PROJECTNAME)-package $(PROJECTNAME)-builder:$(VERSION) /bin/cp /usr/src/$(PROJECTNAME)_$(VERSION)_amd64.deb /tmp/packages/
	@docker run --rm -ti -v $(PWD)/bin:/tmp/packages --name $(PROJECTNAME)-package $(PROJECTNAME)-builder:$(VERSION) /bin/cp /usr/src/$(PROJECTNAME)_$(VERSION)_amd64.changes /tmp/packages/
	@docker run --rm -ti -v $(PWD)/bin:/tmp/packages --name $(PROJECTNAME)-package $(PROJECTNAME)-builder:$(VERSION) /bin/cp /usr/src/$(PROJECTNAME)_$(VERSION)_amd64.buildinfo /tmp/packages/

## help: Show this message.
help: Makefile
	@echo "Available targets:"
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
