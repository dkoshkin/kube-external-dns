# Set the build version
ifeq ($(VERSION),)
	VERSION := $(shell git describe --tags --always --dirty)
endif
# build date
ifeq ($(BUILD_DATE),)
	BUILD_DATE := $(shell date -u)
endif
GO_VERSION = 1.8.0

.PHONY: build build-local push release release-latest run build-builder

vendor:
	#docker run --rm -it -v $(shell pwd):/go/src/github.com/dkoshkin/kube-external-dns -w /go/src/github.com/dkoshkin/kube-external-dns arduima/golang-glide:$(GO_VERSION) glide install
	go get -d -v

build: vendor
	#docker run --rm -v $(shell pwd):/go/src/github.com/dkoshkin/kube-external-dns -w /go/src/github.com/dkoshkin/kube-external-dns -e GOOS=linux arduima/golang-glide:$(GO_VERSION) go build -ldflags "-X main.version=$(VERSION) -X 'main.buildDate=$(BUILD_DATE)'"
	GOOS=linux go build -ldflags "-X main.version=$(VERSION) -X 'main.buildDate=$(BUILD_DATE)'"
	docker build -t arduima/kube-external-dns:latest .

build-local:
	go get -d -v
	go build -ldflags "-X main.version=$(VERSION) -X 'main.buildDate=$(BUILD_DATE)'"
	docker build -t arduima/kube-external-dns:latest .

release: build
	docker tag arduima/kube-external-dns arduima/kube-external-dns:$(VERSION)
	docker push arduima/kube-external-dns:latest
	docker push arduima/kube-external-dns:$(VERSION)

release-latest: build
	docker push arduima/kube-external-dns:latest

run:
	docker run arduima/kube-external-dns

build-builder:
	docker build -t arduima/golang-glide:$(GO_VERSION) -f Dockerfile.build .

release-builder: build-builder
	docker push arduima/golang-glide:$(GO_VERSION)

test: vendor
	#docker run --rm -it -v $(PWD):/go/src/github.com/dkoshkin/kube-external-dns -w /go/src/github.com/dkoshkin/kube-external-dns arduima/golang-glide:$(GO_VERSION) go test -v
	go test -v

default: build
