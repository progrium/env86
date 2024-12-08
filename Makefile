.PHONY: install build assets release guest v86 kernel

VERSION=0.2dev

ifeq ($(OS),Windows_NT)
	ASSETS_DIR := .\assets
else
	ASSETS_DIR := ./assets
endif

all: assets build


build:
	go build -ldflags="-X 'main.Version=${VERSION}'" -o ./env86 ./cmd/env86

install: build
	mv ./env86 /usr/local/bin/env86

release:
	VERSION=$(VERSION) goreleaser release --snapshot --clean


assets: guest kernel v86

guest: export GOOS=linux
guest: export GOARCH=386
guest:
	cd ./cmd/guest86 && go build -o ../../assets/guest86 .

kernel:
	docker build --platform=linux/386 -t env86-kernel -f ./scripts/Dockerfile.kernel ./scripts
	docker run --rm --platform=linux/386 -v $(ASSETS_DIR):/dst env86-kernel

v86:
	docker build -t env86-v86 -f ./scripts/Dockerfile.v86 ./scripts
	docker run --rm -v $(ASSETS_DIR):/dst env86-v86

