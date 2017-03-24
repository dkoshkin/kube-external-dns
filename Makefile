# Set the build version
ifeq ($(origin VERSION), undefined)
	VERSION := $(shell git describe --tags --always --dirty)
endif
# build date
ifeq ($(origin BUILD_DATE), undefined)
	BUILD_DATE := $(shell date -u)
endif
GO_VERSION = 1.8.0

.PHONY: build push release run build-builder

vendor:
	docker run --rm -it -v $(PWD):/go/src/github.com/dkoshkin/kube-external-dns -w /go/src/github.com/dkoshkin/kube-external-dns arduima/golang-glide:$(GO_VERSION) /bin/bash -c "glide install"

build: vendor
	docker run --rm -it -v $(PWD):/go/src/github.com/dkoshkin/kube-external-dns -w /go/src/github.com/dkoshkin/kube-external-dns arduima/golang-glide:$(GO_VERSION) /bin/bash -c "GOOS=linux go build -ldflags \"-X main.version=$(VERSION) -X 'main.buildDate=$(BUILD_DATE)'\""
	docker build -t arduima/kube-external-dns .
	docker tag arduima/kube-external-dns arduima/kube-external-dns:$(VERSION)

push:
	docker push arduima/kube-external-dns:$(VERSION)
	docker push arduima/kube-external-dns
	
release: build
	make push -e VERSION=$(VERSION)

run:
	docker run arduima/kube-external-dns

build-builder: 
	docker build -t arduima/golang-glide:$(GO_VERSION) -f Dockerfile.build .

release-builder: build-builder
	docker push arduima/golang-glide:$(GO_VERSION)

test: vendor
	docker run --rm -it -v $(PWD):/go/src/github.com/dkoshkin/kube-external-dns -w /go/src/github.com/dkoshkin/kube-external-dns arduima/golang-glide:$(GO_VERSION) /bin/bash -c "go test -v"

default: build


