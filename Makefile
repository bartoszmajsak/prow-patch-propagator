PROJECT_NAME:=prow-patcher
ORG_NAME?=bartoszmajsak
PACKAGE_NAME:=github.com/$(ORG_NAME)/$(PROJECT_NAME)

PROJECT_DIR:=$(shell pwd)
BUILD_DIR:=$(PROJECT_DIR)/build
BINARY_DIR:=$(PROJECT_DIR)/dist
BINARY_NAME:=prow-patcher

GOPATH_1:=$(shell echo ${GOPATH} | cut -d':' -f 1)
GOBIN=$(GOPATH_1)/bin
PATH:=${GOBIN}/bin:$(PROJECT_DIR)/bin:$(PATH)

# Call this function with $(call header,"Your message") to see underscored green text
define header =
@echo -e "\n\e[92m\e[4m\e[1m$(1)\e[0m\n"
endef

##@ Default target (all you need - just run "make")
.DEFAULT_GOAL:=all
.PHONY: all
all: deps tools format lint compile ## Runs 'deps format lint test compile' targets

##@ Build

.PHONY: compile
compile: $(BINARY_DIR)/$(BINARY_NAME) ## Compiles binaries

.PHONY: clean
clean: ## Lets you start from clean state
	rm -rf $(BINARY_DIR) $(PROJECT_DIR)/bin/ vendor/

.PHONY: deps
deps:  ## Fetches all dependencies
	$(call header,"Fetching dependencies")
	@go mod download
	@go mod vendor
	@go mod tidy

.PHONY: format
format: ## Removes unneeded imports and formats source code
	$(call header,"Formatting code")
	goimports -l -w -e $(SRCS)

.PHONY: lint-prepare
lint-prepare: deps

.PHONY: lint
lint: lint-prepare ## Concurrently runs a whole bunch of static analysis tools
	$(call header,"Running a whole bunch of static analysis tools")
	golangci-lint run --fix --sort-results

# ##########################################################################
# Build configuration
# ##########################################################################

OS:=$(shell uname -s)
GOOS?=$(shell echo $(OS) | awk '{print tolower($$0)}')
GOARCH:=amd64

BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GITUNTRACKEDCHANGES:=$(shell git status --porcelain --untracked-files=no)
COMMIT:=$(shell git rev-parse --short HEAD)
ifneq ($(GITUNTRACKEDCHANGES),)
	COMMIT:=$(COMMIT)-dirty
endif

BINARY_VERSION?=$(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
GIT_TAG:=$(shell git describe --tags --abbrev=0 --exact-match > /dev/null 2>&1; echo $$?)
ifneq ($(GIT_TAG),0)
	BINARY_VERSION:=$(BINARY_VERSION)-next-$(COMMIT)
else ifneq ($(GITUNTRACKEDCHANGES),)
	BINARY_VERSION:=$(BINARY_VERSION)-dirty
endif

GOBUILD:=GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0
RELEASE?=false
SRCS=$(shell find . -name "*.go" -not -path "./vendor/*")

$(BINARY_DIR):
	[ -d $@ ] || mkdir -p $@

$(BINARY_DIR)/$(BINARY_NAME): $(BINARY_DIR) $(SRCS)
	$(call header,"Compiling... carry on!")
	${GOBUILD} go build -o $@ .

##@ Container image


##@ Images

GITUNTRACKEDCHANGES:=$(shell git status --porcelain --untracked-files=no)
COMMIT:=$(shell git rev-parse --short HEAD)
ifneq ($(GITUNTRACKEDCHANGES),)
	COMMIT:=$(COMMIT)-dirty
endif

# Prefer to use podman if not explicitly set
ifneq (, $(shell which podman))
	IMG_BUILDER?=podman
else
	IMG_BUILDER?=docker
endif

CONTAINER_REGISTRY?=quay.io
CONTAINER_REPOSITORY?=bmajsak

.PHONY: container-image
container-image: container-image--prow-patcher@latest ## Builds container images
container-image--%: ## Builds the container image
	$(eval image_param=$(subst container-image--,,$@))
	$(eval image_type=$(firstword $(subst @, ,$(image_param))))
	$(eval image_tag=$(or $(word 2,$(subst @, ,$(image_param))),latest))
	$(eval image_name:=${image_type})
	$(call header,"Building container image $(image_name)")
	$(IMG_BUILDER) build \
		--label "org.opencontainers.image.title=$(image_name)" \
		--label "org.opencontainers.image.source=https://github.com/$(ORG)/$(REPO)" \
		--label "org.opencontainers.image.licenses=Apache-2.0" \
		--label "org.opencontainers.image.authors=Bartosz Majsak" \
		--label "org.opencontainers.image.vendor=Red Hat, Inc." \
		--label "org.opencontainers.image.revision=$(COMMIT)" \
		--label "org.opencontainers.image.created=$(shell date -u +%F\ %T%z)" \
		--network=host \
		-t $(CONTAINER_REGISTRY)/$(CONTAINER_REPOSITORY)/$(image_name):$(image_tag) \
		-f $(PROJECT_DIR)/Dockerfile $(BINARY_DIR)

.PHONY: container-image-push
container-image-push: container-image--prow-patcher@latest ## Pushes latest container images to the registry
container-image-push: container-push--prow-patcher@latest

container-push--%:
	$(eval image_param=$(subst container-push--,,$@))
	$(eval image_type=$(firstword $(subst @, ,$(image_param))))
	$(eval image_tag=$(or $(word 2,$(subst @, ,$(image_param))),latest))
	$(eval image_name:=${image_type})
	$(call header,"Pushing container image $(image_name)")
	$(IMG_BUILDER) push $(CONTAINER_REGISTRY)/$(CONTAINER_REPOSITORY)/$(image_name):$(image_tag)

###@ K8S Deployment

NAMESPACE?=prow
WORKER_NS?=prow-workers
CLUSTER_DIR:=$(PROJECT_DIR)/cluster

.PHONY: deploy
deploy:  ## Deploys to k8s cluster
	@./scripts/replace.sh placeholders "$(CLUSTER_DIR)/deployment.yaml" '$${NAMESPACE}' '$(NAMESPACE)' | kubectl apply -f -
	@kubectl apply -n "$(NAMESPACE)" -f "$(CLUSTER_DIR)/service.yaml"

##@ Setup

.PHONY: tools
tools: $(PROJECT_DIR)/bin/goimports  ## Installs required go tools
tools: $(PROJECT_DIR)/bin/golangci-lint
	$(call header,"Installing required tools")

$(PROJECT_DIR)/bin/goimports:
	$(call header,"    Installing goimports")
	GOBIN=$(PROJECT_DIR)/bin go install -mod=readonly golang.org/x/tools/cmd/goimports

$(PROJECT_DIR)/bin/golangci-lint:
	$(call header,"    Installing golangci-lint")
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(PROJECT_DIR)/bin v1.43.0

##@ Helpers

.PHONY: help
help:  ## Displays this help \o/
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-25s\033[0m\033[2m %s\033[0m\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@cat $(MAKEFILE_LIST) | grep "^[A-Za-z_]*.?=" | sort | awk 'BEGIN {FS="?="; printf "\n\n\033[1mEnvironment variables\033[0m\n"} {printf "  \033[36m%-25s\033[0m\033[2m %s\033[0m\n", $$1, $$2}'
