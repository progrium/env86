.PHONY: install build assets

all: assets build

build:
	go build -o ./env86 ./cmd/env86

install:
	go build -o /usr/local/bin/env86 ./cmd/env86

assets:
	docker build --platform=linux/386 -t env86-kernel -f ./scripts/Dockerfile.kernel ./scripts
	docker run --rm --platform=linux/386 -v ./assets:/dst env86-kernel
	docker build -t env86-v86 -f ./scripts/Dockerfile.v86 ./scripts
	docker run --rm -v ./assets:/dst env86-v86