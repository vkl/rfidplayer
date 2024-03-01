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
SRC_LIBS := $(wildcard libs/go-cast/*.go) \
  $(wildcard libs/go-cast/*/*.go)

ifdef DEBUG
GOBUILD = go build -gcflags=all="-N -l"
else
GOBUILD = go build -ldflags=-w
endif

all: test build

#build: $(BINARY_DARWIN) $(BINARY_LINUX) $(BINARY_LINUX_ARM) $(BINARY_LINUX_ARMV6)
build: $(BINARY_LINUX_ARMV6)

$(BINARY_DARWIN): $(SRC) $(SRC_PKG) $(SRC_LIBS)
		GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BINARY_DARWIN) $(SRC)
$(BINARY_LINUX): $(SRC) $(SRC_PKG) $(SRC_LIBS)
		GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_LINUX) $(SRC)
$(BINARY_LINUX_ARM): $(SRC) $(SRC_PKG) $(SRC_LIBS)
		GOOS=linux GOARCH=arm $(GOBUILD) -o $(BINARY_LINUX_ARM) $(SRC)
$(BINARY_LINUX_ARMV6): $(SRC) $(SRC_PKG) $(SRC_LIBS)
		GOOS=linux GOARCH=arm GOARM=6 $(GOBUILD) -o $(BINARY_LINUX_ARMV6) $(SRC)

test:
		go test ./...

clean:
		go clean
		rm -rf ${BINARY_PATH}

foo:
		@echo $(BINARY_LINUX_ARMV6)

install: build
ifdef REMOTE_PATH
		rsync -r -av $(BINARY_LINUX_ARMV6) \
		casts.json static templates ${REMOTE_PATH}:/home/vkl/rfidplayer/
else
		$(error REMOTE_PATH is required)
endif

run: install
ifdef REMOTE_PATH
		$(eval PID := $(shell ssh ${REMOTE_PATH} 'cd /home/vkl/rfidplayer; nohup ./$(BINARY_NAME) & echo $$!'))
		read -p "$(PID): enter to stop"
		ssh ${REMOTE_PATH} 'kill $(PID)'

else
		$(error REMOTE_PATH is required)
endif

debug:
		DEBUG=1 $(MAKE) install
ifdef REMOTE_PATH
		nohup ssh ${REMOTE_PATH} 'cd /home/vkl/rfidplayer; gdbserver 192.168.1.105:9999 ./$(BINARY_NAME) & echo $$!'
		gdb ./$(BINARY_NAME) -ex "target remote 192.168.1.105:9999"
else
		$(error REMOTE_PATH is required)
endif
