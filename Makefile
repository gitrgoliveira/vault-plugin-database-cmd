UNAME = $(shell uname -s)
PLUGIN_NAME = vault-plugin-database-cmd
DOCKER_IMAGE = ${PLUGIN_NAME}
DOCKER_IMAGE_TAG = 1.0.0

# To allow the commands to target the local Vault server
export VAULT_ADDR=http://127.0.0.1:8200
export VAULT_TOKEN=root

# This is necessary for Vault to know where to find the Docker socket when runnign in rootless mode
export DOCKER_HOST=unix:///run/user/1000/docker.sock

ifndef OS
	ifeq ($(UNAME), Linux)
		OS = linux
	else ifeq ($(UNAME), Darwin)
		OS = darwin
	endif
endif

.DEFAULT_GOAL := all

all: fmt build start

build: fmt cmd/${PLUGIN_NAME}/main.go $(wildcard *.go)
	GOOS=$(OS) CGO_ENABLED=0 go build -o vault/plugins/${PLUGIN_NAME} cmd/${PLUGIN_NAME}/main.go

start:
	@if lsof -Pi :8200 -sTCP:LISTEN -t >/dev/null ; then \
        echo "Vault server is already running"; \
    else \
	     vault server -dev -dev-root-token-id=root -log-level=trace \
			-dev-listen-address=0.0.0.0:8200 > ./vault/debug.log 2>&1 & \
    fi

clean:
	rm -f ./vault/plugins/${PLUGIN_NAME}
	docker image rm -f $(DOCKER_IMAGE)

fmt:
	go fmt $$(go list ./...)



build-container: build
	tar -czh . | docker build -t $(DOCKER_IMAGE):$(DOCKER_IMAGE_TAG) -

SHA256:=$$(docker images --no-trunc --format="{{ .ID }}" ${DOCKER_IMAGE} | cut -d: -f2 | head -n 1)


register-plugin: start
	vault plugin runtime register -type=container -rootless=true -oci_runtime=runsc runsc

	vault plugin register \
		-sha256=$(SHA256) \
		-oci_image=${DOCKER_IMAGE} \
		-runtime=runsc \
		-version=$(DOCKER_IMAGE_TAG) \
		database ${PLUGIN_NAME}

test: register-plugin
	vault secrets enable -path=database-cmd database

	vault write database-cmd/config/database-cmd \
		plugin_name="${PLUGIN_NAME}" \
		allowed_roles="*" 

	vault list database-cmd/config
	
	vault read database-cmd/config/database-cmd

stop: 
	killall vault

release: build-container
	@echo "Release"
	docker image save $(DOCKER_IMAGE) | gzip > $(DOCKER_IMAGE)_$(DOCKER_IMAGE_TAG)_$(shell date +%Y%m%d).tar.gz

.PHONY: build clean fmt start enable test
