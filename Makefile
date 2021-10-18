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

## clean: Cleanup.
clean:
	@rm -f bin/$(PROJECTNAME)

## help: Show this message.
help: Makefile
	@echo "Available targets:"
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
