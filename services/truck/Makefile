.PHONY: all build clean test lint docker-build docker-push run dev postgres createdb migratedb redis

# Application name
APP_NAME=operations-service
VERSION ?= $(shell git describe --tags --always --dirty)

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOCOVER=$(GOCMD) tool cover
GOLINT=golangci-lint
GOMOD=$(GOCMD) mod

# Database parameters
DB_NAME=operations_db
DB_URI=postgresql://root:password@localhost:5432/$(DB_NAME)?sslmode=disable

# Build parameters
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"

all: clean lint test build

build:
	$(GOBUILD) $(LDFLAGS) -o ./bin/$(APP_NAME) -v

clean:
	rm -f ./bin/$(APP_NAME)
	rm -rf ./coverage
	find . -type f -name '*.test' -delete

test:
	mkdir -p ./coverage
	$(GOTEST) -v -race -coverprofile=./coverage/coverage.out ./...
	$(GOCOVER) -html=./coverage/coverage.out -o ./coverage/coverage.html

lint:
	$(GOLINT) run ./...

docker-build:
	docker build -t $(APP_NAME):$(VERSION) .

docker-push:
	docker push $(APP_NAME):$(VERSION)

run:
	./bin/$(APP_NAME) serve

dev:
	go run . serve --debug

postgres:
	docker run --name datapoint_db -p 5432:5432 -e POSTGRES_USER=root -e POSTGRES_PASSWORD=password -d postgres:13-alpine

createdb:
	docker exec -it datapoint_db createdb --username=root --owner=root $(DB_NAME)

migratedb:
	go run . migrate

redis:
	docker run --name operations_redis -p 6379:6379 -d redis:alpine

# Set up development environment
setup: postgres createdb redis migratedb

# Tear down development environment
teardown:
	docker stop datapoint_db operations_redis || true
	docker rm datapoint_db operations_redis || true