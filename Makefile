# GO_VERSION refers to the version of Golang to be downloaded when running dockerized version
GO_VERSION = 1.20

# IMAGE_REGISTRY used to indicate the registery/group for the operator, bundle and catalog
IMAGE_REGISTRY = quay.io/medik8s

# When no version is set, use latest as image tags
DEFAULT_VERSION := 0.0.1

# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= $(DEFAULT_VERSION)

# Run go in a container
# --rm                                                          = remove container when stopped
# -v $$(pwd):/home/go/src/github.com/medik8s/node-maintenance-operator = bind mount current dir in container
# -u $$(id -u)                                                  = use current user (else new / modified files will be owned by root)
# -w /home/go/src/github.com/medik8s/node-maintenance-operator         = working dir
# -e ...                                                        = some env vars, especially set cache to a user writable dir
# --entrypoint /bin bash ... -c                                 = run bash -c on start; that means the actual command(s) need be wrapped in double quotes, see e.g. check target which will run: bash -c "make test"
export DOCKER_GO=docker run --rm -v $$(pwd):/home/go/src/github.com/medik8s/common \
	-u $$(id -u) -w /home/go/src/github.com/medik8s/common \
	-e "GOPATH=/go" -e "GOFLAGS=-mod=vendor" -e "XDG_CACHE_HOME=/tmp/.cache" \
	-e "VERSION=$(VERSION)" -e "IMAGE_REGISTRY=$(IMAGE_REGISTRY)" \
	--entrypoint /bin/bash golang:$(GO_VERSION) -c


.PHONY: test
test: ## Run tests.
	go test ./... -coverprofile cover.out -v

.PHONY: check
check: ## Dockerized version of make test
	$(DOCKER_GO) "make test"