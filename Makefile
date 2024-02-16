BINARY_NAME := rfidplayer
BINARY_PATH := bin/arch
BINARY_PATH_DARWIN := $(BINARY_PATH)/darwin
BINARY_PATH_LINUX := $(BINARY_PATH)/linux
BINARY_PATH_LINUX_ARM := $(BINARY_PATH)/linux_arm
BINARY_PATH_LINUX_ARMV6 := $(BINARY_PATH)/linux_armv6
BINARY_DARWIN := $(BINARY_PATH_DARWIN)/$(BINARY_NAME)
BINARY_LINUX := $(BINARY_PATH_LINUX)/$(BINARY_NAME)
BINARY_LINUX_ARM := $(BINARY_PATH_LINUX_ARM)/$(BINARY_NAME)
BINARY_LINUX_ARMV6 := $(BINARY_PATH_LINUX_ARMV6)/$(BINARY_NAME)

OS := $(shell go env GOOS)
ARCH := $(shell go env GOARCH)
GOPATH := $(shell go env GOPATH)
PATH := $(GOPATH)/bin:${PATH}

SRC := $(wildcard cmd/*.go)
SRC_PKG := $(wildcard pkg/*/*.go)


all: test build

build: $(BINARY_DARWIN) $(BINARY_LINUX) $(BINARY_LINUX_ARM) $(BINARY_LINUX_ARMV6)

$(BINARY_DARWIN): $(SRC) $(SRC_PKG)
		GOOS=darwin GOARCH=amd64 go build -ldflags=-w -o $(BINARY_DARWIN) $(SRC)
$(BINARY_LINUX): $(SRC) $(SRC_PKG)
		GOOS=linux GOARCH=amd64 go build -ldflags=-w -o $(BINARY_LINUX) $(SRC)
$(BINARY_LINUX_ARM): $(SRC) $(SRC_PKG)
		GOOS=linux GOARCH=arm go build -ldflags=-w -o $(BINARY_LINUX_ARM) $(SRC)
$(BINARY_LINUX_ARMV6): $(SRC) $(SRC_PKG)
		GOOS=linux GOARCH=arm GOARM=6 go build -ldflags=-w -o $(BINARY_LINUX_ARMV6) $(SRC)

test:
		go test ./...

clean:
		go clean
		rm -rf ${BINARY_PATH}

install: build
		rsync -r -av . vkladov@192.168.1.105:/home/vkladov/rfidplayer/

run:
		go run cmd/main.go
