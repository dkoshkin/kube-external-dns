# Set the build version
ifeq ($(origin VERSION), undefined)
	VERSION := $(shell git describe --tags --always --dirty)
endif
# build date
ifeq ($(origin BUILD_DATE), undefined)
	BUILD_DATE := $(shell date -u)
endif
GO_VERSION = 1.8.0

build:
	GOOS=linux go build -ldflags "-X main.version=$(VERSION) -X 'main.buildDate=$(BUILD_DATE)'"
	docker build -t arduima/kube-external-dns .
	
push:
	docker push arduima/kube-external-dns
	
run:
	docker run arduima/kube-external-dns