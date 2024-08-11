.PHONY: install build assets release guest v86 kernel

VERSION=0.1dev

all: assets build


build:
	go build -ldflags="-X 'main.Version=${VERSION}'" -o ./env86 ./cmd/env86

install: build
	mv ./env86 /usr/local/bin/env86

release:
	VERSION=$(VERSION) goreleaser release --snapshot --clean


assets: guest kernel v86

guest:
	cd ./cmd/guest86 && GOOS=linux GOARCH=386 go build -o ../../assets/guest86 .

kernel:
	docker build --platform=linux/386 -t env86-kernel -f ./scripts/Dockerfile.kernel ./scripts
	docker run --rm --platform=linux/386 -v ./assets:/dst env86-kernel

v86:
	docker build -t env86-v86 -f ./scripts/Dockerfile.v86 ./scripts
	docker run --rm -v ./assets:/dst env86-v86

