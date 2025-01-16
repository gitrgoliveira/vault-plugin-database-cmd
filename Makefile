UNAME = $(shell uname -s)
PLUGIN_NAME = vault-plugin-database-cmd
DOCKER_IMAGE = $(PLUGIN_NAME)
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

build: fmt cmd/$(PLUGIN_NAME)/main.go $(wildcard *.go)
	GOOS=$(OS) CGO_ENABLED=0 go build -o vault/plugins/$(PLUGIN_NAME) cmd/$(PLUGIN_NAME)/main.go

start:
	@if lsof -Pi :8200 -sTCP:LISTEN -t >/dev/null ; then \
        echo "Vault server is already running"; exit 0;\
    else \
	     vault server -dev -dev-root-token-id=root -log-level=trace \
			-dev-listen-address=0.0.0.0:8200 > ./vault/debug.log 2>&1 & \
    fi

clean:
	rm -f ./vault/plugins/$(PLUGIN_NAME)
	docker image rm -f $(DOCKER_IMAGE)

fmt:
	go fmt $$(go list ./...)

build-container: build
	tar -czh . | docker build -t $(DOCKER_IMAGE):$(DOCKER_IMAGE_TAG) -

SHA256:=$$(docker images --no-trunc --format="{{ .ID }}" $(DOCKER_IMAGE) | cut -d: -f2 | head -n 1)

register-plugin: start
	vault plugin runtime register -type=container -rootless=true -oci_runtime=runsc runsc

	vault plugin register \
		-sha256=$(SHA256) \
		-oci_image=$(DOCKER_IMAGE) \
		-runtime=runsc \
		-version=$(DOCKER_IMAGE_TAG) \
		database $(PLUGIN_NAME)

	vault plugin reload -type=database -plugin $(PLUGIN_NAME)

test: register-plugin
	-vault secrets enable -path=database-cmd database

	vault write database-cmd/config/my-database \
		plugin_name="$(PLUGIN_NAME)" \
		plugin_version="$(DOCKER_IMAGE_TAG)" \
		allowed_roles="*" \
		username="mandatory" \
		password="mandatory" \
		root_rotation_statements="echo 'Root rotation statements'" \

	vault write -force database-cmd/reload/vault-plugin-database-cmd
	
	vault list database-cmd/config
	
	vault read database-cmd/config/my-database

	vault write -force database-cmd/rotate-root/my-database

	# repeating parameters adds more lines to the script.
	# This is useful for testing the plugin's ability to handle multiple statements
	vault write database-cmd/roles/dynamic-role \
		db_name=my-database \
		creation_statements="echo 'Dynamic creation statements'" \
		creation_statements="ping -c3 www.google.com" \
		revocation_statements="echo 'Dynamic revocation statements'" \
		rollback_statements="echo 'Dynamic rollback statements'" \
		renew_statements="echo 'Dynamic renew statements'" \
		rotation_period="15s" \
		default_ttl="30s" \
		max_ttl="1m"

	vault write database-cmd/static-roles/static-role \
		db_name=my-database \
		credential_type="password" \
		username="static-username" \
		rotation_window="1h" \
		self_managed_password="true" \
		rotation_schedule="0 * * * SAT" \
		rotation_statements="echo 'Rotate static'"

	vault read database-cmd/creds/dynamic-role
	vault read database-cmd/static-creds/static-role
	vault read database-cmd/static-creds/static-role


stop: 
	killall vault

release: build-container
	@echo "Release"
	docker image save $(DOCKER_IMAGE):$(DOCKER_IMAGE_TAG) | gzip > $(DOCKER_IMAGE)_$(DOCKER_IMAGE_TAG)_$(shell date +%Y%m%d).tar.gz

.PHONY: all build clean fmt build-container register-plugin test stop release