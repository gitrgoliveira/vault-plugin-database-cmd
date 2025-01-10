GOARCH = arm64

UNAME = $(shell uname -s)
export VAULT_ADDR=http://127.0.0.1:8200

ifndef OS
	ifeq ($(UNAME), Linux)
		OS = linux
	else ifeq ($(UNAME), Darwin)
		OS = darwin
	endif
endif

.DEFAULT_GOAL := all

all: fmt build start

build: fmt
	GOOS=$(OS) GOARCH="$(GOARCH)" go build -o vault/plugins/vault-plugin-database-cmd cmd/vault-plugin-database-cmd/main.go

start:
	vault server -dev -dev-root-token-id=root \
	-dev-plugin-dir=./vault/plugins -log-level=debug \
	-dev-listen-address=0.0.0.0:8200 &

enable:
	# vault plugin register -sha256=fad4d28b6f57ca6a1acd49b948e0a279d805280c461bb29fcb8781e57c1c3562 auth vault-plugin-auth-tfe
	vault secrets enable -path=database-cmd vault-plugin-database-cmd

clean:
	rm -f ./vault/plugins/vault-plugin-database-cmd

fmt:
	go fmt $$(go list ./...)

test: start
	vault write database-cmd/config/my-database \
    plugin_name="vault-plugin-database-cmd" \
    allowed_roles="*" \
    username="root" \
    password="rootpwd" \

	vault list database-cmd/config
	
	vault read database-cmd/config/my-database




.PHONY: build clean fmt start enable test
